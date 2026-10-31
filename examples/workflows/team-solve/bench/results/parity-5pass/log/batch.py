"""batch.py <ceiling $> <parallel> <ids...>: one parity run per problem, in order, N at a time;
no new run starts once the runs' metered total passes the ceiling. Appends to batch.tsv."""
import json, os, subprocess, sys, time
S = os.path.dirname(os.path.abspath(__file__))
ceiling, par, ids = float(sys.argv[1]), int(sys.argv[2]), sys.argv[3:]
start = time.time()
env = dict(os.environ, WF="gvs5h-parity")

def spent():
    t = 0.0
    for l in open(f"{S}/home2/.memdoor/meter.jsonl"):
        r = json.loads(l)
        if str(r.get("session", "")).startswith("workflow:gvs5h-parity-") and (r.get("ts") or r.get("time") or "") >= "":
            t += r.get("cost_usd") or 0
    return t

base = spent()
running, queue = {}, list(ids)
while queue or running:
    for q, p in list(running.items()):
        if p.poll() is not None:
            out = p.stdout.read().strip()
            with open(f"{S}/batch.tsv", "a") as f:
                f.write((q.rpartition("/")[0] or "p1") + "\t" + out + "\n")
            print(time.strftime("%H:%M:%S"), "finished", out, flush=True)
            del running[q]
    total = spent() - base
    while queue and len(running) < par and total <= ceiling:
        q = queue.pop(0)
        tag, _, pid = q.rpartition("/")
        running[q] = subprocess.Popen([f"{S}/one-call.sh", pid], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, env=dict(env, TAG=tag))
        print(time.strftime("%H:%M:%S"), "started", q, flush=True)
        time.sleep(2)
    if queue and total > ceiling and not running:
        print("ceiling reached: not started:", " ".join(queue), flush=True)
        break
    time.sleep(20)
print("batch done, spent $%.2f" % (spent() - base), flush=True)
