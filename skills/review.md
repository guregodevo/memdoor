# Review (read and understand an existing repo)

Use this when asked to REVIEW, EXPLAIN, or ASSESS existing code ("what does this
repo do?", "review x.go", "is this correct?"). This is a READING job — change
nothing unless the user asks for a fix.

## Workflow

1. **Map the terrain first.** `glob` / `ls` the directory: what files exist, is
   there a go.mod / package.json, which file holds the entry point (main).
   Read NOTHING yet — just list.

2. **Pick the few files that matter.** The one the user named, or the entry
   point plus what it imports. Use `locate` for a symbol you can't place.
   Do NOT read every file — read the 1-3 that answer the question.

3. **read_file each chosen file** (jread with the question for a long one) and note per file, in one or two lines:
   what it defines, what it calls, anything suspicious (unused code, wrong
   logic, missing error handling).

4. **Follow one chain when needed.** To know who calls X or where Y is defined,
   `jgrep` with the question and the name (`grep` when you need every match)
   — don't guess relationships.

5. **Verify claims by RUNNING, not opining.** "Is it correct?" → build/run it
   (`go build` / `go run`, `python3`, `node`) and read the real output. A review
   that says "looks fine" without a build is a guess.

6. **Report grounded findings only.** Every claim cites the file (and function)
   it came from, quoting the REAL line when it matters. If you did not read it,
   do not claim it. If something is unknown, say so.

## Rules

- Reading job = no apply_patch, no file creation. Propose fixes in the report;
  apply them only if the user asks.
- Keep reads targeted — long dumps drown the important lines. Read through
  read_file, jread and jgrep, never cat, sed or grep in bash: the tools read
  for your question, bash returns everything.
- End with a short verdict: what the code does, what is broken or risky (if
  anything), and the ONE next action you would take.
