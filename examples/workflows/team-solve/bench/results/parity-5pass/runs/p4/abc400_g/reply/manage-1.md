### STATUS
done
### NEXT

### TASKS
- [done] Verify the reduction: maximum K-pair price equals maximum sum of chosen coordinates over 2K cakes with even X/Y/Z color counts.
- [done] Implement the Lagrangian 8-state parity DP in Python: double coordinates, precompute scaled values, use encoded (score,count) max-plus DP with correct xor transitions, binary search largest beta with final count >= 2K, output score//2 + beta*K.
- [done] Optimize the DP for Python (unrolled 8-state updates, local variables, fast input, special cases K=1 and K=N//2) to handle sum N <= 1e5 within time.
- [done] Validate on samples and edge cases (zeros, N odd, K=N/2, ties) and with a small random brute-force exact DP to confirm concavity/tie-breaking.
