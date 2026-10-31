### PLAN
Compute the minimum cost to make every length-x window internally equal, using a sliding median structure (two heaps or Fenwick tree) to obtain each window's sum of absolute deviations in O(log n) per slide. Then the problem becomes choosing k non-overlapping windows with minimum total cost. Use DP over prefix length and number of chosen windows: for each position, either skip it or take the window ending there, adding its precomputed cost. Because all costs are nonnegative, minimizing for exactly k windows also minimizes for at least k windows. Return the DP value for k windows over the full array.

### TASKS
- Build a sliding-window median/cost module that, for each start index, returns sum |nums[i+j] - median| for the x elements, using two heaps with lazy deletion or a Fenwick tree over compressed values.
- Implement the DP with dp[j][i] = min cost to form j windows in prefix i, transitions dp[j][i] = min(dp[j][i-1], dp[j-1][i-x] + cost[i-x]) (1-indexed), using 64-bit integers and INF.
- Add edge-case handling for k*x == n, x near n, negative nums, and memory-efficient rolling arrays if needed.
- Validate with the provided examples and a small brute-force checker over all subsets of non-overlapping windows for random tiny inputs.
