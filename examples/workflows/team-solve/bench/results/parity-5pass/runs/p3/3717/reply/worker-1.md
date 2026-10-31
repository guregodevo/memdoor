### CODE
```python
from typing import List


class Fenwick:
    __slots__ = ("n", "cnt", "sm", "top")

    def __init__(self, n: int):
        self.n = n
        self.cnt = [0] * (n + 1)
        self.sm = [0] * (n + 1)
        self.top = 1 << (n.bit_length() - 1)

    def add(self, i: int, dc: int, ds: int) -> None:
        n = self.n
        cnt = self.cnt
        sm = self.sm
        while i <= n:
            cnt[i] += dc
            sm[i] += ds
            i += i & -i

    def prefix(self, i: int) -> tuple:
        cnt = self.cnt
        sm = self.sm
        c = 0
        s = 0
        while i > 0:
            c += cnt[i]
            s += sm[i]
            i -= i & -i
        return c, s

    def kth(self, k: int) -> int:
        idx = 0
        cnt = self.cnt
        n = self.n
        bitmask = self.top
        while bitmask:
            nxt = idx + bitmask
            if nxt <= n and cnt[nxt] < k:
                idx = nxt
                k -= cnt[nxt]
            bitmask >>= 1
        return idx + 1


class Solution:
    def minOperations(self, nums: List[int], x: int, k: int) -> int:
        n = len(nums)

        vals = sorted(set(nums))
        comp = {v: i + 1 for i, v in enumerate(vals)}
        comp_nums = [comp[v] for v in nums]
        del comp

        m = len(vals)
        bit = Fenwick(m)

        total_sum = sum(nums[:x])
        for i in range(x):
            bit.add(comp_nums[i], 1, nums[i])

        costs = [0] * (n - x + 1)
        target = (x + 1) // 2

        for start in range(n - x + 1):
            med_idx = bit.kth(target)
            med_val = vals[med_idx - 1]

            left_count, left_sum = bit.prefix(med_idx)
            right_count = x - left_count
            right_sum = total_sum - left_sum

            costs[start] = (
                med_val * left_count - left_sum
                + right_sum - med_val * right_count
            )

            if start + x < n:
                out_val = nums[start]
                bit.add(comp_nums[start], -1, -out_val)

                in_val = nums[start + x]
                bit.add(comp_nums[start + x], 1, in_val)

                total_sum += in_val - out_val

        INF = 10 ** 30
        prev = [0] * (n + 1)

        for j in range(1, k + 1):
            curr = [INF] * (n + 1)
            best = INF

            prev_local = prev
            costs_local = costs
            x_local = x

            for i in range(j * x, n + 1):
                idx = i - x_local
                cand = prev_local[idx] + costs_local[idx]
                if cand < best:
                    best = cand
                curr[i] = best

            prev = curr

        return prev[n]


def _brute(nums: List[int], x: int, k: int) -> int:
    n = len(nums)
    costs = []
    for s in range(n - x + 1):
        window = nums[s:s + x]
        best_window = min(sum(abs(v - t) for v in window) for t in set(window))
        costs.append(best_window)

    m = len(costs)
    INF = 10 ** 18
    best = INF

    def dfs(last_start: int, chosen: int, total: int) -> None:
        nonlocal best
        if total >= best:
            return
        if chosen == k:
            best = total
            return

        start = max(0, last_start + x)
        remaining = m - start
        if remaining <= 0:
            return
        if chosen + (remaining + x - 1) // x < k:
            return

        for s in range(start, m):
            dfs(s, chosen + 1, total + costs[s])

    dfs(-10 ** 9, 0, 0)
    return best


if __name__ == "__main__":
    import random

    sol = Solution()

    assert sol.minOperations([5, -2, 1, 3, 7, 3, 6, 4, -1], 3, 2) == 8
    assert sol.minOperations([9, -2, -2, -2, 1, 5], 2, 2) == 3

    random.seed(12345)
    for _ in range(200):
        n = random.randint(2, 8)
        x = random.randint(2, n)
        max_k = min(15, n // x)
        if max_k < 1:
            continue
        k = random.randint(1, max_k)
        nums = [random.randint(-5, 5) for _ in range(n)]

        expected = _brute(nums, x, k)
        got = sol.minOperations(nums, x, k)
        assert got == expected, (nums, x, k, got, expected)
```

### NOTES
- **Reduction:** The required subarrays can be treated as chosen disjoint length-x windows. Operations on disjoint windows add independently. For one fixed window, the cheapest common value is a median, so its cost is the sum of absolute deviations to a median. Since all window costs are nonnegative, "at least k" is equivalent to "exactly k".
- **Window costs:** Coordinate-compress all values. Maintain the current sliding window in one Fenwick tree storing both counts and sums. The lower median is the kth order statistic with k=(x+1)//2. Prefix count/sum up to the median gives cost = med*left_count-left_sum + (total_sum-left_sum)-med*(x-left_count).
- **Sliding:** Initialize the first x elements, then for each slide remove the left element and add the new right element, updating counts, sums, and total_sum. Each window cost is computed in O(log n).
- **Fenwick details:** The kth query uses binary lifting on the count tree. The combined prefix query returns count and sum in one loop, keeping the sliding phase efficient with small constants.
- **DP:** dp[j][i] is the minimum cost for j chosen windows inside the first i elements. Transition: skip position i-1 or take the window ending at i-1, which starts at i-x: dp[j][i]=min(dp[j][i-1], dp[j-1][i-x]+cost[i-x]). Rolling arrays over j are used; the inner loop starts at i=j*x.
- **Complexity:** O(n log n + k n) time and O(n) memory, well within n=1e5 and k<=15.
- **Edge cases:** Negative values, duplicates, even x using the lower median, adjacent windows allowed, and k*x<=n feasibility are handled.
- **Testing:** The file includes a guarded main block that checks the two examples and 200 small random cases against a brute-force enumerator of non-overlapping start positions.

### NEXT
- none

### STATUS
solved
