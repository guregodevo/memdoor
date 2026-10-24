package cmd

import (
	"github.com/spf13/cobra"
)

// Groups + assignment for the top-level command list.
//
// Without grouping, `memdoor --help` is a flat wall of thirty commands,
// alphabetically sorted. Cobra supports command groups; they are wired here
// in one place so per-command init() functions do not each need to know
// which group they belong to.
//
// THE GROUPS SAY WHAT THE PRODUCT IS (Greg, 2026-10-23: "the deamon that owns
// the work is the way to go", "repackage and realign commands product to
// option 1"). The gateway is the daemon that owns the work; the window, a
// headless run, a resumed conversation, the phone and the editor are doors
// onto it; workflows and agents are what it runs; providers and models are
// what it runs them on; the workspace and the account are whose it is.
// Until 2026-10-23 the headings were "Workspace & collaboration", "Servers &
// interfaces", "Config, auth & diagnostics", "Advanced & connectors": the
// layout of a platform, not of a daemon.

const (
	groupDaemon   = "daemon"
	groupDoors    = "doors"
	groupWork     = "work"
	groupModels   = "models"
	groupAccount  = "account"
	groupBinary   = "binary"
	groupAdvanced = "advanced"
)

// commandGroupAssignments maps each top-level command name to a group ID.
// Commands not in this map render under cobra's default "Additional
// Commands" section, a soft fallback so adding a command without updating
// this map does not break `--help`.
var commandGroupAssignments = map[string]string{
	// The daemon that owns the work: start it, run it as a service, read it.
	"gateway": groupDaemon,
	"doctor":  groupDaemon,
	"logs":    groupDaemon,
	"health":  groupDaemon,

	// Doors onto it. The bare name is the window; these are the others.
	"tui":           groupDoors,
	"run":           groupDoors,
	"resume":        groupDoors,
	"remote":        groupDoors,
	"peer":          groupDoors,
	"peers":         groupDoors,
	"conversations": groupDoors,
	"acp":           groupDoors,
	"cron":          groupDoors,

	// What it runs.
	"workflow": groupWork,
	"agent":    groupWork,
	"guards":   groupWork,
	"mcp":      groupWork,

	// What it runs them on: providers, their keys, their models, the bill.
	"connect":   groupModels,
	"providers": groupModels,
	"model":     groupModels,
	"meter":     groupModels,
	"savings":   groupModels,

	// Whose it is: the workspace on the gateway, the memdoor.ai account.
	"workspace":  groupAccount,
	"use":        groupAccount, // shortcut for `workspace use <slug>`
	"which":      groupAccount, // shortcut for `workspace which`
	"workspaces": groupAccount, // shortcut for `workspace list`
	"setup":      groupAccount,
	"account":    groupAccount,
	"login":      groupAccount,
	"logout":     groupAccount,
	"auth":       groupAccount,
	"users":      groupAccount,
	"channels":   groupAccount,
	"messages":   groupAccount,
	"sessions":   groupAccount,
	"billing":    groupAccount,

	// This binary and its configuration.
	"config":    groupBinary,
	"version":   groupBinary,
	"upgrade":   groupBinary,
	"uninstall": groupBinary,

	// Connectors the agent reaches through tools.
	"chrome":   groupAdvanced,
	"pipeline": groupAdvanced,
}

// assignCommandGroups installs cobra.Group definitions on rootCmd and tags
// each registered subcommand with its GroupID. Called from Execute() so it
// runs after every per-command init() has registered against rootCmd.
func assignCommandGroups() {
	// NO EMPTY HEADINGS: a heading with nothing under it is one of the first
	// things a stranger reads. Hidden commands do not count as content.
	declared := []*cobra.Group{
		{ID: groupDaemon, Title: "The gateway, the daemon that owns the work:"},
		{ID: groupDoors, Title: "Doors onto it:"},
		{ID: groupWork, Title: "What it runs:"},
		{ID: groupModels, Title: "Providers and models it runs them on:"},
		{ID: groupAccount, Title: "Workspace and account:"},
		{ID: groupBinary, Title: "This binary:"},
		{ID: groupAdvanced, Title: "Connectors:"},
	}
	hidden := make(map[string]bool, len(notCodingCommands))
	for _, n := range notCodingCommands {
		hidden[n] = true
	}
	populated := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		if id, ok := commandGroupAssignments[c.Name()]; ok && !hidden[c.Name()] {
			populated[id] = true
		}
	}
	groups := []*cobra.Group{}
	for _, g := range declared {
		if populated[g.ID] {
			groups = append(groups, g)
		}
	}
	rootCmd.AddGroup(groups...)
	for _, c := range rootCmd.Commands() {
		if id, ok := commandGroupAssignments[c.Name()]; ok && populated[id] {
			// A group that was never declared would fail cobra's check; those
			// commands fall to "Additional Commands", where the hidden ones do
			// not appear at all.
			c.GroupID = id
		}
	}
}
