### STATUS
continue
### NEXT
Implement the primary solver using the left/top edge gauge: for each test case, scan rows left-to-right maintaining row_prefix (parity of A to the left) and col_prefix[j] (parity of A above); for each B cell union row i with column W+j in a weighted XOR DSU with weight 1 ^ row_prefix ^ col_prefix[j]; check that every row and column has even A parity; if any parity is odd or the DSU is inconsistent output 0, otherwise output pow(2, components, 998244353). Use fast buffered I/O and a path-compressing weighted DSU, and report the sample test verdict.
### TASKS
- [done] Ideate and select the row/column gauge reduction to a weighted XOR DSU on H+W nodes.
- [todo] Implement the primary solver: fast input, row/column prefix parity checks, weighted XOR DSU on H+W nodes with B-edge weight 1^row_prefix^col_prefix, and output 0 or pow(2, components, 998244353).
- [todo] Validate the implementation with a brute-force enumerator on tiny random grids (all-A, all-B, odd row/column parity, torus wrap) and report the sample verdict.
- [todo] If validation fails or performance is poor, debug the gauge/label or switch to the full edge-midpoint parity DSU (2HW variables) as an independent cross-check/fallback.
