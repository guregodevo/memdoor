### STATUS
continue
### NEXT
Run the CURRENT SOLUTION on the three provided sample inputs and report the three outputs; do not rewrite or output the full solution unless a sample fails.
### TASKS
- [done] Implement brute-force verifier, baseline open-arc DP, inclusion-exclusion DP, and CDQ/NTT solver.
- [done] Run self-test and medium random stress tests (N up to 20) comparing all solvers; fix any mismatches in CDQ/NTT or DP.
- [done] Handle edge cases (N=1, first vertex must be B, last vertex must be W, automatic N! when R[t]<t).
- [done] Add first-occurrence simplification and constant-balance polynomial-inverse shortcut for large non-diagonal cases.
- [todo] Run the current solution on the three sample inputs and report the outputs (small verification before large stress).
- [todo] Force DIRECT_M small and stress-test solve_large against solve_direct on random and structured N=3000-10000 cases with active m>2000; fix any correctness mismatches.
- [todo] If the full code is too long for a single response, simplify solve_large by removing the prefix-peeling block (keep suffix shortcut + CDQ) and re-verify samples/stress.
- [todo] Run timed N=200000 benchmarks for alternating, long-run, and random strings; fix performance bottlenecks.
- [todo] Finalize solver selection in `solve_problem`, remove fallbacks/debug code, and ensure final program reads stdin and writes stdout.
