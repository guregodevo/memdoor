### STATUS
continue
### NEXT
Implement the complete Python solution in the starter class: coordinate-compress nums, compute all length-x window costs by sliding two Fenwick trees (counts and sums), finding the lower median with a kth-order query and calculating sum of absolute deviations; then run a rolling DP over prefix length and number of chosen windows using dp[j][i] = min(dp[j][i-1], dp[j-1][i-x] + cost[i-x]); test on the two provided examples and a brute-force checker for small random inputs.
### TASKS
- [done] Reduce the problem to selecting k disjoint length-x windows with weights equal to the minimum cost to make each window equal (median-deviation cost).
- [todo] Implement the complete Python solution in the starter class: coordinate-compress nums, compute all length-x window costs by sliding two Fenwick trees (counts and sums), finding the lower median with a kth-order query and calculating sum of absolute deviations; then run a rolling DP over prefix length and number of chosen windows using dp[j][i] = min(dp[j][i-1], dp[j-1][i-x] + cost[i-x]); test on the two provided examples and a brute-force checker for small random inputs.
- [todo] Debug any failing sample or brute-force case, focusing on Fenwick kth query, compression/indexing, negative sums, DP row updates, and non-overlap off-by-one errors.
- [todo] If the Fenwick approach is incorrect or too slow, switch to a different approach such as two-heap lazy deletion or a static range-median structure, rather than continuing to patch the same implementation.
