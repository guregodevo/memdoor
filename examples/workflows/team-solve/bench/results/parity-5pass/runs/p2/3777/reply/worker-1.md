### CODE
```python
from typing import List

class Solution:
    def maxProduct(self, nums: List[int], k: int, limit: int) -> int:
        max_sum = sum(nums)
        if k > max_sum or k < -max_sum:
            return -1

        offset = max_sum
        size = 2 * max_sum + 1
        mask = (1 << size) - 1
        target = 1 << (k + offset)

        # dp0[p] / dp1[p]: reachable alternating sums for non-empty zero-free
        # subsequences with product p and even / odd length.
        dp0 = [0] * (limit + 1)
        dp1 = [0] * (limit + 1)

        # free0 / free1: all zero-free subsequences, including the empty one.
        # zero0 / zero1: subsequences containing at least one zero.
        free0 = 1 << offset
        free1 = 0
        zero0 = 0
        zero1 = 0

        for x in nums:
            if x == 0:
                oz0, oz1 = zero0, zero1
                of0, of1 = free0, free1

                # Taking a zero flips parity and does not change the sum.
                zero0 = (oz0 | oz1 | of1) & mask
                zero1 = (oz1 | oz0 | of0) & mask
            else:
                old0 = dp0[:]
                old1 = dp1[:]

                # Start a new positive-product subsequence with this element.
                if x <= limit:
                    dp1[x] |= 1 << (x + offset)

                # Extend existing positive-product subsequences.
                max_p = limit // x
                m = mask
                for p in range(1, max_p + 1):
                    np = p * x

                    b0 = old0[p]
                    if b0:
                        # Even length -> next index is even -> add x.
                        dp1[np] |= (b0 << x) & m

                    b1 = old1[p]
                    if b1:
                        # Odd length -> next index is odd -> subtract x.
                        dp0[np] |= b1 >> x

                # Update zero-free reachability, ignoring product limits.
                of0, of1 = free0, free1
                free0 = (of0 | (of1 >> x)) & m
                free1 = (of1 | (of0 << x)) & m

                # Update zero-containing reachability, ignoring product limits.
                oz0, oz1 = zero0, zero1
                zero0 = (oz0 | (oz1 >> x)) & m
                zero1 = (oz1 | (oz0 << x)) & m

        # Prefer any positive product over product 0.
        for p in range(limit, 0, -1):
            if (dp0[p] & target) or (dp1[p] & target):
                return p

        if (zero0 & target) or (zero1 & target):
            return 0

        return -1
```

### NOTES
- **State encoding:** Use one Python int as a bitset for alternating sums. `offset = sum(nums)`, `size = 2 * offset + 1`, and bit `i` represents sum `i - offset`. Adding `x` is a left shift by `x`; subtracting `x` is a right shift by `x`. Left shifts are masked to the valid sum range.
- **Positive product DP:** `dp0[p]` and `dp1[p]` store sum bitsets for non-empty zero-free subsequences with product `p <= limit` and even/odd length. No empty state is stored in these arrays. For each positive `x`, snapshot the old arrays, add the singleton subsequence `[x]` to `dp1[x]` if `x <= limit`, then extend old states: old even parity adds `x` and moves to odd parity, old odd parity subtracts `x` and moves to even parity. The snapshot prevents using the same element more than once, which is especially important for `x = 1`.
- **Zero product DP:** `free0` and `free1` store all zero-free sum bitsets, including the empty subsequence. `zero0` and `zero1` store subsequences containing at least one zero. For positive `x`, zero-containing states only extend: `zero0 = old_zero0 | (old_zero1 >> x)` and `zero1 = old_zero1 | (old_zero0 << x)`. For `x = 0`, taking the zero flips parity with no sum change and can start from any zero-free state: `zero0 = old_zero0 | old_zero1 | old_free1`, `zero1 = old_zero1 | old_zero0 | old_free0`.
- **Why free DP is needed:** A positive prefix whose product exceeds `limit` is discarded from the positive-product DP, but if a zero is chosen later, the final product becomes `0` and is valid. The `free`/`zero` DP ignores product limits, so it captures all zero-containing subsequences, including those built from over-limit positive prefixes.
- **Final answer:** If `|k| > sum(nums)`, no subsequence can reach `k`, so return `-1`. Otherwise scan products from `limit` down to `1`; the first product whose even or odd bitset contains the target sum is the maximum valid positive product. If none exists, return `0` if the zero DP reaches the target, otherwise `-1`.
- **Examples:** `[1, 2, 3], k = 2, limit = 10` returns `6`; `[0, 2, 3], k = -5, limit = 12` returns `-1`; `[2, 2, 3, 3], k = 0, limit = 9` returns `9` because product `36` is not stored and `9` is the largest valid product.
- **Complexity:** The bitset length is at most `3601` bits because `12 * 150 = 1800`. The positive-product loop performs at most about `150 * 5000 = 750000` iterations, each with small big-int shifts when a state is non-empty. Memory is `O(limit)` big ints plus four additional bitsets.

### NEXT
- none

### STATUS
solved
