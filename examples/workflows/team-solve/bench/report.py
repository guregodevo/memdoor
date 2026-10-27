#!/usr/bin/env python3
"""The report of a bench arm: one line per problem (rounds, calls, tokens, cost,
seconds, public samples, private tests) and the totals, from the results file,
the per-problem logs (the run id) and the meter (every call stamped with its run).

usage: report.py <arm> [meter.jsonl] [--md]
  arm         the workflow's name (results/<arm>.tsv, logs/<arm>/<id>.log)
  meter.jsonl the gateway's ledger (default ~/.memdoor/meter.jsonl); a run's
              calls carry session "workflow:<run id>"
"""
import json, os, re, sys, glob

def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    md = "--md" in sys.argv
    if not args:
        print(__doc__.strip()); return
    arm = args[0]
    meter = args[1] if len(args) > 1 else os.path.expanduser("~/.memdoor/meter.jsonl")
    here = os.path.dirname(os.path.abspath(__file__))
    problems = os.environ.get("PROBLEMS", os.path.join(here, "problems"))
    rows = []
    results = os.path.join(here, "results", arm + ".tsv")
    if not os.path.exists(results):
        print(f"no results yet for {arm} ({results})"); return
    for line in open(results):
        f = line.rstrip("\n").split("\t")
        if len(f) >= 5:
            rows.append({"id": f[0], "state": f[1], "samples": f[2], "all": f[3], "seconds": int(f[4] or 0)})
    runs = {}
    for r in rows:
        log = os.path.join(here, "logs", arm, r["id"] + ".log")
        if os.path.exists(log):
            m = re.search(r"started as (\S+)", open(log, errors="replace").read())
            if m:
                runs["workflow:" + m.group(1)] = r
    for r in rows:
        r.update(calls=0, tin=0, tout=0, cost=0.0)
    if os.path.exists(meter):
        for line in open(meter):
            try:
                e = json.loads(line)
            except ValueError:
                continue
            r = runs.get(e.get("session", ""))
            if r is None:
                continue
            r["calls"] += 1
            r["tin"] += e.get("input_tokens", 0)
            r["tout"] += e.get("output_tokens", 0)
            r["cost"] += e.get("cost_usd") or 0.0
    for r in rows:
        d = os.path.join(problems, r["id"])
        rounds = len(glob.glob(os.path.join(d, "status-*.txt")))
        done = [n for n in range(1, 11) if os.path.exists(os.path.join(d, f"status-{n}.txt")) and open(os.path.join(d, f"status-{n}.txt")).read().strip() == "done"]
        r["rounds"] = rounds
        r["done_at"] = done[0] if done else "-"
        r["solved"] = r["all"] not in ("-", "") and r["all"].split("/")[0] == r["all"].split("/")[1]
    n = len(rows)
    solved = sum(1 for r in rows if r["solved"])
    tot = dict(calls=sum(r["calls"] for r in rows), tin=sum(r["tin"] for r in rows), tout=sum(r["tout"] for r in rows), cost=sum(r["cost"] for r in rows), seconds=sum(r["seconds"] for r in rows))
    if md:
        print(f"| problem | done at round | calls | tokens in / out | cost | seconds | public samples | private tests |")
        print("|---|---|---|---|---|---|---|---|")
        for r in rows:
            print(f"| {r['id']} | {r['done_at']} | {r['calls']} | {r['tin']} / {r['tout']} | ${r['cost']:.3f} | {r['seconds']} | {r['samples']} | {r['all']} |")
        print(f"\n**{solved} of {n} solved on all tests** · {tot['calls']} calls · {tot['tin']} in / {tot['tout']} out · ${tot['cost']:.2f} · {tot['seconds']//60} min of wall clock summed over problems")
    else:
        print("%-12s %-6s %5s %9s %8s %8s %6s %8s %9s" % ("problem", "done@", "calls", "in", "out", "cost", "sec", "samples", "private"))
        for r in rows:
            print("%-12s %-6s %5d %9d %8d %8.3f %6d %8s %9s" % (r["id"], r["done_at"], r["calls"], r["tin"], r["tout"], r["cost"], r["seconds"], r["samples"], r["all"]))
        print("--- %d of %d solved on all tests · %d calls · %d in / %d out · $%.2f · %d min summed" % (solved, n, tot["calls"], tot["tin"], tot["tout"], tot["cost"], tot["seconds"] // 60))

if __name__ == "__main__":
    main()
