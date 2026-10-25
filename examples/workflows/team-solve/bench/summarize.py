#!/usr/bin/env python3
"""results/*.tsv -> the comparison: pass rate on all tests per arm, by platform, and the paired view."""
import json, os, collections
B = os.path.dirname(os.path.abspath(__file__))
probs = {p["id"]: p for p in json.load(open(os.path.join(B, "problems.json")))}
def load(name):
    out = {}
    path = os.path.join(B, "results", name + ".tsv")
    if not os.path.exists(path): return out
    for line in open(path):
        f = line.rstrip("\n").split("\t")
        if len(f) < 5: continue
        a = f[3].split("/")
        out[f[0]] = {"state": f[1], "samples": f[2], "all": f[3], "ok": len(a) == 2 and a[0] != "" and a[0] == a[1], "secs": int(f[4])}
    return out
arms = {n: load(n) for n in ("single-lcb", "team-llm-lcb")}
print("%-14s %8s %8s %8s %10s" % ("arm", "graded", "solved", "pass@1", "median s"))
for n, r in arms.items():
    if not r: continue
    ok = sum(1 for v in r.values() if v["ok"]); secs = sorted(v["secs"] for v in r.values())
    print("%-14s %8d %8d %7.1f%% %10d" % (n, len(r), ok, 100.0 * ok / len(r), secs[len(secs) // 2]))
print()
print("%-14s %-10s %8s %8s" % ("arm", "platform", "graded", "pass@1"))
for n, r in arms.items():
    by = collections.defaultdict(lambda: [0, 0])
    for pid, v in r.items():
        plat = "leetcode" if "LEETCODE" in probs[pid]["platform"] else "atcoder"
        by[plat][0] += 1; by[plat][1] += v["ok"]
    for plat, (g, ok) in sorted(by.items()):
        print("%-14s %-10s %8d %7.1f%%" % (n, plat, g, 100.0 * ok / g if g else 0))
both = set(arms["single-lcb"]) & set(arms["team-llm-lcb"])
if both:
    s_only = [p for p in both if arms["single-lcb"][p]["ok"] and not arms["team-llm-lcb"][p]["ok"]]
    t_only = [p for p in both if arms["team-llm-lcb"][p]["ok"] and not arms["single-lcb"][p]["ok"]]
    bothok = [p for p in both if arms["single-lcb"][p]["ok"] and arms["team-llm-lcb"][p]["ok"]]
    print("\npaired on %d problems: both solve %d, only the team %d, only the single call %d" % (len(both), len(bothok), len(t_only), len(s_only)))
    if t_only: print("  team only:", " ".join(sorted(t_only)))
    if s_only: print("  single only:", " ".join(sorted(s_only)))
