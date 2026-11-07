# SWE-bench Verified: team-solve against a single call, DeepSeek V4.1 Flash

All 500 instances of [SWE-bench Verified](https://www.swebench.com), two arms,
both Memdoor workflows on the same model, Memdoor v1.798.0 (d98f2db).
Predictions, per-instance logs and traces, and the kit that ran it:
[guregodevo/swe-bench](https://github.com/guregodevo/swe-bench).

- `single`: one coder turn on the issue ([kit/workflows/single](https://github.com/guregodevo/swe-bench/tree/main/kit/workflows/single)).
- `team-solve`: [workflows/team-solve](https://github.com/guregodevo/memdoor/tree/d98f2db/workflows/team-solve)
  at that commit. A direct attempt first, checked with the project's own tests;
  when the check fails, a team (context, two ideators, solver, test, review,
  repair, judge) tries again, and `keep` puts the direct attempt back when
  the team did not do better.

Model line on every step: `openrouter:deepseek/deepseek-v4.1-flash`
(weights: [deepseek-ai/DeepSeek-V4.1-Flash](https://huggingface.co/deepseek-ai/DeepSeek-V4.1-Flash)).
OpenRouter served it as `deepseek-v4.1-flash-20260910`, host Morph, on every
one of twelve calls sampled across the run.

## Result, 2026-11-06

| arm | resolved | % | $ / instance | median s / instance |
|---|---|---|---|---|
| single call | **388** / 500 | 77.6% | 0.012 | 91 |
| team-solve | **400** / 500 | 80.0% | 0.022 | 258 |
| team-solve's direct attempt | 393 / 500 | 78.6% | | |

Paired on the 500: both arms 373, team only 27, single only 15, neither 85
(exact McNemar p = 0.09). Against its own direct attempt the team gained 7
and lost none (p = 0.016). On 17 instances the direct attempt ran past its
30-minute timeout and the run stopped there, unchecked (memdoor#151); its
edits are the run's result, and they count as the direct attempt's.

| repository | instances | single | direct | team |
|---|---|---|---|---|
| django | 231 | 183 | 191 | 195 |
| sympy | 75 | 58 | 59 | 60 |
| sphinx | 44 | 33 | 30 | 30 |
| matplotlib | 34 | 26 | 26 | 27 |
| scikit-learn | 32 | 31 | 30 | 31 |
| astropy | 22 | 15 | 16 | 16 |
| xarray | 22 | 17 | 16 | 16 |
| pytest | 19 | 18 | 16 | 16 |
| pylint | 10 | 3 | 4 | 4 |
| requests | 8 | 3 | 4 | 4 |
| seaborn | 2 | 0 | 0 | 0 |
| flask | 1 | 1 | 1 | 1 |

Scored with SWE-bench's harness (swebench 5.0.2). The resolved lists in each
entry's `results/` were re-derived from every instance's test output by
`swebench submit package`, and `swebench submit verify` passes on both (SWE-bench
main, 02e7a74). Cost is OpenRouter's own `cost_usd` per call, summed from each
run's meter: $6.01 for the single arm, $10.84 for team-solve.

## How it was run

- One container per instance and arm, from the instance's `swebench/sweb.eval.*`
  image (the repository at `/testbed`, its environment installed), with its
  own Memdoor gateway inside. 4 machines, 6 instances in flight on each.
- Only the instance's `problem_statement` goes in: never the patch, the
  tests, `hints_text`, `FAIL_TO_PASS` or `PASS_TO_PASS`.
- No web. The container's only way out is [allow-proxy.py](https://github.com/guregodevo/swe-bench/blob/main/kit/allow-proxy.py),
  which opens openrouter.ai:443 and refuses everything else (every decision
  logged). On the 126 instances run again (below), the coder's `web_search`
  and `web_fetch` are removed before the workflow starts ([weboff.py](https://github.com/guregodevo/swe-bench/blob/main/kit/weboff.py)),
  and the run stops if any model request offers one or any agent calls one;
  none did. On the other 374 the tools were offered and no search answered:
  one `web_fetch` was called (sphinx-doc__sphinx-9230, team) and refused.
- One prediction per instance and arm. team-solve picks between its own
  attempts with the project's tests, never SWE-bench's: Best@k in the
  leaderboard's terms (`attempts: 2+`); the single call is one attempt.

## What went wrong, and what was run again

- Run 1 stopped after about 140 instances on a fault in the bench kit (a
  Docker layer failed to extract). Nothing of it is used; run 2 started again
  from the first instance on the version above.
- In run 2, `web_search` reached the web. It is OpenRouter's own server-side
  search, answered through openrouter.ai, the one host the proxy allowed.
  On 126 instances it returned results in one arm or both, often the upstream
  pull request; on 58 an answer in the team arm named the SWE-bench instance
  itself. `web_fetch` was refused every time. Those 126 instances
  ([kit/rerun-ids.txt](https://github.com/guregodevo/swe-bench/blob/main/kit/rerun-ids.txt)) were run again in both arms with
  the web tools removed, and their run-2 predictions discarded
  ([first-pass/](https://github.com/guregodevo/swe-bench/tree/main/first-pass) keeps their results and scores). The other 374,
  where no search answered, keep their run-2 predictions.
- django__django-16938 ran three times: in the first rerun pass the tools were
  removed after the workflow had read them, the agents searched, the run was
  stopped and the attempt discarded. Removal now happens before the workflow
  starts.
- That stop killed two runs before they wrote a prediction
  (matplotlib__matplotlib-23476 single, matplotlib__matplotlib-26208 team);
  they ran again.

On the 126: single 69.8%, team 71.4%; on the 374: single 80.2%, team 82.9%.
Run 2 with the search had the team at 85.2%.

## What this is not

- Not a leaderboard entry. SWE-bench Verified has accepted submissions only
  from academic teams and research labs since 2025-11-18. The two entries in
  guregodevo/swe-bench are in the leaderboard's layout so the check below works.
- Not a clean held-out set for team-solve. Before this run, pilot runs on
  172 Verified instances, and the ten named in team-solve's README
  (astropy__astropy-13033 and the nine Django instances labelled "1-4 hours"
  from versions 3.2 and 4.1), shaped it: the `keep` step, how a Check that
  already fails counts, a crash on non-ASCII prompts. Those 176 instances
  ([seen-ids.txt](https://github.com/guregodevo/swe-bench/blob/main/kit/seen-ids.txt)),
  mostly the first in the dataset's order, are where the gain is:

  | | instances | single | team | only team | only single | p |
  |---|---|---|---|---|---|---|
  | seen while building team-solve | 176 | 135 | 145 | 14 | 4 | 0.03 |
  | never seen | 324 | 253 | 255 | 13 | 11 | 0.84 |

  Against its own direct attempt, where run-to-run noise does not enter
  (on 47 instances one of the two single calls, the separate one and the
  direct attempt, solved it and the other did not): 2 gained and 0 lost on the seen, 5 gained and 0 lost on the
  never seen (p = 0.06).
- Not a significant gap between the arms: 27 against 15 is p = 0.09. The gap
  between team-solve and its own direct attempt (7 against 0, p = 0.016) is.
- One run per instance and arm, not a mean of several.

## Check it

    git clone https://github.com/guregodevo/swe-bench && cd swe-bench
    pip install "swebench @ git+https://github.com/SWE-bench/SWE-bench@02e7a74"
    d=20261106_memdoor-team-solve_deepseek-v4.1-flash
    swebench submit verify $d -s verified --logs $d/logs

How to run it again: [the kit](https://github.com/guregodevo/swe-bench#run-it-again).
