#!/usr/bin/env python3
"""Grade solution.py of one LiveCodeBench problem fairly (the audit of 2026-11-21, README "Audit of the tests").
   fair.py <id> [solution.py]
Each test runs on its own. A problem that accepts more than one answer is judged by a validator of the
statement's rule; the others by LiveCodeBench's rule (stripped lines, then Decimal tokens). The tests the
audit found wrong are dropped. Exit 0 when every kept test passes. Writes nothing but stdout."""
import json, math, os, subprocess, sys, tempfile
from decimal import Decimal, InvalidOperation
BENCH = os.environ.get("LCB_BENCH", os.path.dirname(os.path.abspath(__file__)))  # where problems.json is
TIMEOUT = 6
WRONG = {"arc192_b": {21}, "arc190_a": {15}, "abc392_f": {17, 26}}  # abc397_d 42 is handled by its validator
probs = {p["id"]: p for p in json.load(open(os.path.join(BENCH, "problems.json")))}
ints = lambda s: list(map(int, s.split()))

def exact(inp, exp, out):
    a, b = [l.strip() for l in out.strip().split("\n")], [l.strip() for l in exp.strip().split("\n")]
    if len(a) != len(b): return False
    for x, y in zip(a, b):
        if x == y: continue
        try:
            if [Decimal(t) for t in x.split()] != [Decimal(t) for t in y.split()]: return False
        except InvalidOperation: return False
    return True

def cube_pair_exists(n):
    # x - y = d divides n and d^3 < n; then y^2 + d*y = (n/d - d^2)/3 has a positive root
    d = 1
    while d ** 3 < n:
        if n % d == 0:
            q = n // d - d * d
            if q > 0 and q % 3 == 0:
                q //= 3; y = (-d + math.isqrt(d * d + 4 * q)) // 2
                if any(y1 > 0 and y1 * y1 + d * y1 == q for y1 in (y - 1, y, y + 1)): return True
        d += 1
    return False

def abc397_d(inp, exp, out):
    n = int(inp.split()[0]); o = out.split()
    if not cube_pair_exists(n): return o == ["-1"]
    if len(o) != 2: return False
    x, y = map(int, o); return x > 0 and y > 0 and x ** 3 - y ** 3 == n

def abc396_e(inp, exp, out):
    v = ints(inp); n, m = v[0], v[1]; e = v[2:]
    if exp.split() == ["-1"]: return out.split() == ["-1"]
    a = ints(out)
    if len(a) != n or min(a) < 0: return False
    if any(a[e[3*i] - 1] ^ a[e[3*i+1] - 1] != e[3*i+2] for i in range(m)): return False
    return sum(a) == sum(ints(exp))

def arc190_a(inp, exp, out):
    v = ints(inp); n, m = v[0], v[1]; lr = v[2:]
    if exp.split() == ["-1"]: return out.split() == ["-1"]
    o = ints(out); k, ops = o[0], o[1:]
    if k != int(exp.split()[0]) or len(ops) != m or any(x not in (0, 1, 2) for x in ops) or sum(x != 0 for x in ops) != k: return False
    d = [0] * (n + 2)
    for i, op in enumerate(ops):
        l, r = lr[2*i], lr[2*i+1]
        if op == 1: d[l] += 1; d[r+1] -= 1
        elif op == 2: d[1] += 1; d[l] -= 1; d[r+1] += 1; d[n+1] -= 1
    c = 0
    for j in range(1, n + 1):
        c += d[j]
        if c <= 0: return False
    return True

def prime_factors(n):
    f, p = set(), 2
    while p * p <= n:
        while n % p == 0: f.add(p); n //= p
        p += 1 if p == 2 else 2
    return f | ({n} if n > 1 else set())

def arc191_c(inp, exp, out):
    ns = ints(inp)[1:]; o = ints(out)
    if len(o) != 2 * len(ns): return False
    for i, n in enumerate(ns):
        a, m = o[2*i], o[2*i+1]
        if not (1 <= a <= 10**18 and 1 <= m <= 10**18) or math.gcd(a, m) != 1 or pow(a, n, m) != 1 % m: return False
        if any(pow(a, n // p, m) == 1 % m for p in prime_factors(n)): return False
    return True

def arc195_c(inp, exp, out):
    # Yes or No is unique and taken from the stored answer; a Yes placement is checked against the rules.
    v = ints(inp); cases = [(v[1+2*i], v[2+2*i]) for i in range(v[0])]
    want = [l.strip() for l in exp.strip().split("\n") if l.strip() in ("Yes", "No")]
    o = [l.strip() for l in out.strip().split("\n") if l.strip()]; k = 0
    for (r, b), w in zip(cases, want):
        if k >= len(o) or o[k] != w: return False
        k += 1
        if w == "No": continue
        pcs = [(x.split()[0], int(x.split()[1]), int(x.split()[2])) for x in o[k:k + r + b]]; k += r + b
        if len(pcs) != r + b or sum(p == "R" for p, _, _ in pcs) != r or sum(p == "B" for p, _, _ in pcs) != b: return False
        if len({(y, z) for _, y, z in pcs}) != r + b or any(not (1 <= y <= 10**9 and 1 <= z <= 10**9) for _, y, z in pcs): return False
        for i, (p, y, z) in enumerate(pcs):
            _, y2, z2 = pcs[(i + 1) % (r + b)]; dy, dz = abs(y - y2), abs(z - z2)
            if (p == "R" and dy + dz != 1) or (p == "B" and (dy, dz) != (1, 1)): return False
    return k == len(o)

def abc385_f(inp, exp, out):
    e, o = exp.split(), out.split()
    if len(o) != 1: return False
    if "-1" in (e[0], o[0]): return e == o
    x, y = float(o[0]), float(e[0]); return abs(x - y) <= 1e-9 * max(1.0, abs(y))

def arc192_b(inp, exp, out): return out.strip().lower() == exp.strip().lower()

VALIDATORS = {f.__name__: f for f in (abc397_d, abc396_e, arc190_a, arc191_c, arc195_c, abc385_f, arc192_b)}

def judge(pid, code_path, tests):
    """One verdict per test: pass, wrong, error or tle."""
    p = probs[pid]; fn = (p.get("metadata") or {}).get("func_name"); res = []
    if fn:  # LeetCode: LiveCodeBench's call-based checker, one test per call
        sys.path.insert(0, os.environ.get("LCB_RUNNER", os.path.expanduser("~/GVS5H/codebase/livecodebench")))
        from lcb_runner.evaluation.compute_code_generation_metrics import check_correctness
        code = open(code_path).read()
        for t in tests:
            s = {"input_output": json.dumps({"inputs": [t["input"]], "outputs": [t["output"]], "fn_name": fn})}
            # The checker's child process prints "timeout occured" to fd 1: keep it off the verdict.
            sys.stdout.flush(); saved = os.dup(1); null = os.open(os.devnull, os.O_WRONLY); os.dup2(null, 1)
            try: r, _ = check_correctness(s, code, timeout=TIMEOUT, debug=False)
            finally: os.dup2(saved, 1); os.close(saved); os.close(null)
            res.append("pass" if r and r[0] is True else ("tle" if r and r[0] in (-1, -3) else "wrong"))
        return res
    check = VALIDATORS.get(pid, exact)
    with tempfile.TemporaryDirectory() as d:
        for t in tests:
            try:
                o = subprocess.run([sys.executable, "-I", os.path.abspath(code_path)], input=t["input"], capture_output=True, text=True, timeout=TIMEOUT, cwd=d)
                try: ok = check(t["input"], t["output"], o.stdout)
                except (ValueError, IndexError): ok = False
                res.append("pass" if ok else ("error" if o.returncode else "wrong"))
            except subprocess.TimeoutExpired: res.append("tle")
    return res

if __name__ == "__main__":
    pid = sys.argv[1]; sol = sys.argv[2] if len(sys.argv) > 2 else "solution.py"
    tests = probs[pid]["public"] + probs[pid]["private"]
    res = judge(pid, sol, tests)
    kept = [(k, r) for k, r in enumerate(res, 1) if k not in WRONG.get(pid, ())]
    passed = sum(r == "pass" for _, r in kept)
    print(f"{pid}: {passed}/{len(kept)} tests passed (fair{', ' + str(len(tests) - len(kept)) + ' wrong dropped' if len(kept) < len(tests) else ''})")
    for k, r in [(k, r) for k, r in kept if r != "pass"][:3]: print(f"  test {k}: {r}")
    sys.exit(0 if passed == len(kept) else 1)
