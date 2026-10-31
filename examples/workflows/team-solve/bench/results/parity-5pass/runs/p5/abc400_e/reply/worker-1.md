### CODE
```python
import sys
import math
from bisect import bisect_right

def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    q = int(data[0])
    queries = [int(x) for x in data[1:1 + q]]
    if not queries:
        return

    max_a = max(queries)
    L = math.isqrt(max_a)

    # Sieve primes up to L.
    is_prime = bytearray(b'\x01') * (L + 1)
    if L >= 0:
        is_prime[0] = 0
    if L >= 1:
        is_prime[1] = 0

    limit = math.isqrt(L)
    for i in range(2, limit + 1):
        if is_prime[i]:
            start = i * i
            is_prime[start:L + 1:i] = b'\x00' * (((L - start) // i) + 1)

    primes = [i for i in range(2, L + 1) if is_prime[i]]

    # cnt[x] = number of distinct prime factors of x.
    cnt = bytearray(L + 1)
    L1 = L + 1
    for p in primes:
        for m in range(p, L1, p):
            cnt[m] += 1

    # All 400 numbers up to max_a are squares of numbers with exactly
    # two distinct prime factors.
    vals = [i * i for i in range(L1) if cnt[i] == 2]

    out = []
    br = bisect_right
    for a in queries:
        out.append(str(vals[br(vals, a) - 1]))

    sys.stdout.write('\n'.join(out))

if __name__ == "__main__":
    main()
```

### NOTES
- **Core equivalence:** A 400 number has factorization p^{2a} q^{2b} with a,b >= 1, so it is m^2 where m = p^a q^b has exactly two distinct prime factors. Conversely, squaring any integer with exactly two distinct prime factors gives a 400 number.
- **Query reduction:** For each A, the answer is the largest square m^2 <= A with omega(m) = 2. Since A <= 10^12, m <= 10^6, so all candidates can be precomputed up to isqrt(max query).
- **Sieve:** Use a bytearray of ones up to L, clear 0 and 1, then for i up to isqrt(L) clear multiples from i*i using extended slice assignment. This yields primes up to L quickly and with low memory.
- **Distinct factor count:** Initialize cnt = bytearray(L+1). For every prime p, loop over p, 2p, 3p, ... <= L and do cnt[m] += 1. Because each prime is processed once, cnt[m] equals the number of distinct prime factors of m, not the total exponent sum. Values stay small, so a byte is enough.
- **Candidate list:** Build vals = [i*i for i in range(L+1) if cnt[i] == 2]. The list is sorted because i increases. The smallest candidate is 6^2 = 36, matching the minimum possible 400 number and the constraint A >= 36.
- **Answering:** For each query A, use bisect_right(vals, A) - 1 to get the last candidate not exceeding A. The problem guarantees this index is valid. Join all string answers with newlines for fast output.
- **Complexity:** Precomputation is O(L log log L) for the sieve and about O(L sum_{p<=L} 1/p), roughly 3e6 increments for L = 10^6. Building vals is O(L). Each query is O(log M), where M is the number of candidates. Memory is O(L + M), well within limits.
- **Edge cases checked:** A = 36 gives L = 6 and vals = [36]. A = 10^12 has root 10^6 = 2^6 * 5^6, so 10^12 itself is included. Non-square A is handled by binary search over squares. Numbers with one distinct prime factor are excluded, while roots like 12 and 20 are included.
- **Implementation notes:** Read all input with sys.stdin.buffer.read().split(). Use math.isqrt for exact integer square roots. The extended slice assignment length is ((L - start)//p + 1), which must match the slice length.

### NEXT
- none

### STATUS
solved
