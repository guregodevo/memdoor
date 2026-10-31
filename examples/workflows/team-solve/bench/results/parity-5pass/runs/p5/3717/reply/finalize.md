### CODE
```python
from typing import List


class Solution:
    def minOperations(self, nums: List[int], x: int, k: int) -> int:
        n = len(nums)
        if k == 0:
            return 0

        # Coordinate compression for all values.
        vals = sorted(set(nums))
        rank = {v: i + 1 for i, v in enumerate(vals)}
        ranks = [rank[v] for v in nums]
        del rank
        m = len(vals)

        # Fenwick trees: counts and sums of actual values.
        bit_cnt = [0] * (m + 1)
        bit_sum = [0] * (m + 1)
        max_pow = 1 << (m.bit_length() - 1)

        # Build the first window.
        total_sum = 0
        for i in range(x):
            v = nums[i]
            r = ranks[i]
            while r <= m:
                bit_cnt[r] += 1
                bit_sum[r] += v
                r += r & -r
            total_sum += v

        num_windows = n - x + 1
        costs = [0] * num_windows
        median_rank = (x + 1) // 2  # 1-indexed lower median
        last_window = num_windows - 1

        # Compute the minimum cost for every length-x window.
        for start in range(num_windows):
            # Find the compressed rank of the lower median by binary lifting.
            idx = 0
            bit = max_pow
            target = median_rank
            while bit:
                nxt = idx + bit
                if nxt <= m and bit_cnt[nxt] < target:
                    idx = nxt
                    target -= bit_cnt[nxt]
                bit >>= 1
            r = idx + 1
            med = vals[r - 1]

            # Prefix count and sum up to the median rank.
            left_cnt = 0
            left_sum = 0
            rr = r
            while rr:
                left_cnt += bit_cnt[rr]
                left_sum += bit_sum[rr]
                rr -= rr & -rr

            right_cnt = x - left_cnt
            right_sum = total_sum - left_sum
            costs[start] = (med * left_cnt - left_sum) + (right_sum - med * right_cnt)

            # Slide the window.
            if start < last_window:
                out = nums[start]
                rr = ranks[start]
                while rr <= m:
                    bit_cnt[rr] -= 1
                    bit_sum[rr] -= out
                    rr += rr & -rr
                total_sum -= out

                inn = nums[start + x]
                rr = ranks[start + x]
                while rr <= m:
                    bit_cnt[rr] += 1
                    bit_sum[rr] += inn
                    rr += rr & -rr
                total_sum += inn

        # Free structures no longer needed before DP.
        del vals, ranks, bit_cnt, bit_sum

        # DP:
        # dp[t][i] = min cost to choose t non-overlapping windows
        # inside the first i elements.
        #
        # Transition:
        # dp[t][i] = min(dp[t][i - 1],
        #                dp[t - 1][i - x] + cost[i - x])
        INF = 10 ** 30
        prev = [0] * (n + 1)

        for _ in range(k):
            cur = [INF] * (n + 1)
            p = prev
            c = cur
            cst = costs
            xx = x
            for i in range(xx, n + 1):
                best = c[i - 1]
                cand = p[i - xx] + cst[i - xx]
                if cand < best:
                    best = cand
                c[i] = best
            prev = cur

        return prev[n]
```

### NOTES
- **Decomposition:** Selected windows are disjoint, so their operation costs add. Each window’s minimum cost is the sum of absolute deviations to a median.
- **Window cost:** Use the lower median rank `(x + 1) // 2`. For even `x`, any median in the middle interval is optimal; the lower median is convenient.
- **Sliding median:** Coordinate-compress all values. Maintain Fenwick trees for counts and sums of actual values over the current length-`x` window. This avoids lazy-deletion heap issues and handles negative values.
- **Fenwick kth:** Binary lifting over the count tree finds the smallest compressed rank whose prefix count reaches the median rank. Start from the highest power of two `<= m`.
- **Cost query:** Let `left_cnt` and `left_sum` be prefix count/sum up to the median rank. Cost = `med * left_cnt - left_sum + (total_sum - left_sum) - med * (x - left_cnt)`. Including all values equal to the median on the left is safe because their contribution is zero.
- **Sliding update:** Initialize the first `x` elements. For each next window, remove `nums[start]` and add `nums[start + x]` to both Fenwick trees and update `total_sum`.
- **Selection DP:** `dp[t][i]` is the minimum cost to choose `t` non-overlapping windows inside the first `i` elements. Recurrence: `dp[t][i] = min(dp[t][i - 1], dp[t - 1][i - x] + cost[i - x])`. The second term takes the window starting at `i - x` and ending at `i - 1`; previous windows must end before `i - x`.
- **At least k:** All window costs are nonnegative, so any solution with more than `k` windows can drop extras without increasing cost. Minimizing exactly `k` windows is therefore equivalent to minimizing at least `k` windows.
- **Memory and complexity:** Two DP rows of length `n + 1` and a cost array of length `n - x + 1`. Overall `O(n log n + k n)` time and `O(n)` memory.
- **Edge cases:** `x = n`, `k = 1`, `k*x = n`, adjacent windows, all equal values, negative values, duplicate medians, even `x`, and large costs with `INF = 10**30`.
- **Validation:** Checked against the two examples and small random brute force enumerating non-overlapping window starts; final code omits the test harness for judge use.

### NEXT
- none

### STATUS
solved
