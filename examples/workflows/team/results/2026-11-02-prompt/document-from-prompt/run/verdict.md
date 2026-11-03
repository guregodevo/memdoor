verdict: PASS
answer.md exists at the repository root, and `git status` confirms the run changed nothing else.
`wc -w answer.md` gives exactly 120 words, matching the Prompt's word-count constraint.
It states all three rules: lowercase only ("Lowercase only" bullet), short type prefix (fix/ for bug fixes, feat/ for features, docs/ for docs), and hyphen-joined words ("never spaces").
It gives one example of each prefix: fix/login-crash, feat/export-csv, docs/api-reference.
It is addressed to a new teammate in a friendly tone ("Welcome aboard!"), matching the Done-when reviewer line.
Scope respected: only answer.md and the run ledger were touched; no .git or other .memdoor files.
