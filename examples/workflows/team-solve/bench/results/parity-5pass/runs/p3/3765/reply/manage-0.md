### STATUS
continue
### NEXT
Implement the one-dimensional O(n^2) cut-path DP: build prefix sums prefNum and prefCost, set dp[0]=0, for each r=1..n compute dp[r] = min over p<r of dp[p] + prefNum[r]*(prefCost[r]-prefCost[p]) + (k*(prefCost[n]-prefCost[p]) if p>0 else 0), then return dp[n] + k*prefCost[n]; run the two provided examples and edge cases n=1, one subarray, and all singletons.
### TASKS
- [done] Ideation and approach selection: use one-dimensional cut-path DP instead of the 2D exact-count DP.
- [todo] Implement the one-dimensional O(n^2) cut-path DP with prefix sums and the p>0 internal-cut penalty, returning dp[n] + k*totalCost.
- [todo] Verify correctness on the two examples, n=1, one-subarray, all-singletons, and small random brute-force partitions.
- [todo] Finalize the solution in the provided starter code with Python integer arithmetic and O(n^2) runtime.
