### PLAN
Model the deletion process as a non-crossing matching, and note that for any even-length block the maximum score is the sum of its largest half minus the sum of its smallest half. This follows from orienting each pair from larger to smaller for an upper bound, and from pairing a balanced set of "large" and "small" positions by repeatedly removing adjacent opposite colors for achievability. If N is even, the whole array is one such block. If N is odd, the final remaining element must have odd original index, and no pair can cross it, so the optimum is the maximum over odd split positions u of the even-block optimum of the prefix before u plus the even-block optimum of the suffix after u. Compute all needed prefix and suffix even-block optima in O(N log N) by scanning with two heaps that maintain the lower and upper halves and their sums.

### TASKS
- Prove the even-block lemma: for 2m elements, max score = sum largest m - sum smallest m, using a sign upper bound and an adjacent opposite-color pairing construction.
- Prove the odd-N structure: the final remaining element must have odd original index, no pair can cross it, and for a fixed odd u the optimum is the sum of the even-block optima of A[1..u-1] and A[u+1..N].
- Implement a two-heap half-maintainer (max-heap lower, min-heap upper, running sums) that reports the even-block score after each even-sized insertion.
- For odd N, scan right to left to compute suff[j] for even j, the even-block score of suffix A[j..N], with suff[N+1]=0.
- For odd N, scan left to right maintaining the prefix score and update the answer with prefix + suff[i+2] for each even prefix length i; for even N, output the full prefix score.
- Implement stdin/stdout and use 64-bit integer arithmetic, ensuring O(N log N) time and O(N) memory for N up to 3e5.
