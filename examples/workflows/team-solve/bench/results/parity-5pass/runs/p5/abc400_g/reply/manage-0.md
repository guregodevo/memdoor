### STATUS
continue

### NEXT
Implement the primary solver using the baseline parity-correction method: for each test case compute m_i=max(X_i,Y_i,Z_i), sort by m_i descending, take S=2K selected cakes, set W0=sum m_i and p=XOR of canonical max-color codes (X=1,Y=2,Z=3) over selected cakes; if p==0 output W0. Otherwise build top-3 candidate lists: for each base color a, selected items with smallest m_i; for each a!=b, selected items of base color a with smallest m_i - value_i[b]; for each target b, unselected items with largest value_i[b]. Generate all recolor operations (selected i, a->b, mask a^b, cost m_i - value_i[b]) and swap operations (selected i base a, unselected j target b, mask a^b, cost m_i - value_j[b]) from these top-3 lists. Find the minimum cost of either one operation with mask p or two operations with XOR p that do not share a selected id or an unselected id, and output W0 - min_loss. Use fast stdin, Python int, and O(N log N) per test case.

### TASKS
- [done] Ideation: reduce to choosing 2K cakes and 3 colors with even color counts; baseline top-2K m_i gives an upper bound and parity correction by at most two operations.
- [todo] Implement the primary solver using the baseline parity-correction method with top-3 candidate operation lists and brute-force one/two-operation disjointness check.
- [todo] Validate the primary solver on the samples and on small random cases against a brute-force DP over (item, selected count, parity mask) or exhaustive pairing for tiny N, especially tie-heavy cases, N=2K, and K=1; fix any discrepancies.
- [todo] If validation fails or a counterexample is found, switch to a different approach such as a larger candidate-pool DP or a carefully proven Lagrangian/parity DP fallback instead of continuing to patch the top-3 operation list.
