#!/usr/bin/env python3
"""selector.py: the evidence the selector reasons over when the fan-out's attempts tie.

    selector.py attempts <run dir> <fan-out output file>
    selector.py run <run dir>

attempts: each attempt that ran, its patch in <run dir>/attempts/<n>.patch;
the attempts with the most points in leaders.txt; and in tie.txt whether the
tie is worth settling: "differ" when two leaders change the code
differently, "same" when their code changes match, "single" when one leads.

run: each leader applied to a clean copy of the tree as the run found it
(base.txt), then the probe (<run dir>/probe) and the reproduction
(<run dir>/repro) run there, when they exist. What each printed goes into
<run dir>/selector-evidence.md, with the leader's code change. It decides
nothing: the selector (selector.yaml) reads it with the task and names the
attempt that does what the task asks. Voting on the outputs picked the
majority, and on LiveCodeBench the majority shared the bug (abc399_e,
2026-11-10); on SWE-bench Verified the fan-out's pick took a wrong attempt
on 5 of 7 runs where right and wrong attempts tied (2026-11-11).
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

TEST_PATH = re.compile(r"(^|/)(tests?|testing|__tests__|specs?)/|(^|/)test_[^/]*$|_tests?\.[^/.]+$|\.test\.[^/]+$")


def code_change(patch):
    """The patch's non-test lines, whitespace and comments aside, in order."""
    out, keep = [], False
    for line in patch.splitlines():
        if line.startswith("diff --git"):
            keep = not TEST_PATH.search(line.split(" b/")[-1])
            continue
        if keep and line[:1] in "+-" and not line.startswith(("+++", "---")):
            text = re.sub(r"#.*$", "", line[1:]).strip()
            if text:
                out.append(line[0] + re.sub(r"\s+", "", text))
    return "\n".join(sorted(out))


def attempts(run, fanout):
    data = json.load(open(fanout))
    os.makedirs(os.path.join(run, "attempts"), exist_ok=True)
    points = data.get("points") or {}
    ok = [a for a in data.get("attempts", []) if a.get("ok")]
    for a in ok:
        with open(os.path.join(run, "attempts", "%d.patch" % a["n"]), "w") as f:
            f.write(a.get("patch") or "")
    if not ok:
        leaders = []
    else:
        top = max(points.get(str(a["n"]), 0) for a in ok)
        leaders = sorted(a["n"] for a in ok if points.get(str(a["n"]), 0) == top)
    with open(os.path.join(run, "leaders.txt"), "w") as f:
        f.write(" ".join(map(str, leaders)) + "\n")
    changes = {code_change(a.get("patch") or "") for a in ok if a["n"] in leaders}
    tie = "single" if len(leaders) < 2 else ("same" if len(changes) < 2 else "differ")
    with open(os.path.join(run, "tie.txt"), "w") as f:
        f.write(tie + "\n")
    print("attempts: %d ran, leaders %s, %s" % (len(ok), leaders or "none", tie))


def normalise(text, root):
    text = text.replace(root, "<tree>")
    text = re.sub(r"0x[0-9a-fA-F]{6,}", "0x?", text)
    text = re.sub(r"\d+\.\d+s\b", "?s", text)
    return text.strip()


def evidence_of(code, here, label):
    try:
        p = subprocess.run([code], cwd=here, capture_output=True, text=True, timeout=300)
        return "%s: exit %d\n%s" % (label, p.returncode, normalise((p.stdout + p.stderr)[-3000:], here))
    except subprocess.TimeoutExpired:
        return "%s: timed out after 300 s" % label


def run(run_dir):
    run_dir = os.path.abspath(run_dir)
    leaders = open(os.path.join(run_dir, "leaders.txt")).read().split()
    probes = [(os.path.join(run_dir, name), label) for name, label in (("probe", "the probe"), ("repro", "the reproduction"))
              if os.access(os.path.join(run_dir, name), os.X_OK)]
    if len(leaders) < 2:
        print("selector: one leader, nothing to settle")
        return
    base = open(os.path.join(run_dir, "base.txt")).read().strip()
    top = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True).stdout.strip()
    prefix = subprocess.run(["git", "rev-parse", "--show-prefix"], capture_output=True, text=True).stdout.strip()
    out = []
    for n in leaders:
        patch = open(os.path.join(run_dir, "attempts", n + ".patch")).read()
        seen = []
        parent = tempfile.mkdtemp(prefix="memdoor-selector-")
        tree = os.path.join(parent, "tree")
        try:
            subprocess.run(["git", "-C", top, "worktree", "add", "-q", "--detach", tree, base], check=True, capture_output=True)
            here = os.path.join(tree, prefix)
            applied = subprocess.run(["git", "-C", here, "apply", "--whitespace=nowarn", "-"], input=patch, capture_output=True, text=True) if patch.strip() else None
            if applied is not None and applied.returncode != 0:
                seen.append("the change does not apply to the tree as the run found it")
            else:
                for code, label in probes:
                    seen.append(evidence_of(code, here, label))
        finally:
            subprocess.run(["git", "-C", top, "worktree", "remove", "--force", tree], capture_output=True)
            shutil.rmtree(parent, ignore_errors=True)
        change = "\n".join(l for l in patch.splitlines() if not l.startswith("index "))[:6000]
        out.append("## Attempt %s\n\n### Its change\n\n```diff\n%s\n```\n\n### What it did\n\n```\n%s\n```\n" % (n, change, "\n\n".join(seen) or "no probe and no reproduction ran"))
    with open(os.path.join(run_dir, "selector-evidence.md"), "w") as f:
        f.write("# The tied attempts\n\n" + "\n".join(out))
    print("selector: evidence for attempts %s (%s)" % (", ".join(leaders), ", ".join(l for _, l in probes) or "no probe, no reproduction"))


if __name__ == "__main__":
    if len(sys.argv) == 4 and sys.argv[1] == "attempts":
        attempts(sys.argv[2], sys.argv[3])
    elif len(sys.argv) == 3 and sys.argv[1] == "run":
        run(sys.argv[2])
    else:
        sys.exit(__doc__)
