### PLAN
Model each candidate length-x window by its minimum cost to make all elements equal, which is the sum of absolute deviations to a median. Compute these window costs for all starts with a sliding median structure, such as a Fenwick tree over compressed values or two heaps, in O(n log n). Then run a DP over prefix length and number of selected windows, using dp[j][i] = min(dp[j][i-1], dp[j-1][i-x] + cost[i-x]) to enforce non-overlap. Return dp[k][n], with impossible states initialized to infinity.

### TASKS
- Compute cost[start] for every start 0..n-x by maintaining a sliding window of size x, finding its median, and summing absolute deviations using count/sum aggregates.
- Implement the sliding median with coordinate compression and two Fenwick trees for counts and sums, or two heaps with lazy deletion, supporting add/remove and kth-order-statistic queries.
- Implement the DP in the provided Solution.minOperations, using O(n) memory per layer: for each j=1..k, fill cur[i] from cur[i-1] and prev[i-x] + cost[i-x].
- Test edge cases including k*x == n, even x, negative numbers, all equal values, and small random arrays against a brute-force checker.
