- **Strong connectivity reduction:** The base edges form a directed path, so forward reachability is automatic. Strong connectivity is equivalent to every prefix cut `{1..i}` having at least one added edge from the right side to the left side. This immediately forces `S_1 = B` and `S_{2N} = W`.
- **Prefix condition:** For a prefix containing `K` black vertices and `Q` white vertices, we need some black among the first `K` to be paired with a white of rank `> Q`. Thus `max(pi_1..pi_K) > Q`. The worst `Q` for fixed `K` is `p_K`, the number of whites before the `(K+1)`-th black.
- **Record pruning:** `p_K` is nondecreasing. For each newly seen value `v`, only the earliest `K` matters because later constraints with the same `v` are weaker. If `v < K`, the constraint is automatic since the maximum of `K` distinct positive ranks is at least `K`. The remaining records `(t_j, v_j)` have strictly increasing `t_j` and `v_j`, and `v_j >= t_j`.
- **Inclusion-exclusion DP:** Bad event `E_j` is “the first `t_j` white ranks are all in `[1..v_j]`.” For a selected chain `i_1 < ... < i_s`, the number of permutations satisfying all events is  
  `fact[v_i1] invfact[v_i1-t_i1] * fact[v_i2-t_i1] invfact[v_i2-t_i2] * ... * fact[N-t_is]`.  
  Define `dp_0 = 1` and  
  `dp_j = -invfact[v_j-t_j] * sum_{i<j} dp_i fact[v_j-t_i]`.  
  The answer is `sum_i dp_i fact[N-t_i]`.
- **CDQ structure:** Maintain `add[j]` as the accumulated sum over all already computed `i < j`. In `solve(l, r)`, compute the left half, add its contributions to the right half, then compute the right half. At a leaf `l > 0`, set `dp[l] = -add[l] * invfact[v_l-t_l]`.
- **Convolution mapping:** For a node `[l, r)` with `mid`, set `x_i = t_i - t_l`, `y_j = v_j - v_mid`, and `c = v_mid - t_{mid-1}`. Then  
  `v_j - t_i = c + y_j + (t_{mid-1} - t_l - x_i)`.  
  Use `A[x_i] = dp_i`, `B[b] = fact[c+b]`, compute `C = A * B`, and add `C[y_j + t_{mid-1} - t_l]` to `add[j]`.
- **Array sizes:** `A` length is `t[mid-1] - t[l] + 1`. `B` length is `v[r-1] - v[mid] + t[mid-1] - t[l] + 1`. All factorial indices stay in `[0, N]`, and the convolution length is at most `2N - 1`.
- **NTT implementation:** Uses the ACL butterfly transform for modulus `998244353` with primitive root `3`. It precomputes `sum_e`, `sum_ie`, and a `rate_index` table for least-zero-bit updates. Forward is DIF, inverse is DIT, and inverse multiplies by `inv(2^h)`. The inverse butterfly uses `(l - r) * inow`, which was verified against small convolutions.
- **Forced CDQ and testing:** `SMALL_M = 0`, so the normal solver uses `solve_cdq` for every nontrivial case. `solve_direct` is kept for comparison. A self-test is available via environment variable `SELFTEST=1`; it compares brute-force graph checking, direct DP, default CDQ, and forced-NTT CDQ on small random balanced strings.
- **Thresholds and fallbacks:** Inside CDQ, direct work is used when `left_len * right_len <= 16384`, or when that product is small compared with the estimated NTT cost `ntt_len * log2(ntt_len) * 16`. This avoids many tiny NTT calls while keeping the worst-case direct work bounded.
- **Edge cases:** If there are no records, the answer is `N!`. If `S_1 != B` or `S_{2N} != W`, the answer is `0`. The case `v_j = t_j` is valid and uses `invfact[0]`. A record with `v_j = N` is handled correctly and can force the answer to `0`.
- **Validation:** The direct DP, CDQ with direct blocks, and CDQ with forced NTT all match brute force on small random balanced strings. The convolution index mapping was checked algebraically and on small hand-computed cases.
