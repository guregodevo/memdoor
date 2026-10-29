# issue-triage

Triages an issue and writes `.memdoor/runs/issue-triage-<partition>.md`: a first line `labels: a, b` (only labels that exist in the repository, none when nothing fits), then **Kind** (bug, feature, question, or docs), a two-sentence **Summary**, **Possible duplicates** (open issues with a few words on why, or none), and **Missing** (what the reporter should add — version, steps to reproduce, logs — or nothing). It applies nothing to the issue itself; a person does the labeling and closing.

The issue comes from the GitHub event payload (`$GITHUB_EVENT_PATH`, on an `issues` event); else from the partition when it is an issue number (`gh issue view`); else from `.memdoor/issue.md`, an issue's text written by hand. It also lists the repository's labels and the open issues' titles, for duplicates, with `gh`.

Run it with `memdoor workflow run issue-triage --partition 42 --wait`, or in GitHub Actions with `uses: guregodevo/memdoor-action@v1` and `workflow: issue-triage` on an `issues` event. Needs a model on your key; with a small local model set `MEMDOOR_LEAN=1` (the GitHub Action sets it with `local-model:`) and `triage-lean` runs instead: one model call, everything inlined, no tools.
