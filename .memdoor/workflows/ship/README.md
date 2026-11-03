# ship

Checks what is committed, then pushes it: build, vet, the Go suite, the unused-code check and the web type check run in parallel; a red one blocks the rest. Then `push`. The workflow never commits: commit first, then `/workflow:ship`; a dirty tree fails at `clean`. Needs the staticcheck binary at `$HOME/go/bin`. Leaves nothing in the project but the run's outputs. A run is for a commit: `--partition $(git rev-parse --short HEAD)` resumes a stopped ship of the same commit.
