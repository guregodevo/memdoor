### STATUS
done
### TASKS
- [done] Reduce the problem to selecting k disjoint length-x windows with weights equal to the minimum cost to make each window equal (median-deviation cost).
- [done] Implement the complete Python solution in the starter class: coordinate-compress nums, compute all length-x window costs with a sliding Fenwick tree storing counts and sums, find the lower median via kth-order query, and run a rolling DP over prefix length and chosen windows.
- [done] Verify the solution against the two provided examples and small brute-force random cases; no failures were found.
- [done] Confirm complexity O(n log n + k n) and memory O(n), with correct handling of negative values, duplicates, even x, adjacent windows, and feasibility k*x <= n.
