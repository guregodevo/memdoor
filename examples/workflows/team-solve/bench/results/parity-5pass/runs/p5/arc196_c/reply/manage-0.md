### STATUS
continue

### NEXT
Implement the black-index inclusion-exclusion solver and validate it before optimizing: compute `h[a]` for `a=1..N-1` as the number of Ws before the `(a+1)`-st B, set `h[N]=N`, early-exit if `S[0]=='W'` or `S[-1]=='B'`; precompute factorials/inv-factorials modulo 998244353; compute `R[1..N]` by the O(N^2) recurrence `R[j] = 0` if `j<N` and `h[j]<j`, otherwise `R[j] = (-fact[h[j]] - sum_{i<j} R[i]*fact[h[j]-i]) * invfact[h[j]-j] mod M`; answer is `(-R[N]) mod M`; then brute-force all matchings for small `N` and compare on all/random small strings plus samples.

### TASKS
- [todo] Derive and lock the black-index recurrence: `h[a]=#W before (a+1)-st B`, sentinel `h[N]=N`, `R[j]=(-fact[h[j]] - sum_{i<j} R[i] fact[h[j]-i]) * invfact[h[j]-j]` for possible `j`, answer `=-R[N]`, with early exits first W / last B.
- [todo] Implement factorial/inv-factorial precomputation up to `N` modulo 998244353.
- [todo] Implement an O(N^2) reference solver for the recurrence and a brute-force matcher for small `N`; validate on all/random small strings and samples.
- [todo] Implement the CDQ/NTT acceleration for the recurrence: for a node `[l,r]`, add contributions from left `i in [l,mid]` to right `j in [mid+1,r]` using `start = first j with h[j]>=j`, `k_min=h[start]-mid`, `k_max=h[r]-l`, convolve `R[l..mid]` with `fact[k_min..k_max]`, and add `conv[h[j]-(l+k_min)]` to `add[j]`.
- [todo] Add a direct-fallback path for small CDQ nodes, maintain `add[]` modulo, and use a `next_good` array for `h[j]>=j`.
- [todo] Optimize the Python NTT (precomputed roots, no bit-reversal if possible, power-of-two lengths) and tune the direct/NTT threshold for `N=2e5`.
- [todo] Run final tests on the provided samples and random small/medium cases, then output the answer for the full input.
