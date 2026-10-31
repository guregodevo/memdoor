### STATUS
continue
### NEXT
Implement the complete Python solution using the derived formula: read N, S, and T; check that each S character maps to a single T character; compute C = number of present S letters c with target[c] != c; compute indeg from present sources; count closed directed cycles of length >= 2 (all vertices have indeg == 1); compute image_count = number of letters with indeg > 0; output -1 if closed_cycles > 0 and image_count == 26, otherwise C + closed_cycles. Run the four samples and basic edge cases to verify.
### TASKS
- [done] Derive the functional-graph/token model and formula: answer C + closed nontrivial cycles, impossible only when a closed cycle exists and no spare label (image_count == 26).
- [todo] Implement the complete Python solution with stdin/stdout and the formula above.
- [todo] Verify samples and edge cases: S == T, inconsistent mapping, full-alphabet closed cycle (no spare), closed cycle with spare, non-closed cycle with incoming edge, multiple closed cycles.
- [todo] Build a small brute-force oracle for alphabets of size 3-5 and stress-test random S,T against the formula.
- [todo] Final review for O(N+26) complexity, cycle-detection correctness, and clean final code without hardcoded tests.
