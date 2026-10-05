package cmd

import (
	"github.com/spf13/cobra"
)

// Groups + assignment for the top-level command list.
//
// Without grouping, `memdoor --help` is a flat wall of 25
// alphabetically-sorted commands — overwhelming for new users and
// hard for agents to navigate when picking the right verb. Cobra
// supports command groups out of the box; we wire them here in a
// single place so per-command init() functions don't each need to
// know which group they belong to.
//
// The grouping mirrors the mental model from AGENTS.md / docs/: a
// workspace+agent+channels are how teams collaborate, gateway+mcp are
// the surfaces, and the rest is config / diagnostics / less-common
// operational stuff.

const (
	groupWorkspace = "workspace"
	groupServer    = "server"
	groupSystem    = "system"
	groupAdvanced  = "advanced"
)

// commandGroupAssignments maps each top-level command name to a
// group ID. Commands not in this map render under cobra's default
// "Additional Commands" section — used as a soft fallback so
// adding a new command without updating this map doesn't break
// `--help`, it just leaves the new command ungrouped until
// somebody slots it in.
var commandGroupAssignments = map[string]string{
	// Workspaces, agents, channels — how teams use the product
	"workspace":  groupWorkspace,
	"use":        groupWorkspace, // top-level shortcut for `workspace use <slug>`
	"which":      groupWorkspace, // top-level shortcut for `workspace which` (PS1-safe with --bare)
	"workspaces": groupWorkspace, // top-level shortcut for `workspace list`
	"setup":      groupWorkspace, // first-time bootstrap; sits next to `workspace use`
	"account":    groupWorkspace, // the creator's sign-in: email + code, engine prepared behind it
	"login":      groupWorkspace, // the same sign-in under its short name
	"logout":     groupWorkspace, // signs out of memdoor.ai; the engine stays signed in
	"users":      groupWorkspace,
	"agent":      groupWorkspace,
	"channels":   groupWorkspace,
	"messages":   groupWorkspace,
	"sessions":   groupWorkspace,

	// Server processes + UIs people start to interact with the platform
	"gateway": groupServer,

	// Config, secrets, auth, diagnostics. setup belongs to
	// Workspace & collaboration since it's the first-time
	// bootstrap a new user runs right alongside `workspace use`.
	"config": groupSystem,
	"auth":   groupSystem,
	"doctor": groupSystem,
	"health": groupSystem,
	"logs":   groupSystem,

	// Less-common / specialized — chrome+pipeline are connectors,
	// cron is scheduling, rag is RAG infra
	"cron":     groupAdvanced,
	"pipeline": groupAdvanced,
	"chrome":   groupAdvanced,
	"mcp":      groupAdvanced, // the coder's MCP servers: add, sign in, state
}

// assignCommandGroups installs cobra.Group definitions on rootCmd
// and tags each registered subcommand with its GroupID. Called from
// Execute() so it runs after every per-command init() has registered
// against rootCmd.
func assignCommandGroups() {
	// NO EMPTY HEADINGS: a heading with nothing under it is one of the first
	// things a stranger reads.
	groups := []*cobra.Group{}
	groups = append(groups,
		&cobra.Group{ID: groupWorkspace, Title: "Workspace & collaboration:"},
		&cobra.Group{ID: groupServer, Title: "Servers & interfaces:"},
		&cobra.Group{ID: groupSystem, Title: "Config, auth & diagnostics:"},
		&cobra.Group{ID: groupAdvanced, Title: "Advanced & connectors:"},
	)
	rootCmd.AddGroup(groups...)
	for _, c := range rootCmd.Commands() {
		if id, ok := commandGroupAssignments[c.Name()]; ok {
			// A group that was never declared would fail cobra's check; those
			// commands fall to "Additional Commands", where the hidden ones do
			// not appear at all.
			c.GroupID = id
		}
	}
}
