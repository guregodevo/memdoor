from typing import List

class Solution:
    def maxProduct(self, nums: List[int], k: int, limit: int) -> int:
        n = len(nums)
        max_val = max(nums) if n else 0
        offset = max_val * ((n + 1) // 2)

        if abs(k) > offset:
            return -1

        size = 2 * offset + 1
        mask = (1 << size) - 1
        target = 1 << (offset + k)

        # Positive-product DP:
        # pos_even[p] / pos_odd[p] are bitsets of achievable alternating sums
        # for non-empty subsequences using only positive numbers, with product p
        # and even/odd length.
        pos_even = [0] * (limit + 1)
        pos_odd = [0] * (limit + 1)

        # Zero-product feasibility DP:
        # no_zero_even includes the empty subsequence (sum 0).
        no_zero_even = 1 << offset
        no_zero_odd = 0
        has_zero_even = 0
        has_zero_odd = 0

        for x in nums:
            if x == 0:
                ne, no, he, ho = no_zero_even, no_zero_odd, has_zero_even, has_zero_odd

                # Taking a zero flips parity and makes the product zero.
                has_zero_even = (he | no | ho) & mask
                has_zero_odd = (ho | ne | he) & mask
                # no_zero_even and no_zero_odd only skip this zero.
            else:
                ne, no, he, ho = no_zero_even, no_zero_odd, has_zero_even, has_zero_odd

                # Sum-only DP for zero-product feasibility. This intentionally
                # keeps positive prefixes even if their product exceeds limit.
                no_zero_even = (ne | (no >> x)) & mask
                no_zero_odd = (no | (ne << x)) & mask
                has_zero_even = (he | (ho >> x)) & mask
                has_zero_odd = (ho | (he << x)) & mask

                # Positive-product DP only stores products <= limit.
                if x <= limit:
                    old_even = pos_even.copy()
                    old_odd = pos_odd.copy()

                    # Start a new subsequence [x].
                    pos_odd[x] |= (1 << (offset + x)) & mask

                    maxp = limit // x
                    pe = pos_even
                    po = pos_odd
                    m = mask
                    xx = x

                    for p in range(1, maxp + 1):
                        np = p * xx

                        b = old_even[p]
                        if b:
                            # Even length -> next sign is plus.
                            po[np] |= (b << xx) & m

                        b = old_odd[p]
                        if b:
                            # Odd length -> next sign is minus.
                            pe[np] |= b >> xx

        # Prefer any positive product over zero.
        for p in range(limit, 0, -1):
            if (pos_even[p] & target) or (pos_odd[p] & target):
                return p

        # Otherwise, zero product is valid if achievable.
        if (has_zero_even & target) or (has_zero_odd & target):
            return 0

        return -1