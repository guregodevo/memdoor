### STATUS
continue
### NEXT
Implement Solution.minOperations: compute cost[start] for every length-x window using coordinate-compressed Fenwick trees (counts and sums) to get the lower median and sum of absolute deviations, then run the prefix DP dp[t][i] = min(dp[t][i-1], dp[t-1][i-x] + cost[i-x]) for t=1..k and return dp[k][n].
### TASKS
- [done] Ideation: reduce to per-window median cost plus weighted k-interval DP.
- [todo] Implement Solution.minOperations: compute cost[start] for every length-x window using coordinate-compressed Fenwick trees (counts and sums) to get the lower median and sum of absolute deviations, then run the prefix DP dp[t][i] = min(dp[t][i-1], dp[t-1][i-x] + cost[i-x]) for t=1..k and return dp[k][n].
- [todo] Test on provided examples, edge cases (k*x == n, even/odd x, negatives, all equal), and small random brute force; fix any failures.
