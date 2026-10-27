#!/usr/bin/env python3
"""Grade solution.py of one LiveCodeBench problem with the benchmark's own grader.
   grade.py --public  <id> [solution.py]   the samples only (what the workflow sees)
   grade.py --private <id> [solution.py]   every test, public and hidden (the benchmark score)
Exit 0 when every graded test passes. Writes nothing but stdout."""
import json, multiprocessing, os, sys
multiprocessing.set_start_method("fork", force=True)  # macOS + Python 3.14 default (forkserver) kills the grader's Manager
BENCH = os.environ.get("LCB_BENCH", os.path.dirname(os.path.abspath(__file__)))  # where problems.json is (materialize.py wrote it)
sys.path.insert(0, os.environ.get("LCB_RUNNER", os.environ.get("LCB_RUNNER", os.path.expanduser("~/GVS5H/codebase/livecodebench"))))
from lcb_runner.evaluation.compute_code_generation_metrics import check_correctness  # noqa: E402
mode, pid = sys.argv[1], sys.argv[2]
sol = sys.argv[3] if len(sys.argv) > 3 else "solution.py"
probs = {p["id"]: p for p in json.load(open(os.path.join(BENCH, "problems.json")))}
p = probs[pid]
tests = p["public"] if mode == "--public" else p["public"] + p["private"]
sample = {"input_output": json.dumps({"inputs": [t["input"] for t in tests], "outputs": [t["output"] for t in tests],
                                      "fn_name": (p.get("metadata") or {}).get("func_name")})}
code = open(sol).read()
def grade(ts):
    smp = {"input_output": json.dumps({"inputs": [t["input"] for t in ts], "outputs": [t["output"] for t in ts],
                                       "fn_name": (p.get("metadata") or {}).get("func_name")})}
    return check_correctness(smp, code, timeout=6, debug=False)
if mode == "--public":
    res, meta = grade(tests)
else:
    # All tests, but stop at the first failure: a solution that times out would
    # otherwise cost 6 s on each of forty hidden tests for the same verdict.
    res, meta = [], None
    for t in tests:
        r, m = grade([t]); res += r; meta = m
        if not (r and (r[0] is True or r[0] == 1)):
            res += [False] * (len(tests) - len(res)); break
passed = sum(1 for r in res if r is True or r == 1)
print(f"{pid}: {passed}/{len(tests)} tests passed ({'samples' if mode == '--public' else 'all tests'})")
if passed != len(tests):
    bad = [i for i, r in enumerate(res) if not (r is True or r == 1)][:3]
    for i in bad:
        kind = {-1: "runtime error or timeout", -2: "failed (compile error or wrong output)"}.get(res[i], "wrong answer")
        print(f"  test {i+1}: {kind}")
        if mode == "--public":
            print("    input:", tests[i]["input"][:300].replace("\n", "\\n"))
            print("    expected:", tests[i]["output"][:200].replace("\n", "\\n"))
    if meta and isinstance(meta, dict) and meta.get("error_message"):
        print("  ", str(meta.get("error_message"))[:300])
sys.exit(0 if passed == len(tests) else 1)
