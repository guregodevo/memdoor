### PLAN
A 400 number N has even exponents for exactly two primes, so N = x^2 where x has exactly two distinct prime factors. Since A <= 10^12, x <= 10^6, so we can precompute all relevant x up to 10^6. Use a sieve to count distinct prime factors for every integer up to 10^6 and mark those with count 2. Build a prefix maximum array giving the largest marked x not exceeding any index. For each query, take floor(sqrt(A)), look up the prefix maximum, and square it. This answers each query in O(1) after O(10^6 log log 10^6) preprocessing.

### TASKS
- Parse input, collect all A values, and determine the maximum needed square root limit M = floor(sqrt(max A)) (or use 10^6).
- Implement a sieve up to M that computes the number of distinct prime factors for each integer, e.g. by incrementing a count array at multiples of each prime.
- Create a prefix-best array where prefix_best[i] is the largest integer <= i with exactly two distinct prime factors.
- For each query A, compute m = floor(sqrt(A)), retrieve x = prefix_best[m], and output x*x.
- Verify correctness and performance on edge cases such as A=36, A just below a valid square, and A=10^12, ensuring O(Q) query time and memory within limits.
