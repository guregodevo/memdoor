### PLAN
A 400 number has exactly two distinct prime factors and all prime exponents even, so it must be of the form \((p^a q^b)^2\) for distinct primes \(p,q\). Since \(A \le 10^{12}\), it is enough to consider squares \(x^2\) where \(x \le 10^6\). Precompute all \(x \le 10^6\) that have exactly two distinct prime factors, square them, and store the resulting 400 numbers. Then answer each query by binary searching for the largest precomputed value not exceeding \(A\). This gives fast preprocessing and logarithmic query time.

### TASKS
- Formalize the characterization: 400 numbers are exactly squares of integers with exactly two distinct prime factors, and the square root is at most \(10^6\).
- Build a sieve up to \(10^6\), preferably a smallest-prime-factor sieve, to factor numbers quickly.
- Iterate over all \(x\) from \(2\) to \(10^6\), count distinct prime factors using the sieve, and append \(x^2\) when the count is exactly 2.
- Sort the generated list of 400 numbers and confirm edge cases such as \(36\) and \(10^{12}\) are handled.
- Process all \(Q\) queries with fast input/output, using binary search to find the largest 400 number \(\le A\) for each query.
