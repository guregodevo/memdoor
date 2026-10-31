### PLAN
Use DP where `dp[t][i]` is the minimum cost to partition the first `i` elements into exactly `t` nonempty subarrays. Let `S` and `C` be prefix sums of `nums` and `cost`; the last segment `p..i-1` has cost `(S[i] + k*t) * (C[i] - C[p])`. Thus `dp[t][i] = (S[i] + k*t) * C[i] + min_{p<i} (dp[t-1][p] - (S[i] + k*t) * C[p])`. For a fixed `t`, each candidate `p` defines a line `y = -C[p] * x + dp[t-1][p]`, and the query point is `x = S[i] + k*t`. Because `C[p]` is strictly increasing and `x` is nondecreasing, maintain a lower convex hull with a deque to answer each transition in amortized `O(1)`, giving `O(n^2)` total. Iterate `t` from `1` to `n` using rolling arrays and return the minimum `dp[t][n]` over all valid `t`.

### TASKS
- Compute prefix arrays `S` and `C` of length `n+1`, and initialize `prev[0] = 0` with `prev[i] = infinity` for `i > 0`.
- For each `t` from `1` to `n`, create an empty monotone CHT and iterate `i` from `t` to `n`, inserting the line for `p = i-1` only when `prev[p]` is finite, with slope `-C[p]` and intercept `prev[p]`.
- Implement the CHT for minimum queries with decreasing slopes and increasing `x`, using integer cross multiplication to discard obsolete lines and popping from the front while the next line is better at the current `x`.
- For each `i`, compute `cur[i] = (S[i] + k*t) * C[i] + hull.query(S[i] + k*t)`, then after the loop set `prev = cur` and update the answer with `prev[n]`.
- Validate edge cases such as `n = 1`, invalid states, and large integer values, and return the best answer.
