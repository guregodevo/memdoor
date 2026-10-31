### PLAN
Observe that a 400 number is exactly the square of an integer whose prime factorization contains exactly two distinct primes, because all exponents in the 400 number are even. Since \(A \le 10^{12}\), \(\sqrt{A} \le 10^6\), so it is enough to precompute information up to \(10^6\). Build a prefix array where each index stores the largest integer up to that index having exactly two distinct prime factors. For each query, take the integer square root of \(A\), look up the prefix array, and square the result. This gives \(O(1)\) query time after \(O(10^6 \log \log 10^6)\) preprocessing.

### TASKS
- Implement a sieve up to \(1{,}000{,}000\) to count the number of distinct prime factors for every integer.
- Build a prefix array `best[i]` storing the largest \(j \le i\) such that \(j\) has exactly two distinct prime factors.
- For each query \(A\), compute \(m = \lfloor \sqrt{A} \rfloor\), retrieve `best[m]`, and output its square.
- Use fast input/output and handle all values with 64-bit-safe integer arithmetic.
