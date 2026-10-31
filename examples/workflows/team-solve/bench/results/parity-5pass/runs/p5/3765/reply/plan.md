### PLAN
Use dynamic programming where dp[j][i] is the minimum cost to partition the first i elements into exactly j nonempty subarrays.  
For a last subarray p..i-1, its cost is (S[i] + k*j) * (C[i] - C[p]), giving dp[j][i] = min_p dp[j-1][p] + (S[i] + k*j)(C[i] - C[p]).  
Expand the transition as (S[i] + k*j)C[i] + min_p (dp[j-1][p] - (S[i] + k*j)C[p]), which is a line query with x = S[i] + k*j, slope -C[p], and intercept dp[j-1][p].  
Since C[p] is increasing and x is increasing for fixed j, maintain a monotone convex hull trick to evaluate each j-layer in O(n), for O(n^2) total.  
Return the minimum dp[j][n] over all j from 1 to n.

### TASKS
- Precompute prefix sums S and C; initialize dp_prev for zero segments with dp_prev[0]=0 and infinity elsewhere.
- For each j = 1..n, iterate i = j..n, inserting the line for p = i-1 when dp_prev[p] is finite, then querying the hull at x = S[i] + k*j to compute dp_cur[i].
- Implement a monotone CHT for minimum queries with decreasing slopes and increasing x, using integer cross multiplication to remove obsolete lines and advance the query pointer.
- Set dp_cur[i] = (S[i] + k*j) * C[i] + hull.query(x), keeping invalid states as infinity, and after each layer update the best answer with dp_cur[n].
- Verify on the provided examples and edge cases (n=1, one segment, maximum values), confirming O(n^2) time and O(n) memory.
