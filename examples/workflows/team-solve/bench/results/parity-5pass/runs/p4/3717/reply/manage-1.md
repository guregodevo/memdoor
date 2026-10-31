### STATUS
done
### NEXT

### TASKS
- [done] Ideation: reduce to selecting k non-overlapping length-x windows with median absolute-deviation costs.
- [done] Implement the full solution in the starter code: coordinate-compressed Fenwick trees for sliding window counts/sums, compute each length-x window cost using the lower median rank (x+1)//2, then run rolling DP dp[c][i]=min(dp[c][i-1], dp[c-1][i-x]+cost[i]) for c=1..k; run the two provided examples.
- [done] Test edge cases: x=n, k=1, even x, negative values, duplicate values, k*x=n, and adjacent windows where one ends at i-x and the next starts at i-x+1.
- [done] Verify final complexity is O(n log n + nk) time and O(n) memory, and that the returned value is an int.
