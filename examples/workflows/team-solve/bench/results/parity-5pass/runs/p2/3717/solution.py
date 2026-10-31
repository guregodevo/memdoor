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