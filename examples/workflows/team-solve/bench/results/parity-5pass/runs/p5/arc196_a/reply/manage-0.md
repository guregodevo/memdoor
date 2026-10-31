### STATUS
continue

### NEXT
Implement the final Python solution using the proven reduction: a two-heap half-maintainer (max-heap lower, min-heap upper, running sums) to compute even-block scores; scan right-to-left to fill suff[i] for even suffix lengths; if N is even print suff[0], otherwise maximize pref[L] + suff[L+1] over even prefix lengths L (including L=0 and L=N-1) while scanning left-to-right; use fast stdin/stdout, Python integers, and O(N log N) time; validate locally on the samples and small random brute-force tests, then output only the final code within the required delimiters.

### TASKS
- [done] Prove the even-block lemma: for 2m elements, max score = sum largest m - sum smallest m, using a sign upper bound and an adjacent opposite-color pairing construction.
- [done] Prove the odd-N structure: the final remaining element must have odd 1-based index, no pair can cross it, and for a fixed odd separator the optimum is the sum of the even-block optima of the left and right parts.
- [todo] Implement the final Python solution with a two-heap half-maintainer, suffix scores for even suffixes, even/odd N handling, fast I/O, and O(N log N) time.
- [todo] Validate the implementation on samples and small random brute-force tests (N <= 10); if it fails, debug or switch to a Fenwick-tree order-statistic implementation as a cross-check.
