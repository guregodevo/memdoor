### STATUS
continue
### NEXT
Write only a compact Python 3 code block (no derivation) implementing the already-derived row/column parity-DSU formula: parse with sys.stdin.buffer.read().split(); for each case set parent=[-1]*(H+W), parity=bytearray(H+W), col=bytearray(W), comps=H+W; scan rows, maintain row_pref, on 'A' flip row_pref and col[j], on 'B' union row i with column H+j with w=1^row_pref^col[j] using the prefix values before the current cell; use an iterative parity find returning (root, xor_to_root) and union by negative size, checking pu^pv==w for same root and setting parity[child]=pu^pv^w when merging; if any contradiction, any row_pref odd at row end, or any col parity odd at case end, output 0, else output pow(2, comps, 998244353); skip DSU after a case is bad but still consume its rows; then verify on the sample and basic edge cases without hardcoding them.
### TASKS
- [done] Derive the GF(2) row/column reduction and parity-DSU counting formula.
- [todo] Write a compact Python 3 code block (no derivation) implementing the formula with fast input, iterative parity DSU, bytearray parity/col, and pow(2, components, MOD).
- [todo] Run the code on the sample and small edge cases (all A, all B, odd row/col parity, 2x2) and fix any failures.
