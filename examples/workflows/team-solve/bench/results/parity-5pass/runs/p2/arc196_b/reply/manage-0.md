### STATUS
continue
### NEXT
Implement the complete Python solution using the parity/DSU reduction: read all input fast; for each case maintain col_prefix[W] and row_prefix; for each B cell unite row i and column H+j with label 1 ^ row_prefix ^ col_prefix[j]; toggle prefixes for A; invalidate if any row or column A parity is odd or DSU inconsistent; output pow(2, components, 998244353) if valid else 0; then run the provided sample input and confirm outputs 2, 0, 2.
### TASKS
- [done] Derive the XOR/parity reduction and identify the DSU formulation and pitfalls.
- [todo] Implement the full solution with fast input, weighted DSU, prefix parity scanning, row/column parity checks, and modular power output.
- [todo] Verify on the sample and on edge cases (all A with odd row/col parity, all B, inconsistent B-label cycles, many small cases, and total HW=1e6 performance).
