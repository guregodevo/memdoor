### PLAN
The fixed edges \(1\to2\to\cdots\to2N\) make the graph strongly connected iff every prefix cut has an added edge from the suffix to the prefix; this immediately forces \(S_1=B\) and \(S_{2N}=W\). For a prefix containing \(a\) white vertices and \(b\) black vertices, the condition is \(\max_{j\le b} q_j \ge a+1\), where \(q_j\) is the white index matched to black \(j\); for fixed \(b\), only the largest \(a\) with that \(b\) (and \(a\le N-1\)) matters, giving \(O(N)\) constraints \((b,a)\) with \(a-b\ge0\). A constraint fails iff the first \(b\) black vertices are all matched to the first \(a\) white vertices. Inclusion-exclusion over these failure events, using the fact that the constraints are ordered by \(b\), yields the DP
\[
dp_i = -\operatorname{invfact}[a_i-b_i]\sum_{j<i} dp_j\,\operatorname{fact}[a_i-b_j],
\]
with virtual \(dp_0=1\) at \(b_0=0\), and
\[
\text{answer}=N!+\sum_i dp_i\,\operatorname{fact}[N-b_i].
\]
The sum is a triangular convolution with the factorial kernel, so it is evaluated with CDQ divide-and-conquer and NTT modulo \(998244353\) after precomputing factorials and inverse factorials.

### TASKS
- Read \(N,S\); if \(S[0]\ne B\) or \(S[-1]\ne W\), print \(0\). Otherwise scan \(S\), maintaining white index \(a\) and black count \(b\); for \(a=1\ldots N-1\), if \(b\le a\), set \(last[b]=a\). Build sorted arrays \(B_i, A_i=last[B_i], D_i=A_i-B_i\) for \(B_i=1\ldots N-1\) with \(last[B_i]>0\).
- Precompute \(fact[0..N]\) and \(invfact[0..N]\) modulo \(998244353\). Initialize an array \(S_i=fact[A_i]\) (the contribution of the virtual \(j=0\) term) and base answer \(=fact[N]\).
- Implement a CDQ solver over the constraint indices: at a leaf set \(dp_i=-S_i\cdot invfact[D_i]\bmod MOD\); after solving the left half, propagate its \(dp_j\) to the right half by convolving an array indexed by \(B_j\) with the factorial series \(fact[t]\), and add \(conv[A_i-\text{offset}]\) to \(S_i\) for each right \(i\). Use NTT for large blocks and a direct loop for small blocks.
- After CDQ completes, for each constraint add \(dp_i\cdot fact[N-B_i]\) to the answer modulo \(998244353\), then print the answer.
- Verify edge cases (\(N=1\), no constraints, invalid endpoints, all constraints automatic) and the provided samples, checking NTT offsets and modulo signs.
