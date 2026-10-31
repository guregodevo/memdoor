### PLAN
If vertex 1 is white or vertex 2N is black, the answer is 0; otherwise strong connectivity is equivalent to a set of prefix conditions just before black vertices. For k=1..N-1, let A_k be the number of whites before black k+1; a pairing (permutation p from black rank to white rank) is valid iff max_{j≤k} p_j > A_k whenever A_k≥k. Events with the same A_k are nested, so only the smallest k for each value b=A_k matters; denote it m_b. Inclusion-exclusion over these minimal events gives the triangular recurrence x_b = -(fact[b]+Σ_{c<b} x_c fact[b-m_c]) / fact[b-m_b], and the answer is fact[N]+Σ_b x_b fact[N-m_b]. The remaining sum is an online convolution with the factorial kernel, computed by CDQ divide-and-conquer with NTT (or an equivalent relaxed convolution) in near-linear time.

### TASKS
- Read N,S; precompute factorials and inverse factorials up to N modulo 998244353; if S[0]=='W' or S[-1]=='B', print 0 and exit.
- Scan S to fill m_of_b[b] = smallest k (1..N-1) such that A_k=b and b>=k; if no such b exists, print fact[N] and exit.
- Implement a fast NTT convolution for MOD=998244353 (primitive root 3), with a direct O(ab) fallback for small arrays.
- Implement CDQ over b=0..N to compute x_b: H[b] accumulates Σ_{c<b} x_c fact[b-m_c]; at leaf b set x_b = -(fact[b]+H[b])*invfact[b-m_of_b[b]] if selected, else 0; after the left half is solved, build A[m_of_b[b]] += x_b for selected b in the left half, convolve A with fact[0..r], and add the coefficients to H[b] for b in the right half.
- Compute ans = fact[N] + Σ_{selected b} x_b * fact[N-m_of_b[b]] modulo MOD and print it.
