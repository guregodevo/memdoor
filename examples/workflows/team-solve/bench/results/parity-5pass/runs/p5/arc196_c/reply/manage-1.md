### STATUS
continue

### NEXT
Implement the CDQ/NTT acceleration for the recurrence: in each CDQ node, add contributions from nonzero R[i] in the left half to good j in the right half by convolving R[first..last] with fact[h[start]-last .. h[last]-first] (using direct loops when nonzero_count*G is small), precompute DIF/DIT NTT roots up to the next power of two >= 2N, and stress-test the result against the existing O(N^2) solver on all small N and random medium N.

### TASKS
- [done] Derive and lock the black-index recurrence: `h[a]=#W before (a+1)-st B`, sentinel `h[N]=N`, `R[j]=(-fact[h[j]] - sum_{i<j} R[i] fact[h[j]-i]) * invfact[h[j]-j]` for possible `j`, answer `=-R[N]`, with early exits first W / last B.
- [done] Implement factorial/inv-factorial precomputation up to `N` modulo 998244353.
- [done] Implement an O(N^2) reference solver for the recurrence and a brute-force matcher for small `N`; validate on samples and small/random strings.
- [todo] Implement the CDQ/NTT accelerated solver with reduced factorial segment, direct fallback, and DIF/DIT root tables; stress-test against the O(N^2) reference.
- [todo] Tune the direct/NTT threshold and optimize the Python NTT implementation for `N=2e5` (memory, transform sizes, root precomputation).
- [todo] Run final validation on samples, exhaustive small cases, random medium cases, and a large performance test, then finalize the submitted code.
