### STATUS
continue
### NEXT
Implement the complete solution in the starter code using a Fenwick tree over compressed values: slide a length-x window, maintain combined count and sum BITs, find the lower median rank (x+1)//2, compute each window's minimum equalization cost as median*count_le - sum_le + (total_sum - sum_le) - median*(x - count_le), store it by 1-indexed end position; then run the rolling O(kn) DP with cur[p] = min(cur[p-1], prev[p-x] + cost[p]) for p >= x and return prev[n]. Run the two provided examples and small random brute-force tests, and report the sample test verdict.
### TASKS
- [done] Ideate and select the main approach: per-window median cost plus O(kn) non-overlapping window selection; use Fenwick tree for sliding median costs.
- [todo] Implement the complete solution in the starter code using a Fenwick tree over compressed values: slide a length-x window, maintain combined count and sum BITs, find the lower median rank (x+1)//2, compute each window's minimum equalization cost as median*count_le - sum_le + (total_sum - sum_le) - median*(x - count_le), store it by 1-indexed end position; then run the rolling O(kn) DP with cur[p] = min(cur[p-1], prev[p-x] + cost[p]) for p >= x and return prev[n]. Run the two provided examples and small random brute-force tests, and report the sample test verdict.
- [todo] Verify edge cases and performance: examples, x=2, k=1, k*x=n, negative values, duplicates, even x, adjacent windows, and a large n=100000 random case.
- [todo] If the Fenwick approach is too slow or has bugs, switch to a two-heap lazy-deletion sliding median or a hybrid direct-computation fallback while keeping the same DP.
