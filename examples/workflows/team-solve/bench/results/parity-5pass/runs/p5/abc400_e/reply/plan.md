### PLAN
A 400 number has exactly two distinct prime factors, each with an even exponent, so it is exactly the square of an integer that has exactly two distinct prime factors. Since \(A \le 10^{12}\), the square root of any answer is at most \(10^6\). We can precompute all integers up to \(10^6\) that have exactly two distinct prime factors, then square them to obtain all possible 400 numbers. The resulting list is naturally sorted, so each query can be answered by binary search for the largest precomputed square not exceeding \(A\). This gives a fast solution with about \(10^6\) preprocessing and \(O(\log 10^6)\) per query.

### TASKS
- Read all queries first and determine the maximum needed root limit as \(\lfloor \sqrt{\max(A)} \rfloor\), capped at \(10^6\).
- Build a smallest-prime-factor sieve up to the limit and compute the number of distinct prime factors for every integer up to the limit.
- Create a sorted list of all 400 numbers by appending \(i^2\) for every \(i\) whose distinct prime factor count is exactly 2.
- For each query value \(A\), use binary search, e.g. `bisect_right`, to find the largest precomputed 400 number not exceeding \(A\).
- Output the answers for all queries using fast I/O, relying on the guarantee that at least one valid number exists for every query.
