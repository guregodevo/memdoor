#!/usr/bin/env python3
"""problems.json -> problems/<id>/task.md (the LiveCodeBench prompt: statement, starter code, samples, the contract)."""
import json, os
BENCH = os.path.dirname(os.path.abspath(__file__))
for p in json.load(open(os.path.join(BENCH, "problems.json"))):
    d = os.path.join(BENCH, "problems", p["id"]); os.makedirs(d, exist_ok=True)
    fn = (p.get("metadata") or {}).get("func_name")
    lines = [f"# {p['title']}", "", p["content"].strip(), ""]
    if p["starter_code"].strip():
        lines += ["## Starter code", "", "Complete this. The grader imports solution.py and calls the method with the inputs; print nothing.", "",
                  "```python", p["starter_code"].rstrip(), "```", ""]
    else:
        lines += ["## Contract", "", "solution.py reads the input from stdin and writes the answer to stdout, exactly in the output format above.", ""]
    lines += ["## Samples", ""]
    for i, t in enumerate(p["public"], 1):
        lines += [f"### Sample {i}", "Input:", "```", t["input"].rstrip("\n"), "```", "Output:", "```", t["output"].rstrip("\n"), "```", ""]
    lines += ["## How the samples are checked", "", f"    python3 {BENCH}/grade.py --public {p['id']}", "",
              "It runs solution.py on every sample with the benchmark's grader and prints which failed and why. Hidden tests decide the final score; the limits are the usual contest limits (a few seconds, Python)."]
    open(os.path.join(d, "task.md"), "w").write("\n".join(lines))
print("materialized", len(os.listdir(os.path.join(BENCH, "problems"))), "problems")
