### STATUS
done
### NEXT

### TASKS
- [done] Ideation: select zero/positive decomposition approach
- [done] Implement the full zero/positive decomposition solution in the starter code: early return -1 if abs(k) > sum(nums); positive-product DP over products 1..limit with parity and sum bitsets (ignore zeros and x>limit, use old/new arrays and active product lists, mask shifts to the valid sum range); zero-product feasibility DP with four sum bitsets (no-zero/zero, even/odd parity), where zeros flip parity and set the zero flag and positives shift sums by +/-x; return the largest positive product with sum k, else 0 if zero-feasible, else -1, and run sample/edge tests.
