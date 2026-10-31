### PLAN
Define prefix sums for `nums` and `cost` so any segment sum can be obtained in O(1). Let `dp[m][i]` be the minimum cost to partition the first `i` elements into exactly `m` subarrays, with transition over the start `j` of the last segment: `dp[m][i] = min_j dp[m-1][j] + (prefix_nums[i] + k*m) * (prefix_cost[i] - prefix_cost[j])`. Rewrite this transition as a line query: for each valid `j`, create a line `y = -prefix_cost[j] * x + dp[m-1][j]`, query it at `x = prefix_nums[i] + k*m`, and add `(prefix_nums[i] + k*m) * prefix_cost[i]`. Since `prefix_cost[j]` is increasing and the query `x` values are increasing for each fixed `m`, a monotone convex hull trick gives O(n) work per `m`, for O(n^2) total. Initialize `dp[0][0] = 0`, compute `m = 1..n`, and return the minimum `dp[m][n]` over all valid `m`.

### TASKS
- Compute prefix sums `S` for `nums` and `C` for `cost`, and initialize DP arrays with infinity and `dp_prev[0] = 0`.
- Implement a monotone convex hull structure for minimum queries with decreasing slopes and increasing `x`, using integer arithmetic to avoid floating-point errors.
- For each segment count `m` from `1` to `n`, add lines for valid previous cut positions `j` as `i` increases, then query the hull to fill `dp_cur[i]` for `i = m..n`.
- Handle invalid states, the base case, large integer values, and track the best answer as `min(dp_cur[n])` across all `m`.
- Verify the solution against the provided examples and a brute-force O(n^3) DP on small random tests.
