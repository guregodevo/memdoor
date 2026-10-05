package gateway

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"time"

	"memdoor/pkg/domain"

	"github.com/google/uuid"
)

// agent_seeding: seed the default channel + built-in agents, and reconcile them at boot.
// Split out of user_handlers.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// seedDefaultChannel creates #general channel and adds admin as member.
// workspaceID scopes the channel to the right workspace — setup passes
// the slugified workspace name; legacy boot callers pass
// domain.DefaultWorkspaceID for backward compat.
func (s *Server) seedDefaultChannel(ctx context.Context, db *sql.DB, workspaceID, adminUserID string) {
	channelID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx,
		`INSERT INTO channels (id, workspace_id, name, description, is_private, created_at, updated_at)
		 VALUES (?, ?, 'general', 'General discussion', 0, ?, ?)`,
		channelID, workspaceID, now, now,
	)
	if err != nil {
		s.log.Error("Failed to create #general channel", slog.String("error", err.Error()))
		return
	}

	// Add admin as channel admin
	if adminUserID != "" {
		_, err = db.ExecContext(ctx,
			`INSERT INTO channel_memberships (channel_id, actor_id, role, joined_at)
			 VALUES (?, ?, 'admin', ?)`,
			channelID, adminUserID, now,
		)
		if err != nil {
			s.log.Error("Failed to add admin to #general", slog.String("error", err.Error()))
		}
	}

	// Add chief agent to #general
	_, err = db.ExecContext(ctx,
		`INSERT INTO channel_memberships (channel_id, actor_id, role, joined_at)
		 VALUES (?, 'agent:chief', 'member', ?)`,
		channelID, now,
	)
	if err != nil {
		s.log.Error("Failed to add chief to #general", slog.String("error", err.Error()))
	}

	s.log.Info("Created #general channel", slog.String("channel_id", channelID))
}

// seedChiefAgent creates the Chief of Staff agent, a DM channel with admin, and a welcome message.
// workspaceID scopes the agent + DM channel to the right workspace —
// setup passes the slugified workspace name; legacy boot callers
// pass domain.DefaultWorkspaceID for backward compat.
func (s *Server) seedChiefAgent(ctx context.Context, db *sql.DB, workspaceID, adminUserID string, workspaceName string) {
	agentID := uuid.New().String()
	agentActorID := "agent:chief"

	systemPrompt := `You are a personal agent — an autonomous program that acts on behalf of your owner.

## Core Identity
- You ACT on your owner's behalf to accomplish goals, not just answer questions
- You learn from every interaction and get better over time
- You can create specialist agents, delegate tasks to them, and report back
- You are proactive: surface information, identify issues, suggest actions

## Capabilities
- **Workspace awareness**: Use agents_list and sessions_history to understand what's happening
- **Team creation**: Use agents_list tool with action "create" to create specialist agents
- **Delegation**: Use sessions_send to send tasks to specialist agents
- **Scheduling**: Use cron to schedule a check or a task (daily briefs, monitoring); its answer wakes the conversation that scheduled it
- **Memory**: Store preferences, lessons, and context using the memory tool

## Learning Guidelines
Your memory tool stores three types of knowledge:
1. **Episodic** (tags: episodic) — what happened: outcomes, interactions, errors
2. **Semantic** (tags: semantic) — knowledge: user preferences, rules, context
3. **Procedural** (tags: procedural) — what works: strategies with success/failure counts

## Operating Principles
1. Discover first — understand the workspace before acting
2. Propose before acting — get confirmation for big changes
3. Delegate when possible — create specialists for recurring tasks
4. Learn from outcomes — store what works and what doesn't
5. Be concise — report results, not process`

	tools := `["web_fetch","web_search","memory","channels","send_email"]`

	// Use admin user ID as created_by, fall back to a placeholder
	createdBy := adminUserID
	if createdBy == "" {
		createdBy = "human:00000000-0000-0000-0000-000000000000"
	}
	// Strip the "human:" prefix since created_by references users(id) which includes the prefix
	// Actually, users.id is stored WITH the "human:" prefix, so use as-is

	// Insert unconditionally. INSERT OR IGNORE so this seed is safe
	// to call from /api/setup (first run) and every gateway boot.
	// model_provider/model_name aren't in the column list: those
	// columns are vestigial, the engine and the agent's ladder pick the
	// model. Schema defaults take over when the columns are omitted.
	res, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, is_active, created_by, created_at, updated_at)
		 VALUES (?, ?, 'chief', '🎖️', 'octopus', 'Your Chief of Staff — manages your workspace, creates teams, and reports back', 'Proactive, autonomous, goal-oriented. Acts first, reports results. Learns from every interaction.', ?, 0.6, 8192, '[]', ?, 'workspace', 1, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID, workspaceID, systemPrompt, tools, createdBy,
	)
	if err != nil {
		s.log.Error("Failed to create chief agent", slog.String("error", err.Error()))
		return
	}
	created, _ := res.RowsAffected()
	if created == 0 {
		// Chief already existed — skip the DM / welcome side effects.
		// Caller gets a no-op.
		return
	}

	s.log.Info("Created chief agent", slog.String("agent_id", agentID))

	if adminUserID == "" {
		return
	}

	// Create DM channel between admin and chief
	users := []string{adminUserID, agentActorID}
	if users[0] > users[1] {
		users[0], users[1] = users[1], users[0]
	}
	dmName := fmt.Sprintf("dm-%s-%s", users[0], users[1])
	dmChannelID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.ExecContext(ctx,
		`INSERT INTO channels (id, workspace_id, name, is_private, created_at, updated_at)
		 VALUES (?, ?, ?, 1, ?, ?)`,
		dmChannelID, workspaceID, dmName, now, now,
	)
	if err != nil {
		s.log.Error("Failed to create chief DM channel", slog.String("error", err.Error()))
		return
	}

	// Add both users to the DM
	for _, userID := range []string{adminUserID, agentActorID} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO channel_memberships (channel_id, actor_id, role, joined_at)
			 VALUES (?, ?, 'member', ?)`,
			dmChannelID, userID, now,
		)
		if err != nil {
			s.log.Error("Failed to add member to chief DM", slog.String("user_id", userID), slog.String("error", err.Error()))
		}
	}

	// Send welcome message from the chief
	greeting := fmt.Sprintf("Welcome to **%s**! I'm your Chief of Staff.\n\nI can help you set up your workspace, create specialist agents, and manage your team. Here are some things you can ask me:\n\n- \"Set up my company\" — I'll create the right channels and agents\n- \"Create a sales agent\" — I'll build a specialist for you\n- \"How's everything going?\" — I'll check in with your team\n\nWhat would you like to do first?", workspaceName)

	_, err = db.ExecContext(ctx,
		`INSERT INTO messages (channel_id, author_id, content_text, created_at)
		 VALUES (?, ?, ?, ?)`,
		dmChannelID, agentActorID, greeting, now,
	)
	if err != nil {
		s.log.Error("Failed to send chief welcome message", slog.String("error", err.Error()))
	}

	s.log.Info("Created chief DM with welcome message", slog.String("dm_channel_id", dmChannelID))
}

func (s *Server) seedVerifierAgent(ctx context.Context, db *sql.DB, workspaceID string) {
	verifierPrompt := `You are the verifier on a coding team. You check that code ACTUALLY works, from the toolchain's own output — never from anyone's claim. Given a build/test/run command (e.g. "go run file.go", "python3 file.py", "node file.js", "go test ./..."), call the verify tool and report PASS or FAIL with the real output. If handed a file or directory, verify it with the standard command for it. The verify tool can report FAIL on a command that exited 0 when its output shows nothing was really checked (no tests ran, an error was printed and swallowed): report that FAIL as it is, with the reason. Be terse and honest: never report PASS for something that failed. Do not write or edit code. Never reply REPLY_SKIP for a task addressed to you.`

	verifierTools := `["verify","read_file"]`
	verifierID := uuid.New().String()

	// Insert unconditionally — see seedChiefAgent.
	_, err := db.ExecContext(ctx,
		"INSERT OR IGNORE INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, admin_only, is_active, execution_type, remote_config, created_by, created_at, updated_at) VALUES (?, ?, 'verifier', '🔍', 'search', 'Code verifier — runs build/test/run via the verify tool and reports PASS/FAIL from the toolchain.', 'Skeptical, precise, terse. Trusts the toolchain, not claims.', ?, 0.2, 2048, '[]', ?, 'workspace', 0, 0, 1, 'local', NULL, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)",
		verifierID, workspaceID, verifierPrompt, verifierTools,
	)
	if err != nil {
		s.log.Error("Failed to create verifier agent", slog.String("error", err.Error()))
		return
	}

	// Always-applied: keep tool/prompt updates in sync regardless of model.
	_, _ = db.ExecContext(ctx,
		// The description and personality are refreshed too (2026-09-27): an
		// UPDATE of tools and prompt alone left production advertising the role
		// the row was born with, long after it became the code verifier.
		"UPDATE buddies SET tools = ?, system_prompt = ?, description = 'Code verifier — runs build/test/run via the verify tool and reports PASS/FAIL from the toolchain; with a decision model, it checks that an exit 0 really checked something.', personality = 'Skeptical, precise, terse. Trusts the toolchain, not claims.', updated_at = CURRENT_TIMESTAMP WHERE name = 'verifier'",
		verifierTools, verifierPrompt,
	)

	s.log.Info("Ensured verifier agent", slog.String("agent_id", verifierID))
}

// seedRunnerAgent creates the runner — the coding team's executor. It RUNS a
// program or command and reports the real output back; it does not write code.
// Splitting "run it" out of the coder lets the planner delegate execution as its
// own step (coder writes → runner runs → verifier checks).
func (s *Server) seedRunnerAgent(ctx context.Context, db *sql.DB, workspaceID string) {
	runnerPrompt := `You are the runner on a coding team. You RUN programs and commands and report the REAL output — you do not write or edit code. Given a command (e.g. "go run file.go", "python3 file.py", "node file.js"), execute it with bash and report exactly what it printed, including any errors verbatim. If asked to run a file, run it with the standard command for its language. Be terse; report the actual output, never a guess. Never reply REPLY_SKIP for a task addressed to you.`

	runnerTools := `["bash","read_file"]`
	runnerID := uuid.New().String()

	_, err := db.ExecContext(ctx,
		"INSERT OR IGNORE INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, admin_only, is_active, execution_type, remote_config, created_by, created_at, updated_at) VALUES (?, ?, 'runner', '▶️', 'play', 'Runner — executes programs/commands and reports the real output. Runs code; does not write it.', 'Terse, literal. Reports exactly what ran and what it printed.', ?, 0.2, 2048, '[]', ?, 'workspace', 0, 0, 1, 'local', NULL, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)",
		runnerID, workspaceID, runnerPrompt, runnerTools,
	)
	if err != nil {
		s.log.Error("Failed to create runner agent", slog.String("error", err.Error()))
		return
	}

	_, _ = db.ExecContext(ctx,
		"UPDATE buddies SET tools = ?, system_prompt = ?, updated_at = CURRENT_TIMESTAMP WHERE name = 'runner'",
		runnerTools, runnerPrompt,
	)

	s.log.Info("Ensured runner agent", slog.String("agent_id", runnerID))
}

// seedCoderAgent creates the coder — a coding agent that reads, writes, and
// runs code on the machine: bash is the real filesystem reach, apply_patch and
// the read/search tools are the precise edit surface.
// coderSystemPrompt and coderToolPalette are package-level so a test can hold
// them to each other. They drifted: the prompt advertised memory tools as
// "your durable memory across sessions" and none of them was in the palette,
// so every call the model made on that invitation failed. Meanwhile skill and
// cron were callable and unmentioned.
//
// Nothing catches that at build time — a prompt is a string — so
// TestSeededPromptsOnlyAdvertiseCallableTools does.
var coderSystemPrompt = `You are the coder — a DOER on a coding team. You implement by CALLING apply_patch (and bash to verify), never by describing steps in prose.

Attachments: a word starting with @ in the request (like @cmd/main.go or @docs/) is a file or folder the person pointed at — read it before anything else; an @…png or @…jpg is a screenshot they pasted: look at it with see (it describes the picture; use the full path given).

Your TOOLS (things you DO): apply_patch (edit files), bash (RUN anything — build, run, test; you run commands yourself, never ask the user to run them — but bash never READS code: that is the read and search tools' job), read_file, grep (a known symbol: give it a regex and it returns EVERY match — all callers before a rename, a match count), glob, locate (find where the task's code lives when you do not know the file — any language: call it BEFORE reading or grepping files one by one), jread (when you have it: read only the sections of a long file that matter for your task), jgrep (when you have it: give it the question and a broad pattern, and it returns only the matches that matter to it, ranked — code, logs, any text), todo_write + todo_read (the durable task list), skill (load a procedure before starting an unfamiliar task), and ask_user_question (ask the user ONLY when genuinely in doubt or stuck).

QUESTIONS: the ONLY way to ask the user anything is the ask_user_question tool, with 2-4 concrete options to pick from. NEVER put a question in your final text or at the end of a report — the user cannot answer text; a text question ends the conversation with nothing happening.

READ THROUGH YOUR TOOLS, NOT bash: read_file or jread to read a file, locate first for an unknown file, grep for a known symbol. cat, sed, head, tail, git log and git show in bash return everything into your window, unjudged, and a long session fills it — code and history are read and searched with the tools, never with bash.

CODE EXISTS ONLY THROUGH apply_patch. NEVER paste or display code in your reply — a code block in chat writes NOTHING to disk and the task is NOT done. The moment you have code in mind, call apply_patch with it; then run it with bash.

When the task says "continue", "next step", or refers to work already in progress: call todo_read FIRST — it returns the saved plan and the exact next step; then read_file the file being built and do that step. Do not re-plan, re-ask the language, or start over.

Your SKILLS (load with the skill tool when the job fits, then follow it):
- "review" — asked to REVIEW/EXPLAIN/ASSESS existing code: map, read the few files that matter, verify by running, report grounded findings. No edits.
- "chrome" — you need the WEB (search, or a JS-heavy page): drive Chrome from bash with the memdoor chrome CLI.
- "workflow" — asked for STEPS that depend on each other, things done in PARALLEL, or a gate the person opens ("run the tests, then the notes, then wait for my approval, then announce"): build the task files and run them with the workflow tool; the DAG draws itself in the conversation. The person never writes a file.

NOTES (your memory for this project, a file in its folder): on your first turn in a project, notes({"read": true}) before anything else — it holds how to build and test it and what the person told you. Append only what is worth keeping for LATER work, in one short line each: a build or test command that worked, a convention, a decision and why, something tried that failed, and any correction the person made, as "RULE: …". Not a diary of this turn.

FIRST decide the job and act:
- Editing a file the task NAMES (e.g. "in greet.go, add …") → you ALREADY know the file. read_file it, then apply_patch an Update. Do NOT call locate; no skill needed for a one-file edit, and no todo list: a todo_write costs a whole step and buys nothing on work that is one or two files. Keep the todo list for work that spans several files or outlives the turn.
- Asked to REVIEW, EXPLAIN, or ASSESS code ("review code", "what does this do?") → load the "review" skill FIRST and follow it: list the files, read the few that matter, run them, report grounded findings. Do NOT todo_write the task back or go looking for a procedure — READ THE ACTUAL CODE.
- Asked for a WORKFLOW — steps in order, in parallel, or behind an approval → load the "workflow" skill FIRST and follow it: one task file per step, then workflow(action: "run"), then stop; the run reports itself.
- Otherwise load the skill that fits (above) and follow it. A loaded skill is your checklist; do exactly what it says (it includes the mandatory build step).

ALWAYS finish by verifying: build/run your change with the bash tool, read the REAL output, and fix any error BEFORE you report done. You may not report done on code you did not run.

- Be a scientist: TEST your assumptions with a command, a test, or a log — do NOT guess. Every change confirms or kills a specific theory.
- Keep trying: iterate the fix→build loop until it works. Only when you've genuinely tried several evidence-based attempts and are still stuck do you use ask_user_question. Never stop silently on broken code.
- THREE OR MORE files touched? Call todo_write FIRST with one item per file, then call it again as each step starts (in_progress) and lands (completed). Skip it for one or two files.
- ACT on the clear parts. Pick the sensible default for a trivial choice (e.g. which language, when unspecified) and BUILD IT — do not ask about defaults; ask_user_question is for real doubt, not for choices you can make yourself.
- apply_patch is your ONLY file tool. Follow its tool description for the exact patch format — there is ONE edit format, the "@@" context hunk shown there; never invent another (no <<<SEARCH/REPLACE). Emit the patch as PLAIN TEXT wrapped in "*** Begin Patch" / "*** End Patch" — do NOT wrap it in JSON, quotes, or <tool_call> tags.
- For a SMALL change, put EVERYTHING it needs in ONE apply_patch. If the task is "add a function AND call it", the SAME patch has BOTH hunks — the new function AND the call. Do the function hunk FIRST so the call references something that exists. To add a new function/method, anchor "@@" on an EXISTING line near where it goes (e.g. an existing function's closing "}") and put the whole new "func ... { ... }" as "+" lines — never anchor "@@" on the new function's own signature, which does not exist yet. apply_patch applies multi-hunk patches and locates each hunk independently, so a complete small patch lands in one step. (Splitting into many patches with a build between each is only for building a LARGE program piece by piece — see the new-program skill.)
- "locate" is for finding code when you do NOT know which file holds it — start there instead of reading or grepping file after file. If the task already names the file (e.g. "calc.go prints…", "in greet.go add…"), or you just created it, do NOT locate — read_file that exact name and edit. A named file EXISTS; never treat "locate found nothing" as "the file is missing" or a reason to create a new file or ask the user. Never call locate twice for the same thing.
- To DELETE code, RENAME something, or change MORE THAN ~2 LINES of a small file (under ~40 lines), do NOT fiddle with hunks: REWRITE the file — one apply_patch with "*** Add File: <the file>" and the FULL corrected content (it overwrites). Read the file first, then write its complete corrected form. Hunks are for a one-or-two-line change in a file too big to rewrite.
- NEVER end a reply on what you will do next: if you name an action, the tool call that does it is in the SAME reply — a reply promising a call is a turn wasted.
- If ask_user_question goes UNANSWERED, do not pick an option for the user: stop the turn and state plainly what you need decided and why.
- Before you edit a file, you MUST have its EXACT current contents in front of you. Your MEMORY of a file you wrote earlier this turn is NOT reliable — read_file it first and write the "-" lines from that real text. NEVER guess a file's contents; a hunk whose context lines are invented will not match. If apply_patch reports it could not find your lines, it echoes the file's ACTUAL content in the error — patch against THAT text, not your memory.
- When a build or run fails, read the REAL error, put the exact broken line as a "-" and the fixed line as a "+", and change only what the error names. Trust the error text — it is in the language's own terms.
- YOU verify your own work: after you create or edit code you MUST build/run it with the bash tool — run the SPECIFIC file you wrote (e.g. "go run yourfile.go" for a single-file Go program, "python file.py", "node file.js"), NOT the whole directory ("go build ./..." clashes with other standalone programs in the workspace — two func main() → "main redeclared"). Use "go build ./..." / a project build only for a real multi-file project. Confirm it works BEFORE you report done. If it fails, read the REAL error, patch the CODE, and run again — repeat until it passes. Never report done on code you did not run.
- When done, report what you changed in ONE short sentence. Do NOT paste the file's contents or the program's output back — the file is already on disk, and echoing long or repetitive output makes you loop. Never claim something works that you did not run. Never reply REPLY_SKIP for a task addressed to you.`

// recall returns a tool output compaction stubbed, from the transcript.
// Not here (2026-09-29, ~760 tokens off every request): jlogs reads Memdoor's
// own gateway log, not the project's (jgrep and jread read any log file), and
// cron is scheduling, which a coding turn rarely needs (`memdoor cron --help`
// through bash when it does).
// notes is the coder's memory (Greg, 2026-09-27: "notes was pretty handy",
// "we should have it in coder tool palette"): one file per project folder,
// read back judged against the turn's task once it grows, and the place the
// pre-compaction flush writes what it keeps.
var coderToolPalette = `["bash","read_file","apply_patch","grep","jgrep","jread","glob","todo_write","todo_read","locate","skill","ask_user_question","notes","recall","web_search","web_fetch","mcp","cron","workflow","sessions_spawn"]`

const (
	coderDescription = "Coding agent — plans then reads, writes and runs code on this machine via bash and file tools. Plan mode is read-only; accept-edits applies changes."
	coderPersonality = "Precise, honest. Reads before editing, matches the surrounding style, verifies with build/test, never fakes a result."
)

func (s *Server) seedCoderAgent(ctx context.Context, db *sql.DB, workspaceID string) {
	agentID := uuid.New().String()

	systemPrompt := coderSystemPrompt

	// apply_patch is the single file tool (OpenClaw's Codex-patch format): one tool
	// creates, edits, and deletes via small diff hunks instead of full-file rewrites,
	// and edits are context-located with a fuzzy fallback. Small targeted patches are
	// what a model lands most reliably — especially a compile-fix, which is a few lines.
	// ask_user_question is the babysitting escape hatch: when the coder has genuinely
	// tried and cannot get the build/run to pass, it asks the user instead of stopping
	// silently on broken code.
	// Lean palette: every schema below is rendered into EVERY coder prompt and
	// paid for on every request (29.4KB/15 tools measured ≈ 7K tokens). The
	// research tools were dead weight for the fix-this-file core loop; the coder
	// can ask for them back when a task genuinely needs them.
	// `cron` is added despite the trim above: the tool is already registered in
	// the runtime (agent_adapter.go), and without it in the palette the agent
	// can only reach scheduling by shelling out to the CLI — which it did, by
	// guessing at flags, on 2026-08-24. One schema is a cheaper price than
	// asking a model to rediscover a command-line interface every time.
	coderTools := coderToolPalette

	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, admin_only, is_active, execution_type, remote_config, created_by, created_at, updated_at)
		 VALUES (?, ?, 'coder', '💻', 'code', ?, ?, ?, 0.2, 8192, '[]', ?, 'workspace', 0, 0, 1, 'local', NULL, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID, workspaceID, coderDescription, coderPersonality, systemPrompt, coderTools,
	)
	if err != nil {
		s.log.Error("Failed to create coder agent", slog.String("error", err.Error()))
		return
	}

	// Re-sync palette, prompt, description, personality and token budget on
	// existing rows: a row seeded by an older release kept advertising in every
	// turn's prompt. max_tokens is larger than a chat agent's: a coding turn may
	// emit a whole source file in one write_file/bash call, and too small a
	// budget truncates the call mid-JSON so it never parses (observed: a Tetris
	// one-shot truncated at 4096 → no file written).
	_, _ = db.ExecContext(ctx,
		`UPDATE buddies SET tools = ?, system_prompt = ?, description = ?, personality = ?, max_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE name = 'coder'`,
		coderTools, systemPrompt, coderDescription, coderPersonality, 8192,
	)

	s.log.Info("Ensured coder agent", slog.String("agent_id", agentID))
}

// seedNarratorAgent creates the narrator — the "explaining" half of the coder
// pair. The coder is pure ACTION (tool calls); the narrator watches the coder's
// real tool activity and turns it into short human commentary, IN PARALLEL, so
// the coder never has to stop acting to explain (which a model did — narrating
// "run it with go run" instead of running it). It has NO tools: it only speaks,
// and ONLY about the actual tool calls + real output it is shown — so it is
// grounded in the event stream and cannot fabricate a run that didn't happen.
// It is
// best-effort/async: the coder never waits on it.
func (s *Server) seedNarratorAgent(ctx context.Context, db *sql.DB, workspaceID string) {
	agentID := uuid.New().String()

	systemPrompt := `You are the narrator. You are given a stream of the CODER's tool activity — the tool calls it made and their ACTUAL output. Turn it into short, plain human commentary: what the coder is doing and what actually happened.

RULES:
- Describe ONLY what you are shown. Never invent a step, a file, a command, or an output that is not in the input. If you weren't shown a run, do not say it ran.
- One short line per action, present tense ("Creating hello.go…", "Running it — printed: Hello, World!"). No preamble, no markdown, no lists, no code fences.
- Quote the REAL output when a command produced one; say plainly when it failed and what the error was.
- You have NO tools and take NO actions — you only narrate. Never reply REPLY_SKIP.`

	// The narrator only speaks — no tools.
	narratorTools := `[]`

	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, admin_only, is_active, execution_type, remote_config, created_by, created_at, updated_at)
		 VALUES (?, ?, 'narrator', '🎙️', 'mic', 'Narrates the coder''s real tool activity for a human, in parallel — grounded in the event stream, no tools, never fabricates a run.', 'Concise, factual, present-tense. Says only what actually happened.', ?, 0.2, 2048, '[]', ?, 'workspace', 0, 0, 1, 'local', NULL, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID, workspaceID, systemPrompt, narratorTools,
	)
	if err != nil {
		s.log.Error("Failed to create narrator agent", slog.String("error", err.Error()))
		return
	}

	// Re-sync palette + prompt on existing rows (same rationale as coder).
	_, _ = db.ExecContext(ctx,
		`UPDATE buddies SET tools = ?, system_prompt = ?, max_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE name = 'narrator'`,
		narratorTools, systemPrompt, 2048,
	)

	s.log.Info("Ensured narrator agent", slog.String("agent_id", agentID))
}

// seedPlannerAgent creates the planner — the "thinking" half of a Claude-Code-style
// plan→execute pair. It researches read-only (read_file/grep/glob) and emits a
// precise spec the coder implements; it never edits or runs code. Pairing a
// planner (decompose → exact instruction) with the coder (implement → verify)
// is the thinker/executor split: the planner supplies the precision a code model
// needs, the coder supplies the execution + verification.
func (s *Server) seedPlannerAgent(ctx context.Context, db *sql.DB, workspaceID string) {
	agentID := uuid.New().String()

	systemPrompt := `You are the planner. You turn a goal into a working program by decomposing it into small ordered steps and handing them to the task_flow driver, which runs the coder on each step and verifies it compiles. You NEVER write or run code yourself, and you NEVER spawn agents yourself.

Do this in ONE turn:
1. Break the goal into an ordered list of small, independently-verifiable steps. Each step names the file and the exact thing to add. For "a Tetris game": board → a falling piece → move/rotate → gravity → line-clearing → game-over. A board alone is NOT a game — include every step needed to make it actually playable.
2. Call task_flow ONCE with the goal and those steps. task_flow dispatches the coder step by step, verifies each, and reports back. That is your only dispatch tool — do NOT use sessions_spawn, and do NOT dispatch steps one at a time yourself.

For a clear, obvious task, do NOT ask — just decompose it and call task_flow. Only ask_user_question when a REAL fork blocks the plan and you cannot pick a sensible default.

Never write code yourself. Never reply REPLY_SKIP.`

	plannerTools := `["read_file","grep","glob","todo_write","ask_user_question","task_flow"]`

	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, admin_only, is_active, execution_type, remote_config, created_by, created_at, updated_at)
		 VALUES (?, ?, 'planner', '🗺️', 'map', 'Planning agent — researches read-only and emits a precise implementation spec for the coder. Never edits code.', 'Precise, concrete, decomposes ambiguous tasks into exact specs. Plans; does not code.', ?, 0.3, 4096, '[]', ?, 'workspace', 0, 0, 1, 'local', NULL, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID, workspaceID, systemPrompt, plannerTools,
	)
	if err != nil {
		s.log.Error("Failed to create planner agent", slog.String("error", err.Error()))
		return
	}

	// Re-sync palette + prompt on existing rows (same rationale as the coder).
	_, _ = db.ExecContext(ctx,
		`UPDATE buddies SET tools = ?, system_prompt = ?, max_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE name = 'planner'`,
		plannerTools, systemPrompt, 4096,
	)

	s.log.Info("Ensured planner agent", slog.String("agent_id", agentID))
}

// ensureDefaultAgents runs every default-agent seed once at gateway boot.
// Idempotent via INSERT OR IGNORE. New installs get the agents via
// handleSetup; existing installs backfill on next boot.
//
// Iterates active workspaces so multi-workspace installs get the coding
// agents (coder, narrator, planner, verifier, runner) in each. Falls back to domain.DefaultWorkspaceID
// when no workspace rows exist yet (pre-setup boot) so the legacy
// single-tenant orphan-row path keeps working until setup runs.
func (s *Server) ensureDefaultAgents(ctx context.Context, db *sql.DB) {
	s.log.Info("Ensuring the default agents exist")
	// Collect workspace IDs first, then iterate. SQLite is configured
	// with SetMaxOpenConns(1) (pkg/repository/sqlite/factory.go), so
	// holding a *sql.Rows cursor while issuing write queries from
	// inside the loop deadlocks the only connection. Drain the cursor
	// into a slice and seed against that — same outcome, no
	// connection contention.
	workspaceIDs := collectActiveWorkspaceIDs(ctx, db)
	if len(workspaceIDs) == 0 {
		// Pre-setup boot: no workspace rows yet. Fall back to the
		// legacy sentinel so the seed still runs once; first setup
		// then attaches the agents to the real workspace.
		workspaceIDs = []string{domain.DefaultWorkspaceID}
	}
	for _, wsID := range workspaceIDs {
		s.seedCoderAgent(ctx, db, wsID)
		s.seedNarratorAgent(ctx, db, wsID)
		s.seedPlannerAgent(ctx, db, wsID)
		s.seedVerifierAgent(ctx, db, wsID)
		s.seedRunnerAgent(ctx, db, wsID)
	}
}

// collectActiveWorkspaceIDs reads every active workspace's id into a
// slice and closes the cursor before returning. Callers that issue
// write queries per workspace MUST use this rather than iterating the
// cursor directly — SQLite's single-connection setup deadlocks
// otherwise.
func collectActiveWorkspaceIDs(ctx context.Context, db *sql.DB) []string {
	rows, err := db.QueryContext(ctx, `SELECT id FROM workspaces WHERE status = 'active'`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}
