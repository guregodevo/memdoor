# Plan — team-solve-v2-2026-11-02T203328

## Task

Rename `ReadGGUFMetadata` to `ParseGGUFMetadata` everywhere in the
llamafit repository: the function, every caller, the tests and their
names, and the comments that name it. `go build ./...` and
`go test ./...` must still pass.

## Repository survey (as of planning)

- Module: `github.com/guregodevo/llamafit` (go 1.24.1), Go library +
  one CLI (`cmd/llamafit`). Package at repo root.
- Definition: `gguf.go:311-312` — comment `// ReadGGUFMetadata reads
  model parameters from a GGUF file header.` and
  `func ReadGGUFMetadata(path string) (*GGUFMetadata, error)`.
- Callers: `server.go:215` (`meta, err := ReadGGUFMetadata(cfg.ModelPath)`)
  and `cmd/llamafit/main.go:62` (`llamafit.ReadGGUFMetadata(resolvedPath)`).
- Tests: `gguf_test.go` — `TestReadGGUFMetadata` (:17), its error string
  `"ReadGGUFMetadata: %v"` (:22), `TestReadGGUFMetadata_Gemma4Fields`
  (:53 comment, :55 func, :60 call, `ReadGGUFMetadata:` in Fatalf),
  `TestReadGGUFMetadata_InvalidFile` (:111-112). The type
  `GGUFMetadata` is NOT to be renamed.
- README.md mentions GGUF parsing generally but does not name the
  identifier; only touch it if a check finds the literal
  `ReadGGUFMetadata` there (current grep: no match).
- Baseline: `go test ./...` PASS on the untouched tree (0.334s, root
  package; cmd/llamafit has no test files).

## Decomposition

1. Enumerate every occurrence of `ReadGGUFMetadata` (definition,
   callers, test names, error strings, comments) — done above.
2. Rename in `gguf.go` (comment + func), `server.go`, `cmd/llamafit/main.go`,
   `gguf_test.go` (test funcs, Fatalf strings, comment on line 53).
3. Re-grep to prove zero remaining occurrences; also grep for
   `ParseGGUFMetadata` to list every new occurrence and confirm it is
   only the intended ones.
4. Run `go build ./...` and `go test ./...`; fix anything the rename
   broke (there should be nothing — it is a pure identifier rename).
5. Reviewer verifies scope: no unrelated lines, no new files,
   go.mod/go.sum untouched.

## Candidate approaches

### Proposal A — targeted sed/patch per file (recommended)

Apply a mechanical rename per known file: `sed -i '' 's/ReadGGUFMetadata/ParseGGUFMetadata/g'`
on gguf.go, server.go, cmd/llamafit/main.go, gguf_test.go (or an
apply_patch with the same hunks), then verify.

Tradeoffs: fastest and safest because the identifier is distinctive
(`GGUFMetadata` substring must stay untouched — sed only replaces the
full token, which is fine since `ReadGGUFMetadata` never overlaps
`GGUFMetadata`). Risk is minimal; no dependency on tooling beyond sed.

### Proposal B — gopls / gofmt-aware rename tooling

Use `gopls` rename or `gofmt -r 'ReadGGUFMetadata -> ParseGGUFMetadata'`
to get an AST-safe rename.

Tradeoffs: safest semantically, but heavier: gopls may not be installed,
`gofmt -r` rewrites formatting of every touched file and can produce
large diffs (violating "no unrelated line changed"), and `gofmt -r`
does not rewrite string literals or comments — which the task requires
(test names in Fatalf messages and the doc comment). Would need manual
string/comment fixes anyway. Overkill for a single unique identifier.

### Proposal C — manual line-by-line edit

Hand-edit each of the ~10 sites with apply_patch.

Tradeoffs: full control and smallest reviewable diff, but slow and
error-prone (easy to miss the comment on gguf.go:311 or a Fatalf
string); adds no benefit over the mechanical replace since every
occurrence should change anyway.

## Instructions for independent ideators

Work only from task.md and this repo; do not assume what other roles
decided. Propose the smallest correct set of edits that renames the
identifier everywhere the Prompt requires (function, callers, tests and
their names, comments) while leaving the `GGUFMetadata` type name and
every unrelated line untouched. Insert your proposal into notes.md at
your marker (`<!-- proposal-a -->` or `<!-- proposal-b -->`) with a small
patch — never overwrite other entries. Include: exact files, how you
handle strings/comments, and how you will verify (build, test, grep).

## Instructions for the solver

- The rename is mechanical: replace the exact token `ReadGGUFMetadata`
  with `ParseGGUFMetadata` in gguf.go (comment + definition), server.go,
  cmd/llamafit/main.go, gguf_test.go (test function names, Fatalf format
  strings, comments). Do NOT rename `GGUFMetadata` — the sed pattern
  only matches the full old identifier.
- Do not touch anything outside Scope in task.md.
- Verify: `grep -rn ReadGGUFMetadata --include='*.go' .` → no matches;
  `go build ./...` exits 0; `go test ./...` exits 0.
- Record what you did under "Solver decision" in notes.md.

## Instructions for the reviewer

- Confirm every occurrence of the old identifier in Go sources is gone
  and that test names now read `TestParseGGUFMetadata*`.
- Confirm comments naming the function were updated (gguf.go doc
  comment, gguf_test.go line-53 comment) — a code-only rename is
  incomplete per the Prompt.
- Confirm scope: only the four files (plus README only if it named the
  identifier, which it does not); no new files; go.mod/go.sum unchanged;
  no unrelated line changed.
- Run `go build ./...` and `go test ./...` yourself; both must exit 0.
- Write findings under "Review" in notes.md; the repair role addresses
  them under "Repair".
