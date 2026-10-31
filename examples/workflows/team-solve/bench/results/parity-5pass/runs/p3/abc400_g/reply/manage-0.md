### STATUS
continue

### NEXT
Implement the core Python solution using the 8-state WQS/Lagrangian DP for the even-parity 3-color selection: doubled values, bit-encoded (value, count) keys with count tie-breaking, state update new[mask] = max(old[mask], old[mask^1]+dx, old[mask^2]+dy, old[mask^4]+dz), binary search over pair penalty P in [0, 2*max_attr+1], and answer reconstruction (dp_value + P*2K)//2; use fast I/O and verify against the provided samples plus a small random exact-DP/brute-force checker.

### TASKS
- [done] Ideation: reduction to even-parity 3-color assignment and WQS/Lagrangian plan.
- [todo] Implement core WQS 8-state DP with doubled values, bit-encoded value+count keys, binary search over pair penalty P, fast I/O, and sample validation.
- [todo] Validate the WQS formula and concavity assumption on random small cases against exact count/parity DP or brute force; diagnose any mismatch in scaling, tie-breaking, or binary-search predicate.
- [todo] Add exact DP for small min(2K, N-2K), especially N-2K=0, to avoid 31 passes in extreme cases and provide a fallback/cross-check.
- [todo] Optimize worst-case middle regime for Python (unrolled 8-state updates, per-test high bound, local variables) and finalize edge-case checks: all zeros, N odd, K=N//2, tied values, maximum 1e9 values, many test cases.
