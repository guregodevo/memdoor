### STATUS
continue

### NEXT
Force DIRECT_M small and stress-test solve_large against solve_direct on random and structured N=3000-10000 cases with active m>2000; fix any correctness mismatches.

### TASKS
- [done] Implement brute-force verifier, baseline open-arc DP, inclusion-exclusion DP, and CDQ/NTT solver.
- [done] Run self-test and medium random stress tests (N up to 20) comparing all solvers; fix any mismatches in CDQ/NTT or DP.
- [done] Handle edge cases (N=1, first vertex must be B, last vertex must be W, automatic N! when R[t]<t).
- [done] Add first-occurrence simplification and constant-balance polynomial-inverse shortcut for large non-diagonal cases.
- [done] Run the current solution on the three sample inputs and report the outputs.
- [todo] Force DIRECT_M small and stress-test solve_large against solve_direct on random and structured N=3000-10000 cases with active m>2000; fix any correctness mismatches.
- [todo] Run timed N=200000 benchmarks for alternating, long-run, and random strings; fix performance bottlenecks.
- [todo] Finalize solver selection, remove fallbacks/debug code, and ensure final program reads stdin and writes stdout.
