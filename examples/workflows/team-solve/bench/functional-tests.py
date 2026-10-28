#!/usr/bin/env python3
"""functional-tests.py [problems dir]: for every LeetCode-style problem (a starter code, a method the
grader calls) write its public samples as tests/N.in (the arguments, one JSON value a line, as
LiveCodeBench gives them), tests/N.out (the expected value) and tests/func_name, so the sample runner
and the stress test have something to run. AtCoder problems already have tests/N.in from stdin."""
import json, os, sys
BENCH = os.environ.get("LCB_BENCH", os.path.dirname(os.path.abspath(__file__)))
problems = sys.argv[1] if len(sys.argv) > 1 else os.path.join(BENCH, "problems")
n = 0
for p in json.load(open(os.path.join(BENCH, "problems.json"))):
    fn = (p.get("metadata") or {}).get("func_name")
    d = os.path.join(problems, p["id"])
    if not fn or not os.path.isdir(d):
        continue
    t = os.path.join(d, "tests"); os.makedirs(t, exist_ok=True)
    for i, case in enumerate(p["public"], 1):
        open(os.path.join(t, f"{i}.in"), "w").write(case["input"].rstrip("\n") + "\n")
        open(os.path.join(t, f"{i}.out"), "w").write(case["output"].rstrip("\n") + "\n")
    open(os.path.join(t, "func_name"), "w").write(fn + "\n")
    n += 1
print("functional samples written for", n, "problems")
