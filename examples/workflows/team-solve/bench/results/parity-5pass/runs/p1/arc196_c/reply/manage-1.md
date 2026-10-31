### STATUS
continue

### NEXT
Implement the fast CDQ divide-and-conquer solver for the record DP: for a segment [l,r) with mid, after dp[l:mid] is known, add to add[j] for j in [mid,r) the sum over i in [l,mid) of dp[i]*fact[v[j]-t[i]]; use direct loops when (mid-l)*(r-mid) is small, otherwise convolve A[t[i]-t[l]]=dp[i] with B[b]=fact[v[mid]-t[mid-1]+b] and add C[v[j]-v[mid]+t[mid-1]-t[l]]; include an optimized NTT for 998244353, keep the O(m^2) fallback for small m, and validate against brute force on small random strings plus all samples.

### TASKS
- [done] Derive the reduction to prefix-max constraints and the inclusion-exclusion DP over records (t_j, v_j).
- [done] Parse input, apply early exits, compute p_k, and extract records (t_j, v_j) where v_j is a newly seen p_k with v_j >= t_j.
- [done] Precompute factorials and inverse factorials up to N modulo 998244353.
- [done] Implement the direct O(m^2) DP for the record recurrence and final answer; keep it as a fallback for small m.
- [done] Implement a brute-force checker for N <= 8 and validate the record extraction and DP on small random strings.
- [todo] Implement CDQ divide-and-conquer with NTT for the record DP: cross-segment convolution formula, optimized NTT, direct threshold, and memory-safe buffers.
- [todo] Validate the fast solver against brute force on small random strings and all samples; stress-test worst-case N=2e5 patterns and tune the direct/NTT threshold.
- [todo] Finalize a single Python program that reads stdin, writes the answer to stdout, uses modulo normalization, and is memory-safe.
