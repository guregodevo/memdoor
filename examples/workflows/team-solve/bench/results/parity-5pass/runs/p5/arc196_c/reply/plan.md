### PLAN
Model each valid pairing as a permutation between the ordered black vertices and ordered white vertices. A pairing is strongly connected exactly when no prefix of the original vertex order is “closed,” i.e., for every prefix with `a` black vertices and `c` white vertices, at least one of the first `a` black vertices is matched to a white vertex after the prefix. This can be reduced to constraints on prefix maxima of a permutation.

Transform the string into a monotone boundary by recording, for each white-count level, how many black vertices appear before it. Then use the inverse permutation viewpoint: reveal white vertices from largest to smallest and track the minimum black position seen so far. The condition becomes that this running minimum stays below a nonincreasing boundary derived from the string.

Apply inclusion-exclusion over the bad events “the first `u` revealed white vertices all lie after the boundary.” The intersection of a chosen set of bad events factorizes over intervals, giving a dynamic programming recurrence over the relevant boundary indices. The recurrence has a convolution-like term involving factorials, so compute it efficiently with divide-and-conquer plus NTT under modulo 998244353.

Finally, combine the DP result with the total number of pairings `N!` using the inclusion-exclusion signs, and output the answer modulo 998244353.

### TASKS
- Parse `N` and `S`; compute prefix counts of black vertices before each white vertex, and derive the boundary array `X_u = N - B_u` for the relevant indices `u`.
- Filter to the nontrivial indices where the corresponding bad event is possible (`X_u >= u`), preserving their original order and values.
- Implement factorial and inverse-factorial precomputation up to `N` modulo 998244353.
- Implement a CDQ divide-and-conquer routine that computes the DP recurrence  
  `dp[j] = -invfact[X_j - u_j] * (fact[X_j] + sum_{i<j} dp[i] * fact[X_j - u_i])`  
  using NTT to evaluate the convolution sums over left-to-right halves.
- Sum the inclusion-exclusion contribution `ans = N! + sum_j dp[j] * fact[N - u_j]` modulo 998244353 and print it.
