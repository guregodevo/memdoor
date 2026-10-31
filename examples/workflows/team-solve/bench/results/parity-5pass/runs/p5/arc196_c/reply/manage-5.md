### STATUS
continue
### NEXT
Implement the full-constraint fast solver in solution.py by keeping the existing active-event recurrence and early exits, but replacing the O(M^2) inner loop with a CDQ divide-and-conquer convolution: maintain add[m] for contributions from earlier active indices, compute A[m] = -invfact[h[m]-a[m]] * (fact[h[m]] + add[m]) at leaves, and after solving a left half add its contributions to the right half either directly for small left*right or by NTT convolution of L[x]=A[p] at x=a[p]-a[l] with R[d]=fact[d] up to h[r-1]-a[l]. Use an optimized in-place NTT for 998244353, a direct cutoff around 4096-16384, and a special fast path for M == N-1 (alternating h[a]=a) using Newton FPS inverse of F(x)=sum_{i=0}^{N-1} i! x^i to get B=1/F and answer fact[N] + sum_{i=1}^{N-1} B[i]*fact[N-i]. For small M keep the direct O(M^2) recurrence.
### TASKS
- [done] Derive and lock the black-index recurrence, the block-start simplification (only first index of each equal-h block can be nonzero), and the answer formula.
- [done] Rewrite solution.py as a complete O(M^2) baseline solver for the block-start recurrence (no NTT/CDQ), with early exits, factorial precomputation, and selftest; make the three samples pass.
- [todo] Implement the full-constraint fast solver with CDQ/NTT for the block-start recurrence, including direct small-segment cutoff, optimized NTT, optional kernel-cache, and the M==N-1 alternating FPS-inverse fast path.
- [todo] Validate the fast solver against the O(M^2) baseline on exhaustive small N and random/constructed medium N, run large N performance tests (alternating, near-alternating, random), and finalize the submitted Python code.
