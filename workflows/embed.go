// Package workflows embeds the built-in workflows into the binary. The
// gateway seeds them to the person's library (~/.memdoor/workflows) at boot,
// so `memdoor workflow run pr-review` works in a repository that has no
// .memdoor of its own — the GitHub action's first job included. A project's
// own workflow of the same name wins (workflow.Find), and an edited library
// copy is never overwritten (gateway.SeedEmbeddedWorkflows).
package workflows

import "embed"

//go:embed */README.md */steps/*.yaml
var FS embed.FS
