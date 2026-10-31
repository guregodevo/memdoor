### STATUS
continue

### NEXT
Implement a complete runnable Python 3 program that reads stdin and prints the answer, using the proven formula: even N = sum largest N/2 - sum smallest N/2; odd N = max over even prefix length l of even-block score of a[0:l] plus even-block score of a[l+1:n], using a two-heap half-maintainer and suffix scores; then run it on the three samples to confirm outputs 5, 14, 0.

### TASKS
- [done] Prove the even-block lemma: for 2m elements, max score = sum largest m - sum smallest m, using a sign upper bound and an adjacent opposite-color pairing construction.
- [done] Prove the odd-N structure: the final remaining element must have odd 1-based index, no pair can cross it, and for a fixed odd separator the optimum is the sum of the even-block optima of the left and right parts.
- [todo] Implement a complete runnable Python 3 program that reads stdin and prints the answer, using the proven formula and a two-heap half-maintainer for even-block scores, with O(N log N) time and fast I/O.
- [todo] Validate the implementation on the three samples and small random brute-force tests (N <= 10); if it fails, debug the heap invariant/indexing or switch to a Fenwick-tree order-statistic implementation as a cross-check.
