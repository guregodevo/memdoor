"""Two read-only questions, asked once each through `memdoor agent`, and what
each turn is billed. Sequential; a fresh channel, and a fresh copy of HEAD at
~/memdoor-coder/<d>, per run.

Metrics: seconds, LLM calls, tool calls, meter tokens + cost_usd in the run's
window, decision events in the window, and the answer checked by regex on the
reply (written to ab_task.replies).

The decision_model off/on axis this harness used to vary went with that setting
(2026-09-29: no toggle, and no seat supplies a key), and so did its named-file
edit task (the broker throttle, deleted with the shared-key broker,
2026-10-04). The two questions are what is left; docs/features/DECIDE.md keeps
the measurements those runs produced.
"""
import json, re, subprocess, time, urllib.request, urllib.parse, collections, os, sys
import os; AK = os.path.dirname(os.path.dirname(os.path.abspath(__file__))); MEM = AK + "/memdoor"; BASE = "http://localhost:18789"
HOME = os.path.expanduser("~")
TOKEN = subprocess.check_output([MEM, "auth", "token"], cwd=AK, text=True).strip()
NO_EDIT = " Do not edit any file; answer in at most five lines."
QUESTIONS = {
    "subagent-rerun": ("The Go source is in ./{d} (relative to your working directory). Find where a finished "
                       "subagent's result is turned into the message that re-runs its requester. Give file:line." + NO_EDIT,
                       [r"server_jobs\.go"]),
    "fail-limit": ("The Go source is in ./{d} (relative to your working directory). Find where the agent turn loop "
                   "gives up after the same tool keeps failing, and name the limits it uses." + NO_EDIT,
                   [r"maxRepeatedFail|maxConsecutiveFail|maxFailsPerTool"]),
}

def api(method, path, body=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body else None,
                                 headers={"Authorization": "Bearer " + TOKEN, "Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read() or b"{}")

def events(since_s):
    q = urllib.parse.urlencode({"limit": 5000, "since": f"{since_s}s", "order": "asc"})
    return api("GET", "/api/logs/query?" + q).get("events", [])

def meter(t0, t1):
    tot = collections.Counter(); models = collections.Counter()
    for l in open(HOME + "/.memdoor/meter.jsonl"):
        r = json.loads(l)
        ts = time.mktime(time.strptime(r["ts"][:19], "%Y-%m-%dT%H:%M:%S"))
        if t0 - 2 <= ts <= t1 + 2 and r.get("cost_usd") is not None:
            tot["calls"] += 1; tot["in"] += r["input_tokens"]; tot["out"] += r["output_tokens"]; tot["cost"] += r["cost_usd"]
            models[r["model"]] += 1
    return tot, dict(models)

def msgs(ch):
    return subprocess.run([MEM, "messages", "--channel", ch, "--limit", "10", "--include-threads"], cwd=AK, capture_output=True, text=True).stdout

def fresh(d):
    subprocess.run(f"rm -rf {HOME}/memdoor-coder/{d} && mkdir -p {HOME}/memdoor-coder/{d} && git -C {AK} archive HEAD | tar -x -C {HOME}/memdoor-coder/{d}", shell=True, check=True)

def run(task):
    d = f"ab-{task}"
    fresh(d)
    ch = f"abt-{task}-{int(time.time())}"
    subprocess.run([MEM, "channels", "create", ch], cwd=AK, capture_output=True)
    t0 = time.time()
    subprocess.run([MEM, "agent", "--channel", ch, "--agent-id", "coder", "--mode", "acceptEdits",
                    "--message", QUESTIONS[task][0].format(d=d)], cwd=AK, capture_output=True, timeout=1800)
    last_n, last_change = -1, time.time()
    while True:
        time.sleep(15)
        evs = events(int(time.time() - t0) + 5)
        n = sum(1 for e in evs if re.search(r"LLM request|Tool execution|Agent execution", e.get("message", "")))
        if n != last_n:
            last_n, last_change = n, time.time()
        done = any(re.search(r"Agent execution (succeeded|failed)", e.get("message", "")) for e in evs)
        if (done and time.time() - last_change > 45) or time.time() - t0 > 1500:
            break
    t1 = time.time()
    evs = events(int(t1 - t0) + 5)
    llm = sum(1 for e in evs if e.get("message", "").startswith("LLM request: model="))
    tools = collections.Counter(e.get("message", "").split("Tool call started: ", 1)[1].strip() for e in evs if e.get("message", "").startswith("Tool call started: "))
    fails = sum(1 for e in evs if e.get("message", "").startswith("Tool execution failed"))
    decisions = collections.Counter(e.get("message", "")[:40] for e in evs if '"Decisions"' in json.dumps(e) or "judged" in e.get("message", "").lower())
    stopped = [e.get("message") for e in evs if "no progress" in e.get("message", "")]
    ends = [e.get("message") for e in evs if re.search(r"Agent execution (succeeded|failed)", e.get("message", ""))]
    m, models = meter(t0, t1)
    reply = msgs(ch)
    row = {"task": task, "channel": ch, "secs": int(t1 - t0 - 45), "llm_calls": llm, "tool_calls": sum(tools.values()),
           "tool_fail": fails, "tools": dict(tools), "meter": dict(m), "models": models,
           "decision_events": sum(decisions.values()), "decision_kinds": dict(decisions), "stopped": stopped, "ends": ends[:3],
           "correct": all(re.search(p, reply) for p in QUESTIONS[task][1])}
    with open("ab_task.replies", "a") as f: f.write(f"==== {task} / {ch}\n{reply}\n")
    print(json.dumps(row), flush=True)

for task in (sys.argv[1:] or list(QUESTIONS)):
    run(task)
print("ABT_DONE", flush=True)
