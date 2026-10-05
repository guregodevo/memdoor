package gateway

import (
	"context"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

// TestAgentWorksOnCodebase locks the contract behind the codebase-agent fix:
// coder and planner palettes are codebase agents (identity-only prompt + file
// tools resolved against the real cwd), while conversational buddies are
// not. The sandbox-file tools routed to real fs are exactly
// read_file/write_file/list_files — not bash/edit_file/grep/glob, which already
// run on the real fs via their legacy Function.
func TestAgentWorksOnCodebase(t *testing.T) {
	ar := &AgentRuntime{} // agentWorksOnCodebase reads only from ctx

	withTools := func(tools ...string) context.Context {
		return context.WithValue(context.Background(), sharedctx.BuddyToolsKey, tools)
	}

	// Codebase agents — both must qualify.
	coder := withTools("bash", "read_file", "write_file", "edit_file", "grep", "glob", "todo_write")
	if !ar.agentWorksOnCodebase(coder) {
		t.Error("coder palette should be a codebase agent")
	}
	planner := withTools("read_file", "grep", "glob", "ask_user_question")
	if !ar.agentWorksOnCodebase(planner) {
		t.Error("planner palette (read-only codebase research) should be a codebase agent")
	}

	// Conversational buddies — none touch the filesystem tools.
	companion := withTools("web_search", "web_fetch", "sessions_spawn")
	if ar.agentWorksOnCodebase(companion) {
		t.Error("companion palette must NOT be a codebase agent")
	}
	chatter := withTools("web_search", "logs_query", "channels")
	if ar.agentWorksOnCodebase(chatter) {
		t.Error("a chat palette must NOT be a codebase agent")
	}
	if ar.agentWorksOnCodebase(context.Background()) {
		t.Error("no palette in ctx should not be a codebase agent")
	}

	// The real-fs override applies to the sandbox-aware file tools only.
	for _, name := range []string{"read_file", "write_file", "list_files"} {
		if !doerFilesystemTools[name] {
			t.Errorf("%q should route to real fs for a codebase agent", name)
		}
	}
	// bash/edit_file/grep/glob already hit the real fs via their legacy Function
	// — they must not be in the override set (there's no sandbox path to skip).
	for _, name := range []string{"bash", "edit_file", "grep", "glob"} {
		if doerFilesystemTools[name] {
			t.Errorf("%q must not be in the codebase-agent fs override set", name)
		}
	}
}

// TestPermissionModeFromContext locks the permission-mode context resolution: an
// unset mode is "default", and a set mode is read back. The coder's tools are never
// blocked by mode — the model + hard-to-fail tools do the work — so there is no
// tool-gating contract to test here.
func TestPermissionModeFromContext(t *testing.T) {
	if permissionModeFromContext(context.Background()) != permissionModeDefault {
		t.Fatal("unset mode should resolve to default")
	}
	plan := context.WithValue(context.Background(), sharedctx.PermissionModeKey, permissionModePlan)
	if permissionModeFromContext(plan) != permissionModePlan {
		t.Fatal("plan mode should be read back from ctx")
	}
}
