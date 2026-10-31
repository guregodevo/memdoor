### STATUS
continue

### NEXT
Optimize/tune the current CDQ Python solver for the maximum constraints (N=4e5, M=Q=2e5): use tuple-sorted lists with C-level sort/merge or an equivalent faster merge, add active min/max coordinate pruning and early break in the timestamped segment tree, skip queries when f[j] >= max_left, tune BASE (test 128/256/512), avoid PyPy GC pitfalls, and benchmark on worst-case random/laminar/crossing inputs; if it still TLEs, switch to a faster crossing-detection approach.

### TASKS
- [done] Implement a standalone brute-force oracle and theorem validator for small cases comparing Bellman-Ford feasibility with the bad-pair condition.
- [done] Replace the oracle with the final standalone solver: read stdin; compute l/r/sign for each person; compute f[j] from last_left/last_right endpoint scans and same-sign strict crossings using CDQ over each sign list (base brute; cross Type A/B sweeps with timestamped segment tree); build prefix f and answer queries Yes iff L-1 > f[R-1]; output Yes/No.
- [done] Stress-test the final solver against the oracle on random small cases and targeted edge cases (touching endpoints, length-two intervals, opposite-sign crossings/nesting, shared endpoints, all same sign), and fix any mismatches.
- [todo] Optimize/tune the current CDQ Python solver for maximum constraints (N=4e5, M=Q=2e5) using tuple/C sort, active min/max pruning, early-break segment tree, query skips, and BASE tuning; benchmark worst-case and, if necessary, switch to a different crossing-detection approach.
