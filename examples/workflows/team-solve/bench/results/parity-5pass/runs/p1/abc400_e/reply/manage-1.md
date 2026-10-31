### STATUS
done
### NEXT

### TASKS
- [done] Formalize the characterization: 400 numbers are exactly squares of integers with exactly two distinct prime factors, and the square root is at most 10^6.
- [done] Implement the full Python solution: read all queries, compute the maximum square root bound 10^6, build a smallest-prime-factor or distinct-prime-count sieve up to 10^6, collect x^2 for every x with exactly 2 distinct prime factors, sort the list, and answer each query with binary search for the largest value <= A; ensure fast I/O and correct handling of A=36 and A=10^12.
- [done] Verify the implementation against the sample cases and edge cases, especially 36, 400, 60, 10^12, and 123456789.
