from typing import List

class Solution:
    def minimumCost(self, nums: List[int], cost: List[int], k: int) -> int:
        n = len(nums)

        pref_nums = [0] * (n + 1)
        pref_cost = [0] * (n + 1)

        for i in range(n):
            pref_nums[i + 1] = pref_nums[i] + nums[i]
            pref_cost[i + 1] = pref_cost[i] + cost[i]

        total_cost = pref_cost[n]
        dp = [0] * (n + 1)
        INF = 10**30

        # dp[i] is the minimum transformed cost for a path ending at an
        # intermediate prefix endpoint i, where 1 <= i < n.
        for i in range(1, n):
            ni = pref_nums[i]
            ci = pref_cost[i]
            add = k * (total_cost - ci)
            best = INF

            for j in range(i):
                cur = dp[j] + ni * (ci - pref_cost[j]) + add
                if cur < best:
                    best = cur

            dp[i] = best

        # Final edge to n has a different k contribution: k * total_cost.
        nn = pref_nums[n]
        cn = total_cost
        add_final = k * cn
        ans = INF

        for j in range(n):
            cur = dp[j] + nn * (cn - pref_cost[j]) + add_final
            if cur < ans:
                ans = cur

        return ans