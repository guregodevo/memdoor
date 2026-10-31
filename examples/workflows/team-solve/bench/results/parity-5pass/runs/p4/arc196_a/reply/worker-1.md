### CODE
```python
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
```

### NOTES
- **Reduction:** A valid deletion order is a noncrossing matching of original indices, with one unmatched index when n is odd. For any remaining element, its current position parity equals its original index parity because each deletion shifts it by 0 or 2. Thus the final survivor in odd n must have odd 1-based index, so it splits the sequence into two independent even-length blocks.
- **Even block lemma:** For any even-length multiset of size 2*r, the maximum score is sum of the r largest values minus sum of the r smallest values. Upper bound: orient each pair from smaller to larger; the r larger endpoints form an r-subset, so total score is at most top-r sum minus bottom-r sum. Achievability: label the r largest values plus and the r smallest values minus, then repeatedly delete adjacent opposite labels. A balanced plus/minus string always has an adjacent opposite pair, and each deleted pair contributes plus minus minus, reaching the bound.
- **Odd n formula:** Let m = n // 2. If the survivor is at 1-based position 2*t + 1, where t = 0..m, the left block is the first 2*t elements and the right block is the last 2*(m-t) elements. The answer is max_t F(first 2*t) + F(last 2*(m-t)). Empty block value is 0.
- **Implementation:** even_prefix_scores scans an iterable and maintains two heaps with sums. lower is a max-heap using negative values and stores the smaller half; upper is a min-heap and stores the larger half. Invariant after i elements: all lower values <= all upper values, and len(lower) - len(upper) = i mod 2. When i is even, lower and upper each have i/2 elements, so the prefix value is sum_upper - sum_lower.
- **Suffix values:** F for a suffix depends only on the multiset, so compute even_prefix_scores(reversed(a)). Index k in that result gives the value for the last 2*k original elements.
- **Complexity:** O(n log n) time and O(n) memory. Odd n uses two heap scans; even n uses one. Sums can be up to about 3e14, which is fine for Python integers.
- **Verification:** Samples match: n=4 gives 5, n=7 gives 14, n=5 all ones gives 0. Small-n brute force over all removal orders agrees for n <= 8; for example n=3 reduces to max of the two adjacent differences, and n=4 matches the top-half/bottom-half value. Edge cases n=2, all equal values, and monotone increasing/decreasing sequences are handled by the heap rebalancing.
- **Pitfalls:** Do not allow the survivor at an even 1-based position. Do not use the whole-array top-half/bottom-half value for odd n; counterexample 1 2 100 has answer 98, not 99. Keep the heap size parity target correct. The 0-based survivor index is 2*t.

### NEXT
- none

### STATUS
solved
