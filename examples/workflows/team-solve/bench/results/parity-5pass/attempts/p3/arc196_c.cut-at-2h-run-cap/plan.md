The fixed edges \(1\to2\to\cdots\to2N\) make the graph strongly connected iff every prefix cut has an added edge from the suffix to the prefix; this immediately forces \(S_1=B\) and \(S_{2N}=W\). For a prefix containing \(a\) white vertices and \(b\) black vertices, the condition is \(\max_{j\le b} q_j \ge a+1\), where \(q_j\) is the white index matched to black \(j\); for fixed \(b\), only the largest \(a\) with that \(b\) (and \(a\le N-1\)) matters, giving \(O(N)\) constraints \((b,a)\) with \(a-b\ge0\). A constraint fails iff the first \(b\) black vertices are all matched to the first \(a\) white vertices. Inclusion-exclusion over these failure events, using the fact that the constraints are ordered by \(b\), yields the DP
\[
dp_i = -\operatorname{invfact}[a_i-b_i]\sum_{j<i} dp_j\,\operatorname{fact}[a_i-b_j],
\]
with virtual \(dp_0=1\) at \(b_0=0\), and
\[
\text{answer}=N!+\sum_i dp_i\,\operatorname{fact}[N-b_i].
\]
The sum is a triangular convolution with the factorial kernel, so it is evaluated with CDQ divide-and-conquer and NTT modulo \(998244353\) after precomputing factorials and inverse factorials.