### PLAN
First observe that the base path plus added edges is strongly connected iff every prefix cut {1..i} has at least one added edge from the right to the left, which immediately forces S_1=B and S_{2N}=W. For a pairing, list black vertices in order and let π_k be the rank of the white vertex matched to the k-th black; if p_k is the number of white vertices before the (k+1)-th black, the condition becomes max(π_1..π_k) ≥ p_k+1 for k=1..N-1. Thus we must count permutations of [N] satisfying these prefix-maximum lower bounds. Inclusion-exclusion over the first index where p_k attains each value v (with v≥k) gives a triangular DP dp_j = -invfact[v_j-t_j] Σ_{i<j} dp_i fact[v_j-t_i], and the answer is Σ_j dp_j fact[N-t_j] with dp_0=1. The required sums are convolution-like with increasing t_j and v_j, so compute them by CDQ divide-and-conquer with NTT (direct for small blocks) modulo 998244353.

### TASKS
- Parse N and S; return 0 immediately if S[0] != 'B' or S[-1] != 'W'. Otherwise compute p_k for k=1..N-1 and compress to records (t_j, v_j) where v_j is a newly attained value of p and v_j ≥ t_j.
- Precompute factorials and inverse factorials up to N modulo 998244353.
- Implement a CDQ divide-and-conquer routine over the records to accumulate S_j = Σ_{i<j} dp_i fact[v_j-t_i], using direct O(L^2) evaluation for small blocks and NTT convolution of the left dp block with the appropriate factorial segment for larger blocks.
- Compute dp_j from S_j using dp_j = -S_j * invfact[v_j-t_j] mod 998244353, then compute the final answer as fact[N] + Σ_j dp_j * fact[N-t_j] mod 998244353.
- Verify the implementation on the provided samples and on small random cases by brute-forcing all N! pairings.
