### NOTES
The key is to replace road strengths by town potentials. Let \(P_1=0\) and \(P_{v+1}=P_v+w_v\). For a person from \(S\) to \(T\), if \(S<T\), the stamina at town \(x\) is \(P_x-P_S\), so the requirements are \(P_S=P_T\) and \(P_x>P_S\) for all \(S<x<T\). If \(S>T\), the stamina is \(P_S-P_x\), so the requirements are \(P_S=P_T\) and \(P_x<P_S\) for all \(T<x<S\). Thus each person becomes an interval \([l,r]\) with sign \(+\) if \(S<T\) and sign \(-\) if \(S>T\).

A pair of people is incompatible exactly in the following cases:
1. They have the same left endpoint or the same right endpoint, regardless of sign.
2. They have the same sign and their endpoints interleave: \(l_1<l_2<r_1<r_2\), or symmetrically.

Same-sign strict containment and opposite-sign overlap/nesting are pairwise compatible; touching at an endpoint is compatible. The reduction to pairwise incompatibility relies on a Helly/acyclicity property: under these pairwise conditions the resulting strict-inequality graph is acyclic. Same-sign intervals form laminar families, and opposite-sign constraints orient from plus-endpoint components to minus-endpoint components; touching chains can merge components but should not create a directed cycle. This should be verified before relying on it; if a longer cycle exists, checking only pairs would be insufficient.

Once pairwise incompatibility is accepted, a range \([L,R]\) is feasible iff it contains no incompatible pair. Therefore it suffices to compute  
\[
\text{prev}[i]=\max\{j<i \mid j \text{ is incompatible with } i\}.
\]
Then query \([L,R]\) is Yes iff  
\[
\max_{i\in[L,R]} \text{prev}[i] < L.
\]
The endpoint part of `prev` is easy by scanning indices and keeping the last seen left endpoint and right endpoint. The hard part is same-sign crossing: for each \(i\), find the latest previous same-sign interval satisfying either  
\[
l_j<l_i<r_j<r_i
\]
or  
\[
l_i<l_j<r_i<r_j.
\]
This is a 3-dimensional offline problem: index order plus two endpoint coordinates, with strict inequalities.

Pitfalls:
- Strict versus non-strict endpoint comparisons are critical. A right endpoint equal to another interval’s left endpoint is allowed; same left endpoint or same right endpoint is forbidden.
- Integer strictness in nested opposite-sign intervals is satisfiable with gaps, but it must not be mistaken for a pairwise conflict.
- The two crossing orientations must be handled separately.
- CDQ/sweep implementations need careful segment-tree clearing across recursive nodes.
- Python performance and memory for \(O(M\log M\log N)\) operations may be tight.
- Query answering over `prev` must use 1-based indices and treat `prev[i]=0` as “no previous incompatible person”.

### NEXT
- Implement the provided CDQ + `prev` + range-max query approach: normalize intervals, compute endpoint `prev` with `last_left`/`last_right`, run CDQ separately on plus and minus subsequences; in each cross step use two monotone sweeps (by \(r\) ascending for \(l_j<l_i<r_j<r_i\), and by \(l\) descending for \(l_i<l_j<r_i<r_j\)) with a range-chmax/point-query segment tree over town coordinates; then answer queries with a segment tree or sparse table over `prev`. Pitfalls: sorting at every CDQ node may be too slow; use presorted/merged coordinate lists or accept a high constant; use timestamps or touched-node lists to reset the segment tree; keep all comparisons strict.
- Try a one-pass online 2D range-maximum structure per sign: process people in index order, insert \((l,r)\) with value index, and for each new interval query the two crossing rectangles (prefix in \(l\) with \(r\) in a range, and range in \(l\) with suffix in \(r\)) to get the latest previous crossing. Use a segment tree over one coordinate with compressed Fenwick/segment trees over the other, or run the first rectangle on reversed coordinates for the second. Pitfalls: \(O(M\log N)\) memory for coordinate compression, max cannot be obtained by prefix subtraction, high implementation complexity, and likely Python MLE/TLE.
- Try a sliding-window / two-pointer method that computes, for every \(L\), the minimal \(R\) making \([L,R]\) infeasible, then answers queries by \(R < \text{badR}[L]\). Maintain the current window’s total number of incompatible pairs under add/remove: endpoint conflicts with \(O(1)\) counters, same-sign crossing conflicts with a dynamic 2D range-counting structure (Fenwick of Fenwick trees or coordinate sqrt decomposition). Pitfalls: deletions make the data structure harder; \(O(M\log^2 N)\) updates with large constants; must carefully restore the window when the first infeasible \(R\) is found; Python may not handle the dynamic 2D counts within limits.
- Try Mo’s algorithm over the query ranges, maintaining the current person-index interval and the same dynamic conflict count; answer Yes iff the count is zero. Pitfalls: the operation count is roughly \((M+Q)\sqrt{M}\) times a 2D update/query cost, which is far too high for the given limits; it is mainly a fallback if a very cheap conflict-maintenance structure is discovered.
- Try a segment tree over person indices whose nodes store compact certificates of feasibility (infeasible flag, used endpoints, laminar interval structures for each sign) and combine \(O(\log M)\) nodes for a query. Pitfalls: there is no obvious small monoid; checking cross conflicts between two certificates may require scanning or 2D queries, certificate size may blow up, and handling mixed components from touching intervals is subtle; likely less practical than the `prev`-array reduction.
- Try parallel binary search or divide-and-conquer on the answer to compute `prev[i]` or `badR[L]`: binary search the latest previous incompatible index or minimal bad \(R\), and test batches of candidate ranges with offline 2D/3D dominance sweeps. Pitfalls: each iteration still needs a robust offline conflict-existence test, the method adds multiple passes and synchronization overhead, and it is essentially as hard as the CDQ computation but with more off-by-one risk.
