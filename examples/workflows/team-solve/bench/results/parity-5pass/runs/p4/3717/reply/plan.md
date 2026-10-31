### PLAN
For each length-x window, the cheapest way to make it constant is to set all elements to a median, so its cost is the sum of absolute deviations from the median. Compute these costs for all windows with a sliding median structure, using coordinate compression and Fenwick trees for counts and sums to support add/remove and kth-value queries in O(log n). Then use dynamic programming over prefix positions and number of selected subarrays: dp[c][i] is the minimum cost to form c non-overlapping valid windows inside the first i elements. Transition by either skipping position i or taking the window ending at i, which adds its precomputed cost and requires the previous c-1 windows to end by i-x. Return dp[k][n], with nonnegative costs ensuring exactly k windows is sufficient for the at-least-k requirement.

### TASKS
- Build a Fenwick tree over compressed nums values that maintains window counts and value sums, with methods to add/remove a value and find the lower median by kth order statistic.
- Slide a length-x window across nums, compute each window's median cost as median*count_le - sum_le + (total_sum - sum_le) - median*(x - count_le), and store cost_by_end[i] for the window ending at i.
- Implement DP with dp[0][i]=0 and dp[c][i]=min(dp[c][i-1], dp[c-1][i-x]+cost_by_end[i]) for c=1..k and i=1..n, using INF for impossible states and 1-indexed positions.
- Test edge cases including x=n, k=1, even x, negative values, duplicate medians, and the non-overlap boundary where one window ends at i-x and the next starts at i-x+1.
- Verify final complexity is O(n log n + nk) time and O(n) memory with rolling DP arrays, and return dp[k][n] as an integer.
