"""Battle-test the TUI: drive the real `memdoor tui` in tmux on a fresh copy of
this repository, one task at a time, and measure what each turn is billed.

    python3 scripts/bench_tui.py q-fail q-rerun
    BENCH_N=3 python3 scripts/bench_tui.py            # the default matrix, 3 each

Each run: a clean `git worktree` of HEAD at /tmp/bench/<id> (so the repo's
AGENTS.md is there, as in real use), the task pasted into a fresh
conversation, the footer waited on until the turn is over, then the result
checked: a regex on the answer. Metrics come from ~/.memdoor/meter.jsonl
(tokens, cached, cost, as OpenRouter billed them) and the gateway's event log
(calls, tools, system prompt size) in the run's window. Rows are appended to
bench_tui.jsonl.

Questions only. The edit task made the broker throttle's wait ceiling
configurable, and that throttle went with the shared-key broker (2026-10-04);
the decision model is not a switch either (removed 2026-09-29, DECIDE.md), so a
run is judged when the gateway holds a decision key and no row claims a
condition.
"""
import collections, json, os, re, shlex, subprocess, sys, time, urllib.parse, urllib.request

AK = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MEM = os.path.join(AK, "memdoor")
HOME = os.path.expanduser("~")
BASE = "http://localhost:18789"
OUT = os.path.join(AK, "bench_tui.jsonl")
TOKEN = subprocess.check_output([MEM, "auth", "token"], cwd=AK, text=True).strip()

TASKS = {
    "q-fail": ("Where does a turn give up when the same tool keeps failing, and what limits does it use? "
               "Answer in at most five lines with file:line. Do not edit anything.",
               [r"agent_runtime_process\.go", r"maxRepeatedFail|maxConsecutiveFail|repeatedFail"]),
    "q-rerun": ("Where is a finished subagent's result turned into the message that re-runs its requester? "
                "Answer in at most five lines with file:line. Do not edit anything.",
                [r"server_jobs\.go"]),
}
DEFAULT = list(TASKS)


def api(path):
    req = urllib.request.Request(BASE + path, headers={"Authorization": "Bearer " + TOKEN})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read() or b"{}")


def events(since_s):
    q = urllib.parse.urlencode({"limit": 20000, "since": f"{int(since_s)}s", "order": "asc"})
    return api("/api/logs/query?" + q).get("events", [])


def meter(t0, t1):
    tot = collections.Counter()
    for line in open(os.path.join(HOME, ".memdoor", "meter.jsonl")):
        r = json.loads(line)
        ts = time.mktime(time.strptime(r["ts"][:19], "%Y-%m-%dT%H:%M:%S"))
        if t0 - 1 <= ts <= t1 + 1 and "openrouter" in r.get("engine", ""):
            tot["calls"] += 1
            tot["in"] += r.get("input_tokens", 0)
            tot["cached"] += r.get("cached_tokens", 0)
            tot["out"] += r.get("output_tokens", 0)
            tot["cost"] += r.get("cost_usd", 0) or 0
    tot["cost"] = round(tot["cost"], 5)
    return dict(tot)


def sh(cmd, **kw):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True, **kw)


def pane(sess):
    return sh(f"tmux capture-pane -p -J -S -3000 -t {sess}").stdout


def run(task, n):
    prompt, checks = TASKS[task]
    rid = f"{task}-{int(time.time())}"
    copy = f"/tmp/bench/{rid}"
    sh(f"git -C {AK} worktree add -q --detach {copy} HEAD")
    sess = "bench" + str(n)
    sh(f"tmux kill-session -t {sess}")
    env = "env -u CMUX_WORKSPACE_ID -u CMUX_BUNDLED_CLI_PATH -u TMUX"
    sh(f"tmux new-session -d -s {sess} -x 140 -y 50 -c {copy} '{env} {MEM} tui'")
    time.sleep(8)
    model = os.environ.get("BENCH_MODEL", "")
    if model:  # pin the conversation's model the way a person would
        sh(f"tmux send-keys -t {sess} -l '/model {model}'")
        time.sleep(0.5)
        sh(f"tmux send-keys -t {sess} Enter")
        time.sleep(4)
        # /model <id> opens the host picker; Enter pins the model and closes it
        # (typed text went into the picker, and no task was ever sent).
        sh(f"tmux send-keys -t {sess} Enter")
        time.sleep(2)
    with open("/tmp/bench/prompt.txt", "w") as f:
        f.write(prompt)
    # The paste can be swallowed while the screen is still redrawing (after a
    # /model pin it was, and every run "finished" in 9 s having sent nothing):
    # check the start of the task is in the input box before pressing Enter.
    # Typed a key at a time: after a /model pin a paste was swallowed whole.
    for ch in prompt:
        if ch == " ":
            sh(f"tmux send-keys -t {sess} Space")
        else:
            sh(f"tmux send-keys -t {sess} -l " + shlex.quote(ch))
    t0 = time.time()
    sh(f"tmux send-keys -t {sess} Enter")
    limit = 300
    # Done is the footer going idle again: "esc interrupt" while a turn runs,
    # "ctrl+c quit" when it is over — not "answered by", which shows only when
    # the gateway holds a decision key.
    idle_seen, started = 0, False
    while time.time() - t0 < limit:
        time.sleep(3)
        tail = "\n".join([l for l in pane(sess).splitlines() if l.strip()][-4:])
        busy = "esc interrupt" in tail
        started = started or busy
        # Idle only counts once the turn has run: a task that never went out
        # must not pass for a fast finish.
        idle_seen = idle_seen + 1 if (started and not busy and "ctrl+c quit" in tail) else 0
        if idle_seen >= 2:
            break
    t1 = time.time()
    time.sleep(3)
    screen = pane(sess)
    sh(f"tmux send-keys -t {sess} C-c; sleep 1; tmux send-keys -t {sess} C-c")
    time.sleep(2)
    sh(f"tmux kill-session -t {sess}")

    evs = events(time.time() - t0 + 5)
    evs = [e for e in evs if t0 - 1 <= time.mktime(time.strptime(e.get("time", "")[:19], "%Y-%m-%dT%H:%M:%S")) <= t1 + 1] if evs and evs[0].get("time") else evs
    tools = collections.Counter()
    sp = []
    for e in evs:
        m = e.get("message", "")
        if m.startswith("Tool call started: "):
            tools[m.split(": ", 1)[1].strip()] += 1
        if m.startswith("LLM request body caller=agent"):
            g = re.search(r"system_prompt_chars=(\d+)", m)
            if g:
                sp.append(int(g.group(1)))
    flat = re.sub(r"\s+", " ", screen)
    ok = all(re.search(p, flat) for p in checks)
    row = {"task": task, "id": rid, "ok": ok, "secs": round(t1 - t0), "finished": started and time.time() - t0 < limit + 10,
           "meter": meter(t0, t1), "tools": dict(tools), "system_prompt_chars": max(sp) if sp else 0,
           "commit": sh(f"git -C {AK} rev-parse --short HEAD").stdout.strip(), "model": os.environ.get("BENCH_MODEL", "")}
    with open(OUT, "a") as f:
        f.write(json.dumps(row) + "\n")
    with open(f"/tmp/bench/{rid}.screen", "w") as f:
        f.write(screen)
    sh(f"git -C {AK} worktree remove --force {copy}")
    print(json.dumps(row), flush=True)


if __name__ == "__main__":
    os.makedirs("/tmp/bench", exist_ok=True)
    specs = sys.argv[1:] or DEFAULT
    for i in range(int(os.environ.get("BENCH_N", "1"))):
        for s in specs:
            run(s, i)
    print("BENCH_DONE", flush=True)
