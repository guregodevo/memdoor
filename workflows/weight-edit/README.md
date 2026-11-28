# weight-edit

Tries one edit of a local model's weights and keeps it only when it beats both the checkpoint and a control on problems the run never tuned on. The control is the checkpoint with a repetition penalty: an edit that only stops answers looping until the token cap should not beat it. A janis server started with `janis serve --edit` serves the model. The run scores the checkpoint, the control and the edited model on dev problems with fixed seeds, pairs the edit with each problem by problem, and on a PASS repeats the comparison on held-out problems and seeds. The edit is fixed in `settings` before the run. The method is arXiv:2611.21885's. The default edit is its known-good one: `dt_bias` +0.5 on every linear-attention layer of a Qwen3.5 model.

```
memdoor workflow run weight-edit --wait     # .memdoor/runs/weight-edit-<run>/report.md
```

```
settings ─ checkpoint ─┬─ read-failures ──────────────────────────────────────────────────────────────┬─ report
                       └─ control ─ edit ─ re-eval ─ judge-rule ─ judge ─┬─ withdraw ──────────────────────┤
                                                                         └─ heldout-edit ─ heldout-base ─ heldout-control ─ final ─ keep ─┘
```

Every step is YAML. The measuring is `janis eval`, `janis eval compare` and `janis weights` (janis's README, "Measuring an edit").

| Step | Kind | Does |
|---|---|---|
| `settings` | template | Every setting as one JSON text; the other steps read it |
| `checkpoint` | command | `janis weights revert`, then `janis eval` on `dev` → `dev-base.json` |
| `read-failures` | llm | Reads `dev-base.json`: how many wrong answers ran out of tokens, how many finished wrong, what the tails show. It goes in the report and chooses nothing |
| `control` | command | `janis eval` on `dev` with `--repetition-penalty <control_penalty>`, the checkpoint still served → `dev-control.json` |
| `edit` | command | `janis weights shift --match <match> --by <by>`, then `janis weights save` → `edit.safetensors` |
| `re-eval` | command | `janis eval` on `dev`, same seeds → `dev-edit.json` |
| `judge-rule` | command | `janis eval compare dev-base dev-control dev-edit`: the edit against each, gained, lost, p of a one-sided exact sign test; last line PASS (gained > lost, p < alpha, against both) or FAIL |
| `judge` | decision | Scores the comparison → `judge.json`; without a decision model, judge-rule's line |
| `withdraw` | command | On a FAIL: `janis weights revert` |
| `heldout-edit` | command | On a PASS: the edited model on `heldout`, seeds from `heldout_seed` → `heldout-edit.json` |
| `heldout-base` | command | On a PASS: revert, the checkpoint on the same → `heldout-base.json` |
| `heldout-control` | command | On a PASS: the control on the same → `heldout-control.json` |
| `final` | command | On a PASS: `janis eval compare` of the three held-out scores |
| `keep` | command | On a held-out PASS: `janis weights load edit.safetensors` |
| `report` | template | `report.md`: the settings, the diagnosis, both comparisons, what the server serves now |

The dev problems decide whether the held-out ones are run, so they are a tuning set. Only `final` measures the edit. A PASS against each of two scores at alpha is an intersection-union test: it stays at level alpha with no correction. The penalty is janis's `repetition_penalty`; janis has no `presence_penalty`, and a server run with `JANIS_REPEAT_PENALTY` set ignores the request's. The run makes one edit: a second edit is a second run.

Args, all in `steps/settings.yaml` (edit `~/.memdoor/workflows/weight-edit/steps/settings.yaml`, or copy the folder to the project's `.memdoor/workflows/`):

| arg | default | |
|---|---|---|
| `janis` | `janis` | the janis binary |
| `url` | `http://127.0.0.1:8090` | the server, started with `--edit` |
| `dev`, `heldout` | empty | JSONL files of `{"question","answer"}`, the answer in `\boxed{}`; no problem in both |
| `dev_seed`, `heldout_seed` | `5000`, `9000` | problem i is asked with seed+i |
| `max_tokens` | `16384` | one reply's cap, thinking included |
| `parallel` | `1` | requests in flight: the server's `JANIS_PARALLEL` |
| `match`, `by` | `linear_attn\.dt_bias$`, `0.5` | the edit |
| `control_penalty` | `1.1` | the control's `repetition_penalty` (janis: a token seen in the last 64 generated has its logit divided by it) |
| `alpha` | `0.05` | PASS needs p below it, against the checkpoint and against the control |

| Leaves | Where |
|---|---|
| `dev-base.json`, `dev-control.json`, `dev-edit.json`, `heldout-*.json`, `edit.safetensors`, `judge.json`, `report.md` | `.memdoor/runs/weight-edit-<run>/` |
| The edit, on a held-out PASS | loaded on the server; the checkpoint otherwise |

Needs janis with `eval` and `weights`, a model of the Qwen3.5 family, and a model on your key for `read-failures`. Weight edits run on janis's MLX engine, on Apple silicon.

| Run, 2026-11-28, Qwen3.5-4B-MLX-4bit, 24 dev and 24 held-out bakery-profit problems, 2,048 tokens, `JANIS_PARALLEL=4`, `dt_bias` +0.5 | Checkpoint | Control | Edit | Edit against checkpoint | Edit against control | judge | Server at the end | $ |
|---|---|---|---|---|---|---|---|---|
| Before the control arm | 9/24 | — | 10/24 | 4 / 3, p 0.50 | — | FAIL (decision, p 0.04) | the checkpoint (withdraw) | 0.0008 |
| With the control, `repetition_penalty` 1.1 | 11/24 | 8/24 | 13/24 | 6 / 4, p 0.38 | 7 / 2, p 0.09 | FAIL (decision, p 0.03) | the checkpoint (withdraw) | 0.0017 |

| Stand-in `janis` that always reports a gain | Steps run | judge | Server at the end |
|---|---|---|---|
| prints one comparison (before the control arm) | all, held-out included | PASS (decision, p 0.98) | the edit (keep) |
| prints only the checkpoint comparison after the control arm | to withdraw | FAIL (decision, p 0.61): the condition asks for both | the checkpoint |
| prints both comparisons | all 15, held-out control included | PASS (decision, p 0.97) | the edit (keep) |

On the 4B, `read-failures` found 12 of the 15 wrong answers at the cap, still re-checking how to read the rent; the 3 that finished took the rent as a daily rate. Within one server start the same seeds give 23 of 24 replies identical, but new seeds change 8 of 24 answers, and the checkpoint scored 9 on one start and 11 on another: compare arms of one run only, and expect hundreds of problems to see a gain of a few points.
