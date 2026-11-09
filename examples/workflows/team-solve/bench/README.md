# The LCB-100 bench: team workflow against a single call, Qwen3.8-27B

The parity result on the authors' first ten problems (9.8 of 10, five
passes): [memdoor.ai/docs/team-solve](https://memdoor.ai/docs/team-solve).
SWE-bench Verified, all 500 instances against a single call on DeepSeek V4.1
Flash (400 and 388): [memdoor-swe-bench/](memdoor-swe-bench/README.md), the
data in [guregodevo/swe-bench](https://github.com/guregodevo/swe-bench).
This folder: the earlier 100-problem bench.

GVS5H's 100 hardest LiveCodeBench problems (`ids.json`, release_v6, contests
of 2024-12 to 2025-04: 73 AtCoder, 27 LeetCode). Each is graded on all of its
tests, public and hidden, with LiveCodeBench's own checker. Two arms, both
Memdoor workflows on the same model and host:

- `single-lcb`: one model call writes the program (4 tasks: read, answer,
  write, grade).
- `team-llm-lcb`: the GVS5H method with every role one model call (11 tasks:
  read, plan, two ideators in parallel, solve, write, test on the samples,
  review, repair, write, verify). Fresh contexts. The roles see each other only
  through `{{ output "…" }}`, the ledger the engine keeps.

Model line on every role: `openrouter:qwen/qwen3.8-27b throughput thinking off`.

## Ranking on the 100

Every measured harness on these 100 problems, by share solved on all tests.
The paper's rows: [GVS5H](https://github.com/slee-persis/GVS5H)
`paper/tab-4new-5pass.tex` (mean of five passes, reasoning on, 128K cap) and
`paper/fig-cost-tables.tex`.

| # | harness | model | solved | $ / pass of 100 | $ / solved | source |
|---|---|---|---|---|---|---|
| 1 | GVS5H manager | Qwen3.8-Flash-Next | 93.0% | 5.76 | 0.062 | paper |
| 2 | GVS5H manager | Qwen3.8-27B | 92.4% | 47.82 | 0.52 | paper |
| 3 | GVS5H manager | DeepSeek V4.1 Flash | 92.0% | 15.43 | 0.17 | paper |
| 4 | Memdoor team-solve-fanout | DeepSeek V4.1 Flash (1017) | 91% | ≤ 5.50 | ≤ 0.060 | Memdoor, 1 pass |
| 5 | single call | Fable 5 | 90.4% | 61.11 | 0.68 | paper |
| 6 | single call | DeepSeek V4.1 Flash | 88.4% | 3.46 | 0.039 | paper |
| 7 | GVS5H manager | GPT-5.6-Terra | 88.0% | 11.71 | 0.13 | paper |
| 8 | single call | Qwen3.8-Flash-Next | 84.2% | 2.73 | 0.032 | paper |
| 9 | GVS5H manager | GPT-5.6-Luna | 81.2% | 1.50 | 0.018 | paper |
| 10 | single call | GPT-5.6-Terra | 80.8% | 3.41 | 0.042 | paper |
| 11 | single call | GPT-5.6-Luna | 70.4% | 0.41 | 0.006 | paper |
| 12 | single call | Qwen3.8-27B | 66.8% | 18.95 | 0.28 | paper |
| 13 | Memdoor team-llm-lcb, thinking off | Qwen3.8-27B | 47% | 3.27 for 13 and 14 | | Memdoor, 1 pass |
| 14 | Memdoor single-lcb, thinking off | Qwen3.8-27B | 35% | | | Memdoor, 1 pass |

- Cost: the paper's is list rate times tokens, no cache discount. Memdoor's is
  what OpenRouter billed. Row 4 includes three `team-solve` runs stopped early.
- Row 4 is one pass against means of five. The paper's 95% intervals are ±1.2
  to ±2.6 points; its rows 1 to 3 do not differ from Fable 5 significantly
  (p = 0.56 to 0.88).
- Rows 13 and 14 ran with thinking off (the reason is under "What this is
  not"); the paper ran Qwen3.8-27B with thinking on.
- Not ranked: the parity runs on the first ten problems (Memdoor's rebuild of
  the GVS5H method on Qwen3.8-27B, 9.8 of 10 over five passes, $35.79 for 50
  runs), a different sample.

## team-solve-fanout, DeepSeek V4.1 Flash, 2026-11-09

The same 100 problems, graded the same way. One pass of
[team-solve-fanout](../../../../workflows/team-solve-fanout/README.md) at main
e98b3c5, `openrouter:deepseek/deepseek-v4.1-flash-1017` on every step.

| arm | solved on all tests | AtCoder (73) | LeetCode (27) | median s / problem |
|---|---|---|---|---|
| team-solve-fanout | **91 / 100** | 65 | 26 | 1424 |

- Not solved: 7 fail a hidden test (abc392_f, abc396_e, abc397_d, abc398_g,
  abc399_e, arc190_a, arc192_b); 3692 exceeds the grader's time limit;
  arc196_c has no `solution.py`.
- 8 problems in flight on one Mac. $5.50 on OpenRouter for 6,594 calls, three
  `team-solve` runs stopped early included.
- The paper's model build predates the 1017 build.

`results/team-solve-fanout-lcb.tsv`: every problem with id, tests passed, all
tests passed, seconds.

## Result, 2026-10-25

| arm | solved on all tests | AtCoder (73) | LeetCode (27) | median s / problem |
|---|---|---|---|---|
| single call | **35 / 100** | 32.9% | 40.7% | 63 |
| team workflow | **47 / 100** | 42.5% | 59.3% | 150 |

Paired on the 100: both arms 30, team only 17, single call only 5. The bench:
828 model calls, 6.8M tokens in, 2.2M out, $3.27 on OpenRouter (Cerebras,
fp16), about 75 minutes with 6 problems in flight. `results/*.tsv`: every
problem with id, run state, samples passed, all tests passed, seconds.

## What this is not

- Not the paper's setting: GVS5H gave Qwen3.8-27B up to 128K tokens to think
  and reports 66.8% single call, 92.4% orchestrated. Here thinking is off:
  with it on, the model reasoned past the fast host's 32K output cap on a
  planning prompt and wrote nothing, and the hosts that allow 235K outputs run
  at 34 to 44 tokens a second, a day of wall clock for the set. The finding is
  the gap between the arms, not the absolute score.
- Not a Fable comparison: Fable 5's 90.4% in the paper is thinking-on, single
  call, on the same problems. Nothing here reaches it.
- An old benchmark: the contests predate the model's training cutoff, the
  contamination concern raised on the HN thread. The paired comparison on
  identical problems and settings holds; the absolute pass rate means less.

## What it took

- A task's `model:` line carries the host preference and the thinking switch
  (`throughput`, `order …`, `thinking off`): the same model at 885 tokens a
  second on the throughput host, 44 on the cheapest.
- The agent-role variant (each role a coder turn with tools), tried first on
  Groq: a 250K tokens a minute rate limit, over two dollars a problem, five to
  eight minutes a role. One call per role is the paper's shape, a tenth of a
  cent per role.
- `grade.py --private` stops at the first failing test; a timing-out solution
  otherwise costs 6 s on each of forty hidden tests for the same verdict.

## Run it again

Python 3 with `datasets<4` and numpy; the LiveCodeBench fork from the GVS5H
repo (`LCB_RUNNER=<path to codebase/livecodebench>`); a gateway with an
OpenRouter key; about four dollars.

    export LCB_BENCH=$PWD LCB_PYTHON=<python with datasets>
    $LCB_PYTHON dump.py ids.json problems.json        # the problems and tests, from Hugging Face
    python3 materialize.py                           # problems/<id>/task.md
    mkdir -p problems-single && for d in problems/*/; do mkdir -p problems-single/$(basename $d) && cp $d/task.md problems-single/$(basename $d)/; done
    MEMDOOR_GATEWAY=http://localhost:18789 ./run.sh $PWD/team-llm-lcb 6
    MEMDOOR_GATEWAY=http://localhost:18789 PROBLEMS=$PWD/problems-single ./run.sh $PWD/single-lcb 3
    python3 summarize.py
