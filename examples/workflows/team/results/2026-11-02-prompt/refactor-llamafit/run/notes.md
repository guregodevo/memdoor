# Notes — team-solve-v2-2026-11-02T203328

Run identifier: `2026-11-02T203328`

Ledger for the rename task (`ReadGGUFMetadata` → `ParseGGUFMetadata`).
Every role appends at its own marker with a small patch. Never
overwrite another role's section; never rewrite this file wholesale.

Ground rules for all roles:

- Read task.md and plan.md first; they are the contract.
- Work only inside the Scope listed in task.md.
- Do not touch other sections of this ledger.
- Record facts (commands run, outputs, file:line), not intentions.

Sections in this ledger:

1. Proposal A — first independent ideator's approach.
2. Proposal B — second independent ideator's approach.
3. Solver decision — what the solver actually changed and verified.
4. Review — reviewer findings, pass or fail per check.
5. Repair — fixes applied in response to Review, if any.

Baseline recorded by the planner: `go test ./...` passes on the
untouched tree (root package ok, cmd/llamafit has no test files).
Occurrences of the old identifier at planning time: gguf.go:311-312
(comment + definition), server.go:215 (caller), cmd/llamafit/main.go:62
(caller), gguf_test.go:17, 20, 22, 53, 55, 58, 60, 111, 112 (tests,
error strings, comments). README.md does not name the identifier.

---

## Proposal A

<br>

**Proposal A — exact-token mechanical replace, per-site `apply_patch` (no
tooling, no collateral edits).**

I re-surveyed the tree from task.md only. The state of the ground truth:

- `go build ./...` and `go test ./...` both exit 0 on the untouched tree
  (cached `ok github.com/guregodevo/llamafit`; `cmd/llamafit` has no
  test files). Baseline in task.md confirmed.
- `grep -rn ReadGGUFMetadata . --include='*.go'` gives exactly 13
  occurrences, in exactly 4 files: gguf.go (2), server.go (1),
  cmd/llamafit/main.go (1), gguf_test.go (9). `grep -rn
  ParseGGUFMetadata . --include='*.go'` gives 0 — no stale pre-existing
  use of the new name to collide with.
- README.md contains no `ReadGGUFMetadata` (only prose "Parses GGUF
  metadata" at README.md:28, which names the activity, not the Go
  identifier). Per the "if and only if" clause in Scope, README.md is
  therefore not touched. No occurrence outside the four Go files
  (Makefile, go.mod, go.sum, .github/, and every other .go file checked:
  all clean).

The 13 sites, verbatim (all verified by reading, not by trusting
plan.md):

- gguf.go:311 — doc comment `// ReadGGUFMetadata reads model parameters
  from a GGUF file header.`
- gguf.go:312 — `func ReadGGUFMetadata(path string) (*GGUFMetadata,
  error) {` (type `GGUFMetadata` in the same line is NOT renamed; the
  replace token `ReadGGUFMetadata` does not overlap the substring
  `GGUFMetadata`, so a whole-token replace touches only the func name)
- server.go:215 — `meta, err := ReadGGUFMetadata(cfg.ModelPath)` (its
  comment on :214 says "Read GGUF metadata", names the activity — not
  updated)
- cmd/llamafit/main.go:62 — `meta, err :=
  llamafit.ReadGGUFMetadata(resolvedPath)` (call through the package
  selector; same rename, no other edit needed)
- gguf_test.go:17 — `func TestReadGGUFMetadata(t *testing.T) {`
- gguf_test.go:20 — call `ReadGGUFMetadata(path)`
- gguf_test.go:22 — `t.Fatalf("ReadGGUFMetadata: %v", err)` (string
  literal)
- gguf_test.go:53 — comment `// TestReadGGUFMetadata_Gemma4Fields
  verifies the sliding window fields`
- gguf_test.go:55 — `func TestReadGGUFMetadata_Gemma4Fields(t
  *testing.T) {`
- gguf_test.go:58 — call `ReadGGUFMetadata(path)`
- gguf_test.go:60 — `t.Fatalf("ReadGGUFMetadata: %v", err)` (string)
- gguf_test.go:111 — `func TestReadGGUFMetadata_InvalidFile(t
  *testing.T) {`
- gguf_test.go:112 — call `ReadGGUFMetadata("/nonexistent.gguf")`

This is a pure identifier rename, so there is no pseudocode, no
invariant, and no complexity to state — the contract is: the resulting
tree's byte diff touches exactly the 13 lines above (each line with the
token substituted, nothing else on the line moved) and no file outside
the four.

**The approach, as one command per file (the whole deliverable):**

```
sed -i '' 's/ReadGGUFMetadata/ParseGGUFMetadata/g' gguf.go
sed -i '' 's/ReadGGUFMetadata/ParseGGUFMetadata/g' server.go
sed -i '' 's/ReadGGUFMetadata/ParseGGUFMetadata/g' cmd/llamafit/main.go
sed -i '' 's/ReadGGUFMetadata/ParseGGUFMetadata/g' gguf_test.go
```

or, equivalently and as the reviewable form I recommend, one
`apply_patch` per file with the same hunks (showing the exact 13 lines
with `-`/`+` twins). Since every occurrence of the old token in these
files is an occurrence we intend to rename, "replace every occurrence in
file" is identical to "replace only the intended sites" — there is no
collateral-edit problem to avoid, so the mechanical command is exactly as
reviewable as hand-editing. That is why Proposal C (hand-edits) and
Proposal B (gopls/gofmt -r) have no edge over A here:

- `gofmt -r` rewrites only identifiers, not comments or string literals
  (which the task requires: the doc comment gguf.go:311, the line-53
  comment, the two Fatalf strings) and it rewrites formatting of every
  touched file — gofmt -l currently flags cmd/llamafit/main.go,
  gguf_test.go, picker_test.go as NOT gofmt-clean (i.e. their current
  formatting would be rewritten by any gofmt pass, which is exactly the
  "no unrelated line changed" violation, and picker_test.go is out of
  scope). A gofmt -r / gopls route would need manual string/comment
  patches on top and would still drag in gofmt's formatting churn.
- `gopls` rename is AST-safe but not installed/verified on this tree,
  drags gofmt formatting with it for the same reason, and the same
  manual string/comment patches. It only helps when identifiers are
  semantically ambiguous, which they are not.

The whole-token pattern `ReadGGUFMetadata` cannot accidentally rename
`GGUFMetadata` (no overlap) and cannot hit a partial match anywhere: the
13 `grep -c` counts above are the entire set.

**Why the rename cannot break build/test:**

The old identifier appears exactly once in the package (definition at
gguf.go:312) and its only uses are the 12 remaining sites, all inside
Scope. It is a Go exported function renamed mechanically; there are no
reflection or string-based lookups of the function name, no interface
conformance that names it, and no assembly. The type `GGUFMetadata` is
untouched, so all signatures, callers, and test bodies type-check
unchanged. The renames of test functions (`TestReadGGUFMetadata`,
`TestReadGGUFMetadata_Gemma4Fields`, `TestReadGGUFMetadata_InvalidFile`)
are just Go function renames — `go test` discovers tests by `Test`
prefix, which all three new names still satisfy. `go vet`-level static
analysis would be no more than type-checking, which passes.

**Done-when proof, step by step:**

1. Run the 4 sed commands (or the equivalent apply_patch hunks).
2. `grep -rn ReadGGUFMetadata . --include='*.go'` → expect empty output,
   exit status 1 (no match). This proves the Reviewer check.
3. `grep -rn ParseGGUFMetadata . --include='*.go'` → expect exactly the
   same 13 lines with the new token. This proves "every replacement is
   ParseGGUFMetadata".
4. `git diff --stat` → expect: `gguf.go | 4 +--`, `server.go | 2 +-`,
   `cmd/llamafit/main.go | 2 +-`, `gguf_test.go | 18 +++++-----` and
   nothing else. `git diff` hunks should show only the 13 lines with
   twins. This proves "no unrelated line changed" — no gofmt reformat,
   no go.mod/go.sum change, no new files.
5. `go build ./...` → exit 0. This proves the Judge check (build).
6. `go test ./...` → exit 0, `ok github.com/guregodevo/llamafit`. This
   proves the Judge check (test).
7. Optional: `grep -rn ReadGGUFMetadata README.md` → empty (was empty
   before, must stay empty: Scope only touches README if it named the
   identifier, which it does not).

**What could break, and how the Check would show it:**

- A missed site (e.g. the gguf.go:311 comment): step 2 would show a
  nonempty `grep` output, immediately localizing the site with a
  `file:line` — fix by re-running the same sed on that file.
- A collateral edit (gofmt churn from the wrong tool): step 4's `git
  diff --stat` would show an oversized diff or a fifth file — revert and
  fall back to the sed/hunks.
- A stale doc-comment or error string: the Prompt explicitly requires
  comments/strings; a pure identifier rename with `gofmt -r` would have
  left gguf.go:311, gguf_test.go:53, and the two Fatalf strings saying
  the old name — the Reviewer check in step 2 catches them because grep
  matches comments and strings too.

**Edge cases:**

- README.md: not touched (prose "Parses GGUF metadata" is not the Go
  identifier).
- .memdoor/ run files (task.md, plan.md, notes.md, prompt.md) contain
  the token; they are explicitly "never part of the work and never
  committed" per the run contract, and `--include='*.go'` on the check
  already excludes them. Even a raw `grep -rn ReadGGUFMetadata .` at
  repo root would only see them under .memdoor/, which the task's own
  rules exclude from the deliverable tree.
- Concurrent Proposal B: I only insert at the unique `<!-- proposal-a
  -->` marker; the other ideator writes at `<!-- proposal-b -->`. No
  other section is read, written, or rewritten.

<!-- proposal-a -->









---

## Proposal B

### B — compiler-guided rename: the type checker as the enumeration oracle

**Family:** verification-driven, not search-driven. The obvious approach
(plan Proposal A) enumerates occurrences by text matching first, replaces
them, then re-greps to prove completeness. This proposal inverts the
direction: rename the *definition* first, then let `go build ./...` /
test compilation fail at every remaining code reference — the compiler,
not a grep, is the authoritative list of call sites. A final grep sweep
then handles the non-semantic occurrences (strings, comments, test
names) that the type checker cannot see.

**Ground truth inventory (verified this session):** 13 occurrences in
4 files. gguf.go:311 (doc comment), gguf.go:312 (definition);
server.go:215; cmd/llamafit/main.go:62 (`llamafit.ReadGGUFMetadata`);
gguf_test.go:17, 55, 111 (test function names), gguf_test.go:19, 58,
112 (calls), gguf_test.go:22, 60 (`t.Fatalf("ReadGGUFMetadata: %v",
err)`), gguf_test.go:53 (comment naming the test). README.md: 0
matches, so it stays untouched per Scope.

**Edits, in order:**

1. `gguf.go` — 2 edits: doc comment `// ReadGGUFMetadata reads model
   parameters…` → `// ParseGGUFMetadata reads…`, and
   `func ReadGGUFMetadata(path string) (*GGUFMetadata, error)` →
   `func ParseGGUFMetadata(...)`. Nothing else in the file changes;
   `GGUFMetadata` (the type) never matches because only the exact old
   identifier is edited.
2. `go build ./...` — expect exactly 2 `undefined` errors, one at
   server.go:215 and one at cmd/llamafit/main.go:62 (the package path
   of the definition is compiled; the callers are not yet). If any
   *other* file appears in that error list, the plan's inventory was
   stale — that is exactly the failure mode this approach is built to
   catch, and the fix is to rename those sites too.
3. Rename the 2 flagged call sites (`meta, err :=
   ReadGGUFMetadata(cfg.ModelPath)` in server.go;
   `llamafit.ReadGGUFMetadata(resolvedPath)` in main.go — note the
   package qualifier stays `llamafit.` because the package name is
   unchanged). Re-run `go build ./...` → must exit 0.
4. `go test -run '^$' ./...` (compiles tests without running them) —
   expect 3 `undefined` errors at gguf_test.go:19, :58, :112. Rename
   those calls. Re-run → exit 0.
5. Grep sweep for what the compiler cannot flag:
   `grep -rn ReadGGUFMetadata --include='*.go' .` — the remaining hits
   are exactly the 6 cosmetic sites: test names :17/:55/:111, Fatalf
   format strings :22/:60, comments :311 (already done in step 1) and
   :53. Rename each by hand with apply_patch. Also grep README.md →
   confirmed 0 matches, no edit.
6. Final verify (the Check): `go build ./...` and `go test ./...` both
   exit 0; `grep -rn ReadGGUFMetadata .` over Go sources → 0 hits;
   `grep -rn ParseGGUFMetadata` → exactly the 13 expected hits; no
   other line in the 4 files differs (verify with `git diff --stat`).

**Invariant:** after step 3, every *code* reference in the module
resolves (compiler proves it); after step 5, every *textual* occurrence
is renamed (grep proves it); together they cover the disjoint union of
all 13 sites, since a Go occurrence of an identifier is either a
resolved reference (compile-time visible) or text only in a
string/comment/identifier-name (grep-visible). There is no third
category in Go — reflection-by-name does not occur in this repo
(confirmed: no `ReadGGUFMetadata` inside any string that is passed to
reflect or plugin machinery; the only strings are error messages).

**What could break, and how the Check shows it:**
- A call site in a file the plan did not survey → `go build ./...`
  fails with a filename/line in step 2, which is a *better* signal than
  sed's silence.
- A missed string/comment → invisible to the build, but the Done-when
  grep in step 6 returns it, so Reviewer catches it; this is the same
  net safety as sed (both ultimately rely on the final grep for
  cosmetics).
- Test *names* (`TestReadGGUFMetadata*`) never break compilation —
  they are independent identifiers — so they must come from the grep
  step, not the compiler. This is the one place the compiler-guided
  approach is strictly weaker than blind replace; the step-5 sweep
  covers it.
- No risk to `GGUFMetadata`: neither the compiler nor the hand edits
  ever touch that token.

**Complexity vs. the obvious sed pass:** ~4 edit rounds and ~4
build/test runs vs. one pass; total extra wall time is seconds, and the
Go toolchain is guaranteed present because it *is* the Check. Cost is
more steps; benefit is that the code-reference enumeration cannot be
wrong even if the planning inventory is stale (concurrent edits, a
caller added outside the surveyed files). On this repo both approaches
converge to the same 13-line diff; choose this one if you distrust the
inventory, sed if you trust it. Scope discipline is identical: only
gguf.go, server.go, cmd/llamafit/main.go, gguf_test.go change; README,
go.mod, go.sum untouched; no new files.

<!-- proposal-b -->









---

## Solver decision

**Approach chosen: Proposal A** (exact-token mechanical replace), with
Proposal B's compiler-driven verification used implicitly through the
final build/test run. Rationale: both independent ideators arrived at
the same 13-site inventory and I confirmed it myself before editing —
`grep -rn ReadGGUFMetadata --include='*.go' .` returned exactly 13 hits
in exactly the 4 in-scope files (gguf.go:311-312, server.go:215,
cmd/llamafit/main.go:62, gguf_test.go:17,20,22,53,55,58,60,111,112),
and `ParseGGUFMetadata` had 0 pre-existing hits. Since every occurrence
of the old token in these files is intended to change, the mechanical
replace is exactly as safe as hand-editing each site, and unlike
`gofmt -r`/gopls it also rewrites the string literals and comments the
Prompt requires (gguf.go:311 doc comment, gguf_test.go:53 comment, the
two `Fatalf` strings) without touching formatting anywhere.
`GGUFMetadata` cannot be affected: the token `ReadGGUFMetadata` has no
overlap with the substring `GGUFMetadata`.

**What was done (facts):**

- Pre-checks: README.md grep → no match (exit 1), so README stays
  untouched per the "if and only if" Scope clause. Pre-existing
  inventory: 13 sites in 4 files, as listed above.
- Edit: `sed -i '' 's/ReadGGUFMetadata/ParseGGUFMetadata/g'`
  on `gguf.go`, `server.go`, `cmd/llamafit/main.go`, `gguf_test.go`.
- Post-checks, all passing:
  1. `grep -rn ReadGGUFMetadata --include='*.go' .` → empty (exit 1):
     zero remaining occurrences in Go sources.
  2. `grep -rn ParseGGUFMetadata --include='*.go' .` → exactly the same
     13 lines with the new token (2 comment/definition lines in gguf.go,
     1 caller in server.go, 1 caller in cmd/llamafit/main.go, 9 in
     gguf_test.go: 3 test function names, 3 calls, 2 Fatalf strings,
     1 comment naming a test).
  3. `git diff --stat` → only the 4 in-scope files, 13 insertions /
     13 deletions; full `git diff` shows only the 13 lines with
     `-`/`+` twins, nothing else moved. `git status --porcelain` →
     those 4 files modified, no new files, go.mod/go.sum untouched
     (the only untracked path is `.memdoor/`, the run directory, which
     is never part of the work).
  4. `go build ./...` → exit 0 (BUILD_OK).
  5. `go test ./...` → exit 0: `ok github.com/guregodevo/llamafit
     0.400s`, `cmd/llamafit [no test files]`.

**Result:** Done. The rename is complete everywhere the Prompt requires
(definition, both callers, all three test functions and their names,
the error strings, and the comments naming it), `GGUFMetadata` and all
out-of-scope files are untouched, and both Check commands exit 0.

<!-- solver-decision -->









---

## Review

**Review verdict: PASS.** Verified against the actual working tree, with the checks re-run
fresh rather than trusting test-status.txt:

- `grep -rn ReadGGUFMetadata --include='*.go' .` → exit 1, zero hits in Go sources.
- `grep -rn ParseGGUFMetadata --include='*.go' .` → exactly 13 sites, all in the 4
  in-scope files (gguf.go:311 doc comment + :312 definition, server.go:215,
  cmd/llamafit/main.go:62, gguf_test.go:17,20,22,53,55,58,60,111,112 — test names, both
  Fatalf strings, and the line-53 comment all renamed).
- `git diff -U0`: 26 body lines, every one containing the token; 0 changed lines without
  it — no unrelated line moved. `git status --porcelain`: exactly the 4 in-scope files
  modified + .memdoor/ untracked; go.mod/go.sum unchanged; no new files.
- README.md contains zero hits of the identifier, so leaving it untouched is correct per
  the Scope's "if and only if" clause.
- Checks run fresh on the modified tree: `go build ./...` exit 0;
  `go clean -testcache && go test -count=1 ./...` exit 0 (`ok
  github.com/guregodevo/llamafit 0.192s`, cmd/llamafit no test files). The run's
  test-status.txt of 0 is consistent.
- gofmt -l flags cmd/llamafit/main.go and gguf_test.go, but stashing the diff proved both
  were already not gofmt-clean on the untouched base (which also flags picker_test.go) —
  pre-existing formatting, not introduced by this change, not a Done-when criterion.

No repair round needed; check status is 0. Full findings in `review.md`.

<!-- review -->









---

## Repair

<!-- repair -->









End of ledger.
