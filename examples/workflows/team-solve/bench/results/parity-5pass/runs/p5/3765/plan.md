Use dynamic programming where dp[j][i] is the minimum cost to partition the first i elements into exactly j nonempty subarrays.  
For a last subarray p..i-1, its cost is (S[i] + k*j) * (C[i] - C[p]), giving dp[j][i] = min_p dp[j-1][p] + (S[i] + k*j)(C[i] - C[p]).  
Expand the transition as (S[i] + k*j)C[i] + min_p (dp[j-1][p] - (S[i] + k*j)C[p]), which is a line query with x = S[i] + k*j, slope -C[p], and intercept dp[j-1][p].  
Since C[p] is increasing and x is increasing for fixed j, maintain a monotone convex hull trick to evaluate each j-layer in O(n), for O(n^2) total.  
Return the minimum dp[j][n] over all j from 1 to n.