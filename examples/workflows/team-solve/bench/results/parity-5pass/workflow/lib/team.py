"""GVS5H's multi-agent loop (escalation/multiagent.py) as a workflow's state machine.

The workflow is a fixed chain: `team.py advance`, then an llm task, then
`team.py advance` again. Each advance applies the reply the last llm task wrote
(reply/<call>.md) to the workspace, the way the authors' code does, and writes
the exact system and user messages of the next call (msg/<call>.system,
msg/<call>.user). An llm task runs only when its messages exist, so the calls
the authors would not make are skipped. State lives in state.json.
"""
import json
import os
import re
import subprocess
import sys

MAX_ITERS = 10
MAX_TASKS = 12
MAX_PLAN_CHARS = 4000
CUTOFF_MARK = "[cut off at the reply limit]"
TEMPERATURE = {"plan": 0.3, "ideate": 0.4, "manage": 0.2, "worker": 0.2, "cutoff": 0.2, "finalize": 0.2}

SOLVER_SYSTEM = (
    "You are an elite competitive programmer. Solve the given problem in Python. "
    "Think carefully about algorithmic complexity and edge cases. "
    "Output EXACTLY ONE complete, self-contained Python program inside a single "
    "```python ...``` fenced block, and nothing else after it."
)


# --- the authors' parsing helpers -------------------------------------------

def strip_think(text):
    return re.sub(r"<think>.*?</think>", "", text, flags=re.DOTALL).strip()


def strip_code(text):
    return re.sub(r"```.*?```", "[code omitted -- approach only]", text, flags=re.DOTALL).strip()


def sections(text):
    out, cur, buf = {}, None, []
    for line in strip_think(text).splitlines():
        s = line.strip()
        m = (re.match(r"^#{1,3}\s*([A-Za-z_]+)\s*$", s)
             or re.match(r"^\*\*([A-Za-z_]+)\*\*\s*:?\s*$", s)
             or re.match(r"^([A-Z_]{3,})\s*:\s*$", s))
        if m:
            if cur is not None:
                out[cur] = "\n".join(buf).strip()
            cur, buf = m.group(1).upper(), []
        else:
            buf.append(line)
    if cur is not None:
        out[cur] = "\n".join(buf).strip()
    return out


def bullets(text):
    items = []
    for line in text.splitlines():
        m = re.match(r"^\s*(?:[-*]|\d+[.)])\s+(.*\S)", line)
        if m:
            items.append(m.group(1).strip())
    return items


def extract_py(text):
    m = re.search(r"```(?:python)?\s*\n(.*?)```", strip_think(text), re.DOTALL)
    return m.group(1).strip() if m else ""


def parse_tasks(text):
    out = []
    for b in bullets(text):
        m = re.match(r"\[\s*([a-z_ ]+?)\s*\]\s*(.*)", b, re.I)
        if m:
            raw = m.group(1).lower().replace(" ", "").replace("_", "")
            desc = m.group(2).strip()
            status = "done" if raw == "done" else ("in_progress" if raw in ("wip", "inprogress") else "pending")
        else:
            desc, status = b, "pending"
        if desc:
            out.append({"id": len(out) + 1, "desc": desc, "status": status, "result": ""})
    return out[:MAX_TASKS]


def add_tasks(tasks, descs):
    have = {t["desc"].lower() for t in tasks}
    nid = max([t["id"] for t in tasks], default=0)
    for d in descs:
        d = d.strip()
        if d and d.lower() not in have and len(tasks) < MAX_TASKS:
            nid += 1
            tasks.append({"id": nid, "desc": d, "status": "pending", "result": ""})
            have.add(d.lower())


# --- workspace --------------------------------------------------------------

def read(name):
    return open(name).read() if os.path.exists(name) else ""


def write(name, content):
    d = os.path.dirname(name)
    if d:
        os.makedirs(d, exist_ok=True)
    with open(name, "w") as f:
        f.write(content)


def load():
    return json.loads(read("state.json") or "{}")


def save(st):
    write("state.json", json.dumps(st, indent=1))


def ask(st, call, system, user):
    write(f"msg/{call}.system", system)
    write(f"msg/{call}.user", user)
    st["pending"] = call


def reply_of(call):
    """The reply an llm task wrote (mario adds one newline) and whether it was cut at the cap."""
    path = f"reply/{call}.md"
    if not os.path.exists(path):
        raise SystemExit(f"no reply for {call}: the llm task did not write {path}")
    text = open(path).read()
    if text.endswith("\n"):
        text = text[:-1]
    if text == CUTOFF_MARK or text.startswith(CUTOFF_MARK + "\n"):
        return text[len(CUTOFF_MARK) + 1:], "length"
    return text, "stop"


# --- samples (the authors' _run_samples / _sample_feedback) -----------------

def stdin_tests():
    if not os.path.isdir("tests") or os.path.exists("tests/func_name"):
        return []
    ids = sorted((f[:-3] for f in os.listdir("tests") if f.endswith(".in")), key=lambda s: (len(s), s))
    return [(read(f"tests/{i}.in"), read(f"tests/{i}.out")) for i in ids]


def run_samples(python):
    code = read("solution.py")
    tests = stdin_tests()
    if not code.strip() or not tests:
        return {"ran": False}
    sol = os.path.abspath("solution.py")
    passed, fail = 0, None
    for inp, out in tests:
        exp = (out or "").strip()
        try:
            r = subprocess.run([python, sol], input=inp, capture_output=True, text=True, timeout=10)
            got = (r.stdout or "").strip()
            if r.returncode != 0 and not got:
                got = f"<runtime error: {(r.stderr or '')[:200]}>"
        except subprocess.TimeoutExpired:
            got = "<timed out (>10s)>"
        except Exception as e:
            got = f"<error: {e}>"
        if got == exp:
            passed += 1
        elif fail is None:
            fail = {"input": inp[:600], "expected": exp[:400], "got": got[:400]}
    return {"ran": True, "passed": passed, "total": len(tests), "fail": fail}


def sample_feedback(res):
    if not res or not res.get("ran"):
        return ""
    if res["passed"] == res["total"]:
        return f"[SAMPLE TESTS: PASSED all {res['total']} public samples -- the solution looks correct.] "
    f = res.get("fail") or {}
    return ("[SAMPLE TESTS: FAILED -- passed {p}/{t}. The current solution is WRONG. First failing "
            "case: input={i!r} expected={e!r} got={g!r}. Fix the bug or, if this approach keeps "
            "failing, switch to a DIFFERENT approach.] ").format(
                p=res["passed"], t=res["total"], i=f.get("input", ""),
                e=f.get("expected", ""), g=f.get("got", ""))


# --- the roles' messages ----------------------------------------------------

def plan_messages(problem):
    sys_ = (
        "You are the PRIMARY orchestrator (manager) of a small team of workers, all "
        "expert at competitive programming. "
        "Given a problem, produce a short overarching plan to solve it, then a task list "
        "the workers can pick up. Respond with EXACTLY these sections:\n"
        "### PLAN\n<3-6 sentence strategy>\n"
        "### TASKS\n<3-6 bullet tasks, each a concrete unit of work>"
    )
    return sys_, problem


def ideate_messages(problem):
    sys_ = (
        "You are the FIRST WORKER. Do NOT solve the problem and do NOT write any code. "
        "Just think about it: identify the core difficulty, then list SEVERAL DISTINCT "
        "candidate approaches (genuinely different algorithms / data structures / problem "
        "reductions, not variations of one idea), and note pitfalls for each. Describe each "
        "approach in prose only -- absolutely no code blocks; a later worker will implement. "
        "Respond with EXACTLY:\n### NOTES\n<your analysis>\n"
        "### NEXT\n<bullet list of distinct approaches to try next>"
    )
    return sys_, f"PROBLEM:\n{problem}\n\nPLAN:\n{read('plan.md')}"


def manage_messages(problem, tasks, proposals, last_summary):
    cur = read("solution.py").strip()
    task_lines = "\n".join(
        f"- [{'done' if t['status'] == 'done' else 'todo'}] {t['desc']}" for t in tasks
    ) or "(none)"
    prop_lines = "\n".join(f"- {p}" for p in proposals) or "(none)"
    sys_ = (
        "You are the PRIMARY orchestrator and manager. You OWN the task list and decide "
        "when the problem is solved. Review the current progress and the latest worker's "
        "result, then:\n"
        "- The LATEST WORKER RESULT may include a SAMPLE TESTS verdict from actually running "
        "the code. Treat it as ground truth: only set STATUS 'done' if the solution PASSED the "
        "sample tests; if it FAILED, you MUST set STATUS 'continue' and choose a task that "
        "fixes the failing case or switches to a different approach.\n"
        "- If the current solution/answer is complete and correct, set STATUS to 'done'.\n"
        "- Otherwise CURATE the task list: merge duplicates, drop finished or irrelevant "
        "items, mark completed ones [done], and fold in ONLY genuinely new sub-tasks from "
        "the proposals. Then choose the single most valuable next task.\n"
        "- IMPORTANT: if the current solution keeps failing, or the last worker made no real "
        "progress, do NOT keep refining the same idea. Switch to a DIFFERENT approach "
        "(a different algorithm / data structure / reduction) from the notes, or ask for a "
        "new one. You have many rounds -- use them to try distinct approaches, not to polish "
        "a stuck one.\n"
        "Respond with EXACTLY these sections:\n"
        "### STATUS\n<done|continue>\n"
        "### NEXT\n<exact text of the ONE task to do next; omit if done>\n"
        "### TASKS\n<curated list, one per line, each '- [done] ...' or '- [todo] ...'>"
    )
    user = (
        f"PROBLEM:\n{problem}\n\nCURRENT SOLUTION:\n"
        f"{cur or '(none yet)'}\n\nNOTES:\n{read('notes.md')}\n\n"
        f"CURRENT TASK LIST:\n{task_lines}\n\nLATEST WORKER RESULT: {last_summary}\n\n"
        f"PROPOSED NEW STEPS:\n{prop_lines}"
    )
    return sys_, user


def worker_messages(problem, desc, finalize):
    cur = read("solution.py")
    notes_fmt = (
        "### NOTES\n<the COMPLETE notes file, rewritten. You are shown the current NOTES "
        "above: fold your findings into them, keep what still matters, and DELETE anything "
        "superseded, disproven, or now obvious. This REPLACES the file, so whatever you omit "
        "is gone. Organise it as '- **Topic:** ...' bullets, under ~800 words. Do NOT use "
        "markdown headings (#, ##), bold-only lines, or ALLCAPS: lines anywhere inside this "
        "section -- the reply is split on those, so they would truncate your notes.>\n"
    )
    out_fmt = (
        "### CODE\n```python\n<the FULL updated self-contained program>\n```\n"
        + notes_fmt +
        "### NEXT\n<bullet list of remaining steps, or 'none'>\n### STATUS\n<solved|continue>"
    )
    goal = (
        "Produce the DEFINITIVE final solution now, using all notes and current work."
        if finalize else f"Complete this task: {desc}"
    )
    sys_ = (
        f"You are a WORKER subagent, {SOLVER_SYSTEM} "
        "You share a workspace with the team. Build on the current work and notes where "
        "useful -- but if your task is to try a different approach, write a FRESH solution "
        "for that approach instead of patching the stuck one. "
        f"Respond with EXACTLY these sections:\n{out_fmt}"
    )
    user = (
        f"PROBLEM:\n{problem}\n\nPLAN:\n{read('plan.md')}\n\n"
        f"NOTES:\n{read('notes.md')}\n\nCURRENT WORK:\n{cur or '(none yet)'}\n\n"
        f"YOUR TASK: {goal}"
    )
    return sys_, user


def cutoff_messages(reply, desc):
    txt = strip_think(reply) or reply
    snippet = txt if len(txt) <= 9000 else txt[:3500] + "\n...[middle omitted]...\n" + txt[-5500:]
    sys_ = (
        "A worker's solution attempt was CUT OFF when it hit the token limit. Summarize its "
        "partial attempt in 3-5 sentences: which approach it was pursuing, what it established "
        "or ruled out, how far it got, and what remained unfinished. Be concrete so another "
        "worker can resume or judge it. Do NOT try to finish the solution yourself."
    )
    return sys_, f"TASK: {desc}\n\nCUT-OFF ATTEMPT:\n{snippet}"


# --- the loop ---------------------------------------------------------------

def next_worker(st, problem):
    """The authors' while-loop head: the next worker call, or the end of the loop."""
    if st["status"] == "continue" and st["next"] and st["iters"] < MAX_ITERS:
        if st["prev"] is not None and st["next"].strip().lower() == st["prev"].strip().lower():
            st["stopped"] = "reissued"
        else:
            st["prev"] = st["next"]
            st["iters"] += 1
            st["task"] = st["next"]
            ask(st, f"worker-{st['iters']}", *worker_messages(problem, st["task"], False))
            return
    if st["status"] == "done" and read("solution.py").strip():
        st["pending"] = None
        return
    ask(st, "finalize", *worker_messages(problem, "finalize", True))


def after_worker(st, problem, summary, python):
    res = None
    if st["wrote"]:
        res = run_samples(python)
        if res.get("ran"):
            summary = sample_feedback(res) + summary
    st["res"] = res
    ask(st, f"manage-{st['iters']}", *manage_messages(problem, st["tasks"], st["nexts"], summary))


def apply_worker(st, reply, finish, finalize):
    sec = sections(reply)
    code = extract_py(sec.get("CODE", "")) or extract_py(reply)
    wrote = False
    if code:
        write("solution.py", code)
        wrote = True
    if sec.get("NOTES") and finish != "length":
        write("notes.md", sec["NOTES"].strip()[:MAX_PLAN_CHARS * 2] + "\n")
    st["nexts"] = [b for b in bullets(sec.get("NEXT", "")) if b.lower() != "none"]
    st["wrote"] = wrote
    return sec


def apply_manage(st, reply):
    cur = read("solution.py").strip()
    sec = sections(reply)
    curated = parse_tasks(sec.get("TASKS", "")) or st["tasks"]
    status = "done" if sec.get("STATUS", "").strip().lower().startswith("done") else "continue"
    if status == "done" and not cur:
        status = "continue"
    nxt = (bullets(sec.get("NEXT", ""))[:1] or [sec.get("NEXT", "").strip()])[0]
    nxt = re.sub(r"^[-*]\s*", "", nxt).strip()
    if status == "continue" and not nxt:
        todo = [t for t in curated if t["status"] != "done"]
        nxt = todo[0]["desc"] if todo else ""
    if not cur and not nxt:
        status, nxt = "continue", "Implement the full working solution for the most promising approach in the notes."
    st["status"], st["next"], st["tasks"] = status, nxt, curated


def advance(python):
    problem = read("problem.md")
    st = load()
    if not st:
        for f in ("notes.md", "plan.md", "solution.py", "tasks.json"):
            write(f, "")
        st = {"pending": None, "tasks": [], "proposals": [], "nexts": [], "status": "continue",
              "next": "", "prev": None, "iters": 0, "task": "", "wrote": False, "res": None,
              "stopped": None, "calls": []}
        ask(st, "plan", *plan_messages(problem))
        save(st)
        return
    call = st["pending"]
    if call is None or not os.path.exists(f"reply/{call}.md"):
        return  # the calls between here and the pending one were skipped
    reply, finish = reply_of(call)
    st["calls"].append({"call": call, "finish": finish})
    if call == "plan":
        sec = sections(reply)
        write("plan.md", (sec.get("PLAN") or reply.strip())[:MAX_PLAN_CHARS])
        st["tasks"] = []
        add_tasks(st["tasks"], bullets(sec.get("TASKS", "")))
        ask(st, "ideate", *ideate_messages(problem))
    elif call == "ideate":
        sec = sections(reply)
        with open("notes.md", "a") as f:
            f.write(f"\n## ideation\n{strip_code(sec.get('NOTES') or reply.strip())[:MAX_PLAN_CHARS * 2]}\n")
        st["proposals"] = [strip_code(p) for p in bullets(sec.get("NEXT", ""))]
        ask(st, "manage-0", *manage_messages(problem, st["tasks"], st["proposals"], "ideation complete"))
    elif call.startswith("manage-"):
        apply_manage(st, reply)
        res = st.get("res")
        if call != "manage-0" and res and res.get("ran") and res["passed"] < res["total"] and st["status"] == "done":
            st["status"] = "continue"
            if not st["next"]:
                st["next"] = "The solution fails the public sample tests; fix it or try a different approach."
        write("tasks.json", json.dumps(st["tasks"], indent=2))
        next_worker(st, problem)
    elif call.startswith("worker-"):
        sec = apply_worker(st, reply, finish, False)
        if finish == "length":
            st["cutoff_reply"] = reply
            ask(st, f"cutoff-{st['iters']}", *cutoff_messages(reply, st["task"]))
        else:
            status = sec.get("STATUS", "continue").strip().lower()
            summary = f"worker reported '{status}' on: {st['task'][:80]}. " + (sec.get("NOTES", "")[:200])
            after_worker(st, problem, summary, python)
    elif call.startswith("cutoff-"):
        digest = strip_think(reply).strip() or "(the cut-off attempt could not be summarized)"
        summary = (f"worker EXCEEDED THE TOKEN LIMIT and was cut off before finishing task: "
                   f"{st['task'][:80]}. No complete solution was produced -- the approach was "
                   f"too long to finish in one call, so prefer a simpler or different approach "
                   f"next. Summary of its partial thinking: {digest[:500]}")
        st.pop("cutoff_reply", None)
        after_worker(st, problem, summary, python)
    elif call == "finalize":
        apply_worker(st, reply, finish, True)
        st["pending"] = None
    save(st)


if __name__ == "__main__":
    if sys.argv[1:2] != ["advance"]:
        raise SystemExit("usage: team.py advance [python]")
    advance(sys.argv[2] if len(sys.argv) > 2 else sys.executable)
