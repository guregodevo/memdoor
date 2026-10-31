### STATUS
continue
### NEXT
Implement and validate the full Python solution using the parity DP + Lagrangian (Aliens) approach: parse input, precompute shifted values 2*X*SHIFT, 2*Y*SHIFT, 2*Z*SHIFT with SHIFT=N+1, implement an unrolled 8-state packed DP calc(lam) that maximizes value*SHIFT+count (tie by maximum count) and returns (scaled_value, pair_count), binary search the largest lam with pair_count >= K, output (scaled_value + 2*lam*K)//2; run samples and small brute-force random checks (N<=8/10) to confirm correctness before finalizing.
### TASKS
- [done] Ideation: reduce to selecting 2K cakes with even counts per color / max-weight K-matching; use Lagrangian/Aliens with 8 parity masks; concavity expected from matching value.
- [todo] Implement full Python solution: fast input; per test precompute shifted coordinate values; unrolled 8-state packed DP calc(lam) using old states for transitions; binary search largest lam in [0, 2*maxv+1) with pair_count >= K; compute answer (scaled_value + 2*lam*K)//2.
- [todo] Validate correctness on samples and small random brute-force cases (exact colored DP/enumeration for N<=8/10, all K), including edge cases all zeros, N=2, odd N, K=N//2, ties, and 1e9 values; if any failure, switch to a different exact approach.
- [todo] Performance check/optimize for sum N=1e5: ensure ~31 O(N) passes with unrolled DP, local bindings, precomputed shifted values, no per-item allocations in calc, and fast I/O.
- [todo] Fallback if Aliens validation fails: develop an alternative exact method (e.g., top-candidate parity correction or min-cost flow for 3 colors).
