# End-to-end walk: validate the 2026-09-27 features yourself

About 20 minutes. Every step says what to type and what you should see. Run it
on your Mac, in a throwaway project, with your normal `~/.memdoor` (signed in
with the Pro seat).

## 0. Put the new build in place

The installed binary predates these features, and none of them is deployed
yet, so build from the repository. One command builds it, points `memdoor` on
your PATH at it, and restarts your gateway on it:

```
cd ~/Dev/aktapus    # direnv exports OPEN_ROUTER_API_KEY here
make up
memdoor --version
```

A throwaway project to work in:

```
mkdir -p ~/tmp/walk && cd ~/tmp/walk
git init -q -b main
printf 'package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("hi") }\n' > main.go
printf 'module example.com/walk\n\ngo 1.22\n' > go.mod
git add . && git commit -qm first
```

## 1. Account and decision model

```
memdoor account status
```

Expect `seat, monthly`. There is no decision-model switch (2026-09-29): with
the seat, decisions are on.

Run one turn that reads a file, then:

```
memdoor savings
```

Expect the judged-reads call count to have grown: the decision model answered
through the subscription, not your key.

## 2. The tab and the input hints

Open the window in the project, in a terminal where you can see the tab title.

```
memdoor tui
```

- **Idle.** The tab reads `walk`.
- Type `add a func Add(a, b int) int to main.go, nothing else` and press Enter.
- **Running.** The tab reads `● walk` while it works.
- **Finished.** The tab reads `✓ walk`. Press any key, and it goes back to `walk`.
- **Hint after a change.** The empty box shows `Commit the change with a
  message that says why.` Press Tab: it is sent as your message.
- Ask `what does Add return for 2 and 3? one line`.
- **Hint after an answer, with a seat.** The box shows `/usage — what the
  decision model kept out of your bill this month`. Tab runs `/usage`.
- **Bell when away.** Send a slow task (`explain every file here in detail`),
  switch to another window or tab, and wait. When it finishes you should get a
  bell or a notification, and the tab stays marked. In the window you are
  looking at, nothing rings.

Quit with Ctrl+C.

## 3. A rule for later goes in AGENTS.md

```
memdoor tui
```

Type: `In this project always run tests with 'go test -race ./...' and never add
dependencies. Keep that for later, then say ok.`

Expect a `notes(rule: …)` frame answered "kept in …/AGENTS.md", then `Ok`.
Quit.

```
cat AGENTS.md
```

The rule is in the file under "## Rules kept by Memdoor", where you can read
and review it. Now a brand-new
conversation:

```
memdoor tui
```

Type only `run the tests`. Expect `go test -race ./...` without you repeating
the rule: AGENTS.md is read every turn. (Notes are per conversation — a new
conversation starts with none, and `/fresh` wipes them.)

## 4. Worktrees

```
memdoor tui --worktree add-greeting
```

- The line above the window reads `Working in worktree
  …/walk/.memdoor/worktrees/add-greeting (branch memdoor/add-greeting)`.
- The tab reads `add-greeting`.
- Type `add a func Greeting() string that returns "hi" to main.go, nothing else`.
- Quit with Ctrl+C. The last lines say the work is on branch
  `memdoor/add-greeting` with uncommitted changes, and give three commands:
  continue, bring back, throw away.

Check your checkout was not touched:

```
git diff --stat       # empty: no tracked file changed here
grep Greeting main.go # nothing
git worktree list     # main, plus .memdoor/worktrees/add-greeting
```

(`git status` may list `.memdoor/` from step 2 — Memdoor's undo
checkpoints, not the agent's code — and `AGENTS.md` from step 3.)

Continue the same work, then bring it back:

```
memdoor tui --worktree add-greeting   # same worktree, same branch; quit again
cd .memdoor/worktrees/add-greeting && git add -A && git commit -qm "Add Greeting"
cd ~/tmp/walk && git merge memdoor/add-greeting
grep Greeting main.go                 # now it is here
```

An untouched worktree cleans itself up:

```
memdoor tui --worktree nothing-here   # quit immediately
git worktree list                     # nothing-here is not listed
git branch --list 'memdoor/*'         # memdoor/nothing-here is gone
```

A worktree holds only what git tracks. The untracked paths a build needs —
build output, a native library — are listed in `.worktreeinclude`, one per
line, and linked in from your checkout (never committed):

```
printf 'bin/\n' > .worktreeinclude && mkdir -p bin && echo x > bin/tool
memdoor tui --worktree with-deps      # quit immediately
ls -l .memdoor/worktrees/with-deps/bin   # a link to this checkout's bin
```

Outside a repository it refuses in words:

```
cd /tmp && memdoor tui --worktree x   # "--worktree needs a git repository"
```

## 5. The verifier checks that an exit 0 checked something

The package has no test files, so `go test` exits 0 while checking nothing.

```
cd ~/tmp/walk
memdoor agent -a verifier -c general -m "Verify this package: run go test ./... in $PWD"
sleep 30; memdoor messages --channel general --include-threads | tail -8
```

Expect FAIL: it exited 0 but no tests ran. Then add one test and ask again:

```
printf 'package main\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) { if Add(2, 3) != 5 { t.Fatal("bad") } }\n' > main_test.go
memdoor agent -a verifier -c general -m "Verify again: run go test ./... in $PWD"
sleep 30; memdoor messages --channel general --include-threads | tail -6
```

Expect PASS. (Adjust the test if step 2 did not add `Add`.)

## 6. Models

```
memdoor model check --saved | head        # stored probe reports, each with its age
memdoor model check --stale               # re-probes only reports older than a week
```

In the window, `/model z-ai/glm-5.3` pins a model the probe passed, so no
warning. The warning only appears for a model that is in the catalogue and
failed its probe; the three failures on your machine are dead ids the
catalogue refuses before a pin, so you will not see it there. `/model auto`
lets go.

## 7. Free versus Pro

Free is the same agent, the decision model and workflows on your key; Pro adds
remote control. To see it, sign out: `memdoor logout`, open the window, and
after a turn the box still shows `/usage — what the decision model kept out of
your bill this month`, and `/remote` says it is Pro. Sign back in with
`memdoor account login you+pro@example.com --send-only`, then
`--code <digits>`.

## 8. The site

Open https://memdoor.ai/pricing: two tiers, checkout at $10. The Free column
includes the decision model and workflows on your key; Pro is remote
control and the hosted scheduler. No page mentions the retired surfaces, and no
page has a mail link.

## Clean up

```
rm -rf ~/tmp/walk
```
