### STATUS
continue

### NEXT
Implement the full Python solution using an omega sieve: precompute the number of distinct prime factors for every integer up to 1_000_000, build the sorted list of all 400 numbers as squares of bases with exactly 2 distinct prime factors, then read all queries and answer each with bisect_right on that list.

### TASKS
- [done] Reduce the problem to finding the largest square of a base <= 10^6 that has exactly 2 distinct prime factors.
- [todo] Implement Python solution: omega sieve up to 1_000_000, build sorted list of squares for bases with omega == 2, and answer each query with bisect_right.
- [todo] Verify sample and edge cases by running the code: A=36, A=404, A=10^12, and values just below valid 400 numbers.
