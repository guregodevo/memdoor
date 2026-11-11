---
name: test-windows
description: Test a change on Windows from a Mac — a temporary branch whose workflow runs main and the fix on GitHub's windows-latest, the same coder turn on both, the window in a ConPTY, and what to assert
user-invocable: true
disable-model-invocation: false
---

# Testing on Windows

There is no Windows machine here. GitHub's `windows-latest` runner is one: x64
Windows Server with Git Bash, Windows PowerShell 5.1 and PowerShell 7.
Reproduce the bug on main there, then run the same steps on the fix.

## 1. A temporary branch

Start from the fix's branch, in a worktree. Add one workflow file,
`.github/workflows/windows-check.yml`, that runs only on a push to that branch:

```yaml
on:
  push:
    branches: [ windows-check ]
```

Push it. Delete the branch when you are done; nothing goes onto main.

## 2. Build on ubuntu, run on Windows

An `ubuntu-latest` job builds the web frontend (`cd web && npm ci && npm run
build`; `web/dist` is embedded) and cross-compiles both binaries:

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o out/memdoor-fix.exe ./cmd/cli
git checkout -q origin/main          # checkout with fetch-depth: 0
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o out/memdoor-main.exe ./cmd/cli
```

Upload `out/` with `actions/upload-artifact`. A `windows-latest` job downloads
it. `shell: bash` there is Git Bash.

## 3. Go tests on Windows

`CGO_ENABLED=0 go test <packages> -run <tests>` on the runner. A test that only
means something on Windows (Git Bash paths, Windows PowerShell writing a file)
goes in a `_windows_test.go` file with `//go:build windows`. Give the step
`continue-on-error: true` so the steps after it still run.

## 4. The same coder turn on main and on the fix

For each binary:

1. Start its gateway on its own port, with `HOME` and `USERPROFILE` set to a
   folder of its own (Windows reads `USERPROFILE`) and
   `OPEN_ROUTER_API_KEY: ${{ secrets.OPEN_ROUTER_API_KEY }}`.
2. Wait for `/api/status`, then run `memdoor --gateway http://localhost:<port> setup --admin-email … --admin-password … --workspace-name …`.
3. In a fresh `git init` folder, give the coder the same task:
   `memdoor --gateway http://localhost:<port> run "<the task>"`.
4. Assert on the files the turn left, not on what the model says. Print paths
   and bytes: `cmd //c "dir /s /b x.txt"`, a hex dump of the first bytes.

To test a machine with no Git Bash, start that gateway with:

- `PATH` set to `System32`, `C:\Windows` and `System32\WindowsPowerShell\v1.0`;
- `ProgramFiles`, `ProgramFiles(x86)`, `ProgramW6432` and `LOCALAPPDATA` pointing at an empty folder.

The coder's commands then run in Windows PowerShell 5.1. `System32\bash.exe`
is the WSL launcher, and memdoor skips it.

## 5. The window

tmux does not exist on Windows. Run `memdoor tui` in a Windows
pseudo-terminal with Python: `pip install pywinpty pyte`. Then:

- spawn it with `winpty.PtyProcess.spawn([exe, "--gateway", url, "tui"], cwd=…, env=…, dimensions=(46, 150))`;
- feed what it prints into a `pyte.Screen(150, 46)`, which gives the screen as text, like `tmux capture-pane`;
- write keys to it: `\r` for Enter, `\x1b` for Esc, `\x03` for ctrl+c.

Set `PYTHONUTF8=1`, or printing the screen fails on the console's code page.

Check what a person does: the first screen draws, a typed request runs a turn
and writes its file, `/` opens the menu, Esc interrupts a long command (the
screen shows "Interrupted"), and one ctrl+c quits.

## 6. Read it and clean up

`gh run list --branch windows-check` to find the run, `gh run view <id> --log`
to read it. Post main's result and the fix's on the PR. Then
`git push origin --delete windows-check`.

## Limits

- x64 Windows Server only: not Windows on ARM, not Windows 11 Home or Pro.
- The window is checked in ConPTY, which Windows Terminal also uses.
- Runs queued during a GitHub Actions outage stay queued. Push an empty commit
  once Actions is back.
