from typing import List
import random


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

        total_cost = C[n]
        INF = 10**30

        # dp[v] is the minimum transformed path cost from 0 to v.
        # The final answer is S[n] * C[n] + dp[n].
        dp = [INF] * (n + 1)
        dp[0] = 0

        # For a fixed u, the transition to v is:
        #   dp[u] + (S[u] - S[v]) * C[u] + k * (C[n] - C[u])
        #
        # Rewrite it as:
        #   (dp[u] + S[u] * C[u] + k * (C[n] - C[u])) - S[v] * C[u]
        #
        # Store the first part in A[u] to make the inner loop cheap.
        A = [0] * (n + 1)
        A[0] = k * total_cost

        for v in range(1, n + 1):
            Sv = S[v]
            best = INF

            for u in range(v):
                val = A[u] - Sv * C[u]
                if val < best:
                    best = val

            dp[v] = best
            A[v] = best + S[v] * C[v] + k * (total_cost - C[v])

        return S[n] * total_cost + dp[n]


def _brute_force(nums: List[int], cost: List[int], k: int) -> int:
    n = len(nums)

    S = [0] * (n + 1)
    C = [0] * (n + 1)
    for i in range(n):
        S[i + 1] = S[i] + nums[i]
        C[i + 1] = C[i] + cost[i]

    best = 10**30

    # Enumerate all subsets of cut positions after indices 0..n-2.
    for mask in range(1 << max(0, n - 1)):
        total = 0
        left = 0
        segment_index = 0

        for right in range(n):
            # Cut after right if it is the last element or the mask says so.
            if right == n - 1 or ((mask >> right) & 1):
                segment_index += 1
                total += (S[right + 1] + k * segment_index) * (C[right + 1] - C[left])
                left = right + 1

        if total < best:
            best = total

    return best


if __name__ == "__main__":
    sol = Solution()

    # Provided examples.
    assert sol.minimumCost([3, 1, 4], [4, 6, 6], 1) == 110
    assert (
        sol.minimumCost(
            [4, 8, 5, 1, 14, 2, 2, 12, 1],
            [7, 2, 8, 4, 2, 2, 1, 1, 2],
            7,
        )
        == 985
    )

    # Small random validation against brute force.
    random.seed(0)
    for _ in range(200):
        n = random.randint(1, 8)
        nums = [random.randint(1, 10) for _ in range(n)]
        cost = [random.randint(1, 10) for _ in range(n)]
        k = random.randint(1, 10)

        expected = _brute_force(nums, cost, k)
        got = sol.minimumCost(nums, cost, k)

        assert got == expected, (nums, cost, k, expected, got)