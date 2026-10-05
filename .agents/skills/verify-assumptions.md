---
name: verify-assumptions
description: After any runtime change, run CLI commands and check logs/output to verify the change actually works
user-invocable: false
disable-model-invocation: false
metadata:
  openclaw:
    emoji: "\U0001F50D"
    skillKey: verify-assumptions
    always: true
    os:
      - darwin
      - linux
---

# Verify Assumptions with CLI

After any change that affects runtime behavior, you MUST:

1. **Build and restart**: `make build && make clean stop start`
2. **Run a CLI command** that exercises the changed code path
3. **Read the CLI output** and check it matches expectations
4. **Check logs** with `./memdoor logs query --regex "PATTERN" --limit 10 --since 2m` for the specific behavior
5. **Report what you observed** — not what you expected

## What to run

| Change type | Run this | Check this |
|---|---|---|
| Agent prompt/behavior | `./memdoor agent --message "test" --channel test2 --agent-id AGENT` | Response text + logs for prompt content |
| Web page rendering | `echo 'navigate URL\nwait 2\nscreenshot /tmp/v.png' \| ./memdoor chrome run` then read screenshot | Visual output |
| API endpoint | `./memdoor agent --message "..." --channel test2 --agent-id chief` | `./memdoor messages --channel test2 --limit 3` |
| Language enforcement | `./memdoor agent --message "test in target lang" --channel test2 --agent-id coder` | Response language + `./memdoor logs query --regex "language\|Language" --since 2m` |
| Auth/permissions | Run the protected action | Check for 401/403 in logs |

## Rules

- Never say "this should work" — run it and show the output
- Never trust "it compiles" as proof of correctness
- If the server isn't running, start it before testing
- If a test fails, debug using `./memdoor logs query` and `./memdoor logs errors --since 5m`
- Always check both the CLI output AND the logs — the output shows what the user sees, the logs show what happened internally

## Skip only for

- Markdown/docs-only changes
- .gitignore or CI config
- Comment-only changes
