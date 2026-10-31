### STATUS
continue
### NEXT
Stress-test the final solver against the oracle on random small cases and targeted edge cases (touching endpoints, length-two intervals, opposite-sign crossings/nesting, shared endpoints, all same sign), and fix any mismatches.
### TASKS
- [done] Implement a standalone brute-force oracle and theorem validator for small cases comparing Bellman-Ford feasibility with the bad-pair condition.
- [done] Replace the oracle with the final standalone solver: read stdin; compute l/r/sign for each person; compute f[j] from last_left/last_right endpoint scans and same-sign strict crossings using CDQ over each sign list (base 128 brute; cross Type A sweep right by l ascending, insert left with l_i<l_j at r_i, query max in [l_j+1,r_j-1]; cross Type B sweep right by r descending, insert left with r_i>r_j at l_i, query max in [l_j+1,r_j-1]); use a timestamped iterative segment tree for point-max/range-max; build pref and answer each query Yes iff L>pref[R]; output only Yes/No.
- [todo] Stress-test the final solver against the oracle on random small cases and targeted edge cases (touching endpoints, length-two intervals, opposite-sign crossings/nesting, shared endpoints, all same sign), and fix any mismatches.
- [todo] Optimize/tune the final Python solver for max constraints (fast I/O, base threshold, local bindings, timestamped segment tree) and verify it handles the full limits within time/memory.
