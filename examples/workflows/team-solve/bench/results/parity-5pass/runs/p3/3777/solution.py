from typing import List

class Solution:
    def maxProduct(self, nums: List[int], k: int, limit: int) -> int:
        total = sum(nums)
        if abs(k) > total:
            return -1

        offset = total
        size = 2 * total + 1
        mask = (1 << size) - 1
        k_bit = 1 << (offset + k)

        # Positive-product DP: subsequences with no zeros.
        # even[p] / odd[p] are bitsets of achievable alternating sums
        # for non-empty zero-free subsequences with product p and even/odd length.
        even = [0] * (limit + 1)
        odd = [0] * (limit + 1)
        active = []
        products_range = range(1, limit + 1)

        for x in nums:
            if x == 0 or x > limit:
                continue

            new_even = even[:]
            new_odd = odd[:]

            # Start a new subsequence with this element.
            new_odd[x] |= (1 << (offset + x)) & mask

            # Append this element to previous zero-free subsequences.
            max_p = limit // x
            for p in active:
                if p > max_p:
                    break

                be = even[p]
                if be:
                    # even length -> odd length, next sign is +x
                    new_odd[p * x] |= (be << x) & mask

                bo = odd[p]
                if bo:
                    # odd length -> even length, next sign is -x
                    new_even[p * x] |= (bo >> x)

            even, odd = new_even, new_odd
            active = [p for p in products_range if even[p] or odd[p]]

        # Any positive product is better than product 0.
        for p in reversed(active):
            if (even[p] & k_bit) or (odd[p] & k_bit):
                return p

        # Zero-product feasibility DP: subsequences containing at least one zero.
        # no_even/no_odd: zero-free subsequences, including the empty subsequence.
        # z_even/z_odd: subsequences that already contain at least one zero.
        no_even = 1 << offset
        no_odd = 0
        z_even = 0
        z_odd = 0

        for x in nums:
            if x == 0:
                # Taking zero does not change sum, flips parity, and sets zero flag.
                nz_even = z_even | z_odd | no_odd
                nz_odd = z_odd | z_even | no_even
                z_even, z_odd = nz_even, nz_odd
            else:
                # Positive x: skip it, or append it with +/-x sum shift.
                nn_even = no_even | (no_odd >> x)
                nn_odd = no_odd | ((no_even << x) & mask)

                nz_even = z_even | (z_odd >> x)
                nz_odd = z_odd | ((z_even << x) & mask)

                no_even, no_odd = nn_even, nn_odd
                z_even, z_odd = nz_even, nz_odd

        if (z_even & k_bit) or (z_odd & k_bit):
            return 0

        return -1


if __name__ == "__main__":
    sol = Solution()

    # Provided examples.
    assert sol.maxProduct([1, 2, 3], 2, 10) == 6
    assert sol.maxProduct([0, 2, 3], -5, 12) == -1
    assert sol.maxProduct([2, 2, 3, 3], 0, 9) == 9

    # Edge cases.
    assert sol.maxProduct([0], 0, 1) == 0
    assert sol.maxProduct([0], 1, 1) == -1
    assert sol.maxProduct([0, 0], 0, 1) == 0
    assert sol.maxProduct([1, 1], 0, 1) == 1
    assert sol.maxProduct([2], 0, 10) == -1
    assert sol.maxProduct([0, 1], -1, 10) == 0
    assert sol.maxProduct([1, 0], -1, 10) == -1
    assert sol.maxProduct([1, 0], 1, 10) == 1
    assert sol.maxProduct([2, 1], 1, 100) == 2
    assert sol.maxProduct([1, 2], -1, 100) == 2
    assert sol.maxProduct([2, 2], 0, 1) == -1
    assert sol.maxProduct([0, 2, 2], 0, 1) == 0
    assert sol.maxProduct([0, 12], -12, 10) == 0