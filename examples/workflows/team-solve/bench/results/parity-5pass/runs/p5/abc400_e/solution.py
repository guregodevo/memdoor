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