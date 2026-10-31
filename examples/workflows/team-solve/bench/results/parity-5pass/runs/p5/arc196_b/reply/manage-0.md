### STATUS
continue
### NEXT
Implement the primary reduced parity union-find solution in Python and verify it on the sample plus small brute-force/random grids: use fast tokenized input, an iterative parity DSU over H+W row/column nodes, maintain col_pref and row_pref, union row i with column W+j for each B cell with weight 1 ^ col_pref[j] ^ row_pref, update prefixes for A cells, check all row and column A counts are even and DSU consistency, and output pow(2, components, 998244353) or 0; provide the final code in the required delimiters.
### TASKS
- [done] Derive the reduced edge-bit parity model and the row/column xor-equation/DSU counting formula.
- [todo] Implement the primary reduced parity union-find solution in Python and verify it on the sample plus small brute-force/random grids: use fast tokenized input, an iterative parity DSU over H+W row/column nodes, maintain col_pref and row_pref, union row i with column W+j for each B cell with weight 1 ^ col_pref[j] ^ row_pref, update prefixes for A cells, check all row and column A counts are even and DSU consistency, and output pow(2, components, 998244353) or 0; provide the final code in the required delimiters.
- [todo] If verification fails, debug prefix/DSU parity or switch to a full-edge parity DSU / sparse GF(2) rank checker for small cases.
- [todo] Finalize the code for performance and formatting: O(sum HW alpha(H+W)) time, O(H+W) memory per case, no hardcoded sample behavior, output one answer per test case.
