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

        # Fenwick trees: counts and sums of actual values.
        cnt_bit = [0] * (m + 1)
        sum_bit = [0] * (m + 1)

        total = 0
        rank = (x + 1) // 2          # lower median rank
        top_bit = 1 << (m.bit_length() - 1)

        # cost[p] = minimum operations to make the length-x window ending at p constant.
        cost = [0] * (n + 1)

        for i, val in enumerate(nums):
            # Add nums[i].
            j = idxs[i]
            while j <= m:
                cnt_bit[j] += 1
                sum_bit[j] += val
                j += j & -j
            total += val

            # Remove nums[i - x] to keep a sliding window of length x.
            if i >= x:
                old = nums[i - x]
                j = idxs[i - x]
                while j <= m:
                    cnt_bit[j] -= 1
                    sum_bit[j] -= old
                    j += j & -j
                total -= old

            # Once the window has length x, compute its median-based cost.
            if i >= x - 1:
                # Find the compressed index of the lower median.
                idx_med = 0
                bitmask = top_bit
                kk = rank
                while bitmask:
                    nxt = idx_med + bitmask
                    if nxt <= m and cnt_bit[nxt] < kk:
                        idx_med = nxt
                        kk -= cnt_bit[nxt]
                    bitmask >>= 1
                med_idx = idx_med + 1

                # Prefix count and prefix sum up to the median value.
                count_le = 0
                sum_le = 0
                j = med_idx
                while j:
                    count_le += cnt_bit[j]
                    sum_le += sum_bit[j]
                    j -= j & -j

                med = vals[med_idx - 1]
                cost[i + 1] = (
                    med * count_le
                    - sum_le
                    + (total - sum_le)
                    - med * (x - count_le)
                )

        INF = 10 ** 30

        # DP over number of chosen windows and prefix length.
        # prev[p] = min cost for t-1 windows inside first p positions.
        prev = [0] * (n + 1)

        for t in range(1, k + 1):
            cur = [INF] * (n + 1)
            start = t * x

            for p in range(start, n + 1):
                best = cur[p - 1]
                take = prev[p - x] + cost[p]
                if take < best:
                    best = take
                cur[p] = best

            prev = cur

        return prev[n]
```

### NOTES
- **Decomposition:** The final array only needs k disjoint length-x blocks. For any chosen block, the cheapest operations to make it constant are independent of other blocks, so the problem becomes choosing k non-overlapping windows with minimum total median cost.
- **Window cost:** For a fixed window, the optimal target value is a median. Using the lower median rank `(x + 1) // 2`, the cost is `med * count_le - sum_le + (total - sum_le) - med * (x - count_le)`, where `count_le` and `sum_le` are the count and sum of elements less than or equal to `med`.
- **Fenwick sliding:** Compress all values. Maintain two Fenwick trees over compressed indices: one for counts and one for actual value sums. Add the incoming element, remove the outgoing element, and update the running total. Find the median index by kth-order query on the count tree, then one combined prefix query gives both `count_le` and `sum_le`.
- **DP:** Let `prev[p]` be the minimum cost for `t - 1` windows inside the first `p` positions. For each `t`, compute `cur[p] = min(cur[p - 1], prev[p - x] + cost[p])` for `p >= t * x`. Taking a window ending at `p` forces previous windows to lie in the first `p - x` positions, so adjacent windows are allowed but overlapping windows are not.
- **At least vs exactly:** All window costs are nonnegative. If a solution has more than k valid disjoint windows, selecting any k of them gives a lower bound no larger than the total operations, so the optimum for at least k equals the optimum for exactly k.
- **Complexity:** Sorting/compression is `O(n log n)`, sliding median costs are `O(n log n)`, and DP is `O(k n)` with `k <= 15`. Memory is `O(n)`.
- **Edge cases handled:** Even `x` uses the lower median; duplicates at the median are included in `count_le`; negative values and large totals are safe with Python integers; adjacent windows are allowed; `k * x == n` works.
- **Sample verdict:** The two provided examples produce 8 and 3. Small random brute-force checks comparing the DP against exhaustive selection of non-overlapping windows with median costs matched.

### NEXT
- none

### STATUS
solved
