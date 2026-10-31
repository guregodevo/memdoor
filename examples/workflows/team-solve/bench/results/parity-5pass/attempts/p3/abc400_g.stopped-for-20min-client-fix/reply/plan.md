### PLAN
Transform each pair price by choosing, for every pair, the coordinate that attains the maximum; this is equivalent to selecting 2K cakes and assigning each selected cake one of the three coordinates, with each coordinate used an even number of times. For a penalty C per pair, the best penalized total over any number of pairs can be computed by a small DP over the 3-bit parity of how many selected cakes use each coordinate: each cake is either skipped or contributes 2*value-C to one parity bit. The maximum total for exactly K pairs is concave in K (it is the value of a maximum-weight k-matching), so use the Aliens trick: binary search an integer penalty C, keeping the largest number of pairs among optimal penalized solutions, and recover the answer as (penalized_value + 2*C*K)/2. The complexity is O(N * 8 * 3 * log V) per test case, with total N at most 1e5, which is feasible.

### TASKS
- Parse all test cases and precompute doubled X, Y, Z values for each cake.
- Implement calc(C): an 8-state parity DP over cakes, where skipping keeps the state and choosing a coordinate adds 2*value-C and flips the corresponding bit; tie-break equal values by larger selected-cake count, and return (doubled penalized value for mask 0, selected_count//2).
- For each test case, binary search the largest integer C in [0, 2_000_000_001) such that calc(C) gives at least K pairs.
- After the search, run calc(C) once more and output (value + 2*C*K)//2.
- Handle edge cases (all zero values, N odd, ties, large coordinates) and ensure the negative infinity is far below any possible adjusted score.
