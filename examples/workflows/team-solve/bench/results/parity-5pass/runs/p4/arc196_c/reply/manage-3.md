### STATUS
continue
### NEXT
Stress-test the large solver path (force DIRECT_M small) against solve_direct on random and structured N=3000-10000 cases with active m>2000, then run timed N=200000 benchmarks for alternating, long-run, and random strings; fix any correctness mismatches or performance bottlenecks.
### TASKS
- [done] Implement brute-force verifier, baseline open-arc DP, inclusion-exclusion DP, and CDQ/NTT solver.
- [done] Run self-test and medium random stress tests (N up to 20) comparing all solvers; fix any mismatches in CDQ/NTT or DP.
- [done] Handle edge cases (N=1, first vertex must be B, last vertex must be W, automatic N! when R[t]<t).
- [done] Add first-occurrence simplification and constant-balance polynomial-inverse shortcut for large non-diagonal cases.
- [todo] Stress-test the large solver path (force DIRECT_M small) against solve_direct on random and structured N=3000-10000 cases with active m>2000, then run timed N=200000 benchmarks for alternating, long-run, and random strings; fix any correctness mismatches or performance bottlenecks.
- [todo] Finalize solver selection in `solve_problem`, remove fallbacks/debug code, and ensure final program reads stdin and writes stdout.
