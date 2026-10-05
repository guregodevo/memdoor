# workflow-selftest

Tests the workflow engine end to end, positive and negative, in a throwaway
scratch project: parallel steps and a merge, answers flowing downstream, an
approval gate, the review loop (send a step back, approve), fail-fix-resume, a
named partition skipped on rerun, an output schema, stop, and the refusals and
failures (bad key, missing dependency, cycle, bad type, unknown workflow, a
failing proof, a wrong file, an undeclared output, a timeout). Every scenario
step writes PASS or FAIL with its evidence; the report collects them into
`SELFTEST-<partition>.md` and fails if any failed. Model steps use DeepSeek
Flash (`deepseek:deepseek-flash` in `fixtures/*/steps/*.yaml.in`). Run it with
`/workflow:workflow-selftest`; it needs `memdoor` on the PATH (or MEMDOOR_BIN).
