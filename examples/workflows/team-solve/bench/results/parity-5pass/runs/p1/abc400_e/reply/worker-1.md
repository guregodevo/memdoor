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
    del data

    max_a = max(queries)
    m = math.isqrt(max_a)

    # cnt[x] = number of distinct prime factors of x
    cnt = [0] * (m + 1)
    for p in range(2, m + 1):
        if cnt[p] == 0:  # p is prime
            for j in range(p, m + 1, p):
                cnt[j] += 1

    # A 400 number is exactly x^2 where x has exactly 2 distinct prime factors.
    values = [i * i for i in range(2, m + 1) if cnt[i] == 2]

    br = bisect_right
    out = []
    append = out.append
    for a in queries:
        idx = br(values, a) - 1
        append(str(values[idx]))

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    main()
```

### NOTES
- **Form:** A 400 number has exactly two distinct prime factors and all exponents even, so it is always of the form `(p^a q^b)^2`. Equivalently, it is a square of an integer `x` whose number of distinct prime factors is exactly 2.
- **Bound:** Since `A <= 10^12`, any relevant square root `x` satisfies `x <= 10^6`. Using `math.isqrt(max_query)` gives the exact needed bound and can be smaller for small inputs.
- **Sieve:** Maintain `cnt[n]` as the count of distinct prime divisors of `n`. Iterate `p` from 2 to `M`. If `cnt[p] == 0`, then `p` has no smaller prime divisor, so it is prime. Increment `cnt[j]` for every multiple `j` of `p`. This counts each distinct prime factor exactly once.
- **Collection:** For every `i` with `cnt[i] == 2`, append `i * i`. Because `i` is iterated in increasing order, the resulting list of squares is already sorted.
- **Queries:** For each query `A`, use `bisect_right(values, A) - 1` to get the largest precomputed 400 number not exceeding `A`. The constraints guarantee at least one valid number, and `A >= 36`, so the index is valid.
- **Complexity:** Preprocessing is `O(M log log M)` with `M <= 10^6`, which is very small. Memory is `O(M + K)`, where `K` is the number of valid squares. Each query is answered in `O(log K)`.
- **Edge cases:** `A = 36` gives `M = 6` and the only value `36`. `A = 10^12` includes `10^6 = 2^6 * 5^6`, so `10^12` itself is a valid 400 number and is included.

### NEXT
- none

### STATUS
solved
