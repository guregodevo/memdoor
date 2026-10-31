### PLAN
Treat positive and zero products separately because a zero can make the final product 0 even if earlier positive product exceeds limit. For positive products, ignore zeros and run a DP over parity, product up to limit, and alternating sum represented as a bitset; each positive x shifts sums by +x or -x depending on parity and multiplies product by x. Maintain only non-empty states to avoid returning the empty subsequence when k=0. For product 0, run a second bitset DP over parity and sum with no-zero/has-zero flags to test whether any subsequence containing a zero reaches k. Finally scan products from limit down to 1 for a reachable k, return the first found, else return 0 if the zero-product DP is reachable, else -1.

### TASKS
- Set offset = sum(nums), mask = (1 << (2*offset+1)) - 1, and early-return -1 if abs(k) > offset.
- Implement positive-product DP: non[2][limit+1] bitsets; for each x > 0, copy states, transition parity 0 -> 1 with <<x and parity 1 -> 0 with >>x when p*x <= limit, and add the single-element state (parity 1, product x, sum x).
- Implement zero-product DP: nz[2] and z[2] bitsets; process all nums, updating no-zero states for x > 0, moving no-zero to has-zero when x == 0, flipping parity for has-zero on any taken element, and adding empty-to-single states.
- Combine: check bit k in non[0][p] | non[1][p] for p from limit down to 1; return p if found, otherwise return 0 if bit k is in z[0] | z[1], else -1.
- Validate edge cases: all zeros, k=0, product 1 from repeated ones, zeros after products exceeding limit, negative k, and limit smaller than any positive element.
