### STATUS
continue
### NEXT
Implement a small brute-force oracle and random/exhaustive tester to validate the forbidden-pair characterization: for tiny N and M, compare direct feasibility (union equalities + directed cycle check) against the condition that the range contains a bad pair (same left, same right, identical opposite, or same-sign strict crossing); if any mismatch, report a counterexample and stop.
### TASKS
- [todo] Implement a small brute-force oracle and random/exhaustive tester to validate the forbidden-pair characterization and query reduction on tiny cases.
- [todo] Normalize input into 0-based L, R, sign arrays and compute earliest[i] from next same L/R by a backward scan.
- [todo] Implement the reusable orientation routine: segment tree over a with leaf max-heaps of b, reporting/deleting all previous same-sign items with a in (a_cur, b_cur) and b > b_cur, updating earliest[prev] = min(earliest[prev], cur).
- [todo] Run the orientation routine four times: sign + and sign - on original coordinates (a=L, b=R) and reversed coordinates (a=N-1-R, b=N-1-L).
- [todo] Build suffix minimum bad[i] and answer each query Yes iff bad[L_k] > R_k.
- [todo] Test samples and edge cases (touching endpoints, identical opposite, strict crossing, memory cleanup, 1-indexing) and performance on max constraints.
