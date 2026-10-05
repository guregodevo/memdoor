package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"memdoor/gateway/broadcast"
	"strings"
	"time"

	"memdoor/gateway/channels/adapters"
	"memdoor/gateway/config"
	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/gateway/subagents"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// Background job execution handlers
// Pattern: OpenClaw src/process/command-queue.ts

// executeAgentJob executes an agent job from the queue
// Pattern: OpenClaw src/process/command-queue.ts
// This is called by the QueueManager when a job is ready to execute
func (s *Server) executeAgentJob(ctx context.Context, job *queue.AgentJob) error {
	queueLog := logs.New("Queue")
	sessionLog := queueLog.WithSession(job.SessionKey)

	// Make this turn cancellable so a client "cancel" message (Esc) can interrupt
	// it mid-generation. The cancel propagates through ctx → ProcessMessage →
	// the LLM call → the engine's per-token check.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.registerRunCancel(job.SessionKey, cancel)
	defer s.clearRunCancel(job.SessionKey)

	sessionLog.Debug("Executing job",
		slog.String("lane", string(job.GlobalLane)))

	// Extract message from job
	message, ok := job.Message.(string)
	if !ok {
		return fmt.Errorf("job message is not a string")
	}

	// Get session
	session, err := s.sessions.GetOrCreateSession(job.SessionKey, "main")
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Extract parent_message_id from job context (for subagent announcement threading)
	// This ensures the agent's response is posted as a thread reply to the original user message
	if job.Context != nil {
		sessionLog.Debug("Job context exists, checking for parent_message_id")
		if parentMsgID, ok := job.Context.Value(ctxParentMessageID).(int64); ok && parentMsgID > 0 {
			session.SetMetadata("parent_message_id", parentMsgID)
			sessionLog.Info("✓ Set parent_message_id from job context for threading",
				slog.Int64("parent_message_id", parentMsgID))
		} else {
			sessionLog.Debug("No parent_message_id found in job context")
		}
	} else {
		sessionLog.Debug("Job context is nil")
	}

	if job.Workdir != "" {
		ctx = context.WithValue(ctx, sharedctx.WorkdirKey, job.Workdir)
	}
	if job.AgentID != "" {
		ctx = context.WithValue(ctx, sharedctx.AgentIDKey, job.AgentID)
	}
	// Transfer buddy context values from job.Context to execution context
	// These were set in handleAgentMention and need to flow to ProcessMessage
	if job.Context != nil {
		if buddyTools := job.Context.Value(sharedctx.BuddyToolsKey); buddyTools != nil {
			ctx = context.WithValue(ctx, sharedctx.BuddyToolsKey, buddyTools)
		}
		if isLearning, ok := job.Context.Value(sharedctx.BuddyLearningKey).(bool); ok && isLearning {
			ctx = context.WithValue(ctx, sharedctx.BuddyLearningKey, true)
		}
		if isBuddy, ok := job.Context.Value(sharedctx.IsBuddyChatKey).(bool); ok && isBuddy {
			ctx = context.WithValue(ctx, sharedctx.IsBuddyChatKey, true)
		}
		// Who asked: the authenticated actor of the request that posted the
		// message rides on the job, so a metered remote turn is attributed
		// to a user (the flat plan's cost line).
		if actor := job.Context.Value(sharedctx.ActorIDKey); actor != nil {
			ctx = context.WithValue(ctx, sharedctx.ActorIDKey, actor)
		}
		// The working directory follows the work: a spawned subagent carries its
		// requester's directory on the job, and an announce-back carries the
		// subagent's on its context, so a planner → coder → planner loop stays
		// in the planner's directory throughout (2026-09-02).
		if wd, ok := job.Context.Value(sharedctx.WorkdirKey).(string); ok && wd != "" {
			ctx = context.WithValue(ctx, sharedctx.WorkdirKey, wd)
		}
		if temp, ok := job.Context.Value(sharedctx.BuddyTemperatureKey).(float64); ok && temp > 0 {
			ctx = context.WithValue(ctx, sharedctx.BuddyTemperatureKey, temp)
		}
		if sandboxCtx := job.Context.Value(sharedctx.SandboxContextKey); sandboxCtx != nil {
			ctx = context.WithValue(ctx, sharedctx.SandboxContextKey, sandboxCtx)
		}
		// Claude-style permission mode (plan/acceptEdits/default) set by the WS chat
		// handler — must flow to executeTool or plan mode's read-only gate is a no-op.
		if pm, ok := job.Context.Value(sharedctx.PermissionModeKey).(string); ok && pm != "" {
			ctx = context.WithValue(ctx, sharedctx.PermissionModeKey, pm)
		}
		// The running agent's name (set by the message service). Without propagating
		// it, sessions_spawn falls back to the runtime's static config id and records
		// the wrong requester — so the announce-back returns to the wrong agent.
		if name, ok := job.Context.Value("buddy_agent_name").(string); ok && name != "" {
			ctx = context.WithValue(ctx, "buddy_agent_name", name)
		}
	}

	// Deliver the target agent's config to a spawned subagent. EnqueueSubagentJob
	// carries only the agent id + task on an EMPTY context, so without this the
	// spawned agent runs as a generic default agent — no palette, no prompt (e.g. a
	// coder that can't write). Load its buddy config so the job actually runs AS that
	// agent. This is config delivery, not behavioral branching.
	extraSystemPrompt := job.ExtraSystemPrompt
	if job.AgentID != "" {
		// The agent's config (palette + prompt) was resolved AT SPAWN TIME and is
		// carried on the job — the agent was built with it. Use it directly: no DB
		// fetch at execution, so no single-connection contention race. FAIL FAST if a
		// job with an agent id arrives WITHOUT its palette — that means it wasn't
		// built properly, and running it on the default profile is a silent bug (the
		// coder-with-no-palette we chased). An unbuilt agent should error, not degrade.
		if len(job.BuddyTools) == 0 {
			return fmt.Errorf("agent %q arrived without its tool palette — config was not resolved at spawn", job.AgentID)
		}
		ctx = context.WithValue(ctx, sharedctx.BuddyToolsKey, append([]string(nil), job.BuddyTools...))
		ctx = context.WithValue(ctx, sharedctx.IsBuddyChatKey, true)
		// Identify the running agent so its own spawns record the right requester
		// (keeps the planner→coder→announce-back→planner loop pointing home).
		ctx = context.WithValue(ctx, "buddy_agent_name", job.AgentID)
	}

	// Subagent jobs are enqueued with an EMPTY context (queue/lanes.go
	// EnqueueSubagentJob), so a spawned agent would run WITHOUT its buddy palette
	// — meaning a coder spawned by the planner never gets the codebase-agent
	// treatment (force-first-tool-call, workdir confinement, doer window) that
	// keys on BuddyToolsKey, and so narrates instead of writing. When the key is
	// absent, load it from the job's target agent (the "agent:<id>:<label>"
	// session key) so a subagent run matches a direct run. Mirrors the WS path.
	if ctx.Value(sharedctx.BuddyToolsKey) == nil {
		if agentID := agentIDFromSessionKey(job.SessionKey); agentID != "" && s.repoFactory != nil {
			if buddy, err := s.repoFactory.Buddies().GetByName(context.Background(), agentID); err == nil && buddy != nil {
				if len(buddy.Tools) > 0 {
					ctx = context.WithValue(ctx, sharedctx.BuddyToolsKey, append([]string(nil), buddy.Tools...))
				}
				ctx = context.WithValue(ctx, sharedctx.IsBuddyChatKey, true)
			}
		}
	}

	// Execute agent run. The coder verifies its OWN work: it runs the project's
	// build/test with the bash tool, reads the real error, and fixes it — which is
	// language-agnostic (the model runs whatever the toolchain is) and keeps the
	// harness thin. The language-neutral forced-retry in the agent tool loop is what
	// keeps a small model ACTING on a failure instead of narrating; there is no
	// behind-the-scenes Go-specific verify/repair pass here anymore.
	runID := generateID("run")
	if job.Timeout > 0 {
		// A spawned sub-session runs under its requester's limit: the turn is
		// cancelled when it expires (the inference request and any waiting
		// tool are context-bound), and the requester hears about it below.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.Timeout)
		defer cancel()
	}
	response, err := s.agent.ProcessMessage(ctx, message, session, runID, extraSystemPrompt)
	if err != nil {
		sessionLog.WithError(err).Warn("Agent run failed")

		// Send error via ResponseWriter if available
		if job.ResponseWriter != nil {
			job.ResponseWriter(err)
		}
		// A failed sub-session used to end here, silently: the requester
		// waited for an announcement that never came. Record the outcome and
		// announce it like a completion, so the requester can act on it.
		if s.subagentRegistry != nil && isSubagentSession(job.SessionKey) {
			status, text := subagentFailure(err, job.Timeout)
			s.subagentRegistry.MarkFailed(job.SessionKey, status, text)
			if aerr := s.handleSubagentCompletion(context.Background(), job.SessionKey, &AgentResponse{Error: text}); aerr != nil {
				logs.New("Subagents").WithError(aerr).Warn("Failed to announce the failure",
					slog.String("session", job.SessionKey))
			}
		}
		return err
	}

	// Post-turn KV snapshot, once per boot: on a streamed-MoE model the boot
	// warmup can lose its deadline race by minutes, in which case no snapshot
	// ever saves and EVERY boot re-pays the full cold prefill (observed
	// repeatedly: warmup dead at 90m, cancel-path prefix resumed by the first
	// real turn, then lost at the next restart). After the first turn that
	// COMPLETES, the main session holds the true full prefix — capture it.
	// ~1-2s of disk write, once; warmup-saved boots skip it via the same flag.

	// Send full AgentResponse back to client if ResponseWriter is available
	// This allows synchronous callers (like chat_server) to get the complete response
	if job.ResponseWriter != nil {
		if err := job.ResponseWriter(response); err != nil {
			sessionLog.WithError(err).Warn("Failed to send response")
		}
	}

	// For channel-based sessions, save agent response as a message (with threading support)
	// Pattern: Use messageService to post agent responses to channels
	if s.messageService != nil {
		// Extract channel ID from session key (format: "workspace:X:channel:Y")
		channelID := extractChannelIDFromSessionKey(job.SessionKey)
		if channelID != "" {
			// Get parent_message_id from session metadata (set earlier from job context)
			var parentMessageID int64
			if parentMsgID, exists := session.GetMetadataValue("parent_message_id"); exists {
				if msgID, ok := parentMsgID.(int64); ok {
					parentMessageID = msgID
				}
			}

			// Attribute the message to the agent that ACTUALLY ran, not a
			// hardcoded name. A channel session key ("workspace:X:channel:Y")
			// doesn't encode the agent, so resolve it from what the run already
			// carries. This used to hardcode "writer" — an agent that need not
			// exist — so a chief/coder answer was posted under the wrong author.
			agentID := resolvePostingAgentID(ctx, job, session)
			if agentID == "" {
				// A turn ran but we can't say as whom. Don't invent an author:
				// the response already went to any synchronous caller via
				// ResponseWriter above; surface the gap instead of mislabeling.
				sessionLog.Warn("Skipping channel post: could not resolve the running agent's id",
					slog.String("session_key", job.SessionKey),
					slog.String("channel_id", channelID))
				return nil
			}

			// Create ActorID for the agent
			actorID := shared.NewAgentActorID(agentID)

			// Check if this is a DM channel - DM channels should not use threading
			// Simpler approach: check if channel ID matches DM pattern (dm-*)
			// DM channels are named like "dm-agent:writer-human:uuid" or "dm-human:uuid1-human:uuid2"
			isDMChannel := strings.HasPrefix(channelID, "dm-")

			// Post agent response as a message
			// - In DM channels: always post as regular message (no threading)
			// - In regular channels: use threading if parent_message_id exists
			if isDMChannel {
				// DM channel: post as regular message (no parent_id)
				if err := s.messageService.PostSystemAnnouncement(ctx, channelID, actorID, response.Text, 0); err != nil {
					sessionLog.WithError(err).Warn("Failed to post agent response in DM",
						slog.String("channel_id", channelID))
				} else {
					sessionLog.Info("✅ Posted agent response in DM channel",
						slog.String("channel_id", channelID),
						slog.String("agent_id", agentID))
				}
			} else if parentMessageID > 0 {
				// Regular channel with threading: post as threaded reply
				if err := s.messageService.PostSystemAnnouncement(ctx, channelID, actorID, response.Text, parentMessageID); err != nil {
					sessionLog.WithError(err).Warn("Failed to post agent response as threaded message",
						slog.String("channel_id", channelID),
						slog.Int64("parent_message_id", parentMessageID))
				} else {
					sessionLog.Info("✅ Posted agent response as threaded message",
						slog.String("channel_id", channelID),
						slog.String("agent_id", agentID),
						slog.Int64("parent_message_id", parentMessageID))
				}
			}
		}
	}

	// Handle subagent completion and announcement
	// Pattern: OpenClaw announce-back mechanism
	if s.subagentRegistry != nil && isSubagentSession(job.SessionKey) {
		// The announcement is a turn on the REQUESTER, queued to run after
		// this function returns — and this ctx is cancelled on return (the
		// sub-session's own limit). Keep the values, drop the cancellation,
		// or the requester's turn dies with "context canceled".
		if err := s.handleSubagentCompletion(context.WithoutCancel(ctx), job.SessionKey, response); err != nil {
			subLog := logs.New("Subagents")
			subLog.WithError(err).Warn("Failed to handle completion",
				slog.String("session", job.SessionKey))
			// Don't fail the job if announcement fails
		}
	}

	sessionLog.Debug("Job completed")

	return nil
}

// executeCronJob executes a cron job
// Pattern: OpenClaw runIsolatedAgentJob for cron execution
// Routes through queue to prevent race conditions on same session
func (s *Server) executeCronJob(ctx context.Context, job *config.CronJob, agentID string, sessionKey string) error {
	log := logs.New("Cron")

	// A SCHEDULED WORKFLOW (Greg, 2026-10-04: "everyone can run schedule with
	// local cron"): a job whose message is "/workflow run <name> [partition]"
	// starts that workflow in the job's project, the same words a person
	// types in the window.
	if name, partition, ok := scheduledWorkflow(job.Message); ok {
		if job.Workdir == "" {
			return fmt.Errorf("cron job %s runs workflow %s but names no project directory", job.ID, name)
		}
		run, err := s.startWorkflow(job.Workdir, name, "", shared.ParseSessionID(sessionKey).WorkspaceID, string(authorization.GetActorID(ctx)), partition, 0)
		if err != nil {
			log.Warn("Scheduled workflow did not start", slog.String("job_id", job.ID), slog.String("workflow", name), slog.String("error", err.Error()))
			return err
		}
		log.Info("Scheduled workflow started", slog.String("job_id", job.ID), slog.String("workflow", name), slog.String("run", run.ID), slog.String("dir", job.Workdir))
		return nil
	}

	log.Debug("Executing job",
		slog.String("job_id", job.ID),
		slog.String("agent", agentID),
		slog.String("message", job.Message))

	// Enqueue cron job execution through queue for per-session serialization
	// This prevents concurrent cron executions on the same session
	responseChan := make(chan *AgentResponse, 1)
	errorChan := make(chan error, 1)

	responseWriter := func(resp interface{}) error {
		// Handle both success (*AgentResponse) and error (error) responses
		if agentResp, ok := resp.(*AgentResponse); ok {
			responseChan <- agentResp
		} else if err, ok := resp.(error); ok {
			errorChan <- err
		} else {
			errorChan <- fmt.Errorf("invalid response type: %T", resp)
		}
		return nil
	}

	// THE CHECK RUNS WHERE IT WAS ASKED FOR. Without a workdir a scheduled
	// turn falls back to ~/memdoor-coder, so "watch
	// the build" watched a scratch directory (measured 2026-10-01). A job a
	// person configured has no workdir and keeps the old behaviour.
	// A job that names an agent must carry that agent's palette, or the queue
	// refuses it on dequeue ("arrived without its tool palette"). Resolved the
	// way a spawn resolves its requester: from the stored buddy.
	// The scheduler resolves agents against the CONFIG's list and falls back
	// to the default for a name it does not know — and the coder is a stored
	// buddy, not a config agent. So the job's own name is tried first, the
	// scheduler's answer second (live 2026-10-01: "coder" became the default
	// profile, 7 tools, and six runs answered "I have no shell").
	var buddyTools []string
	if s.repoFactory != nil {
		for _, name := range []string{job.AgentID, agentID} {
			if name == "" {
				continue
			}
			if buddy, err := s.repoFactory.Buddies().GetByName(ctx, name); err == nil && buddy != nil {
				agentID, buddyTools = name, append([]string(nil), buddy.Tools...)
				break
			}
		}
	}
	queueJob := &queue.AgentJob{
		SessionKey:     sessionKey,
		Message:        job.Message,
		GlobalLane:     queue.LaneMain,
		EnqueueTime:    time.Now(),
		Context:        ctx,
		ResponseWriter: responseWriter,
		Workdir:        job.Workdir,
	}
	if len(buddyTools) > 0 {
		queueJob.AgentID = agentID
		queueJob.BuddyTools = buddyTools
	}
	// Say what the run got: a scheduled turn with the wrong palette answers
	// "I have no shell tools" six times and nothing else explains why.
	log.Info("Cron job agent resolved",
		slog.String("job_id", job.ID),
		slog.String("agent", agentID),
		slog.Int("tools", len(buddyTools)),
		slog.String("workdir", job.Workdir))

	if err := s.queueManager.EnqueueJob(queueJob); err != nil {
		log.WithError(err).Warn("Failed to enqueue cron job", slog.String("job_id", job.ID))
		return err
	}

	// Wait for response from queue execution (synchronous)
	var response *AgentResponse
	select {
	case response = <-responseChan:
		// Success
	case err := <-errorChan:
		log.WithError(err).Warn("Job failed", slog.String("job_id", job.ID))
		return err
	case <-time.After(jobWaitTimeout()):
		log.Warn("Cron job timeout", slog.String("job_id", job.ID))
		return fmt.Errorf("cron job execution timed out")
	}

	log.Info("Job completed",
		slog.String("job_id", job.ID),
		slog.String("response", truncateForLog(response.Text, 300)))

	// A RUN NOBODY SEES IS A RUN THAT DID NOT HAPPEN. A job an agent scheduled
	// answers into the conversation that asked for it, so the poll's result
	// arrives in the window you are looking at rather than in the logs.
	if job.SessionKey != "" && strings.TrimSpace(response.Text) != "" {
		s.postCronAnswer(ctx, job, agentID, response.Text)
	}

	return nil
}

// postCronAnswer puts a scheduled run's answer in front of the person, in the
// session that scheduled it, and WAKES that conversation with it — the
// heartbeat, as OpenClaw has it (Greg, 2026-10-04: "heartbeat should be used
// like in openclaw for subpawned or cron"): the agent that scheduled the
// check gets the answer as a turn of its own and acts on it, and the answer
// is in the conversation's history rather than in a note nobody saw when no
// window was attached. A check with nothing to report answers HEARTBEAT_OK
// and wakes nobody; the same answer twice in a row wakes nobody either (a
// poll saying "still running" on every tick).
func (s *Server) postCronAnswer(ctx context.Context, job *config.CronJob, agentID, text string) {
	// The answer alone: the run's receipt line ("⚠ nothing changed · ran: …")
	// is true of every check that changes nothing by design, and under each
	// ⏱ note it was noise (Greg, 2026-10-05). It stays in the logs.
	text = cronAnswerBody(text)
	note := fmt.Sprintf("⏱ %s — %s", job.ID, text)
	if s.broadcaster != nil {
		s.broadcaster.BroadcastToSession(job.SessionKey, broadcast.Event{
			Type: "cron_answer",
			Data: map[string]interface{}{"job_id": job.ID, "text": note},
		})
	}
	last, _ := s.cronLastAnswer.Load(job.ID)
	lastText, _ := last.(string)
	wake, why := cronAnswerWakes(text, lastText)
	s.cronLastAnswer.Store(job.ID, text)
	log := logs.New("Cron")
	if !wake {
		log.Info("Scheduled check answered; the conversation sleeps on", slog.String("job_id", job.ID), slog.String("why", why))
		return
	}
	msg := fmt.Sprintf("⏱ %s, the check you scheduled, answered:\n\n%s\n\nAct on it. If this is what the check was waiting for, "+
		"stop the job: cron(action:\"stop\", id:\"%s\").", job.ID, text, job.ID)
	if err := s.wakeConversation(ctx, job.SessionKey, agentID, msg, job.Workdir); err != nil {
		log.Warn("Scheduled check's answer could not wake the conversation", slog.String("job_id", job.ID), slog.String("error", err.Error()))
		return
	}
	log.Info("Scheduled check's answer woke the conversation", slog.String("job_id", job.ID), slog.String("session", job.SessionKey))
}

// cronAnswerWakes says whether a scheduled check's answer wakes the
// conversation that scheduled it, and why not when it does not. Judged on
// the answer's body: the verification footer under it (turn_done.go — "⚠
// nothing changed · ran: …", "✓ checked: …") names the commands that run
// happened to run, so two runs saying the same thing never match with it.
func cronAnswerWakes(text, last string) (bool, string) {
	body := cronAnswerBody(text)
	switch {
	case isHeartbeatOK(body):
		return false, "HEARTBEAT_OK"
	case last != "" && body == cronAnswerBody(last):
		return false, "the same answer as the last run"
	}
	return true, ""
}

// cronAnswerBody is the answer without its verification footer.
func cronAnswerBody(text string) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "⚠ ") || strings.HasPrefix(t, "✓ checked: ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// errNoAgentConfig: the conversation's agent has no palette on record, so a
// turn built for it would run as nobody.
var errNoAgentConfig = errors.New("the agent's config could not be resolved")

// wakeConversation gives a conversation a turn it did not ask for, with text
// as the message: the result of something it was waiting on — a spawned
// run's report, a scheduled check's answer. The turn runs AS the
// conversation's agent (its palette and prompt), in its project directory,
// and queues behind any turn already running on that session.
func (s *Server) wakeConversation(ctx context.Context, sessionKey, agentID, text, workdir string) error {
	var agentTools []string
	var agentPrompt string
	if s.repoFactory != nil && agentID != "" {
		if buddy, err := s.repoFactory.Buddies().GetByName(context.Background(), agentID); err == nil && buddy != nil {
			agentTools = buddy.Tools
			if buddy.SystemPrompt != nil {
				agentPrompt = *buddy.SystemPrompt
			}
		}
	}
	if len(agentTools) == 0 {
		return fmt.Errorf("%w: %q", errNoAgentConfig, agentID)
	}
	// The caller's context is a run that is about to end (a scheduled
	// check's, with the scheduler's cancel waiting on its return): the woken
	// turn would die with it mid-answer — "⏹ Interrupted" three times in the
	// window, live 2026-10-05. Its values (the actor) travel; its cancel does not.
	return s.queueManager.EnqueueJob(&queue.AgentJob{
		SessionKey:        sessionKey,
		Message:           text,
		GlobalLane:        queue.LaneMain,
		EnqueueTime:       time.Now(),
		Context:           context.WithoutCancel(ctx),
		AgentID:           agentID,
		BuddyTools:        agentTools,
		ExtraSystemPrompt: agentPrompt,
		Workdir:           workdir,
	})
}

// Put the workspace ON THE CONTEXT. The runner resolves it correctly and
// passes it in; until 2026-08-29 this function took it as a parameter and
// never used it, so every tool that needs a workspace saw none, and a
// heartbeat that could never succeed burned an LLM turn on every tick. Go
// does not warn on an unused PARAMETER, only an unused local, which is why
// nothing ever flagged it.

// Build session key for heartbeat execution
// Pattern: heartbeat:agentID format (isolated from main session)

// Build heartbeat prompt with checklist

// Enqueue heartbeat execution through queue for per-session serialization
// This prevents concurrent heartbeat executions on the same session

// Handle both success (*AgentResponse) and error (error) responses

// Pull the agent's per-buddy system_prompt through the buddies repo
// so the LLM actually receives the agent's curated rules. Without
// this the LLM only ever sees the generic SystemPromptBuilder
// output and the per-agent guidance is dead text. Repo handles
// the SQL — this file stays free of buddy-schema knowledge.

// Wait for response from queue execution (synchronous)

// Success

// Reaping here only stops the wait — the enqueued job keeps running and
// holding the engine. Match the execution backstop so a slow-but-healthy
// turn isn't declared dead while it is still legitimately prefilling.

// Check if response is HEARTBEAT_OK (suppression)

// If not suppressed, response contains something that needs attention

// Post heartbeat findings to channel if channel_id is provided

// Create ActorID for the agent

// Post heartbeat findings as a message in the channel
// Use parentMessageID=0 to indicate non-threaded message (handled specially by PostSystemAnnouncement)

// Don't fail the heartbeat if posting fails - just log the error

// isHeartbeatOK: a scheduled check with nothing to report answers this
// token (OpenClaw's rule), and the conversation that scheduled it is not
// woken. Short, so "HEARTBEAT_OK — but note that…" still wakes.
const (
	heartbeatOKToken    = "HEARTBEAT_OK"
	heartbeatOKMaxChars = 100
)

func isHeartbeatOK(content string) bool {
	// Check if content contains the HEARTBEAT_OK token
	if !strings.Contains(content, heartbeatOKToken) {
		return false
	}

	// Check if content is short enough (under 100 chars)
	if len(content) > heartbeatOKMaxChars {
		return false
	}

	return true
}

// isSubagentSession checks if a session key belongs to a subagent
func isSubagentSession(sessionKey string) bool {
	// Subagent sessions have format: agent:{agentID}:subagent:{runID}
	if !strings.HasPrefix(sessionKey, "agent:") {
		return false
	}
	return strings.Contains(sessionKey, ":subagent:")
}

// handleSubagentCompletion handles subagent completion, announcement, and cleanup
// Pattern: OpenClaw announce-back mechanism
func (s *Server) handleSubagentCompletion(ctx context.Context, sessionKey string, response *AgentResponse) error {
	log := logs.New("Subagents")
	log.Info("🔔 handleSubagentCompletion called",
		slog.String("session_key", sessionKey))

	// Task Flow interception: if a managed flow is waiting on this child, the flow
	// driver advances it deterministically (dispatches the next step, or finishes)
	// — the model never has to decide the next step. Skip the generic announce-back
	// entirely; the child is ephemeral, so delete its isolated session.
	if s.flowRegistry != nil {
		// Mark the run ended first so the flow-step completion waiter
		// (waitForFlowStepCompletion) sees a normal completion and returns early,
		// rather than waiting out its full timeout. Must run BEFORE OnStepComplete,
		// whose advance reactivates (deletes) this run record on the next dispatch.
		s.subagentRegistry.MarkEnded(sessionKey)
		var resultText string
		if response != nil {
			resultText = response.Text
		}
		if handled, finished := s.flowRegistry.OnStepComplete(sessionKey, resultText); handled {
			log.Info("subagent completion handled by task-flow driver",
				slog.String("session_key", sessionKey),
				slog.Bool("flow_finished", finished))
			// The flow reuses one coder session across steps (continuity), so keep
			// it alive between steps — only delete it once the flow has finished.
			if finished {
				if err := s.sessions.DeleteSession(sessionKey); err != nil {
					log.WithError(err).Debug("flow session cleanup failed",
						slog.String("session", sessionKey))
				}
			}
			return nil
		}
	}

	// The subagent's ProcessMessage has returned, so the run is definitively over.
	// Mark it ended synchronously to beat the async lifecycle event — ShouldAnnounce
	// keys on EndedAt, and losing that race silently drops the announce-back.
	s.subagentRegistry.MarkEnded(sessionKey)

	// Find run record by session key
	var record *subagents.SubagentRunRecord
	for _, run := range s.subagentRegistry.ListAll() {
		if run.ChildSessionKey == sessionKey {
			record = run
			break
		}
	}

	if record == nil {
		log.Warn("Subagent run record not found",
			slog.String("session_key", sessionKey))
		return fmt.Errorf("subagent run not found for session %s", sessionKey)
	}

	log.Info("Found subagent run record",
		slog.String("run_id", record.RunID),
		slog.Int64("parent_message_id", record.ParentMessageID))

	// Wait for registry to be updated by lifecycle event (max 500ms)
	// The lifecycle event is processed in a goroutine, so there's a race between
	// this function being called and the registry being updated
	maxWait := 500 * time.Millisecond
	pollInterval := 10 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		if subagents.ShouldAnnounce(record) {
			break
		}
		time.Sleep(pollInterval)

		// Re-fetch record in case it was updated
		for _, run := range s.subagentRegistry.ListAll() {
			if run.ChildSessionKey == sessionKey {
				record = run
				break
			}
		}
	}

	// Check if we should announce
	if !subagents.ShouldAnnounce(record) {
		log.Debug("Skipping announce (run not marked ended)",
			slog.String("run_id", record.RunID))
		return nil
	}

	// Is the result the task done? Judged before the announcement; the first
	// unfinished result goes back to the child (result_acceptance.go).
	review := s.acceptance.review(ctx, record, response.Text)
	if review.HandedBack {
		log.Info("Result handed back to the child; announcing its next result instead",
			slog.String("run_id", record.RunID))
		return nil
	}

	// Calculate stats
	stats := subagents.CalculateStats(record)

	// Build announcement message
	announceParams := subagents.AnnounceParams{
		Check:               review.Note,
		Verdict:             verdictOf(review),
		RunID:               record.RunID,
		Label:               record.Label,
		Task:                record.Task,
		RequesterSessionKey: record.RequesterSessionKey,
		ChildSessionKey:     record.ChildSessionKey,
		FinalOutput:         response.Text,
		Stats:               stats,
		Outcome:             record.Outcome,
	}

	announcement := subagents.BuildAnnouncementMessage(announceParams)

	log.Debug("Announcing completion of run",
		slog.String("run_id", record.RunID),
		slog.String("requester_session", record.RequesterSessionKey),
		slog.Int64("parent_message_id", record.ParentMessageID))

	// Add parent message ID to context for threading the announcement response
	// This ensures the writer agent's response to the subagent completion
	// is posted as a thread reply to the original user message
	announcementCtx := ctx
	if record.ParentMessageID > 0 {
		announcementCtx = context.WithValue(ctx, ctxParentMessageID, record.ParentMessageID)
		log.Debug("Added parent_message_id to announcement context for threading",
			slog.Int64("parent_message_id", record.ParentMessageID))
	}

	// Enqueue announcement to requester session. This re-invokes the REQUESTING
	// agent (e.g. the planner) with the subagent's result so it can coordinate the
	// next step. Carry the requester's agent id so it runs AS that agent (its palette
	// + prompt), not a generic default — RequesterDisplayKey is "Agent <id>".
	// Re-invoke the requester WITH its config — same "built with palette" rule as a
	// spawn — so the announce runs AS that agent (e.g. the planner keeps its
	// sessions_spawn palette to drive the next step).
	// The requester's turn on this announcement runs where the requester
	// works — the child session recorded it at spawn (tools.RequesterWorkdirKey).
	var requesterWorkdir string
	if child, err := s.sessions.GetSession(record.ChildSessionKey); err == nil && child != nil {
		if wd, ok := child.GetMetadataValue(tools.RequesterWorkdirKey); ok {
			requesterWorkdir, _ = wd.(string)
		}
	}
	requesterAgentID := strings.TrimPrefix(record.RequesterDisplayKey, "Agent ")
	log.Debug("Announcing subagent result back to requester",
		slog.String("run_id", record.RunID),
		slog.String("requester_agent", requesterAgentID),
		slog.String("requester_session", record.RequesterSessionKey))
	switch err := s.wakeConversation(announcementCtx, record.RequesterSessionKey, requesterAgentID, announcement, requesterWorkdir); {
	case errors.Is(err, errNoAgentConfig):
		log.Warn("announce-back skipped — requester config could not be resolved",
			slog.String("requester_agent", requesterAgentID))
		return nil
	case err != nil:
		return fmt.Errorf("failed to enqueue announcement: %w", err)
	}

	// Handle session cleanup based on cleanup mode
	if record.Cleanup == "delete" {
		if err := s.sessions.DeleteSession(record.ChildSessionKey); err != nil {
			log.WithError(err).Warn("Failed to delete session",
				slog.String("session", record.ChildSessionKey))
		} else {
			log.Debug("Deleted session",
				slog.String("session", record.ChildSessionKey),
				slog.String("cleanup_mode", "delete"))
		}
	} else {
		log.Debug("Keeping session",
			slog.String("session", record.ChildSessionKey),
			slog.String("cleanup_mode", "keep"))
	}

	// Mark cleanup as handled
	if err := s.subagentRegistry.MarkCleanupHandled(record.RunID); err != nil {
		log.WithError(err).Warn("Failed to mark cleanup handled",
			slog.String("run_id", record.RunID))
	}

	return nil
}

// registerChannelMessageHandler registers a handler for incoming channel messages
// Pattern: OpenClaw multi-channel routing
func (s *Server) registerChannelMessageHandler() {
	handler := func(msg *adapters.IncomingMessage) error {
		log := logs.New("Channels")

		log.Debug("Received message from channel",
			slog.String("channel_id", msg.ChannelID),
			slog.String("channel_user_id", msg.ChannelUserID),
			slog.String("text", msg.Text))

		// Get session key from channel router binding
		sessionKey, err := s.channelRouter.GetSessionKey(msg.ChannelID, msg.ChannelUserID)
		if err != nil {
			log.Debug("No binding found, using default session",
				slog.String("channel_id", msg.ChannelID),
				slog.String("channel_user_id", msg.ChannelUserID))
			// Create default session key
			sessionKey = fmt.Sprintf("main:default:%s-%s", msg.ChannelID, msg.ChannelUserID)

			// Auto-bind this user to the default session
			s.channelRouter.BindUser(msg.ChannelID, msg.ChannelUserID, sessionKey)
		}

		// Create response writer callback to send response back to the channel
		responseWriter := func(response interface{}) error {
			if responseMap, ok := response.(map[string]interface{}); ok {
				if text, ok := responseMap["text"].(string); ok {
					return s.channelRouter.SendMessage(msg.ChannelID, msg.ChannelUserID, text)
				}
			}
			return fmt.Errorf("invalid response format")
		}

		// Enqueue job for execution
		job := &queue.AgentJob{
			SessionKey:     sessionKey,
			Message:        msg.Text,
			GlobalLane:     queue.LaneMain,
			EnqueueTime:    time.Now(),
			Context:        context.Background(),
			ResponseWriter: responseWriter,
		}

		return s.queueManager.EnqueueJob(job)
	}

	// Register handler with all adapters
	for _, adapter := range s.channelRouter.ListAdapters() {
		adapter.RegisterHandler(handler)
	}
}

// extractChannelIDFromSessionKey extracts the channel ID from a session key
// Format: "workspace:X:channel:Y" → returns "Y"
func extractChannelIDFromSessionKey(sessionKey string) string {
	parts := strings.Split(sessionKey, ":")
	if len(parts) >= 4 && parts[2] == "channel" {
		return parts[3]
	}
	return ""
}

// resolvePostingAgentID returns the id of the agent that ran this job, for
// attributing its channel message. It reads, in order of authority: the job's
// explicit target agent (spawned subagents / planner→coder), the running
// agent's name carried on the context (WS/CLI channel turns), the name the run
// stashed in session metadata (transcript_agent, set during ProcessMessage),
// and finally an "agent:<id>:<label>" session key. Empty means genuinely
// unknown — the caller must not fabricate an author.
func resolvePostingAgentID(ctx context.Context, job *queue.AgentJob, session *Session) string {
	if job != nil && job.AgentID != "" {
		return job.AgentID
	}
	if name, _ := ctx.Value("buddy_agent_name").(string); name != "" {
		return name
	}
	if session != nil {
		if v, ok := session.GetMetadataValue("transcript_agent"); ok {
			if name, _ := v.(string); name != "" {
				return name
			}
		}
	}
	if job != nil {
		return agentIDFromSessionKey(job.SessionKey)
	}
	return ""
}

// agentIDFromSessionKey pulls the agent id out of a subagent session key of the
// form "agent:<agentId>:<label>" (see the A2A SessionResolver). Empty for a
// channel session key ("workspace:X:channel:Y"), where the agent isn't encoded.
func agentIDFromSessionKey(sessionKey string) string {
	parts := strings.Split(sessionKey, ":")
	if len(parts) >= 3 && parts[0] == "agent" {
		return parts[1]
	}
	return ""
}

// scheduledWorkflow reads "/workflow run <name> [partition]" from a cron
// job's message.
func scheduledWorkflow(message string) (name, partition string, ok bool) {
	f := strings.Fields(strings.TrimSpace(message))
	if len(f) < 3 || f[0] != "/workflow" || f[1] != "run" {
		return "", "", false
	}
	if len(f) > 3 {
		partition = f[3]
	}
	return f[2], partition, true
}
