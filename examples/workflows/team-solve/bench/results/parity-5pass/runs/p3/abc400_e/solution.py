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
    m = math.isqrt(max_a)

    distinct = bytearray(m + 1)
    for p in range(2, m + 1):
        if distinct[p] == 0:
            for multiple in range(p, m + 1, p):
                distinct[multiple] += 1

    roots = [i for i in range(2, m + 1) if distinct[i] == 2]

    out = []
    isqrt = math.isqrt
    br = bisect_right

    for a in queries:
        r = isqrt(a)
        idx = br(roots, r) - 1
        if idx >= 0:
            root = roots[idx]
            out.append(str(root * root))
        else:
            out.append("0")

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    main()