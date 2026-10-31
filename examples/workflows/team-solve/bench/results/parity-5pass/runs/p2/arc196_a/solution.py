import sys
from heapq import heappush, heappop


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    n = int(data[0])
    a = list(map(int, data[1:1 + n]))
    del data

    # pref[t] = optimal value of the even prefix of length 2*t
    pref = [0] * (n // 2 + 1)

    # Two heaps maintaining the smallest half (lower) and largest half (upper).
    # lower is a max-heap stored with negated values.
    lower = []
    upper = []
    sum_lower = 0
    total = 0
    lo = hi = 0

    for idx, x in enumerate(a, 1):
        total += x

        if lo == 0 or x <= -lower[0]:
            heappush(lower, -x)
            sum_lower += x
            lo += 1
        else:
            heappush(upper, x)
            hi += 1

        if lo > hi + 1:
            v = -heappop(lower)
            sum_lower -= v
            heappush(upper, v)
            lo -= 1
            hi += 1
        elif lo < hi:
            v = heappop(upper)
            heappush(lower, -v)
            sum_lower += v
            lo += 1
            hi -= 1

        if (idx & 1) == 0:
            pref[idx >> 1] = total - 2 * sum_lower

    if (n & 1) == 0:
        sys.stdout.write(str(pref[n >> 1]) + "\n")
        return

    # N is odd: try every possible remaining position with even 0-index.
    lower = []
    upper = []
    sum_lower = 0
    total = 0
    lo = hi = 0
    ans = 0

    for i in range(n - 1, -1, -1):
        if (i & 1) == 0:
            # Suffix a[i+1:] currently has even length.
            cand = pref[i >> 1] + total - 2 * sum_lower
            if cand > ans:
                ans = cand

        x = a[i]
        total += x

        if lo == 0 or x <= -lower[0]:
            heappush(lower, -x)
            sum_lower += x
            lo += 1
        else:
            heappush(upper, x)
            hi += 1

        if lo > hi + 1:
            v = -heappop(lower)
            sum_lower -= v
            heappush(upper, v)
            lo -= 1
            hi += 1
        elif lo < hi:
            v = heappop(upper)
            heappush(lower, -v)
            sum_lower += v
            lo += 1
            hi -= 1

    sys.stdout.write(str(ans) + "\n")


if __name__ == "__main__":
    solve()