#!/usr/bin/env python3
"""Find tests that solutions which pass the rest all fail (the audit of 2026-11-21).
   audit.py <folder>...
Every solution.py under the folders is matched to a problem by its folder name (<id> or <prefix>-<id>) or by
the statement in its task.md; up to 10 distinct ones per problem run on every test with fair.py's judge, the
wrong tests kept. A test is suspect when two or more solutions that fail at most two tests all fail it."""
import collections, concurrent.futures as cf, glob, hashlib, multiprocessing, os, random, re, sys
import fair
K = 10
norm = lambda s: re.sub(r"\W+", "", s)
starts = {norm(p["content"])[:120]: pid for pid, p in fair.probs.items()}

def problem_of(d):
    b = os.path.basename(d)
    for pid in fair.probs:
        if b == pid or b.endswith("-" + pid): return pid
    t = os.path.join(d, "task.md")
    if os.path.exists(t):
        text = norm(open(t, errors="ignore").read())
        return next((pid for k, pid in starts.items() if k in text), None)

def job(a):
    pid, sol = a
    return pid, fair.judge(pid, sol, fair.probs[pid]["public"] + fair.probs[pid]["private"])

if __name__ == "__main__":
    multiprocessing.set_start_method("fork", force=True)
    by = collections.defaultdict(dict)
    for root in sys.argv[1:]:
        for f in glob.glob(os.path.join(root, "**", "solution.py"), recursive=True):
            pid = problem_of(os.path.dirname(f))
            if pid: by[pid].setdefault(hashlib.sha1(open(f, "rb").read()).hexdigest(), f)
    random.seed(100)
    jobs = [(pid, f) for pid in sorted(by) for f in random.sample(sorted(by[pid].values()), min(K, len(by[pid])))]
    rows = collections.defaultdict(list)
    with cf.ProcessPoolExecutor(max(1, os.cpu_count() - 1)) as ex:
        for pid, res in ex.map(job, jobs): rows[pid].append(res)
    solved = 0
    for pid in sorted(rows):
        rs = rows[pid]; n = len(rs[0])
        if any(all(x == "pass" for x in r) for r in rs): solved += 1
        strong = [r for r in rs if sum(x != "pass" for x in r) <= 2]
        sus = [k + 1 for k in range(n) if len(strong) >= 2 and all(r[k] != "pass" for r in strong)]
        if sus: print(f"{pid}: suspect test(s) {sus}, failed by all {len(strong)} solutions that pass the rest ({collections.Counter(r[sus[0]-1] for r in strong).most_common(1)[0][0]})")
    print(f"{solved} of {len(rows)} problems have a solution that passes every test")
