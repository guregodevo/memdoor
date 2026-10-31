### CODE
```python
from typing import List

class Solution:
    def minOperations(self, nums: List[int], x: int, k: int) -> int:
        n = len(nums)

        # Coordinate compression for Fenwick trees.
        vals = sorted(set(nums))
        m = len(vals)
        comp = {v: i + 1 for i, v in enumerate(vals)}
        idxs = [comp[v] for v in nums]

        # Fenwick trees: counts and sums over compressed values.
        bit_c = [0] * (m + 1)
        bit_s = [0] * (m + 1)

        # Build the first window.
        window_sum = 0
        for pos in range(x):
            v = nums[pos]
            i = idxs[pos]
            window_sum += v
            while i <= m:
                bit_c[i] += 1
                bit_s[i] += v
                i += i & -i

        # costs[end] = minimum cost to make nums[end-x+1 : end+1] constant.
        costs = [0] * n
        rank = (x + 1) // 2  # lower median rank
        top_bit = 1 << (m.bit_length() - 1)

        # Local bindings for speed.
        bc = bit_c
        bs = bit_s
        vv = vals
        nn = nums
        ii_arr = idxs
        mm = m
        tb = top_bit
        xx = x
        ws = window_sum

        for end in range(xx - 1, n):
            # Find the lower median index in the current window.
            kk = rank
            idx = 0
            bit = tb
            while bit:
                nxt = idx + bit
                if nxt <= mm and bc[nxt] < kk:
                    idx = nxt
                    kk -= bc[nxt]
                bit >>= 1
            med_idx = idx + 1
            med = vv[med_idx - 1]

            # Prefix count and sum up to the median.
            c = 0
            s = 0
            i = med_idx
            while i:
                c += bc[i]
                s += bs[i]
                i -= i & -i

            # Cost = sum_{v <= med} (med - v) + sum_{v > med} (v - med).
            costs[end] = med * c - s + (ws - s) - med * (xx - c)

            # Slide the window.
            if end != n - 1:
                out_pos = end - xx + 1
                out_v = nn[out_pos]
                ws -= out_v
                i = ii_arr[out_pos]
                ds = -out_v
                while i <= mm:
                    bc[i] -= 1
                    bs[i] += ds
                    i += i & -i

                in_pos = end + 1
                in_v = nn[in_pos]
                ws += in_v
                i = ii_arr[in_pos]
                ds = in_v
                while i <= mm:
                    bc[i] += 1
                    bs[i] += ds
                    i += i & -i

        # DP over number of chosen windows and prefix length.
        INF = 10 ** 30
        prev = [0] * (n + 1)  # dp for 0 chosen windows

        for _ in range(k):
            curr = [INF] * (n + 1)
            best = INF
            pv = prev
            co = costs

            for i in range(1, n + 1):
                if i >= xx:
                    p = pv[i - xx]
                    if p != INF:
                        cand = p + co[i - 1]
                        if cand < best:
                            best = cand
                curr[i] = best

            prev = curr

        return prev[n]
```

### NOTES
- **Reduction:** Any valid final array contains at least k disjoint length-x constant blocks. Restrict attention to k of those blocks. For each block, the operations spent on its indices are at least the minimum cost to make that original block constant, which is the sum of absolute deviations from a median. Since the blocks are disjoint, the total cost is at least the sum of their individual minimum costs. Conversely, choosing k disjoint windows and changing each independently to its median achieves exactly that sum. Thus the problem is minimum total weight selection of k non-overlapping fixed-length intervals.
- **Window cost:** For a window, use the lower median rank `(x + 1) // 2`. If `med` is the lower median, `c` is the count of elements `<= med`, and `s` is their sum, then cost is `med*c - s + (window_sum - s) - med*(x - c)`. This handles even-length windows, duplicates, and negative values correctly.
- **Sliding median structure:** Coordinate-compress all `nums` values. Maintain two Fenwick trees over compressed values: one for counts and one for actual value sums. The current window sum is maintained separately. The kth smallest value is found by Fenwick binary lifting in `O(log m)`. Prefix count and sum are obtained in one combined Fenwick prefix loop. Sliding removes the left element and adds the right element with two combined count/sum updates.
- **Dynamic programming:** Let `dp[c][i]` be the minimum cost to choose `c` valid windows inside the first `i` elements. Base `dp[0][i] = 0`. Transition: `dp[c][i] = min(dp[c][i-1], dp[c-1][i-x] + cost[i-1])` for `i >= x`, where `cost[i-1]` is the cost of the window ending at index `i-1`. Rolling arrays are used because only the previous `c-1` layer is needed.
- **At least k vs exactly k:** All window costs are nonnegative. If a solution uses more than k windows, removing extra windows cannot increase cost, so the optimal at-least-k cost equals the optimal exactly-k cost.
- **Complexity:** There are at most `n` windows. Each window costs `O(log n)` for median query and prefix sums, and sliding uses `O(log n)` updates. DP costs `O(k*n)`. Total time is `O(n log n + k*n)`, memory is `O(n)`, which fits `n <= 10^5` and `k <= 15`.
- **Edge cases:** `x = n` gives one window; `k = 1` works; all equal values give zero cost; negative values are handled by sum Fenwick tree; `m = 1` works because `top_bit = 1`; even `x` uses the lower median, which is a valid minimizer; `INF = 10**30` is safely larger than any possible answer.
- **Examples:** The first example selects windows ending at indices 3 and 7 with costs 5 and 3, total 8. The second selects windows ending at indices 2 and 4 with costs 0 and 3, total 3.

### NEXT
none

### STATUS
solved
