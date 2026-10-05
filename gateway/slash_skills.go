package gateway

import (
	"regexp"

	"memdoor/tools"
)

// Slash skills: a user message of the form "/skill:name [args...]" invokes
// the named skill DETERMINISTICALLY — the harness injects the skill's content
// as the turn's task instead of asking the model to call the skill tool
// (which a small model forgets or paraphrases). The same resolution chain as
// the tool (project .agents/skills → managed ~/.memdoor/skills → embedded)
// means a skill the coder wrote for itself is immediately a user-invocable
// command.
//
// The explicit "skill:" namespace keeps skills from colliding with built-in
// or custom slash commands and makes the dropdown self-documenting; anything
// NOT matching the prefix passes through untouched, so the transform is
// strictly additive.
//
// A leading "@agent " mention is tolerated and dropped: the TUI prefixes every
// dispatch with the target mention ("@coder /skill:name"), and routing has
// already happened by the time ProcessMessage sees the text.
var slashSkillRe = regexp.MustCompile(`(?s)^(?:@\S+[ \t]+)?/skill:([a-zA-Z0-9][a-zA-Z0-9._-]*)(?:[ \t\n]+(.+))?$`)

// expandSlashSkill rewrites "/skill:name args" into the skill's content plus
// the user's arguments. Returns (expanded, true) only when the name resolves.
func expandSlashSkill(msg, projectDir string) (string, bool) {
	m := slashSkillRe.FindStringSubmatch(msg)
	if m == nil {
		return "", false
	}
	content, ok := tools.LookupSkill(m[1], projectDir)
	if !ok {
		return "", false
	}
	expanded := "# Skill: " + m[1] + "\n\n" + content + "\n\nFollow this workflow now."
	if args := m[2]; args != "" {
		expanded += "\n\n## User input\n" + args
	}
	return expanded, true
}
