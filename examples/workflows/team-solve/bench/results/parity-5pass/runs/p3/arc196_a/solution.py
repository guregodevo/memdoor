import sys
from heapq import heappush, heapreplace


class MedianSplit:
    __slots__ = ("lo", "hi", "sum_lo", "sum_hi")

    def __init__(self):
        self.lo = []
        self.hi = []
        self.sum_lo = 0
        self.sum_hi = 0

    def add_pair(self, x, y, _heappush=heappush, _heapreplace=heapreplace):
        lo = self.lo
        hi = self.hi
        s_lo = self.sum_lo
        s_hi = self.sum_hi

        if not lo:
            if x <= y:
                _heappush(lo, -x)
                s_lo += x
                _heappush(hi, y)
                s_hi += y
            else:
                _heappush(lo, -y)
                s_lo += y
                _heappush(hi, x)
                s_hi += x
        else:
            max_lo = -lo[0]
            x_small = x <= max_lo
            y_small = y <= max_lo

            if x_small and y_small:
                if x > y:
                    x, y = y, x
                _heappush(lo, -x)
                s_lo += x
                v = -_heapreplace(lo, -y)
                s_lo += y - v
                _heappush(hi, v)
                s_hi += v
            elif not x_small and not y_small:
                if x > y:
                    x, y = y, x
                _heappush(hi, x)
                s_hi += x
                v = _heapreplace(hi, y)
                s_hi += y - v
                _heappush(lo, -v)
                s_lo += v
            else:
                if x_small:
                    _heappush(lo, -x)
                    s_lo += x
                    _heappush(hi, y)
                    s_hi += y
                else:
                    _heappush(hi, x)
                    s_hi += x
                    _heappush(lo, -y)
                    s_lo += y

        self.sum_lo = s_lo
        self.sum_hi = s_hi

    def score(self):
        return self.sum_hi - self.sum_lo


def main():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return
    n = data[0]
    a = data[1:]
    del data

    if n & 1:
        k = n // 2
        suff = [0] * (k + 1)

        ms = MedianSplit()
        add_pair = ms.add_pair
        score = ms.score
        idx = 2 * k - 1
        for i in range(k - 1, -1, -1):
            add_pair(a[idx], a[idx + 1])
            idx -= 2
            suff[i] = score()

        ans = suff[0]

        del ms, add_pair, score
        ms = MedianSplit()
        add_pair = ms.add_pair
        score = ms.score
        idx = 0
        pref = 0
        for i in range(1, k + 1):
            add_pair(a[idx], a[idx + 1])
            idx += 2
            pref = score()
            val = pref + suff[i]
            if val > ans:
                ans = val

        print(ans)
    else:
        ms = MedianSplit()
        add_pair = ms.add_pair
        idx = 0
        for _ in range(n // 2):
            add_pair(a[idx], a[idx + 1])
            idx += 2
        print(ms.score())


if __name__ == "__main__":
    main()