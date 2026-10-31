import sys
from heapq import heappush, heapreplace


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    n = int(data[0])
    a = list(map(int, data[1:1 + n]))
    del data

    if n % 2 == 0:
        a.sort()
        h = n // 2
        print(sum(a[h:]) - sum(a[:h]))
        return

    push = heappush
    replace = heapreplace

    suffix = [0] * (n + 1)
    low = []
    high = []
    s_low = 0
    s_high = 0

    for i in range(n - 1, -1, -1):
        x = a[i]
        odd = i & 1
        if odd:
            push(high, x)
            s_high += x
        else:
            push(low, -x)
            s_low += x

        if low and high:
            v = -low[0]
            w = high[0]
            if v > w:
                d = v - w
                replace(low, -w)
                replace(high, v)
                s_low -= d
                s_high += d

        if odd:
            suffix[i] = s_high - s_low

    ans = suffix[1]

    low = []
    high = []
    s_low = 0
    s_high = 0

    for i, x in enumerate(a):
        odd = i & 1
        if odd:
            push(high, x)
            s_high += x
        else:
            push(low, -x)
            s_low += x

        if low and high:
            v = -low[0]
            w = high[0]
            if v > w:
                d = v - w
                replace(low, -w)
                replace(high, v)
                s_low -= d
                s_high += d

        if odd:
            cand = s_high - s_low + suffix[i + 2]
            if cand > ans:
                ans = cand

    print(ans)


if __name__ == "__main__":
    main()