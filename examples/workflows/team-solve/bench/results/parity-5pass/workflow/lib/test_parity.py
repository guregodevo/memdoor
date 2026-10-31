"""Our helper against the authors' own multiagent.py, call for call, on a scripted model.

python3 test_parity.py <path to GVS5H escalation dir>
"""
import importlib
import os
import shutil
import sys
import tempfile
from types import SimpleNamespace

HERE = os.path.dirname(os.path.abspath(__file__))
ESC = sys.argv[1]
sys.path.insert(0, HERE)
import team  # noqa: E402

PROBLEM = "### Question\nAdd two numbers.\n\n### Format: Read the inputs from stdin solve the problem and write the answer to stdout (do not directly test on the sample inputs). Enclose your code within delimiters as follows."
TESTS = [("2 3\n", "5\n"), ("10 7\n", "17\n")]
GOOD = "import sys\na,b=map(int,sys.stdin.read().split())\nprint(a+b)"
BAD = "print(0)"


def role(messages):
    s = messages[0]["content"]
    if s.startswith("You are the PRIMARY orchestrator (manager)"):
        return "plan"
    if s.startswith("You are the FIRST WORKER"):
        return "ideate"
    if s.startswith("You are the PRIMARY orchestrator and manager"):
        return "manage"
    if s.startswith("A worker's solution attempt was CUT OFF"):
        return "cutoff"
    if messages[1]["content"].endswith("Produce the DEFINITIVE final solution now, using all notes and current work."):
        return "finalize"
    return "worker"


def worker_reply(code, notes, nxt="none", status="continue"):
    return f"### CODE\n```python\n{code}\n```\n### NOTES\n- **Idea:** {notes}\n### NEXT\n- {nxt}\n### STATUS\n{status}"


SCENARIOS = {
    # plan, ideation, manager names a task, the worker solves it, the manager says done: no finalize
    "happy": {
        "plan": ["### PLAN\nAdd them.\n### TASKS\n- Read input\n- Print sum\n- Read input"],
        "ideate": ["### NOTES\nTrivial.\n```python\nx\n```\n### NEXT\n- Use int()\n- Use `eval` ```code```"],
        "manage": ["### STATUS\ncontinue\n### NEXT\n- Implement the sum\n### TASKS\n- [todo] Implement the sum\n- [done] Read input",
                   "### STATUS\ndone\n### TASKS\n- [done] Implement the sum"],
        "worker": [worker_reply(GOOD, "sum works", status="solved")],
    },
    # a capped plan, a capped worker (cut-off summary), a wrong program, a 'done' the samples
    # overrule, the same task reissued: the loop stops and finalize runs
    "hard": {
        "plan": [("We need answer user's request. Let's think about the cuts..." * 40, "length")],
        "ideate": ["no sections at all, just prose about approaches"],
        "manage": ["### STATUS\ncontinue\n### NEXT\nTry approach A\n### TASKS\n- [todo] Try approach A\n- [wip] Think",
                   "### STATUS\ncontinue\n### NEXT\n* Try approach B\n### TASKS\n- [todo] Try approach B",
                   "### STATUS\ndone\n### TASKS\n- [done] Try approach B",
                   "### STATUS\ncontinue\n### NEXT\nFix the bug\n### TASKS\n- [todo] Fix the bug",
                   "### STATUS\ncontinue\n### NEXT\nfix the bug \n### TASKS\n- [todo] Fix the bug"],
        "worker": [("We need solve. " + "derivation " * 3000, "length"),
                   worker_reply(BAD, "B is wrong somewhere", nxt="check"),
                   worker_reply(BAD, "still wrong")],
        "cutoff": ["<think>hmm</think>It tried approach A and got half way."],
        "finalize": [worker_reply(GOOD, "final", status="solved")],
    },
    # the manager never says done: ten rounds, then finalize, which is itself cut off
    "ten": {
        "plan": ["### PLAN\nP\n### TASKS\n" + "\n".join(f"- task {i}" for i in range(15))],
        "ideate": ["### NOTES\nN\n### NEXT\n- a"],
        "manage": [f"### STATUS\ncontinue\n### NEXT\nround {i}\n### TASKS\n- [todo] round {i}" for i in range(12)],
        "worker": [worker_reply(BAD if i % 2 else GOOD, f"r{i}") for i in range(10)],
        "finalize": [("cut " * 100, "length")],
    },
    # the manager answers nothing usable and no worker writes code
    "empty": {
        "plan": [""],
        "ideate": [""],
        "manage": ["", "", "", "### STATUS\ndone"] + [""] * 10,
        "worker": ["no code here"] * 10,
        "finalize": ["still nothing"],
    },
}


class Model:
    def __init__(self, script):
        self.script, self.used, self.calls = script, {}, []

    def __call__(self, messages, temperature):
        r = role(messages)
        i = self.used.get(r, 0)
        self.used[r] = i + 1
        entry = self.script[r][min(i, len(self.script[r]) - 1)]
        content, finish = (entry, "stop") if isinstance(entry, str) else entry
        self.calls.append((r, temperature, messages[0]["content"], messages[1]["content"]))
        return content, finish


def run_authors(script):
    ws_root = tempfile.mkdtemp()
    os.environ.update(MULTIAGENT_WS=ws_root, MULTIAGENT_MODEL="groq:small-model",
                      MULTIAGENT_MAX_ITERS="10", MULTIAGENT_MAX_TASKS="12")
    sys.path.insert(0, ESC)
    orch = importlib.import_module("orchestrator")
    ma = importlib.reload(importlib.import_module("multiagent"))
    model = Model(script)

    def chat(m, messages, temperature=0.2, meta=None):
        content, finish = model(messages, temperature)
        if meta is not None:
            meta["finish_reason"] = finish
        return content
    ma.chat = chat
    tests = [SimpleNamespace(testtype="stdin", input=i, output=o) for i, o in TESTS]
    cwd = os.getcwd()
    os.chdir(ws_root)
    try:
        ma.multiagent_solve(PROBLEM, orch.CODE_SPEC, tests=tests)
    finally:
        os.chdir(cwd)
    ws = os.path.join(ws_root, ma._slug(PROBLEM))
    final = open(os.path.join(ws, "solution.py")).read()
    shutil.rmtree(ws_root)
    return model.calls, final


def run_ours(script):
    d = tempfile.mkdtemp()
    os.makedirs(os.path.join(d, "tests"))
    for n, (i, o) in enumerate(TESTS, 1):
        open(os.path.join(d, "tests", f"{n}.in"), "w").write(i)
        open(os.path.join(d, "tests", f"{n}.out"), "w").write(o)
    open(os.path.join(d, "problem.md"), "w").write(PROBLEM)
    model = Model(script)
    cwd = os.getcwd()
    os.chdir(d)
    try:
        team.advance(sys.executable)
        while team.load()["pending"]:
            call = team.load()["pending"]
            system, user = open(f"msg/{call}.system").read(), open(f"msg/{call}.user").read()
            kind = "worker" if call.startswith("worker") else call.split("-")[0]
            content, finish = model([{"content": system}, {"content": user}], team.TEMPERATURE[kind])
            # the gateway trims the answer; with mark_cutoff a capped one starts with the mark;
            # mario writes the answer and a newline to the task's file target
            answer = content.strip()
            if finish == "length":
                answer = (team.CUTOFF_MARK + "\n" + answer).strip()
            os.makedirs("reply", exist_ok=True)
            open(f"reply/{call}.md", "w").write(answer + "\n")
            team.advance(sys.executable)
        final = open("solution.py").read()
    finally:
        os.chdir(cwd)
    shutil.rmtree(d)
    return model.calls, final


failed = 0
for name, script in SCENARIOS.items():
    a, af = run_authors(script)
    o, of = run_ours(script)
    same = len(a) == len(o)
    for k, (x, y) in enumerate(zip(a, o)):
        if x != y:
            same = False
            for field, xv, yv in zip(("role", "temperature", "system", "user"), x, y):
                if xv != yv:
                    print(f"  {name} call {k} ({x[0]}): {field} differs\n    authors: {str(xv)[:300]!r}\n    ours:    {str(yv)[:300]!r}")
            break
    if af != of:
        same = False
        print(f"  {name}: final solution differs")
    print(f"{name}: {'IDENTICAL' if same else 'DIFFERENT'} ({len(a)} calls authors, {len(o)} ours: {' '.join(c[0] for c in a)})")
    failed += not same
sys.exit(1 if failed else 0)
