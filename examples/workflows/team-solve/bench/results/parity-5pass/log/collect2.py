"""collect.py <out dir>: the evidence for the five parity passes on the ten.

For every run: problem.md, msg/ (every request's system and user message), reply/ (every reply),
state.json (calls in order, which hit the cap), solution.py, the grader's full output (public and
hidden), and the run's meter rows (cost, tokens). Plus the authors' own per-pass verdicts from
their repo, the parity test and its result, and a summary table."""
import csv, glob, json, os, re, shutil, subprocess, sys, urllib.request

S = os.path.dirname(os.path.abspath(__file__))
OUT = sys.argv[1]
TEN = ["arc196_a", "arc196_b", "arc196_c", "arc196_d", "3777", "abc400_e", "abc400_g", "3717", "3765", "abc399_e"]
GRADE = os.path.expanduser("~/Dev/aktapus/examples/workflows/team-solve/bench/grade.py")
PY = f"{S}/lcb/venv/bin/python"
ENV = dict(os.environ, LCB_BENCH="$HOME/memdoor-coder/lcb-bench",
           LCB_RUNNER="$HOME/Dev/GVS5H/codebase/livecodebench")
AUTH = os.path.expanduser("~/Dev/GVS5H/codebase/v2-current/escalation/runs/firstparty-128k-reasoning-on-5pass/results")
WF = "$HOME/memdoor-coder/lcb-bench/workflows/gvs5h-parity"

meter = [json.loads(l) for l in open(f"{S}/home2/.memdoor/meter.jsonl")]
os.makedirs(OUT, exist_ok=True)

# which host served each call: OpenRouter's generation record, cached (lookups are free)
HC = f"{S}/hosts-cache.json"
hosts = json.load(open(HC)) if os.path.exists(HC) else {}
def host_of(gid):
    if gid and gid not in hosts:
        try:
            q = urllib.request.Request("https://openrouter.ai/api/v1/generation?id=" + gid,
                                       headers={"Authorization": "Bearer " + os.environ["OPEN_ROUTER_API_KEY"]})
            d = json.load(urllib.request.urlopen(q, timeout=20))["data"]
            hosts[gid] = {"host": d.get("provider_name"), "generation_ms": d.get("generation_time")}
        except Exception:
            return {}
    return hosts.get(gid, {})

# runs still going are left out (collected on the next pass)
running = set()
for pid in subprocess.run(["pgrep", "-f", "workflow run --wait"], capture_output=True, text=True).stdout.split():
    o = subprocess.run(["lsof", "-a", "-p", pid, "-d", "cwd", "-Fn"], capture_output=True, text=True).stdout
    running |= {l[1:] for l in o.splitlines() if l.startswith("n")}

def collect(src, dst):
    os.makedirs(dst, exist_ok=True)
    for f in ("problem.md", "state.json", "solution.py", "plan.md", "notes.md", "tasks.json", "resume-fp8.log"):
        if os.path.exists(f"{src}/{f}"):
            shutil.copy(f"{src}/{f}", dst)
    for d in ("msg", "reply"):
        if os.path.isdir(f"{src}/{d}"):
            shutil.copytree(f"{src}/{d}", f"{dst}/{d}", dirs_exist_ok=True)
rows = []
for p in range(1, 6):
    for q in TEN:
        src = f"{S}/onecall/{q}" if p == 1 else f"{S}/onecall/p{p}/{q}"
        dst = f"{OUT}/runs/p{p}/{q}"
        if not os.path.exists(f"{src}/state.json"):
            rows.append({"pass": p, "problem": q, "hidden": "not run"})
            continue
        if os.path.realpath(src) in {os.path.realpath(r) for r in running}:
            rows.append({"pass": p, "problem": q, "hidden": "running"})
            continue
        collect(src, dst)
        grades = {}
        for kind in ("public", "private"):
            if os.path.exists(f"{src}/solution.py"):
                r = subprocess.run([PY, GRADE, f"--{kind}", q, f"{src}/solution.py"], capture_output=True, text=True, env=ENV)
                open(f"{dst}/grade-{kind}.txt", "w").write(r.stdout + r.stderr)
                grades[kind] = (r.stdout.splitlines() or [""])[0]
        # the run's id: its partition, recorded in the run directory's own .memdoor/runs.db
        st = json.load(open(f"{src}/state.json"))
        parts = set()
        for db in glob.glob(f"{src}/.memdoor/runs.db*"):
            parts |= set(re.findall(rb"2026-\d\d-\d\dT\d{6}", open(db, "rb").read()))
        for d in glob.glob(f"{src}/.memdoor/runs/*/steps/*/2026-*"):  # the outputs name the partition too
            parts.add(os.path.basename(d).encode())
        if not parts:
            raise SystemExit(f"{src}: no run partition")
        ids = set()
        for part in sorted(x.decode() for x in parts):  # a run cut by a clock and resumed has more than one
            k = part[:10].replace("-", "") + "-" + part[11:]
            ids |= {"gvs5h-parity-" + k, "gvs5h-parity-fp8-" + k}
        if os.path.exists(f"{src}/resume-fp8.log"):
            ids |= set(re.findall(r"started as (\S+)", open(f"{src}/resume-fp8.log").read()))
        run_rows = [m for m in meter if m.get("session", "").removeprefix("workflow:") in ids]
        rid = " ".join(sorted({m["session"].removeprefix("workflow:") for m in run_rows})) or " ".join(sorted(ids))
        open(f"{dst}/run-id", "w").write(rid + "\n")
        with open(f"{dst}/meter.jsonl", "w") as f:
            for m in run_rows:
                f.write(json.dumps(dict(m, **host_of(m.get("gen_id")))) + "\n")
        used = sorted({host_of(m.get("gen_id")).get("host") or "?" for m in run_rows})
        calls = st.get("calls", [])
        rows.append({"pass": p, "problem": q, "run": rid,
                     "samples": grades.get("public", ""), "hidden": grades.get("private", ""),
                     "calls": len(calls), "capped": sum(c["finish"] == "length" for c in calls),
                     "rounds": st.get("iters"), "manager_done": st.get("status") == "done",
                     "cost_usd": round(sum(m.get("cost_usd") or 0 for m in run_rows), 4),
                     "output_tokens": sum(m.get("output_tokens") or 0 for m in run_rows),
                     "hosts": ",".join(used),
                     "workflow": "moved to fp8 mid-run" if os.path.exists(f"{src}/resume-fp8.log") else ("gvs5h-parity-fp8" if any("fp8" in i for i in rid.split()) else "gvs5h-parity")})
with open(f"{OUT}/runs.tsv", "w") as f:
    w = csv.DictWriter(f, fieldnames=list({k: 0 for r in rows for k in r}), delimiter="\t")
    w.writeheader()
    w.writerows(rows)

json.dump(hosts, open(HC, "w"))

# every attempt that is not the final run (cut by a run clock, stopped for a fix, paused for slow hosts): kept as is
for src in sorted(glob.glob(f"{S}/onecall/p*/*.*/")):
    src = src.rstrip("/")
    collect(src, f"{OUT}/attempts/{os.path.basename(os.path.dirname(src))}/{os.path.basename(src)}")
for f in ("batch.tsv", "host-switch.txt", "one-call.sh", "batch.py", "resume-fp8.sh", "collect2.py"):
    if os.path.exists(f"{S}/{f}"):
        os.makedirs(f"{OUT}/log", exist_ok=True)
        shutil.copy(f"{S}/{f}", f"{OUT}/log/")

# the authors' own verdicts on the ten, per pass, from their repo: the regraded files, the paper's numbers
# (Appendix "evaluator": the stdin-mock fix, which our grader has, and four answer keys, none of them in the ten)
with open(f"{OUT}/authors.tsv", "w") as f:
    f.write("pass\tarm\t" + "\t".join(TEN) + "\tsolved\n")
    for arm in ("multiagent", "single"):
        for p in range(1, 6):
            rec = {r["question_id"]: r["passed"] for r in json.load(open(f"{AUTH}/q38_{arm}_p{p}.regraded.json"))["lcb"]["records"]}
            f.write(f"{p}\t{arm}\t" + "\t".join("1" if rec.get(q) else "0" for q in TEN) + f"\t{sum(bool(rec.get(q)) for q in TEN)}\n")
    FAB = os.path.expanduser("~/Dev/GVS5H/codebase/v2-current/escalation/runs/fable5-128k-reasoning-on-5pass/results")
    for p in range(1, 6):
        rec = {r["question_id"]: r["passed"] for r in json.load(open(f"{FAB}/fable5_single_p{p}.regraded.json"))["lcb"]["records"]}
        f.write(f"{p}\tfable5-single\t" + "\t".join("1" if rec.get(q) else "0" for q in TEN) + f"\t{sum(bool(rec.get(q)) for q in TEN)}\n")

# the workflow, the helper, and the byte-for-byte test against the authors' code
shutil.copytree(f"{WF}/lib", f"{OUT}/workflow/lib", dirs_exist_ok=True, ignore=shutil.ignore_patterns("__pycache__"))
shutil.copy(f"{WF}/gen.py", f"{OUT}/workflow/")
os.makedirs(f"{OUT}/workflow-fp8", exist_ok=True)
shutil.copy(f"{WF}-fp8/gen.py", f"{OUT}/workflow-fp8/")
r = subprocess.run(["python3", f"{WF}/lib/test_parity.py", os.path.expanduser("~/Dev/GVS5H/codebase/v2-current/escalation")],
                   capture_output=True, text=True, cwd=S)
open(f"{OUT}/parity-test.txt", "w").write(r.stdout + r.stderr)
print(open(f"{OUT}/parity-test.txt").read())
print(f"{len([r for r in rows if r.get('run')])} runs collected into {OUT}")
