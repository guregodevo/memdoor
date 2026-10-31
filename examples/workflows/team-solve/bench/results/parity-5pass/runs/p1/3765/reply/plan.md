### PLAN
Use dynamic programming over the number of subarrays and the prefix length. Let `S` and `C` be prefix sums of `nums` and `cost`; a transition ending at position `e` with `j` subarrays is `min_l dp[j-1][l] + (S[e] + k*j) * (C[e] - C[l])`. Rewrite this as a constant `(S[e] + k*j) * C[e]` plus a line evaluation `min_l(dp[j-1][l] - C[l] * (S[e] + k*j))`, where each `l` defines a line with slope `-C[l]` and intercept `dp[j-1][l]`. Because `C[l]` and `S[e] + k*j` are monotone, optimize each DP layer with a monotone convex hull trick, or a Li Chao tree if preferred, in `O(n)` per layer. Iterate `j` from `1` to `n`, keep only the previous DP layer, and take the minimum `dp[j][n]` over all valid numbers of subarrays. Validate the optimized DP against a brute-force partition check on small random cases.

### TASKS
- Compute prefix sums `S` and `C` of length `n + 1`, initialize `dp_prev[0] = 0` and all other entries to a large `INF` for the zero-subarray base case.
- Implement a minimum convex hull trick supporting line insertion with monotonically decreasing slopes and queries with increasing `x`, using integer cross multiplication to avoid floating-point errors.
- For each `j` from `1` to `n`, scan `e` from `1` to `n`: add the line for `l = e - 1` if `dp_prev[l]` is finite, query at `x = S[e] + k*j`, and set `dp_cur[e] = query_result + (S[e] + k*j) * C[e]` when finite.
- Track the answer as `min(answer, dp_cur[n])` after each layer, ensure invalid states remain `INF`, and return the final minimum.
- Write a brute-force checker for small `n` and run randomized tests comparing it with the optimized solution, including the provided examples.
