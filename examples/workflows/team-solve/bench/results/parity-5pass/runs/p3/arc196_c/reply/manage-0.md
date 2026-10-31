### STATUS
continue

### NEXT
Implement and test the complete Python solution using the derived recurrence: extract events from S as each "WB" where white_count >= black_count (m=black_count before W, b=white_count including W); if S[0]=='W' or S[-1]=='B' print 0; if no events print fact[N]; otherwise compute A_i = (fact[b_i] - sum_{j<i} A_j * fact[b_i - m_j]) * invfact[b_i - m_i] mod 998244353 using CDQ divide-and-conquer with shifted NTT convolutions (direct O(left*right) for small blocks), then print (fact[N] - sum_i A_i * fact[N - m_i]) mod 998244353; include a small brute-force checker for N<=8 in development and run the provided samples.

### TASKS
- [done] Derive reduction to prefix events and inclusion-exclusion/renewal recurrence (ideation complete).
- [todo] Implement input handling, factorials/invfactorials, endpoint zero cases, event extraction from "WB" positions, and no-event case.
- [todo] Implement a brute-force strong-connectivity checker and an O(M^2) DP for small N to validate the recurrence on random small cases.
- [todo] Implement fast NTT for MOD=998244353 (prefer DIF forward / DIT inverse without bit-reversal, precomputed roots and inverse lengths) and a convolve helper that extends arrays in place.
- [todo] Implement CDQ over event index: solve left, add left-to-right contributions using direct loops for small products or shifted NTT convolution (Aarr over m-range, kernel segment fact[min_d : min_d + Blen+Mlen-1], result index v + Mlen - 1), then solve right.
- [todo] Run sample tests and small random brute-force comparisons; tune direct/NTT threshold for performance; finalize stdout code.
