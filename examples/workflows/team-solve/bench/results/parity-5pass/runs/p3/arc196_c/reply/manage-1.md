### STATUS
continue

### NEXT
Finalize the production code: remove all sample/random self-test and brute-force code from `main` so it only reads stdin, calls `solve_case`, and prints the answer; then optimize the NTT/CDQ path for `N=2e5` (DIF/DIT NTT without bit-reversal or precomputed stage roots, tighter local variable usage, tuned direct/NTT threshold, and a fast path for the alternating worst case `S == "BW" * N` if feasible), and re-verify samples plus a max-size smoke test.

### TASKS
- [done] Derive reduction to prefix events and inclusion-exclusion/renewal recurrence.
- [done] Implement input handling, factorials/invfactorials, endpoint zero cases, event extraction, and no-event case.
- [done] Implement brute-force strong-connectivity checker and small random validation harness (dev only).
- [done] Implement NTT for MOD=998244353 and CDQ divide-and-conquer with direct/NTT convolution.
- [todo] Finalize production code: remove all sample/random self-test/brute-force code from `main`, keeping only stdin -> `solve_case` -> stdout.
- [todo] Optimize NTT/CDQ for `N=2e5` (no-bit-reversal or precomputed-root NTT, local bindings, tuned threshold, and a fast path for the alternating worst case if needed); verify samples and a max-size smoke test.
- [todo] If performance is still insufficient, switch the CDQ convolution kernel to a faster approach (e.g. big-integer convolution or a block/online FFT variant).
