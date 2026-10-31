### PLAN
A valid removal order is a noncrossing matching, and for any even-length block the optimal score is the sum of its largest half minus the sum of its smallest half. This follows from orienting each pair from smaller to larger for an upper bound, and from coloring the largest half + and smallest half - then repeatedly deleting adjacent opposite colors for achievability. If N is odd, the final unpaired element must be at an odd position and no pair can cross it, so the score is the sum of the optimal scores of an even prefix and an even suffix. Compute these half-differences for all even prefixes and suffixes with two heaps and running sums, then take the maximum over complementary prefix/suffix lengths (or the whole value when N is even).

### TASKS
- Implement `half_diffs(arr)` that returns `D` where `D[0]=0` and `D[k]` is the sum of the largest `k` minus the sum of the smallest `k` among the first `2k` elements; maintain a max-heap for the lower half and a min-heap for the upper half with running sums, rebalancing after each insertion and recording after every two elements.
- For even `N`, read input, call `half_diffs(A)`, and print `D[N//2]`.
- For odd `N`, let `L=(N-1)//2`, compute `P=half_diffs(A)` and `S=half_diffs(reversed(A))`, and print `max(P[k]+S[L-k] for k=0..L)`.
- Ensure the implementation uses `O(N log N)` time and `O(N)` memory, parses `N` and `A` from stdin, and prints the answer as a 64-bit integer.
