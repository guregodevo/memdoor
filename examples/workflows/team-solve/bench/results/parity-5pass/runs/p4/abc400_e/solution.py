import sys
import math
from array import array


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    q = int(data[0])
    queries = [0] * q
    max_a = 0

    for i in range(q):
        a = int(data[i + 1])
        queries[i] = a
        if a > max_a:
            max_a = a

    del data

    if q == 0:
        return

    isqrt = math.isqrt
    n = isqrt(max_a)

    if n < 6:
        sys.stdout.write("\n".join("0" for _ in range(q)))
        return

    n1 = n + 1

    # omega[x] = number of distinct prime factors of x
    omega = bytearray(n1)
    for i in range(2, n1):
        if omega[i] == 0:  # i is prime
            for j in range(i, n1, i):
                omega[j] += 1

    # best[x] = largest y <= x such that omega[y] == 2
    best = array("I", [0]) * n1
    last = 0
    for i in range(6, n1):
        if omega[i] == 2:
            last = i
        best[i] = last

    out = []
    append = out.append
    for a in queries:
        b = best[isqrt(a)]
        append(str(b * b))

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    main()