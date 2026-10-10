#!/usr/bin/env python3
"""The cost table: the same coder tasks with the decision model off and on,
measured the way a reader will believe (docs/internal/GROWTH_STRATEGY.md,
move 3): did it pass, what did it read and write, how much of that was the
provider's cache, what did OpenRouter bill, how long did it take.

Two scratch gateways run the same binary on the same OpenRouter key and the
same first rung; one has no decision provider (MEMDOOR_SYSTEMONE_URL points
at a non-local host with no key, which the decision client refuses at
construction, so nothing is judged). Each task is a small Go module from
eval/editbench with one test broken by one mutation (`go run ./eval/editbench
-dump <dir>` writes them), and the prompt is the benchmark's: make `go test
./...` pass. Every task runs under both conditions, in alternating order, in a
fresh copy, through the real `memdoor tui` in tmux (scripts/bench_tui.py's
drive). Pass is `go test ./...` on the copy afterwards, never the model's
word. Tokens and cache come from the gateway's meter; the dollars from
OpenRouter's own record of each generation (/api/v1/generation), which is what
the key is billed.

    MEM=~/memdoor-scratch-free/memdoor \\
    ON_HOME=… ON_GATEWAY=http://localhost:18797 \\
    OFF_HOME=… OFF_GATEWAY=http://localhost:18799 \\
    TASKS=/tmp/ct/tasks OUT=/tmp/ct/rows.jsonl N=10 python3 scripts/cost_table.py

Rows are appended to OUT (resumable: a task+condition already there is
skipped); the table is printed at the end and by `--table OUT`.
"""
import collections, json, os, shlex, subprocess, sys, time, urllib.request

MEM = os.environ.get("MEM", "memdoor")
COND = {
    "on": (os.environ.get("ON_HOME"), os.environ.get("ON_GATEWAY", "http://localhost:18797")),
    "off": (os.environ.get("OFF_HOME"), os.environ.get("OFF_GATEWAY", "http://localhost:18799")),
}
TASKS = os.environ.get("TASKS", "/tmp/ct/tasks")
OUT = os.environ.get("OUT", "/tmp/ct/rows.jsonl")
N = int(os.environ.get("N", "10"))
PROMPT = "Make `go test ./...` pass."
# A task directory may carry its own PROMPT.txt (else PROMPT) and CHECK.txt:
# a shell command run in the copy whose exit 0 is the pass (else `go test
# ./...`), or `regex: <pattern> && <pattern>` matched against the screen (a
# question).
LIMIT = int(os.environ.get("LIMIT", "420"))
KEY = os.environ.get("OPEN_ROUTER_API_KEY", "")


def sh(cmd, **kw):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True, **kw)


def pane(sess):
    return sh(f"tmux capture-pane -p -J -S -3000 -t {sess}").stdout


def meter_rows(home, t0, t1):
    rows = []
    path = os.path.join(home, ".memdoor", "meter.jsonl")
    if not os.path.exists(path):
        return rows
    for line in open(path):
        try:
            r = json.loads(line)
        except Exception:
            continue
        ts = time.mktime(time.strptime(r["ts"][:19], "%Y-%m-%dT%H:%M:%S"))
        if t0 - 2 <= ts <= t1 + 2 and "openrouter" in r.get("engine", ""):
            rows.append(r)
    return rows


def billed(gen_id):
    """OpenRouter's record of one generation: what was billed, and the cache."""
    if not KEY or not gen_id:
        return None
    try:
        req = urllib.request.Request(f"https://openrouter.ai/api/v1/generation?id={gen_id}", headers={"Authorization": "Bearer " + KEY})
        with urllib.request.urlopen(req, timeout=20) as r:
            return json.loads(r.read()).get("data") or None
    except Exception:
        return None


def run(task_dir, cond, n):
    home, gateway = COND[cond]
    name = os.path.basename(task_dir)
    prompt = PROMPT
    if os.path.exists(os.path.join(task_dir, "PROMPT.txt")):
        prompt = open(os.path.join(task_dir, "PROMPT.txt")).read().strip()
    check = "go test ./... 2>&1"
    if os.path.exists(os.path.join(task_dir, "CHECK.txt")):
        check = open(os.path.join(task_dir, "CHECK.txt")).read().strip()
    rid = f"{name}-{cond}-{int(time.time())}"
    copy = f"/tmp/ct/runs/{rid}"
    sh(f"rm -rf {copy} && mkdir -p {copy} && cp -R {task_dir}/. {copy}/ && git -C {copy} init -q && git -C {copy} add -A && git -C {copy} -c user.name=ct -c user.email=ct@localhost commit -qm start")
    sess = f"ct{n}"
    sh(f"tmux kill-session -t {sess}")
    env = f"env -u CMUX_WORKSPACE_ID -u CMUX_BUNDLED_CLI_PATH -u TMUX HOME={shlex.quote(home)} MEMDOOR_GATEWAY={gateway} MEMDOOR_NO_BROWSER=1"
    sh(f"tmux new-session -d -s {sess} -x 140 -y 50 -c {copy} '{env} {MEM} tui'")
    time.sleep(10)
    sh(f"tmux send-keys -t {sess} Escape")
    for ch in prompt:
        sh(f"tmux send-keys -t {sess} Space" if ch == " " else f"tmux send-keys -t {sess} -l " + shlex.quote(ch))
    t0 = time.time()
    sh(f"tmux send-keys -t {sess} Enter")
    idle_seen, started = 0, False
    while time.time() - t0 < LIMIT:
        time.sleep(3)
        tail = "\n".join([l for l in pane(sess).splitlines() if l.strip()][-4:])
        busy = "esc interrupt" in tail
        started = started or busy
        idle_seen = idle_seen + 1 if (started and not busy and "ctrl+c quit" in tail) else 0
        if idle_seen >= 2:
            break
    t1 = time.time()
    time.sleep(2)
    screen = pane(sess)
    sh(f"tmux send-keys -t {sess} C-c; sleep 1; tmux send-keys -t {sess} C-c")
    time.sleep(1)
    sh(f"tmux kill-session -t {sess}")
    if check.startswith("regex:"):
        import re
        flat = re.sub(r"\s+", " ", screen)
        passed = all(re.search(pat.strip(), flat) for pat in check[len("regex:"):].split("&&"))
        test = subprocess.CompletedProcess(check, 0 if passed else 1, stdout="regex " + ("matched" if passed else "did not match"), stderr="")
    else:
        test = sh(check, cwd=copy)
        passed = test.returncode == 0
    rows = meter_rows(home, t0, t1)
    tot = collections.Counter()
    cost = 0.0
    cost_known = True
    for r in rows:
        tot["calls"] += 1
        tot["in"] += r.get("input_tokens", 0)
        tot["cached"] += r.get("cached_tokens", 0)
        tot["out"] += r.get("output_tokens", 0)
        g = billed(r.get("gen_id"))
        if g and g.get("total_cost") is not None:
            cost += float(g["total_cost"])
        else:
            cost_known = False
    jev = "(Jev)" in screen
    row = {"task": name, "cond": cond, "id": rid, "pass": passed, "finished": started and (time.time() - t0 < LIMIT + 10),
           "secs": round(t1 - t0), "calls": tot["calls"], "in": tot["in"], "cached": tot["cached"], "out": tot["out"],
           "cost_usd": round(cost, 6), "cost_known": cost_known, "jev_in_footer": jev,
           "model": (rows[-1].get("model") if rows else ""), "test_tail": test.stdout.strip().splitlines()[-1:] }
    with open(OUT, "a") as f:
        f.write(json.dumps(row) + "\n")
    with open(f"/tmp/ct/runs/{rid}.screen", "w") as f:
        f.write(screen)
    print(json.dumps(row), flush=True)
    return row


def table(path):
    rows = [json.loads(l) for l in open(path) if l.strip()]
    by = collections.defaultdict(dict)
    for r in rows:
        by[r["task"]][r["cond"]] = r
    print("| Task | Condition | Passed | Input tokens | Cached | Output tokens | Calls | $ billed | Seconds |")
    print("|---|---|---|---|---|---|---|---|---|")
    agg = {c: collections.Counter() for c in ("off", "on")}
    for t in sorted(by):
        for c in ("off", "on"):
            r = by[t].get(c)
            if not r:
                continue
            a = agg[c]
            a["n"] += 1; a["pass"] += int(r["pass"]); a["in"] += r["in"]; a["cached"] += r["cached"]; a["out"] += r["out"]; a["cost"] += r["cost_usd"]; a["secs"] += r["secs"]; a["calls"] += r["calls"]
            print(f"| {t} | {c} | {'✓' if r['pass'] else '✗'} | {r['in']:,} | {int(100*r['cached']/r['in']) if r['in'] else 0}% | {r['out']:,} | {r['calls']} | ${r['cost_usd']:.4f} | {r['secs']} |")
    for c in ("off", "on"):
        a = agg[c]
        if a["n"]:
            print(f"| **mean, decisions {c}** | | {a['pass']}/{a['n']} | {a['in']//a['n']:,} | {int(100*a['cached']/a['in']) if a['in'] else 0}% | {a['out']//a['n']:,} | {a['calls']/a['n']:.1f} | ${a['cost']/a['n']:.4f} | {a['secs']//a['n']} |")
    if agg["off"]["n"] and agg["on"]["n"]:
        f, o = agg["off"], agg["on"]
        d = lambda k: (o[k] / o["n"]) / (f[k] / f["n"]) - 1 if f[k] else 0
        print(f"\ndecisions on vs off: input tokens {d('in'):+.0%}, $ per task {d('cost'):+.0%}, seconds {d('secs'):+.0%}, pass {o['pass']}/{o['n']} vs {f['pass']}/{f['n']}")


if __name__ == "__main__":
    if len(sys.argv) > 2 and sys.argv[1] == "--table":
        table(sys.argv[2])
        sys.exit(0)
    os.makedirs("/tmp/ct/runs", exist_ok=True)
    done = set()
    if os.path.exists(OUT):
        for l in open(OUT):
            if l.strip():
                r = json.loads(l)
                done.add((r["task"], r["cond"]))
    import re
    skip = re.compile(os.environ.get("SKIP", r"^$"))
    tasks = [d for d in sorted(os.listdir(TASKS)) if os.path.isdir(os.path.join(TASKS, d)) and not skip.match(d)][:N]
    for i, t in enumerate(tasks):
        order = ("off", "on") if i % 2 == 0 else ("on", "off")
        for cond in order:
            if (t, cond) in done:
                continue
            run(os.path.join(TASKS, t), cond, i % 2)
    print("COST_TABLE_DONE", flush=True)
    table(OUT)
