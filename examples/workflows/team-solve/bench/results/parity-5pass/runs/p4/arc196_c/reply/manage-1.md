### STATUS
continue

### NEXT
Run `python program.py --selftest` and extend the self-test with random N=8..20 cases comparing `dp_open_arc_count` and `solve_cdq_ntt`; fix any mismatches in the CDQ/NTT implementation.

### TASKS
- [done] Implement brute-force verifier, baseline open-arc DP, inclusion-exclusion DP, and CDQ/NTT solver.
- [todo] Run self-test and medium random stress tests (N up to 20) comparing all solvers; fix any mismatches in CDQ/NTT or DP.
- [todo] Benchmark large N=200000 (worst-case alternating and random) and optimize/replace the CDQ/NTT solver if too slow.
- [done] Handle edge cases (N=1, first vertex must be B, last vertex must be W, automatic N! when R[t]<t).
- [todo] Finalize solver selection in `solve_problem`, remove fallbacks/debug code, and ensure final program reads stdin and writes stdout.
