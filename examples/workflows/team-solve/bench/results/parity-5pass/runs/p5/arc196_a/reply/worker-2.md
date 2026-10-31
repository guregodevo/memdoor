### CODE
```python
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
```

### NOTES
- **Deletion model:** A deletion sequence is a non-crossing matching of removed positions. For odd N the survivor has odd 1-index (even 0-index): deleting a pair before it shifts it by 2, so parity is invariant and the final current index is 1.
- **No crossing survivor:** If a pair (i, j) had i < survivor < j, then at the moment i and j are removed all positions between them must already be gone, impossible because the survivor remains. Thus pairs lie entirely in the prefix before the survivor and the suffix after it.
- **Even block value F:** For 2k elements, orient each pair from larger to smaller. Exactly k elements get coefficient +1 and k get -1, so total <= sum of largest k minus sum of smallest k. This bound is achievable: color largest k as L and smallest k as S. Any balanced binary string has an adjacent L-S pair; deleting such pairs keeps counts balanced and ends empty. Each deletion contributes L-S, total sum(L)-sum(S). Ties can be colored arbitrarily.
- **Odd N reduction:** The survivor can be any even 0-index position p. The score is at most F(a[0:p]) + F(a[p+1:n]), and the bound is achievable by reducing the two even blocks independently. Hence answer = max over even prefix length l=p of F(prefix l) + F(suffix l+1).
- **Computing F for all needed prefixes/suffixes:** Maintain two heaps for the current multiset: low is a max-heap (stored negated) of the smaller half, high is a min-heap of the larger half, with sums s_low, s_high. For an even total length the sizes are equal and F = s_high - s_low. In the odd-N passes, insertion side is determined by index parity: in a left-to-right pass push to low on even i and high on odd i; in a right-to-left pass (N odd) the same parity rule works. After insertion, if max(low) > min(high), swap the two tops with heapreplace and adjust sums by d = v - w.
- **Implementation:** If N is even, sort A and output sum(upper half) - sum(lower half). If N is odd, first scan right-to-left and store suffix[i] = F(a[i:n]) for odd i (even-length suffixes); suffix[n]=0. Then scan left-to-right, and at odd i (prefix length i+1 even) update answer with current F + suffix[i+2]. Complexity O(N log N), memory O(N).
- **Samples:** Sample 1 gives 5, sample 2 gives 14, sample 3 gives 0.

### NEXT
none

### STATUS
solved
