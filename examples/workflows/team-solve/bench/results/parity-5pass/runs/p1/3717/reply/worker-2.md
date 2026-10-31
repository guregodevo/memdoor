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
- **Sample tests:** PASS. Example 1 returns 8 and example 2 returns 3.
- **Edge cases:** PASS. x=n with k=1 returns the single-window median cost; k*x=n forces the only feasible adjacent chain and the DP carries it correctly; even and odd x use the lower median rank (x+1)//2; negative values are handled correctly by the signed sum Fenwick tree; all-equal arrays return 0.
- **Random brute force:** PASS. Small exhaustive comparisons against direct median costs over all non-overlapping window selections found no mismatches for small n, random values, and all valid x/k combinations.
- **Failing input:** none found.
- **Window cost:** For each length-x window, the cheapest uniform value is a median; the cost is the sum of absolute deviations to that median. For even x, the lower median rank (x+1)//2 is optimal.
- **Sliding median:** Coordinate-compress all values and maintain two Fenwick trees, one for counts and one for signed value sums, plus a frequency array and total_sum for the current window.
- **Fenwick kth:** The lower median is found by binary lifting on the count Fenwick tree. top_bit is the highest power of two not exceeding m.
- **Cost formula:** With median value mv, prefix count/sum up to mv, and cnt_eq=freq[mv], compute cnt_less, sum_less, cnt_greater, and sum_greater. The cost is mv*cnt_less - sum_less + sum_greater - mv*cnt_greater.
- **Sliding:** Initialize the first x elements. After computing the cost for start, remove nums[start] and add nums[start+x] to form the next window.
- **DP:** dp[t][i] is the minimum cost to choose exactly t non-overlapping length-x windows inside the first i elements. Transition: dp[t][i] = min(dp[t][i-1], dp[t-1][i-x] + cost[i-x]). The second term chooses the window ending at i-1 and starting at i-x.
- **Two-row DP:** Since k <= 15, keep only prev and curr rows of length n+1. Base row dp[0][i] = 0 for all i. Impossible states remain INF.
- **Exactly k vs at least k:** All window costs are nonnegative, so any feasible solution with more than k windows contains a subset of k non-overlapping windows with no larger total cost. Minimizing exactly k windows is therefore equivalent to satisfying at least k.
- **Complexity:** O(n log n + k n) time and O(n + m) memory, where m is the number of distinct values.
- **Edge cases:** x=n with k=1, adjacent windows allowed, negative values, duplicate values, even-length windows, and large costs are all handled; Python integers and INF=10**30 avoid overflow concerns.

### NEXT
- none

### STATUS
solved
