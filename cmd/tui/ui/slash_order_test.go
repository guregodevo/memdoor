package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The dropdown lists the built-in commands in the help table's order, /w
// first and /help last, so / and /help say the same thing in the same order.
func TestDropdownFollowsTheHelpTable(t *testing.T) {
	cmds := loadSlashCommands("coder")
	var builtIn []string
	for _, c := range cmds {
		if strings.ContainsAny(c, " :") {
			continue // "/w <name>" runs and "/skill:<name>" come from the project
		}
		builtIn = append(builtIn, c)
	}
	if builtIn[0] != "/w" {
		t.Fatalf("first built-in is %q, want /w", builtIn[0])
	}
	var fromTable []string
	for _, h := range slashHelp {
		if strings.ContainsAny(h.usage, " <:") {
			continue // /skill:<name>, /<server>:<prompt>: not one command
		}
		fromTable = append(fromTable, h.usage)
	}
	if strings.Join(builtIn, " ") != strings.Join(fromTable, " ") {
		t.Fatalf("dropdown:\n%s\ntable:\n%s", strings.Join(builtIn, " "), strings.Join(fromTable, " "))
	}
}

// Every command of the table is in the CLI reference's "In the window"
// section, so the docs cannot drift from the window (the maintainer,
// 2026-11-06: "make sure they are documented in docs").
func TestEverySlashCommandIsInTheCLIReference(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "reference", "CLI.md"))
	if err != nil {
		t.Skip("no docs beside this package")
	}
	doc := string(b)
	i := strings.Index(doc, "## In the window")
	if i < 0 {
		t.Fatal("docs/reference/CLI.md has no \"## In the window\" section")
	}
	section := doc[i:]
	for _, h := range slashHelp {
		u := h.usage
		if u == "/exit" || u == "/quit" {
			u = "/exit or /quit" // one row for the same door
		}
		if !strings.Contains(section, "| `"+u+"` |") {
			t.Errorf("%s is not in docs/reference/CLI.md's In the window table", h.usage)
		}
	}
}
