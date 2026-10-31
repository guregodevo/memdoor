### STATUS
continue
### NEXT
Fix the `IndexError` in `solve_fast` by allocating `good_pref = [0] * (n + 2)` (and audit all prefix/array lengths for off-by-one), then run the three samples and `python solution.py --selftest`; if any mismatch remains, debug the CDQ/NTT contribution mapping.
### TASKS
- [done] Derive and lock the black-index recurrence: `h[a]=#W before (a+1)-st B`, sentinel `h[N]=N`, `R[j]=(-fact[h[j]] - sum_{i<j} R[i] fact[h[j]-i]) * invfact[h[j]-j]` for possible `j`, answer `=-R[N]`, with early exits first W / last B.
- [done] Implement factorial/inv-factorial precomputation up to `N` modulo 998244353.
- [done] Implement an O(N^2) reference solver for the recurrence and a brute-force matcher for small `N`; validate on samples and small/random strings.
- [todo] Fix the `good_pref` off-by-one in `solve_fast` (length `n + 2`), audit array lengths, and make the built-in selftest (samples, exhaustive N<=5, random medium, forced NTT) pass.
- [todo] Tune the direct/NTT threshold and optimize the Python NTT/CDQ implementation for `N=2e5` (transform sizes, root tables, modulo operations, direct fallback); if performance is still inadequate, switch to a faster online-convolution or specialized constant-`h` method.
- [todo] Run final validation on samples, exhaustive small cases, random medium cases, and a large performance test, then finalize the submitted code.
