### PLAN
Treat the max in each pair price as choosing one of the three attributes for that pair; then cakes assigned to the same attribute can be paired arbitrarily, so a solution is just a selection of 2K cakes assigned to X/Y/Z with each attribute used an even number of times. For a penalty λ per pair, scale the objective by 2 and subtract λ for every selected cake; the best penalized value with even parities is computed by an 8-state DP over the parity mask of the three attributes, where each cake is skipped or assigned to one attribute. The optimal value F(k) for k pairs is concave in k (standard for maximum-weight matchings), so use the Aliens trick: binary search the integer λ for which the DP's optimal pair count crosses K, tie-breaking to maximize the count. Finally recover the true answer from the penalized DP value as (scaled_value + 2λK)/2, giving O(N log 2e9) per test case.

### TASKS
- Parse all test cases; for each, store the three coordinate arrays (pre-scaled by 2*SHIFT if desired) and set the binary-search upper bound to 2*max(maxX,maxY,maxZ) or 2_000_000_000.
- Implement `calc(lambda)` using 8 packed DP states `(value*SHIFT + count)` with `SHIFT=N+1`; for each cake update each parity mask by max(skip, previous mask^1 + (2X-lambda)*SHIFT+1, mask^2 + ..., mask^4 + ...), and return `dp[0]//SHIFT` and `(dp[0]%SHIFT)//2`.
- Binary search the largest `lambda` in `[0, hi]` such that `calc(lambda)` returns at least K pairs; use unrolled state updates and a large negative sentinel for speed.
- After finding `lambda`, call `calc(lambda)` once more and output `(scaled_value + 2*lambda*K)//2` for that test case.
- Check edge cases: all zero values, N=2, odd N, K=floor(N/2), ties at zero penalty, and maximum 1e9 coordinates.
