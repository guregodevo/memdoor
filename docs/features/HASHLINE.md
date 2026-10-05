# Line-anchored edits ("hashline") — design

Status: design, 2026-09-29. Code follows this document; the default edit
format follows the measurement in §7, not this document.

## 1. Why

Edits are output tokens, the most expensive kind, and today's format
(Codex-style `apply_patch`: context hunks) spends them twice:

- **Re-typed code.** Of the bytes in 176 update patches in this machine's
  coder transcripts, 35% re-type code that already exists: context lines
  23%, removed lines 12% (added 57%, headers 8%).
- **Retries.** 60 of 314 `apply_patch` calls on the gateway in the last 30
  days failed (19%); the top reason is re-typed context that did not match
  the file ("failed to find expected line"). Each failure is a retry: the
  whole patch again, and another turn.

## 2. What the evidence says

No peer-reviewed comparison exists, and nothing was measured on our models.

| Source | Finding |
|---|---|
| The hashline author's benchmark (16 models, 180 tasks, self-run) | vs the patch format: +5 to +65 points (patch failed 46-51% for GLM-4.7, Grok 4). vs str_replace: small — GLM-4.7 +8.3, GLM-4.5 Air +0.4, Qwen Turbo −1.7, **DeepSeek V3.2 −8.3** (and +20% output tokens). Output tokens −20% on average, mostly from fewer retries. |
| edit-bench (independent, 3 models × 3 languages × 20 tasks) | replace matched or beat hashline in 5 of 9 cells; one model scored 0/20 until the tool names were changed, then 19/20. "The model matters more than the format." |
| Diff-XYZ (JetBrains, arXiv 2510.12487) | line numbers are scaffolding: removing them from hunk headers dropped a strong model from 0.82 to 0.02 exact match; explicit tags beat +/- for small models. |
| SWE-agent (NeurIPS 2024) | line-range edits with a lint gate: 18.0% vs 15.0% without the gate; 51.7% of trajectories had a failed edit, and recovery got less likely with each failure. |
| Aider | "GPT is terrible at working with source code line numbers" (2023-24 GPT-4 only); flexible matching cut edit errors 9×. |

What follows for us: leave the patch format as the default for non-OpenAI
models only after measuring; expect the gain from **fewer failed edits**
more than from shorter ones; guard line numbers with a little content;
reject rather than guess; show fresh numbers after every edit.

## 3. Reads

Every read the model can edit from shows the file's **tag** and numbered
lines:

```
[main.go#3F2A]
1:package main
2:
3:func Add(a, b int) int { return a + b }
```

- **Tag**: 4 hex digits of a 32-bit FNV-1a hash of the content, trailing
  whitespace of each line and `\r` ignored. One per file, not per line
  (per-line hashes cost 30-40% more input on reads; bare numbers ~15%).
- **Numbers**: `N:` with no padding (the judged-read format `%5d  ` spends
  spaces on every line).
- **Where**: `read_file` (whole, truncated, judged sections), `jread`
  sections. A judged read shows the tag once and its sections numbered.
- **Seen lines**: a line cut with `…` (judged reads cut long lines) or
  outside what was shown is unseen; an edit that replaces or deletes an
  unseen line is refused (§5).
- Every read records a **snapshot**: path, tag, the text, which lines were
  shown — in memory, per gateway, the last 256.

## 4. Edits

In `apply_patch` (same tool, same name — a model that knows it keeps
working), a section per file opened by the tag it was read at:

```
[main.go#3F2A]
replace 3-3 "func Add(":
+func Add(a, b int) int {
+	return a + b
+}
insert after 3:
+
+func Sub(a, b int) int { return a - b }
delete 7-9 "// old"
```

- `replace N-M "<start>":` then the new lines, each `+` and verbatim with
  its indentation; a lone `+` is a blank line.
- `insert before N:` / `insert after N:` then lines (`before 1`: the top;
  `after $`: the end).
- `delete N-M "<start>"`.
- **`"<start>"`** is the first characters of line N as read, indentation
  ignored — at least 3, or the whole line when shorter. It is what catches
  an off-by-one, which a tag alone cannot. It is the only existing code an
  edit re-types.
- **Numbers are from the read the tag names**, never shifted by the
  section's earlier operations; ranges may not overlap; operations apply
  bottom-up.
- New files, deleting and moving files: the Codex format (`*** Add File`,
  `*** Delete File`) or bash, as today. Both formats stay accepted.
- If every new line starts `N:` (a model pasting the read's prefixes), the
  prefixes are stripped.

## 5. Checks, in order; any failure rejects the whole patch

1. The tag is the file's current one — or a snapshot with that tag exists
   and every targeted line maps to an unchanged line of the current file
   (go-difflib's matcher, already in go.mod): the edit is applied at the
   mapped numbers. This is what lets several edits in one turn use the one
   read. Otherwise: rejected (step 5 message).
2. Ranges are inside the file and do not overlap.
3. Every replaced or deleted line was seen.
4. `"<start>"` matches line N.
5. A rejection names the reason and shows the current `[path#TAG]` and ±5
   numbered lines around each targeted range, so the retry needs no re-read.

After applying, the guards `apply_patch` runs today (Go syntax regression,
import grounding, atomic multi-file write) run unchanged.

## 6. The answer

`[path#NEWTAG]` and, per changed region, the new lines ±3, numbered: fresh
anchors for the next edit without a read. A snapshot of the new text is
recorded, all lines seen only inside those windows.

## 7. Measurement, before any default changes

- **Set**: a small Go module and 20 tasks, each a mutation that breaks one
  test (a flipped comparison, a wrong constant, a missing nil check, an
  off-by-one, a wrong call, an inverted condition…), stated as "make
  `go test` pass". Pass = `go test ./...` green after the turn.
- **Arms**: `edit_format` workspace setting — `patch` (today) or `hashline`
  (reads numbered and tagged, the tool describes the hashline form first);
  × GLM 5.3 Flash and DeepSeek V4.1 Flash; one run each: 80 runs.
- **Per run**: pass, output tokens, input tokens, `apply_patch` calls and
  failures, turn time.
- **Rule**: hashline becomes a model's default when its pass rate is within
  2 points of the patch format's (or better) and its output tokens per
  passed task are lower. Otherwise that model keeps the patch format.
  Results and the decision go in this document.

### Result, 2026-09-30

`eval/editbench`, 80 runs, the coder's own `read_file` and `apply_patch`
(the `memdoor/tools` package, `edit_format` set per arm):

| Model | Format | Passed | Output tokens per pass | Edits | Refused edits |
|---|---|---|---|---|---|
| DeepSeek V4.1 Flash | patch | 20/20 | 404 | 20 | 0 |
| DeepSeek V4.1 Flash | hashline | 20/20 | 427 | 29 | 7 |
| GLM 5.3 Flash | patch | 15/20 | 358 | 22 | 4 |
| GLM 5.3 Flash | hashline | 19/20 | 409 | 50 | 23 |

**Decision: both models keep the patch format.** Hashline's output tokens
per passed task are higher for both, which the rule requires to be lower;
it also took more edits and was refused more often.

The GLM patch arm's 5 failures are not failed edits: GLM found the bug,
said "Let me fix it." and ended its reply without calling `apply_patch`.
The bench has none of the coder's handling of a reply that announces work
and stops, so it counts these as failures; the first sweep's 11 such stops
fell to 5 on a rerun of the same tasks. The pass rate of that arm measures
the bench's missing handling, not the format.

## 8. Not now

- Syntactic blocks (`N*`, omp): need a parser per language.
- Cut/paste registers, move: bash does moves today.
- Fast-apply (a second model rewriting the file): a second bill on
  OpenRouter.
