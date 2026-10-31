from typing import List
import random


class Solution:
    def minimumCost(self, nums: List[int], cost: List[int], k: int) -> int:
        n = len(nums)

        # Prefix sums of nums and cost.
        S = [0] * (n + 1)
        C = [0] * (n + 1)
        for i in range(n):
            S[i + 1] = S[i] + nums[i]
            C[i + 1] = C[i] + cost[i]

        total_c = C[n]

        # dp[p] = minimum cost contribution for prefix p, excluding the constant
        # base k * total_c, but including future cut penalties for cuts already made.
        dp = [0] * (n + 1)

        # penalty[q] is the extra k-cost caused by a cut after prefix q.
        penalty = [0] * (n + 1)
        for q in range(1, n):
            penalty[q] = k * (total_c - C[q])

        for p in range(1, n + 1):
            sp = S[p]

            # best represents min(0, min_{1 <= q < p} ...).
            # The 0 corresponds to taking no previous cut, i.e. q = 0.
            best = 0
            for q in range(1, p):
                val = dp[q] + penalty[q] - sp * C[q]
                if val < best:
                    best = val

            dp[p] = sp * C[p] + best

        # Add the constant k * 1 contribution for every element.
        return dp[n] + k * total_c


def _brute_force(nums: List[int], cost: List[int], k: int) -> int:
    """Directly enumerate all partitions for small n."""
    n = len(nums)

    S = [0] * (n + 1)
    C = [0] * (n + 1)
    for i in range(n):
        S[i + 1] = S[i] + nums[i]
        C[i + 1] = C[i] + cost[i]

    best = None

    def dfs(pos: int, idx: int, total: int) -> None:
        nonlocal best

        if pos == n:
            if best is None or total < best:
                best = total
            return

        # All remaining block costs are positive, so this prune is safe.
        if best is not None and total >= best:
            return

        for end in range(pos, n):
            block_cost = (S[end + 1] + k * (idx + 1)) * (C[end + 1] - C[pos])
            dfs(end + 1, idx + 1, total + block_cost)

    dfs(0, 0, 0)
    return best


if __name__ == "__main__":
    sol = Solution()

    # Provided examples.
    assert sol.minimumCost([3, 1, 4], [4, 6, 6], 1) == 110
    assert sol.minimumCost(
        [4, 8, 5, 1, 14, 2, 2, 12, 1],
        [7, 2, 8, 4, 2, 2, 1, 1, 2],
        7
    ) == 985

    # One-element edge case.
    assert sol.minimumCost([5], [7], 3) == 56
    print("examples passed")

    # Small random validation against brute force.
    random.seed(2024)
    for _ in range(200):
        n = random.randint(1, 10)
        nums = [random.randint(1, 10) for _ in range(n)]
        cost = [random.randint(1, 10) for _ in range(n)]
        k = random.randint(1, 10)

        expected = _brute_force(nums, cost, k)
        got = sol.minimumCost(nums, cost, k)

        if expected != got:
            raise AssertionError(
                f"mismatch: nums={nums}, cost={cost}, k={k}, "
                f"expected={expected}, got={got}"
            )

    print("random tests passed")