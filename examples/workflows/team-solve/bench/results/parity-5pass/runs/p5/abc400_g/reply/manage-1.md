### STATUS
continue

### NEXT
Validate the current solver on small random tests (N <= 8, all K, values 0..3 plus tie-heavy, all-zero, N=2K, and K=1 cases) against a brute-force exhaustive pairing or subset DP; if any mismatch is found, debug and fix the top-3 parity-correction implementation.

### TASKS
- [done] Ideation: reduce to choosing 2K cakes and 3 colors with even color counts; baseline top-2K m_i gives an upper bound and parity correction by at most two operations.
- [done] Implement the primary solver using the baseline parity-correction method with top-3 candidate operation lists and brute-force one/two-operation disjointness check.
- [todo] Validate the current solver on small random tests (N <= 8, all K, values 0..3 plus tie-heavy, all-zero, N=2K, and K=1 cases) against a brute-force exhaustive pairing or subset DP; if any mismatch is found, debug and fix the top-3 parity-correction implementation.
- [todo] If validation fails or a counterexample is found, switch to a different approach such as a larger candidate-pool DP or a carefully proven Lagrangian/parity DP fallback instead of continuing to patch the top-3 operation list.
