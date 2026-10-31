### PLAN
For any pair, color both cakes with a coordinate attaining the pair's maximum; conversely, any selection of 2K cakes colored X/Y/Z with even counts per color can be paired inside each color to form a valid matching whose price is at least the colored sum, so the problem is exactly the even-parity coordinate-selection problem.  
The optimum as a function of K is concave (it is a maximum-weight matching/M-concave selection problem), so a Lagrangian penalty on the number of pairs can be used.  
For an integer penalty beta per pair, double all coordinate values and subtract beta for every selected cake; the best penalized even-parity selection is computed by an 8-state parity DP over cakes, where each cake is unused or toggles one of the three color parity bits.  
Tie-break equal penalized values by maximizing the selected count, e.g. by encoding (value, count) as value*(N+1)+count.  
Binary search the largest beta whose optimal selected count is at least 2K; then K is an optimal cardinality and the answer is DP_value/2 + beta*K.

### TASKS
- Verify the reduction: each cake may choose at most one of X/Y/Z, total chosen must be 2K, each color count must be even, and maximizing the chosen-coordinate sum is equivalent to the original pairing problem.
- Implement penalized(beta) using an unrolled 8-mask max-plus DP over encoded scores; transitions are stay unused or add (2*coord - beta)*(N+1)+1 and toggle the corresponding parity bit.
- Precompute scaled values 2X*(N+1), 2Y*(N+1), 2Z*(N+1) per test case and use a large negative sentinel for unreachable states.
- Binary search beta in [0, 2*max_coordinate+1) for the largest beta where penalized(beta) returns selected_count >= 2K.
- After finding beta, recompute penalized(beta), decode D = score // (N+1), and output D//2 + beta*K for each test case.
- Test samples and edge cases (all zeros, N odd, K=N/2, ties) and ensure fast I/O for sum N <= 1e5.
