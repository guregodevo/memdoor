### CODE
```python
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
```

### NOTES
- **Reduction:** A removal order induces a non-crossing matching on original positions. A matched pair must have an even number of original positions between them, so the two positions have opposite 1-indexed parity. If N is odd, parity leaves exactly one unmatched position, and it must be odd 1-indexed, i.e. even 0-indexed. No matched interval can cross that position, so the array splits into an even-length left block and an even-length right block.
- **Even block value:** In an even block of 2m elements, each removed pair contributes max-min, so the total is a signed sum with m plus signs and m minus signs. This is at most sum(largest m) - sum(smallest m). The bound is attainable by marking the largest m elements plus and the smallest m elements minus, then repeatedly deleting adjacent opposite signs. Such a pair always exists while signs remain, and every plus is at least every minus, so each deletion contributes plus-minus.
- **Prefix/suffix values:** For an even multiset, value = total - 2 * sum(smallest half). Need this for all even prefixes, and for odd N also for suffixes to the right of each even 0-indexed split.
- **Two-heap maintenance:** Keep a max-heap lower for the smallest half and a min-heap upper for the largest half. Invariant: all lower values <= all upper values; after an even number of elements len(lower)=len(upper), after an odd number len(lower)=len(upper)+1. Track sum_lower. Insert into lower if x <= current max lower, else upper, then rebalance by moving the boundary element. This gives the smallest-half sum in O(log N) per element.
- **Algorithm:** Process A left to right and store pref[t] for prefix length 2t. If N is even, answer is pref[N//2]. If N is odd, scan i from N-1 down to 0 while maintaining the same two heaps for the suffix A[i+1:]. At even 0-indexed i, the suffix length is even, so candidate = pref[i//2] + total_suffix - 2*sum_lower_suffix; take the maximum, then insert A[i]. Empty suffix has value 0.
- **Edge cases and arithmetic:** N=2 works; duplicates are fine because plus values can equal minus values; empty blocks contribute 0. Maximum answer is about 1.5e14, so 64-bit is needed, but Python ints are used.
- **Complexity:** O(N log N) time and O(N) memory for A, pref, and heaps. Fast input via sys.stdin.buffer.read().split().
- **Verification:** The three samples give 5, 14, and 0. A brute-force DP over all adjacent removals for N<=8, using the current tuple as a memoized state, matches the formula on small random cases.

### NEXT
none

### STATUS
solved
