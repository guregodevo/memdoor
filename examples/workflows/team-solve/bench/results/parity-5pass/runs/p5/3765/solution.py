from typing import List

class Solution:
    def minimumCost(self, nums: List[int], cost: List[int], k: int) -> int:
        n = len(nums)

        # Prefix sums:
        # S[i] = nums[0] + ... + nums[i-1]
        # C[i] = cost[0] + ... + cost[i-1]
        S = [0] * (n + 1)
        C = [0] * (n + 1)
        for i in range(n):
            S[i + 1] = S[i] + nums[i]
            C[i + 1] = C[i] + cost[i]

        Cn = C[n]
        dp = [0] * (n + 1)

        # dp[i] is the minimum accumulated cost for a path ending at prefix i,
        # including internal-boundary penalties for all cuts before i.
        # If i < n, i is treated as an internal boundary and receives
        # k * (C[n] - C[i]).
        for i in range(1, n + 1):
            si = S[i]
            ci = C[i]

            # p = 0 transition: dp[0] + S[i] * (C[i] - C[0])
            best = si * ci

            # Try all previous cuts p, where 1 <= p < i.
            for p in range(1, i):
                val = dp[p] + si * (ci - C[p])
                if val < best:
                    best = val

            if i < n:
                best += k * (Cn - ci)

            dp[i] = best

        # Add the constant k * C[n] part of the ordinal contribution.
        return dp[n] + k * Cn


if __name__ == "__main__":
    sol = Solution()

    # Provided examples.
    assert sol.minimumCost([3, 1, 4], [4, 6, 6], 1) == 110
    assert sol.minimumCost(
        [4, 8, 5, 1, 14, 2, 2, 12, 1],
        [7, 2, 8, 4, 2, 2, 1, 1, 2],
        7
    ) == 985

    # n = 1.
    assert sol.minimumCost([5], [2], 3) == 16

    # One-segment optimal case.
    assert sol.minimumCost([1, 1], [1, 1], 100) == 204

    # All-singleton optimal case, n = 2.
    assert sol.minimumCost([1, 100], [100, 1], 1) == 303

    # All-singleton optimal case, n = 3.
    assert sol.minimumCost([1, 100, 100], [100, 1, 1], 1) == 507