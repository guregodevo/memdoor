import sys
from heapq import heappush, heappop


def even_prefix_scores(a):
    """
    Returns a list res where res[k] is the optimal score for the prefix
    of length 2*k.  res[0] = 0.

    For an even-length multiset of size 2*r, the optimum is
    sum(largest r) - sum(smallest r).
    """
    lower = []  # max-heap via negative values: smaller half
    upper = []  # min-heap: larger half
    sum_lower = 0
    sum_upper = 0
    res = [0]

    push = heappush
    pop = heappop
    append = res.append

    for i, x in enumerate(a, 1):
        # Insert into the heap according to the current median boundary.
        if lower and x > -lower[0]:
            push(upper, x)
            sum_upper += x
        else:
            push(lower, -x)
            sum_lower += x

        # Desired size difference: len(lower) - len(upper) = i mod 2.
        target = i & 1
        diff = len(lower) - len(upper)

        if diff > target:
            v = -pop(lower)
            sum_lower -= v
            push(upper, v)
            sum_upper += v
        elif diff < target:
            v = pop(upper)
            sum_upper -= v
            push(lower, -v)
            sum_lower += v

        if (i & 1) == 0:
            append(sum_upper - sum_lower)

    return res


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    n = int(data[0])
    a = [int(x) for x in data[1:1 + n]]
    del data

    pref = even_prefix_scores(a)

    if n % 2 == 0:
        ans = pref[n // 2]
    else:
        m = n // 2
        suff = even_prefix_scores(reversed(a))

        # Survivor is at 1-based position 2*t + 1, t = 0..m.
        # Left block length is 2*t, right block length is 2*(m-t).
        ans = 0
        for t in range(m + 1):
            val = pref[t] + suff[m - t]
            if val > ans:
                ans = val

    sys.stdout.write(str(ans) + "\n")


if __name__ == "__main__":
    main()