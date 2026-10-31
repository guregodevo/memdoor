### STATUS
continue

### NEXT
Implement the baseline direct DP and brute-force validator: parse N and S; return 0 immediately if S[0] != 'B' or S[-1] != 'W'; compute p_k = number of W before the (k+1)-th black for k=1..N-1; extract records (t_j, v_j) where v_j is a newly seen p_k and v_j >= t_j; precompute factorials and inverse factorials up to N modulo 998244353; compute dp_j = -(fact[v_j] + sum_{i<j} dp_i * fact[v_j - t_i]) * invfact[v_j - t_j] mod MOD; answer = fact[N] + sum_j dp_j * fact[N - t_j] mod MOD; also implement a brute-force checker for N <= 8 enumerating all N! matchings and checking all prefix cuts, and validate the baseline on small random cases.

### TASKS
- [done] Derive the reduction to prefix-max constraints and the inclusion-exclusion DP over records (t_j, v_j).
- [todo] Parse input, apply early exits, compute p_k, and extract records (t_j, v_j) where v_j is a newly seen p_k with v_j >= t_j.
- [todo] Precompute factorials and inverse factorials up to N modulo 998244353.
- [todo] Implement the direct O(m^2) DP for the record recurrence and final answer; keep it as a fallback for small m.
- [todo] Implement a brute-force checker for N <= 8 and validate the record extraction and DP on small random strings.
- [todo] Implement CDQ divide-and-conquer over records to compute the same DP faster; for cross [l,mid) -> [mid,r), use X = t[mid-1]-t[l]+1, Y = v[r-1]-v[mid]+1, G = v[mid]-t[mid-1], convolve A[t_i-t_l]=dp_i with B[b]=fact[G+b] of length X+Y-1, and add C[v_j-v[mid]+t[mid-1]-t[l]] to add[j]; use direct loops for small segments.
- [todo] Implement an optimized NTT for mod 998244353 (primitive root 3), tune the direct/NTT threshold, and test worst-case patterns such as alternating colors and high-balance strings up to N = 2e5.
- [todo] Finalize a single Python program that reads stdin, writes the answer to stdout, uses modulo normalization, and is memory-safe for the CDQ/NTT implementation.
