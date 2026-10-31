### STATUS
continue
### NEXT
Implement and validate the O(n^2) cut-penalty DP in the starter code: build prefix sums S and C, set dp[0]=0, for p=1..n compute dp[p] = S[p]*C[p] + min(0, min_{1<=q<p} dp[q] + k*(C[n]-C[q]) - S[p]*C[q]), return dp[n] + k*C[n], and test on the provided examples plus small random brute force.
### TASKS
- [done] Ideation: derive the cut-penalty reduction and one-dimensional DP recurrence.
- [todo] Implement and validate the O(n^2) cut-penalty DP in the starter code: build prefix sums S and C, set dp[0]=0, for p=1..n compute dp[p] = S[p]*C[p] + min(0, min_{1<=q<p} dp[q] + k*(C[n]-C[q]) - S[p]*C[q]), return dp[n] + k*C[n], and test on the provided examples plus small random brute force.
- [todo] If the O(n^2) DP fails validation, switch to a different approach such as direct layered DP with monotone CHT or Li Chao, and revalidate.
- [todo] Finalize the submitted code with typing import if needed, integer-only arithmetic, and edge cases including n=1 and maximum constraints.
