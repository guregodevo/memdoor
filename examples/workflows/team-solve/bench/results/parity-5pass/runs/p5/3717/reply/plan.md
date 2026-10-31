### PLAN
Precompute the minimum cost to make every length-x window constant, which is the sum of absolute deviations to the window median. Compute these costs in O(n log n) using a sliding median structure with coordinate compression/Fenwick trees or two heaps, maintaining counts and sums. Then solve a weighted interval scheduling DP: dp[j][i] is the minimum cost to choose j non-overlapping windows in the first i elements. Transition by skipping element i or taking the window ending at i, adding its cost and using dp[j-1][i-x] to enforce non-overlap, returning dp[k][n] because extra windows only add nonnegative cost, with O(kn) time and O(n) memory.

### TASKS
- Implement coordinate compression and Fenwick trees for counts and sums, with operations to add/remove a value and find the kth smallest value in the current window.
- Slide a length-x window over nums to produce cost[start] for each start: maintain the window, find its median, and compute the total absolute deviation using maintained counts and sums.
- Implement the DP over j = 1..k and prefix length i: curr[i] = min(curr[i-1], prev[i-x] + cost[i-x]) for i >= x, with prev initialized to zeros for j = 0.
- Handle edge cases and large values: ensure k*x <= n, use 64-bit/Python int for costs, and guard indices when x or k are at limits.
- Test on the provided examples and small random cases against a brute-force search over non-overlapping windows and median costs.
