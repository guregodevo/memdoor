### STATUS
continue

### NEXT
Implement the complete product-major bitset DP solution in the provided starter class: set max_sum=12*len(nums), offset=max_sum, mask=(1<<(2*max_sum+1))-1, return -1 if abs(k)>max_sum; maintain F0/F1 all-subsequence feasibility including empty, P0/P1 bitsets for non-empty positive products 1..limit, and Z0/Z1 zero-product bitsets; for each x snapshot old F/P/Z, add singleton P1[x] at sum x if x>0 and x<=limit, extend old P[p] to P[p*x] with parity flip and sum +x/-x only when p*x<=limit, extend old Z states for every x, when x==0 seed Z1 from old F0 and Z0 from old F1, update F0/F1 with skip/take shifts, then scan p from limit down to 1 for target bit k in P0/P1 and return p, else return 0 if target bit in Z0/Z1 else -1; run the three examples and edge cases.

### TASKS
- [done] Ideate and select product-major bitset DP with separate feasibility and zero-product tracking.
- [todo] Implement the complete product-major bitset DP solution in the provided starter class: set max_sum=12*len(nums), offset=max_sum, mask=(1<<(2*max_sum+1))-1, return -1 if abs(k)>max_sum; maintain F0/F1 all-subsequence feasibility including empty, P0/P1 bitsets for non-empty positive products 1..limit, and Z0/Z1 zero-product bitsets; for each x snapshot old F/P/Z, add singleton P1[x] at sum x if x>0 and x<=limit, extend old P[p] to P[p*x] with parity flip and sum +x/-x only when p*x<=limit, extend old Z states for every x, when x==0 seed Z1 from old F0 and Z0 from old F1, update F0/F1 with skip/take shifts, then scan p from limit down to 1 for target bit k in P0/P1 and return p, else return 0 if target bit in Z0/Z1 else -1; run the three examples and edge cases.
- [todo] Run the provided examples and edge cases (non-empty k=0, zeros, overlimit prefix before zero, all ones, x>limit, negative k, abs(k)>12n) and fix any failures.
