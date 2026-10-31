### STATUS
continue
### NEXT
Write the final Python class Solution method in the required delimiters, implementing the telescoped O(n^2) DAG DP: compute prefix sums S and C, set dp[0]=0, for v=1..n compute dp[v]=min_{u<v} dp[u] + (S[u]-S[v])*C[u] + k*(C[n]-C[u]), return S[n]*C[n]+dp[n]; run the provided sample tests and a small brute-force random validation.
### TASKS
- [done] Derive and verify the telescoped endpoint-only edge-weight formulation on the examples.
- [todo] Write the final Python class Solution method in the required delimiters, implementing the telescoped O(n^2) DAG DP: compute prefix sums S and C, set dp[0]=0, for v=1..n compute dp[v]=min_{u<v} dp[u] + (S[u]-S[v])*C[u] + k*(C[n]-C[u]), return S[n]*C[n]+dp[n]; run the provided sample tests and a small brute-force random validation.
- [todo] If performance or robustness requires it, optimize the DP with a monotone convex hull or Li Chao tree using integer arithmetic.
