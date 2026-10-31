### STATUS
continue
### NEXT
Implement the full Python solution: read all queries, compute L = isqrt(max(A)); sieve primes up to L; use a bytearray distinct-prime-factor counter incremented once for each multiple of each prime; build sorted list vals = [i*i for i in range(L+1) if cnt[i] == 2]; answer each A with vals[bisect_right(vals, A) - 1]; print all answers with fast I/O.
### TASKS
- [done] Reduce the problem to a predecessor query over squares of integers up to 10^6 that have exactly two distinct prime factors.
- [todo] Implement the full Python solution using the maximum query root limit, a bytearray prime sieve, distinct-prime-factor counting, a sorted list of valid squares, and bisect_right.
- [todo] Verify sample cases and edge cases such as A=36, A=400, A=10^12, and non-square A, and confirm performance for Q=2e5.
