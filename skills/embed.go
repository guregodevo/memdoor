// Package skills embeds the built-in skill library into the binary, so skills
// resolve no matter which directory memdoor runs from (a gateway started outside
// the repo has no ./skills on disk). Disk copies still take precedence — a user
// can override any skill by dropping <name>.md into
// ~/.memdoor/workspace/skills — the embedded set is the always-available floor.
package skills

import "embed"

//go:embed *.md
var FS embed.FS
