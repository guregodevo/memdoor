### CODE
```python
from typing import List

class Solution:
    def minOperations(self, nums: List[int], x: int, k: int) -> int:
        n = len(nums)

        # Coordinate compression for Fenwick trees.
        vals = sorted(set(nums))
        m = len(vals)
        idx_of = {v: i + 1 for i, v in enumerate(vals)}
        comp = [idx_of[v] for v in nums]
        val_at = [0] + vals

        # Fenwick trees: counts and sums of values in the current window.
        bit_cnt = [0] * (m + 1)
        bit_sum = [0] * (m + 1)
        freq = [0] * (m + 1)
        total_sum = 0

        def add(idx: int, delta: int) -> None:
            nonlocal total_sum
            v = val_at[idx]
            dv = delta * v
            total_sum += dv
            freq[idx] += delta
            i = idx
            while i <= m:
                bit_cnt[i] += delta
                bit_sum[i] += dv
                i += i & -i

        def prefix(idx: int):
            c = 0
            s = 0
            i = idx
            while i > 0:
                c += bit_cnt[i]
                s += bit_sum[i]
                i -= i & -i
            return c, s

        top_bit = 1 << (m.bit_length() - 1)

        def kth(k: int) -> int:
            """Return the 1-indexed compressed index of the k-th smallest value."""
            idx = 0
            bit = top_bit
            while bit:
                nxt = idx + bit
                if nxt <= m and bit_cnt[nxt] < k:
                    idx = nxt
                    k -= bit_cnt[nxt]
                bit >>= 1
            return idx + 1

        # Initialize the first window.
        for i in range(x):
            add(comp[i], 1)

        w = n - x + 1
        costs = [0] * w
        median_rank = (x + 1) // 2  # lower median, 1-indexed

        # Compute cost for every length-x window.
        for start in range(w):
            mid_idx = kth(median_rank)
            mv = val_at[mid_idx]

            cnt_le, sum_le = prefix(mid_idx)
            cnt_eq = freq[mid_idx]

            cnt_less = cnt_le - cnt_eq
            sum_less = sum_le - cnt_eq * mv

            cnt_greater = x - cnt_le
            sum_greater = total_sum - sum_le

            costs[start] = (
                mv * cnt_less - sum_less
                + sum_greater - mv * cnt_greater
            )

            # Slide the window.
            if start + 1 < w:
                add(comp[start], -1)
                add(comp[start + x], 1)

        # DP over prefix length and number of chosen windows.
        INF = 10 ** 30
        prev = [0] * (n + 1)  # dp[0][i] = 0

        for t in range(1, k + 1):
            curr = [INF] * (n + 1)
            start_i = t * x

            for i in range(start_i, n + 1):
                best = curr[i - 1]
                p = prev[i - x]
                if p < INF:
                    cand = p + costs[i - x]
                    if cand < best:
                        best = cand
                curr[i] = best

            prev = curr

        return prev[n]
```

### NOTES
- **Window cost:** For each length-x window, the cheapest way to make all elements equal is to move every element to a median; the cost is the sum of absolute deviations. For even x, the lower median, rank `(x + 1) // 2` in 1-indexed order, is optimal.
- **Sliding median:** Coordinate-compress all values in `nums`. Maintain the current window in two Fenwick trees: one for counts and one for value sums. Also maintain a frequency array and `total_sum` for O(1) equal-value subtraction.
- **Fenwick kth:** The lower median is found by binary lifting on the count Fenwick tree. The sum Fenwick tree gives prefix sums of actual values, including negative values safely because Python integers are arbitrary precision.
- **Cost formula:** If the median value is `mv`, let `cnt_le` and `sum_le` be prefix count/sum up to `mv`, and `cnt_eq = freq[mv]`. Then `cnt_less = cnt_le - cnt_eq`, `sum_less = sum_le - cnt_eq * mv`, `cnt_greater = x - cnt_le`, and `sum_greater = total_sum - sum_le`. The cost is `mv * cnt_less - sum_less + sum_greater - mv * cnt_greater`.
- **Sliding:** Initialize the first x elements. For each start, compute its cost, then remove `nums[start]` and add `nums[start + x]` to move to the next window.
- **DP:** Let `dp[t][i]` be the minimum cost to choose exactly t non-overlapping length-x windows inside the first i elements. Transition: `dp[t][i] = min(dp[t][i - 1], dp[t - 1][i - x] + cost[i - x])` when `i >= x`. The second term chooses the window ending at `i - 1`, starting at `i - x`; the first term skips position `i - 1`.
- **Two-row DP:** Since `k <= 15`, keep only `prev` and `curr` rows of length `n + 1`. Base row `dp[0][i] = 0` for all i. Impossible states remain `INF`.
- **Exactly k vs at least k:** All window costs are nonnegative, so any feasible solution with more than k windows contains a subset of k non-overlapping windows with no larger total cost. Therefore minimizing exactly k windows is equivalent to satisfying at least k.
- **Complexity:** Computing all window costs is `O(n log n)` with coordinate compression and Fenwick trees. DP is `O(k n)`. Memory is `O(n + m)`, where `m` is the number of distinct values.
- **Edge cases:** `x = n` with `k = 1`; adjacent windows are allowed; negative values; duplicate values; even-length windows; large costs handled by Python integers and `INF = 10 ** 30`.

### NEXT
- none

### STATUS
solved
