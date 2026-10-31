### STATUS
continue

### NEXT
Implement the O(n^2) boundary-penalty DP in the starter code: compute prefix sums S and C, set dp[0]=0, for i=1..n compute dp[i]=min_{p<i}(dp[p]+S[i]*(C[i]-C[p])) and add k*(C[n]-C[i]) when i<n, then return dp[n]+k*C[n]; run the two provided examples plus n=1, one-segment, and all-singleton cases.

### TASKS
- [done] Derive the one-dimensional boundary-penalty recurrence from the ordinal-dependent subarray cost.
- [todo] Implement the O(n^2) boundary-penalty DP in the starter code with prefix sums S and C, dp[0]=0, internal-boundary penalty k*(C[n]-C[i]) for i<n, and final answer dp[n]+k*C[n].
- [todo] Verify the implementation on the provided examples and edge cases (n=1, one segment, all singletons, maximum values), and cross-check small random cases against a brute-force partition enumerator.
- [todo] If any test fails, debug the recurrence/penalty placement; if performance becomes an issue (unlikely for n=1000), switch to the monotone convex-hull version of the same one-dimensional recurrence.
