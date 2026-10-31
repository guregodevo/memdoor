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