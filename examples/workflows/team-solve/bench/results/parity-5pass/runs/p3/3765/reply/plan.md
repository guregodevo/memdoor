### PLAN
Use prefix sums and dynamic programming over the number of subarrays: dp_m[r] is the minimum cost to partition the first r elements into exactly m subarrays. The transition is dp_m[r] = min_{p<r} dp_{m-1}[p] + (prefNum[r] + k*m)*(prefCost[r] - prefCost[p]), which can be rewritten as (prefNum[r]+k*m)*prefCost[r] + min_p(-prefCost[p]*(prefNum[r]+k*m) + dp_{m-1}[p]). For a fixed m, the line slopes -prefCost[p] are monotone decreasing and the query x = prefNum[r]+k*m is monotone increasing, so a monotone convex hull trick gives O(n) per m and O(n^2) total. Keep only the previous and current DP rows and return the minimum dp_m[n] over all valid m.

### TASKS
- Compute prefix sums of nums and cost, and initialize dp_prev for 0 subarrays with dp_prev[0]=0 and all other entries infinite.
- For each m from 1 to n, iterate r from 1 to n, and before querying r insert the line for p=r-1 if dp_prev[p] is finite, using slope -prefCost[p] and intercept dp_prev[p].
- Implement a lower convex hull for monotone decreasing slopes and increasing x queries, using integer cross multiplication to avoid floating-point errors, and compute dp_cur[r] = (prefNum[r] + k*m)*prefCost[r] + hull.query(prefNum[r] + k*m).
- After finishing each m, update the answer with dp_cur[n], set dp_cur[0]=inf, and replace dp_prev with dp_cur for the next layer.
- Verify against the given examples and edge cases such as n=1, one subarray, and maximum constraints to confirm correctness and O(n^2) runtime.
