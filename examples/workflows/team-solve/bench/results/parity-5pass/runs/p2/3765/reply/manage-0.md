### STATUS
continue

### NEXT
Implement the telescoped one-dimensional O(n^2) DP in `Solution.minimumCost`: compute prefix sums `N` for `nums` and `C` for `cost`; set `dp[0] = 0`; for `i = 1..n-1`, compute `dp[i] = min_{0 <= j < i} dp[j] + N[i] * (C[i] - C[j]) + k * (C[n] - C[i])`; then return `min_{0 <= j < n} dp[j] + N[n] * (C[n] - C[j]) + k * C[n]`; run the provided examples and confirm outputs `110` and `985`.

### TASKS
- [done] Derive and select the telescoped one-dimensional DP recurrence that removes the segment-count dimension.
- [todo] Implement the O(n^2) telescoped DP in `Solution.minimumCost` and verify the two provided examples.
- [todo] Validate the implementation against a brute-force partition checker on small random inputs, including edge case `n = 1`.
- [todo] Only if performance or robustness requires it, optimize the inner minimum with a monotone convex hull using lines `y = -C[j] * x + dp[j]` queried at `x = N[i]`.
