import sys
from heapq import heappush, heappop


def compute_scores(a, reverse=False):
    """
    Returns res where res[t] is the maximum pairing score for the first/last
    2*t elements, depending on reverse.
    """
    n = len(a)
    m = n // 2
    res = [0] * (m + 1)

    low = []   # max-heap via negative values: smallest floor(count/2) values
    high = []  # min-heap: largest ceil(count/2) values
    sum_low = 0
    sum_high = 0

    push = heappush
    pop = heappop

    count = 0
    it = reversed(a) if reverse else a

    for x in it:
        count += 1

        if low and x <= -low[0]:
            push(low, -x)
            sum_low += x
        else:
            push(high, x)
            sum_high += x

        desired_low_size = count >> 1

        if len(low) > desired_low_size:
            v = -pop(low)
            sum_low -= v
            push(high, v)
            sum_high += v
        elif len(low) < desired_low_size:
            v = pop(high)
            sum_high -= v
            push(low, -v)
            sum_low += v

        # Safety guard; normally the invariant already holds.
        if low and high and -low[0] > high[0]:
            v1 = -pop(low)
            sum_low -= v1
            v2 = pop(high)
            sum_high -= v2
            push(low, -v2)
            sum_low += v2
            push(high, v1)
            sum_high += v1

        if (count & 1) == 0:
            res[count >> 1] = sum_high - sum_low

    return res


def solve(a):
    n = len(a)
    m = n // 2

    pref = compute_scores(a, False)

    if n % 2 == 0:
        return pref[m]

    suff = compute_scores(a, True)

    best = 0
    for t in range(m + 1):
        candidate = pref[t] + suff[m - t]
        if candidate > best:
            best = candidate

    return best


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    n = int(data[0])
    a = [int(x) for x in data[1:1 + n]]

    print(solve(a))


if __name__ == "__main__":
    main()