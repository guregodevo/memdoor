### STATUS
continue

### NEXT
Run the current code on the two provided examples and report a SAMPLE TESTS verdict (PASS/FAIL). Also run edge cases: x=n with k=1, k*x=n, even/odd x, negative values, all equal, and a small random brute-force comparison; if any fail, provide the failing input and expected vs actual.

### TASKS
- [done] Ideation: reduce to per-window median cost plus weighted k-interval DP.
- [done] Implement Solution.minOperations: compute cost[start] for every length-x window using coordinate-compressed Fenwick trees (counts and sums) to get the lower median and sum of absolute deviations, then run the prefix DP dp[t][i] = min(dp[t][i-1], dp[t-1][i-x] + cost[i-x]) for t=1..k and return dp[k][n].
- [todo] Test on provided examples, edge cases (k*x == n, even/odd x, negatives, all equal), and small random brute force; fix any failures.
