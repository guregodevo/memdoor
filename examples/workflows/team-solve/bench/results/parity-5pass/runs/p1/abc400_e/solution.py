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