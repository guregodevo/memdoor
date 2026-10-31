### STATUS
continue

### NEXT
Replace the current CDQ/NTT solver with the simplified 1-D recurrence derived from the inclusion-exclusion/Cartesian-right-spine reduction: compute `h[a]` for `a=1..N-1`, then for `t=1..N-1` with `h[t] >= t` compute `A[t] = -invfact[h[t]-t] * (fact[h[t]] + sum_{u<t} A[u] * fact[h[t]-u])` (otherwise `A[t]=0`), and output `fact[N] + sum_{t=1}^{N-1} A[t] * fact[N-t]` modulo 998244353. Validate this new implementation on the three samples, exhaustive small `N` against brute force, and random medium cases; if performance for `N=2e5` is still inadequate, optimize the online convolution for this recurrence (e.g., special-case `h[t]=t` with FPS inverse, or use a faster CDQ/relaxed convolution) rather than the previous per-index CDQ.

### TASKS
- [done] Derive and lock the black-index recurrence: `h[a]=#W before (a+1)-st B`, sentinel `h[N]=N`, `R[j]=(-fact[h[j]] - sum_{i<j} R[i] fact[h[j]-i]) * invfact[h[j]-j]` for possible `j`, answer `=-R[N]`, with early exits first W / last B.
- [done] Implement factorial/inv-factorial precomputation up to `N` modulo 998244353.
- [done] Implement an O(N^2) reference solver for the recurrence and a brute-force matcher for small `N`; validate on samples and small/random strings.
- [done] Fix the `good_pref` off-by-one in `solve_fast` (length `n + 2`), audit array lengths, and make the built-in selftest (samples, exhaustive N<=5, random medium, forced NTT) pass.
- [todo] Replace the current CDQ/NTT solver with the simplified 1-D recurrence derived from the inclusion-exclusion/Cartesian-right-spine reduction: compute `h[a]` for `a=1..N-1`, then for `t=1..N-1` with `h[t] >= t` compute `A[t] = -invfact[h[t]-t] * (fact[h[t]] + sum_{u<t} A[u] * fact[h[t]-u])` (otherwise `A[t]=0`), and output `fact[N] + sum_{t=1}^{N-1} A[t] * fact[N-t]` modulo 998244353.
- [todo] Validate the new simplified solver on the three samples, exhaustive small `N` against brute force, and random medium cases.
- [todo] Tune/optimize the online convolution for the simplified recurrence for `N=2e5` (special-case constant `h[t]-t`, e.g. `h[t]=t`, with FPS inverse or a faster CDQ/relaxed convolution); run a large performance test and finalize the submitted code.
