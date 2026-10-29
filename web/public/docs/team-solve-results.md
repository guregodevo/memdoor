# Team-solve results

A paper claimed that five Qwen3.8-27B agents sharing a ledger on disk match Claude
Fable 5 on LiveCodeBench. We asked Memdoor's coder to build that harness from one
prompt, as a workflow, and ran it on the paper's own problems on our own key. Every
number here traces to a file in the
[memdoor-demos](https://github.com/guregodevo/memdoor-demos) repository: the ledger each
run left behind, the meter rows of every call, the runner's rows and the per-problem
report ([results/README.md](https://github.com/guregodevo/memdoor-demos/blob/main/results/README.md)).

## The ten

The authors' first ten problem ids, one at a time, one pass, Qwen3.8-27B with thinking on
and a 128K output cap, through the harness with a brute-force stress test as the ground
truth for "done". Beside it, the authors' 27B harness and Claude Fable 5 alone, their five
passes on the same ids.

![The ten: ours beside the authors' harness and Fable 5 alone](/team-solve-the-ten.svg)

| arm | solved of 10 |
|---|---|
| Memdoor stress harness, one pass | 7, for $4.95 including every failed attempt |
| authors' 27B harness, five passes | 9 to 10 by pass |
| authors' 27B single call, five passes | 8 to 10 by pass |
| Claude Fable 5 alone, five passes | 7 to 10 by pass |

The two misses are different in kind. abc400_g is the honest one: the stress test refused
the solution in nine rounds of ten, the hidden grader agreed, and the 27B never found it.
arc196_c is a harness loss: the plan came back empty four times, and once it landed the
brute-force reference crashed and the harness blamed the solution for three rounds. Both
gaps are fixed in the workflow and the gateway; the hundred is next.

## How it was built

One prompt, six lines, naming the paper's file, the five roles, the loop and the model line:

```
Reproduce the GVS5H harness in <path to>/escalation/multiagent.py
as a workflow named gvs5h: five agents of one model sharing a ledger on disk, the
manager and worker alternating for up to ten rounds with the public samples in tests/
as ground truth, keeping the file's roles, prompts and loop. Every role runs on
`openrouter:qwen/qwen3.8-27b thinking high order modal,coreweave,parasail,wafer`.
Then run it on the problem in this folder and show me the ledger.
```

The coder reads the paper's code, loads the [workflow skill](/docs/workflows) and writes
one YAML task per role per round: 92 tasks, the ledger passed between them through files
and `requires`, no orchestrator code. A second prompt added the stress test as standard
tasks: a brute-force reference and a random generator written once, a command task per
round that diffs the two on 200 small inputs, gated with `when:` on that round's samples
passing, and the next manager reading a mismatch as a failing test.

One line solves a problem: `/workflow:gvs5h` in the problem's folder.

## What is not apple to apple

Our grades are raw with the benchmark's own grader; the authors also report a patched
grade. They served the model on their own hardware; ours went to whichever of four
OpenRouter hosts answered first, with different output windows. Ten problems and one pass
against their hundred times five.

## What it taught

Eleven lessons, each from a run that went wrong first, in
[the write-up](https://github.com/guregodevo/memdoor-demos/blob/main/blog/what-i-learned-building-a-team-solve-harness.md):
ground truth decides the harness, the reference can be wrong, a loop needs a mechanical
stop, cap the reply, reasoning can eat the answer, a harness that knows one input format
runs blind on the other, the workflow engine is part of the harness, one problem and one
receipt at a time, let the agent build it and fix only what a run proved wrong, keep the
data.
