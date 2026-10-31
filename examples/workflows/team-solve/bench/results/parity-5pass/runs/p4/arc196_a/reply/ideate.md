### NOTES
The core difficulty is that the operation is local in the current sequence, but the final pairing of original elements is global and noncrossing. A naive interval DP over subarrays is far too slow for N = 3e5. The useful structural facts are:

1. Any valid deletion order corresponds to a noncrossing matching of original indices, with one unmatched index when N is odd.
2. Every matched pair must join original indices of opposite parity. If N is odd, the final survivor must have an even number of elements on both sides, so in 1-based indexing it must be at an odd position.
3. For any even-length block, the maximum score depends only on the multiset of that block: it is the sum of the largest half minus the sum of the smallest half. The upper bound comes from counting how many elements act as the larger endpoint versus the smaller endpoint. Achievability comes from labeling the largest half positive and the smallest half negative, then repeatedly cancelling adjacent opposite labels; this gives a valid noncrossing removal order.
4. For odd N, the survivor splits the sequence into two independent even blocks, so the answer is the maximum over valid survivor positions of the even-block value of the left part plus that of the right part.

The remaining practical difficulty is computing these even-block values for all prefixes and suffixes efficiently and without off-by-one errors.

Candidate approaches and pitfalls:

- Interval DP / noncrossing matching DP: define a DP for a contiguous interval, pairing the leftmost element with a possible partner and adding the values of the inside and outside intervals. For odd lengths, add a state for the survivor. Pitfalls: O(N^3) time and O(N^2) memory in the straightforward form; no obvious Monge or divide-and-conquer optimization because values are arbitrary; easy to allow invalid matchings where a pair encloses the survivor.

- Order-statistic prefix/suffix with two heaps: maintain the lower and upper halves of the current prefix, and separately of the current suffix, using two priority queues with their sums. When the current length is even, the difference of the upper and lower sums is the block value. For odd N, combine prefix and suffix values at odd split points. Pitfalls: keeping both size and order invariants after each insertion; handling empty blocks; 1-based vs 0-based split parity; processing suffixes in reverse; large integer sums; ties at the median boundary.

- Fenwick tree / wavelet tree range order statistics: compress values and maintain counts and sums in a binary indexed tree while scanning prefixes and suffixes; query the sum of the k smallest elements in the current multiset to get the block value. Alternatively, build a wavelet tree or merge-sort tree to answer the same statistic for arbitrary ranges. Pitfalls: more implementation complexity; duplicate values require careful k-th query and partial counts; off-by-one in choosing k; wavelet tree is overkill and error-prone; still O(N log N).

- Greedy adjacent maximum: keep current adjacent differences in a max-heap and a linked list, repeatedly removing the adjacent pair with largest difference. Pitfalls: this is not generally optimal. A local maximum can pair two elements that both should be in the upper half in a global optimum; for example, a sequence like 10000, 1000, 1, 2, 3, 100 makes greedy take 10000 and 1000 first, which is bad. No simple exchange argument supports it.

- Uncrossing / assignment reduction: argue that crossing pairs can be replaced by noncrossing pairs without decreasing the absolute-difference total, so the noncrossing constraint can be ignored for the value. Then the problem becomes choosing half of the elements as larger endpoints and half as smaller endpoints, which is solved by taking the largest half and smallest half. For odd N, combine this with the parity/split condition. Pitfalls: the uncrossing inequality must be proved for all four value orders; it does not by itself handle the positional restriction of the survivor; it is mainly a proof tool and still leaves the same order-statistic computation.

- Parity-based bipartite matching or flow: use the fact that every pair joins opposite original parity. Model even N as matching odd positions to even positions, and odd N as leaving one odd position unmatched. Try maximum-weight noncrossing bipartite matching, or a min-cost flow on a path. Pitfalls: the noncrossing constraint makes this much harder than ordinary bipartite matching; arbitrary values do not give a Monge structure in index order; a flow formulation may have too many edges or be too slow for 3e5.

- Brute force / exhaustive small-N validation: enumerate all removal orders or all noncrossing matchings for small N to compute exact answers. Pitfalls: exponential/Catalan growth; useful only for testing and counterexample hunting, not as the final solution.

### NEXT
- Implement the two-heap prefix/suffix order-statistic approach first, since it matches the structural formula and is O(N log N).
- Use a Fenwick tree over compressed values as an alternative if maintaining two heaps for both prefixes and suffixes is too error-prone.
- Build a small brute-force checker for tiny N to validate the even-block formula and odd-split rule, and to test edge cases.
- Investigate the greedy adjacent-maximum heuristic only as a sanity check, and use the known counterexample to confirm it is not final.
- If correctness proof is needed, develop the uncrossing/exchange argument and the sign-cancellation achievability argument before coding.
