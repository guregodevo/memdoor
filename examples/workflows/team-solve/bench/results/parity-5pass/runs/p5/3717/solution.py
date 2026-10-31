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