#!/usr/bin/env python3
"""selector.py: settles the fan-out's tie by what the tied attempts do.

    selector.py attempts <run dir> <fan-out output file>
    selector.py run <run dir>

attempts: each attempt that ran, its patch in <run dir>/attempts/<n>.patch;
the attempts with the most points in leaders.txt; and in tie.txt whether the
tie is worth settling: "differ" when two leaders change the code
differently, "same" when their code changes match, "single" when one leads.

run: each leader applied to a clean copy of the tree as the run found it
(base.txt), the probe (<run dir>/probe) run there. The leaders whose probe
printed the same thing group together; the answer is the lowest attempt in
the largest group, or `none` when the largest groups tie or there is no
probe, and choose falls back to prefer. <run dir>/selector-evidence.md keeps
each leader's change and what it printed.

On the 7 SWE-bench Verified runs where right and wrong attempts tied
(DeepSeek V4.1 Flash, every attempt scored, 2026-11-11), this took a right
attempt where prefer took a wrong one twice, and a wrong one where prefer was
right once: two wrong attempts that share a mistake outvote a right one. A
model reading the same evidence did worse (#241).
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

TEST_PATH = re.compile(r"(^|/)(tests?|testing|__tests__|specs?)/|(^|/)test_[^/]*$|_tests?\.[^/.]+$|\.test\.[^/]+$")


def sh(args, **kw):
    """subprocess.run with its output captured as UTF-8 text, in what Python 3.6 has (its locale can be ASCII)."""
    return subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, encoding="utf-8", errors="replace", **kw)


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
    data = json.load(open(fanout, encoding="utf-8", errors="replace"))
    os.makedirs(os.path.join(run, "attempts"), exist_ok=True)
    points = data.get("points") or {}
    ok = [a for a in data.get("attempts", []) if a.get("ok")]
    for a in ok:
        with open(os.path.join(run, "attempts", "%d.patch" % a["n"]), "w", encoding="utf-8") as f:
            f.write(a.get("patch") or "")
    if not ok:
        leaders = []
    else:
        top = max(points.get(str(a["n"]), 0) for a in ok)
        leaders = sorted(a["n"] for a in ok if points.get(str(a["n"]), 0) == top)
    with open(os.path.join(run, "leaders.txt"), "w", encoding="utf-8") as f:
        f.write(" ".join(map(str, leaders)) + "\n")
    changes = {code_change(a.get("patch") or "") for a in ok if a["n"] in leaders}
    tie = "single" if len(leaders) < 2 else ("same" if len(changes) < 2 else "differ")
    with open(os.path.join(run, "tie.txt"), "w", encoding="utf-8") as f:
        f.write(tie + "\n")
    print("attempts: %d ran, leaders %s, %s" % (len(ok), leaders or "none", tie))


def normalise(text, root):
    text = text.replace(os.path.realpath(root), "<tree>").replace(root, "<tree>")
    text = re.sub(r"0x[0-9a-fA-F]{6,}", "0x?", text)
    text = re.sub(r"\d+\.\d+s\b", "?s", text)
    return text.strip()


def evidence_of(code, here, label):
    try:
        p = sh([code], cwd=here, timeout=300)
        return "%s: exit %d\n%s" % (label, p.returncode, normalise((p.stdout + p.stderr)[-3000:], here))
    except subprocess.TimeoutExpired:
        return "%s: timed out after 300 s" % label


def run(run_dir):
    run_dir = os.path.abspath(run_dir)
    leaders = open(os.path.join(run_dir, "leaders.txt"), encoding="utf-8", errors="replace").read().split()
    probe = os.path.join(run_dir, "probe")
    if len(leaders) < 2 or not os.access(probe, os.X_OK):
        print("none")
        return
    base = open(os.path.join(run_dir, "base.txt"), encoding="utf-8", errors="replace").read().strip()
    top = sh(["git", "rev-parse", "--show-toplevel"]).stdout.strip()
    prefix = sh(["git", "rev-parse", "--show-prefix"]).stdout.strip()
    seen, out = {}, []
    for n in leaders:
        patch = open(os.path.join(run_dir, "attempts", n + ".patch"), encoding="utf-8", errors="replace").read()
        parent = tempfile.mkdtemp(prefix="memdoor-selector-")
        tree = os.path.join(parent, "tree")
        try:
            sh(["git", "-C", top, "worktree", "add", "-q", "--detach", tree, base], check=True)
            here = os.path.join(tree, prefix)
            applied = sh(["git", "-C", here, "apply", "--whitespace=nowarn", "-"], input=patch) if patch.strip() else None
            if applied is not None and applied.returncode != 0:
                seen[n] = "(attempt %s: the change does not apply)" % n
            else:
                seen[n] = evidence_of(probe, here, "the probe")
        finally:
            sh(["git", "-C", top, "worktree", "remove", "--force", tree])
            shutil.rmtree(parent, ignore_errors=True)
        change = "\n".join(l for l in patch.splitlines() if not l.startswith("index "))[:6000]
        out.append("## Attempt %s\n\n### Its change\n\n```diff\n%s\n```\n\n### What it printed\n\n```\n%s\n```\n" % (n, change, seen[n]))
    with open(os.path.join(run_dir, "selector-evidence.md"), "w", encoding="utf-8") as f:
        f.write("# The tied attempts\n\n" + "\n".join(out))
    groups = {}
    for n in leaders:
        groups.setdefault(seen[n], []).append(int(n))
    with open(os.path.join(run_dir, "probe-groups.json"), "w", encoding="utf-8") as f:
        json.dump([{"attempts": v, "printed": k[:2000]} for k, v in groups.items()], f, indent=1)
    sizes = sorted((len(v) for v in groups.values()), reverse=True)
    if len(sizes) < 2 or sizes[0] == sizes[1]:
        print("none")
        return
    print("attempt %d" % min(max(groups.values(), key=len)))


if __name__ == "__main__":
    if len(sys.argv) == 4 and sys.argv[1] == "attempts":
        attempts(sys.argv[2], sys.argv[3])
    elif len(sys.argv) == 3 and sys.argv[1] == "run":
        run(sys.argv[2])
    else:
        sys.exit(__doc__)
