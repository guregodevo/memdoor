### STATUS
continue

### NEXT
Implement the Lagrangian 8-state parity DP solution in Python: for each test case, double all X/Y/Z values, precompute them scaled by M=N+1, run an unrolled 8-mask max-plus DP maximizing encoded (score,count) with transitions new[m]=max(old[m], old[m^1]+(2X-beta)*M+1, old[m^2]+(2Y-beta)*M+1, old[m^4]+(2Z-beta)*M+1), binary search the largest beta in [0, 2*max_coord+1) whose final state 0 selected count is at least 2K, then output score//2 + beta*K; validate on the provided samples and small random brute-force cases.

### TASKS
- [done] Verify the reduction: maximum K-pair price equals maximum sum of chosen coordinates over 2K cakes with even X/Y/Z color counts.
- [todo] Implement the Lagrangian 8-state parity DP in Python: double coordinates, precompute scaled values with M=N+1, use encoded (score,count) max-plus DP with correct xor transitions, binary search largest beta with final count >=2K, output score//2 + beta*K.
- [todo] Optimize the DP for Python (unrolled 8-state updates, local variables, fast input, cache the final beta DP value) to handle sum N <= 1e5 within time.
- [todo] Validate on samples and edge cases (zeros, N odd, K=N/2, ties) and with a small random brute-force exact DP to confirm concavity/tie-breaking.
- [todo] If validation or performance fails, switch to a different approach (slope-trick/concave-function DP or bounded parity-loss correction).
