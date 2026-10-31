### STATUS
continue
### NEXT
Implement the complete Python solution: read all input with sys.stdin.buffer.read().split(); for each case, scan the grid once to compute row A-parities, column A-parities, and b_count, breaking early if a row parity is odd; if any row/column parity is odd output 0; if b_count == 0 output pow(2, H+W, 998244353); if b_count == H*W output 2; otherwise scan left-to-right maintaining row_pref and col_pref[j], and for each B cell unite row node i with column node H+j in a parity DSU with label w = 1 ^ row_pref ^ col_pref[j], updating row_pref and col_pref[j] on A cells; if any contradiction output 0, else output pow(2, components, 998244353). Use an iterative parity DSU (e.g. negative-parent sizes and xor-to-parent) with path compression and union by size, ensure isolated row/column nodes are counted as free components, and return the final program enclosed in the required delimiters.
### TASKS
- [done] Derive the row/column base reduction: row/column A-parity checks, B-cell parity constraints between row and column bases, and answer 2^(DSU components) if consistent.
- [todo] Implement the complete Python solution as specified in NEXT, with fast buffered input, iterative parity DSU, early exits for odd parity/all-A/all-B, and no hardcoded sample answers.
- [todo] Verify correctness with a separate brute-force enumerator for small grids (all rotations) and edge cases H=2/W=2, all A, all B, odd parity, inconsistent B labels, isolated nodes, and many small test cases.
- [todo] Check performance/memory for total HW=1e6 and T up to 1e5: O(HW α(H+W)) per case, compact per-case arrays, early exit on contradiction, and efficient modular exponentiation or precomputed powers.
