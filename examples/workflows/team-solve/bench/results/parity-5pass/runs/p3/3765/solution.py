from typing import List

class Solution:
    def minimumCost(self, nums: List[int], cost: List[int], k: int) -> int:
        n = len(nums)
        pref_num = [0] * (n + 1)
        pref_cost = [0] * (n + 1)

        for i in range(n):
            pref_num[i + 1] = pref_num[i] + nums[i]
            pref_cost[i + 1] = pref_cost[i] + cost[i]

        total_cost = pref_cost[n]

        # penalty[p] is the k-term added when p is an internal previous cut.
        penalty = [0] * (n + 1)
        for p in range(1, n + 1):
            penalty[p] = k * (total_cost - pref_cost[p])

        dp = [0] * (n + 1)

        for r in range(1, n + 1):
            nr = pref_num[r]
            cr = pref_cost[r]

            # p = 0 case:
            # dp[0] + pref_num[r] * (pref_cost[r] - pref_cost[0])
            best = nr * cr

            # p >= 1 cases
            for p in range(1, r):
                val = dp[p] + nr * (cr - pref_cost[p]) + penalty[p]
                if val < best:
                    best = val

            dp[r] = best

        return dp[n] + k * total_cost


if __name__ == "__main__":
    sol = Solution()

    # Provided examples.
    assert sol.minimumCost([3, 1, 4], [4, 6, 6], 1) == 110
    assert sol.minimumCost(
        [4, 8, 5, 1, 14, 2, 2, 12, 1],
        [7, 2, 8, 4, 2, 2, 1, 1, 2],
        7,
    ) == 985

    # n = 1.
    assert sol.minimumCost([5], [2], 3) == (5 + 3) * 2

    # A case where one subarray is optimal.
    nums = [1000, 1]
    cost = [1, 1000]
    k = 1000
    assert sol.minimumCost(nums, cost, k) == (sum(nums) + k) * sum(cost)

    # A case where all singletons are optimal.
    nums = [1, 1000]
    cost = [1000, 1]
    k = 1
    expected_singletons = (1 + 1) * 1000 + (1 + 1000 + 2 * 1) * 1
    assert sol.minimumCost(nums, cost, k) == expected_singletons

    def brute_force(nums: List[int], cost: List[int], k: int) -> int:
        n = len(nums)
        pref_num = [0] * (n + 1)
        pref_cost = [0] * (n + 1)

        for i in range(n):
            pref_num[i + 1] = pref_num[i] + nums[i]
            pref_cost[i + 1] = pref_cost[i] + cost[i]

        best = 10**30

        def dfs(pos: int, pieces: int, total: int) -> None:
            nonlocal best
            if pos == n:
                if total < best:
                    best = total
                return

            for end in range(pos + 1, n + 1):
                add = (
                    (pref_num[end] + k * (pieces + 1))
                    * (pref_cost[end] - pref_cost[pos])
                )
                dfs(end, pieces + 1, total + add)

        dfs(0, 0, 0)
        return best

    # Cross-check on small deterministic cases.
    small_cases = [
        ([5], [2], 3),
        ([1000, 1], [1, 1000], 1000),
        ([1, 1000], [1000, 1], 1),
        ([3, 1, 4], [4, 6, 6], 1),
        ([1, 2, 3], [4, 5, 6], 10),
        ([1, 2, 3, 4], [1, 1, 1, 1], 1),
        ([4, 8, 5, 1, 14, 2, 2, 12, 1], [7, 2, 8, 4, 2, 2, 1, 1, 2], 7),
    ]

    for nums, cost, k in small_cases:
        assert sol.minimumCost(nums, cost, k) == brute_force(nums, cost, k)

    # Deterministic random cross-check.
    import random
    rng = random.Random(12345)

    for _ in range(300):
        n = rng.randint(1, 8)
        nums = [rng.randint(1, 10) for _ in range(n)]
        cost = [rng.randint(1, 10) for _ in range(n)]
        k = rng.randint(1, 10)
        assert sol.minimumCost(nums, cost, k) == brute_force(nums, cost, k)