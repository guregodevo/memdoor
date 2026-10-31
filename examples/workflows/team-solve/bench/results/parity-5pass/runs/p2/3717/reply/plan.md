### PLAN
Compute the minimum operations needed to make every possible length-x window constant, then select k non-overlapping windows with minimum total cost. For a fixed window, the optimal constant is a median, so maintain a sliding median and the sums of the lower/upper halves to get the sum of absolute deviations in logarithmic time per shift. Use DP over the number of selected windows and array prefix: dp[t][i] is the minimum cost to form t valid windows inside the first i positions. Transition by skipping position i or taking the window ending at i, which forces the previous t-1 windows to lie in the first i-x positions. Because costs are nonnegative, satisfying at least k windows is equivalent to choosing exactly k windows, and k is small enough for O(kn) DP.

### TASKS
- Implement a sliding-window median structure (two heaps with lazy deletion or a Fenwick tree over compressed values) that maintains the median and sums of the lower and upper halves for each length-x window.
- For every window start/end, compute its minimum constant-making cost as `median * left_count - left_sum + right_sum - median * right_count` and store it by end index.
- Implement the DP with 1D arrays: `prev` for t-1 windows and `cur` for t windows, initialized with 0 for t=0 and INF otherwise, and update `cur[i] = min(cur[i-1], prev[i-x] + cost_end[i])` for i >= x.
- Enforce non-overlap and exact length by only allowing a taken window ending at i to combine with states from prefix i-x, and use 64-bit integer arithmetic for large costs.
- Test the solution on the given examples and on small random cases against a brute-force search over window choices, including negative numbers, adjacent windows, and `k * x == n`.
