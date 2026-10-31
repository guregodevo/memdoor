"""Writes steps/*.yaml: advance, llm call, advance, ... in the authors' order (lib/team.py)."""
import os

D = os.path.dirname(os.path.abspath(__file__))
TEAM = os.path.join(D, "lib", "team.py")
PY = "<scratch>/lcb/venv/bin/python"  # runs the samples, as the authors' sys.executable did
MODEL = os.environ.get("GEN_MODEL") or "openrouter:qwen/qwen3.8-27b order modal,coreweave,parasail,wafer temperature {t} top_p 0.95 top_k 20 max_tokens 128000 mark_cutoff"
TEMP = {"plan": 0.3, "ideate": 0.4, "manage": 0.2, "worker": 0.2, "cutoff": 0.2, "finalize": 0.2}

calls = ["plan", "ideate", "manage-0"]
for n in range(1, 11):
    calls += [f"worker-{n}", f"cutoff-{n}", f"manage-{n}"]
calls.append("finalize")

steps, prev = [], None
for i, call in enumerate(calls + [None]):
    adv = f"advance-{i:02d}"
    # The call before this step wrote its reply to its output (empty when skipped):
    # kept as reply/<call>.md only when it ran, a newline after it as a file target would.
    keep = ""
    if prev and prev != "advance" and not prev.startswith("advance-"):
        keep = (f"  if [ -f msg/{prev}.user ]; then mkdir -p reply; cat > reply/{prev}.md <<'MEMDOOR_REPLY_EOF'\n"
                f"  {{{{ output \"{prev}\" }}}}\n  MEMDOOR_REPLY_EOF\n  fi\n")
    steps.append((adv, f"type: command\n" + (f"requires:\n  - table_pattern: {prev}\n" if prev else "")
                  + f"command: |\n{keep}  python3 -I {TEAM} advance {PY}\ntimeout: 5m\nmax_retries: 0\n"))
    prev = adv
    if call is None:
        break
    kind = call.split("-")[0]
    steps.append((call, f"type: llm\nwhen: test -f msg/{call}.user\n"
                  f"model: {MODEL.format(t=TEMP[kind])}\n"
                  f"requires:\n  - table_pattern: {prev}\n"
                  f"system: '{{{{ file \"msg/{call}.system\" }}}}'\n"
                  f"prompt: '{{{{ file \"msg/{call}.user\" }}}}'\n"
                  f"timeout: 4h\nmax_retries: 3\n"))
    prev = call

out = os.environ.get("GEN_OUT") or D
os.makedirs(os.path.join(out, "steps"), exist_ok=True)
for name, body in steps:
    open(os.path.join(out, "steps", name + ".yaml"), "w").write(body)
print(len(steps), "steps")
