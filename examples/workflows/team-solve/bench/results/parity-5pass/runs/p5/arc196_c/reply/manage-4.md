### STATUS
continue
### NEXT
Rewrite solution.py from scratch as a short, complete Python program that implements only the O(M^2) block-start recurrence solver (no NTT/CDQ/FPS). Read N and S. If S[0]=='W' or S[-1]=='B', print 0. Compute h[a] for a=1..N-1 as the number of Ws before the (a+1)-th B. Precompute fact and invfact up to N modulo 998244353. Build the active list of 1-indexed a where h[a]>=a and (a==1 or h[a]!=h[a-1]). If active is empty, print fact[N]. Otherwise compute A[m] in increasing m: A[m] = -invfact[h[a]-a] * (fact[h[a]] + sum_{p<m} A[p]*fact[h[a]-j[p]]) mod MOD, where j[p] is the active index. Answer = fact[N] + sum_m A[m]*fact[N-j[m]] mod MOD. Include an optional --selftest that checks the three samples and brute force for small N, but normal execution must just read stdin and print the answer. Make the three provided samples pass.
### TASKS
- [done] Derive and lock the black-index recurrence, the block-start simplification (only first index of each equal-h block can be nonzero), and the answer formula.
- [todo] Rewrite solution.py as a complete O(M^2) baseline solver for the block-start recurrence (no NTT/CDQ), with early exits, factorial precomputation, and selftest; make the three samples pass.
- [todo] Add a fast CDQ divide-and-conquer solver for the block-start recurrence using hybrid direct/NTT convolutions over active indices, with small-segment direct cutoff and j-span/h-span length control.
- [todo] Validate the fast solver against the O(M^2) baseline on exhaustive small N and random medium N, run a large N performance test, and finalize the submitted Python code.
