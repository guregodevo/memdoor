### STATUS
continue
### NEXT
Benchmark `solve_problem` on N=200000 for worst-case strings (alternating `BW`*N, a long initial W-run followed by alternating `WB` to stress non-diagonal active cuts, and random) with a timeout; if any case is too slow, implement the first-occurrence simplification (H_t=0 unless t is the first index with R_t=r) and a constant-balance polynomial-inverse shortcut to reduce CDQ/NTT work, then re-benchmark.
### TASKS
- [done] Implement brute-force verifier, baseline open-arc DP, inclusion-exclusion DP, and CDQ/NTT solver.
- [done] Run self-test and medium random stress tests (N up to 20) comparing all solvers; fix any mismatches in CDQ/NTT or DP.
- [done] Handle edge cases (N=1, first vertex must be B, last vertex must be W, automatic N! when R[t]<t).
- [todo] Benchmark large N=200000 (worst-case alternating and random) and optimize/replace the CDQ/NTT solver if too slow.
- [todo] Add first-occurrence simplification (H_t=0 unless t is the first index with R_t=r) and constant-balance polynomial-inverse shortcut for large non-diagonal cases.
- [todo] Finalize solver selection in `solve_problem`, remove fallbacks/debug code, and ensure final program reads stdin and writes stdout.
