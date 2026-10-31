### CODE
```python
import sys
from bisect import bisect_right

def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    q = int(data[0])

    M = 1_000_000

    # omega[n] = number of distinct prime factors of n
    omega = bytearray(M + 1)
    for i in range(2, M + 1):
        if omega[i] == 0:  # i is prime
            for j in range(i, M + 1, i):
                omega[j] += 1

    # All 400 numbers up to 10^12 are squares of bases <= 10^6
    # having exactly two distinct prime factors.
    vals = [i * i for i in range(2, M + 1) if omega[i] == 2]

    out = []
    append = out.append
    br = bisect_right

    for idx in range(1, q + 1):
        a = int(data[idx])
        append(str(vals[br(vals, a) - 1]))

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    main()
```

### NOTES
- **Core reduction:** A 400 number has all prime exponents even, so it is a perfect square. If N = p^{2a} q^{2b} with distinct primes p and q, then x = p^a q^b has exactly two distinct prime factors and N = x^2. Since N <= 10^12, x <= 10^6. Conversely, every x <= 10^6 with exactly two distinct prime factors gives a valid 400 number x^2.
- **Precomputation:** Use an omega sieve up to M = 1_000_000. `omega[n]` stores the number of distinct prime divisors of n. Initialize a bytearray of zeros. For i from 2 to M, if `omega[i] == 0`, then i is prime; increment `omega[j]` for every multiple j of i. This counts each prime divisor exactly once.
- **Why bytearray is safe:** The maximum number of distinct prime factors for n <= 10^6 is 7, because 2*3*5*7*11*13*17 = 510510 and multiplying by the next prime exceeds 10^6. This is far below 255.
- **Building answer list:** Iterate i from 2 to M and include i*i when `omega[i] == 2`. Because i increases, the squares are already strictly increasing, so no separate sort is needed. The list includes 36 and 10^12, since base 1_000_000 = 2^6 * 5^6 has exactly two distinct prime factors.
- **Query handling:** For each A, use `bisect_right(vals, A) - 1` to get the largest 400 number not exceeding A. `bisect_right` is needed so that exact matches are included. The constraints guarantee A >= 36, so the index is always valid.
- **Complexity:** The sieve performs about M * sum_{p <= M} 1/p, roughly 3 million increments, plus 1 million outer checks. Building the list is O(M). Each query is O(log K), where K is the number of stored 400 numbers. Memory is O(M + K + Q) for the omega array, value list, and output buffer.
- **Pitfalls avoided:** Do not count total prime factors with multiplicity; do not allow one prime factor or p = q; do not rely on floating-point square roots near 10^12; include the upper bound M = 1_000_000; avoid duplicates, which do not occur because bases are unique; keep precomputation independent of the queries.

### NEXT
- none

### STATUS
solved
