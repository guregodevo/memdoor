from typing import List
import random


class Solution:
    def maxProduct(self, nums: List[int], k: int, limit: int) -> int:
        n = len(nums)
        max_sum = 12 * n
        if abs(k) > max_sum:
            return -1

        offset = max_sum
        size = 2 * max_sum + 1
        mask = (1 << size) - 1
        target_bit = 1 << (offset + k)

        # F0/F1: all subsequences, including empty, by parity and sum.
        F0 = 1 << offset
        F1 = 0

        # Z0/Z1: non-empty zero-product subsequences by parity and sum.
        Z0 = 0
        Z1 = 0

        # P0[p]/P1[p]: non-empty positive-product subsequences with exact product p.
        P0 = [0] * (limit + 1)
        P1 = [0] * (limit + 1)
        active = [False] * (limit + 1)
        active_products = []

        for x in nums:
            oldF0, oldF1 = F0, F1
            oldZ0, oldZ1 = Z0, Z1

            if x == 0:
                # Taking zero keeps sum, flips parity, and makes product zero.
                # Seed zero-product states from every old all-subsequence state.
                Z0 = (oldZ0 | oldZ1 | oldF1) & mask
                Z1 = (oldZ1 | oldZ0 | oldF0) & mask

                # Zero can be skipped or taken, so both parities get all old sums.
                F0 = (oldF0 | oldF1) & mask
                F1 = (oldF1 | oldF0) & mask
                continue

            # Positive products: only keep products <= limit.
            if x <= limit:
                newP0 = P0[:]
                newP1 = P1[:]

                # Singleton subsequence [x].
                newP1[x] |= 1 << (offset + x)
                touched = [x]

                for p in active_products:
                    q = p * x
                    if q <= limit:
                        b0 = P0[p]
                        if b0:
                            # Old even length: next element is added.
                            newP1[q] |= (b0 << x) & mask

                        b1 = P1[p]
                        if b1:
                            # Old odd length: next element is subtracted.
                            newP0[q] |= b1 >> x

                        touched.append(q)

                for q in touched:
                    if not active[q] and (newP0[q] or newP1[q]):
                        active[q] = True
                        active_products.append(q)

                P0, P1 = newP0, newP1

            # Zero-product states can take positive x and remain zero-product.
            Z0 = (oldZ0 | (oldZ1 >> x)) & mask
            Z1 = (oldZ1 | (oldZ0 << x)) & mask

            # All-subsequence feasibility, used later to seed zeros.
            F0 = (oldF0 | (oldF1 >> x)) & mask
            F1 = (oldF1 | (oldF0 << x)) & mask

        # Prefer the largest positive product not exceeding limit.
        for p in range(limit, 0, -1):
            if (P0[p] & target_bit) or (P1[p] & target_bit):
                return p

        # Otherwise, product 0 is valid if achievable.
        if (Z0 & target_bit) or (Z1 & target_bit):
            return 0

        return -1


def _brute_force(nums: List[int], k: int, limit: int) -> int:
    n = len(nums)
    best = -1
    for mask in range(1, 1 << n):
        s = 0
        prod = 1
        idx = 0
        for i in range(n):
            if mask & (1 << i):
                if idx & 1:
                    s -= nums[i]
                else:
                    s += nums[i]
                prod *= nums[i]
                idx += 1
        if s == k and prod <= limit:
            if prod > best:
                best = prod
    return best


def _run_cross_check() -> bool:
    sol = Solution()

    examples = [
        ([1, 2, 3], 2, 10, 6),
        ([0, 2, 3], -5, 12, -1),
        ([2, 2, 3, 3], 0, 9, 9),
        ([0], 0, 1, 0),
        ([1], 0, 1, -1),
        ([10, 0], 10, 5, 0),
    ]

    for nums, k, limit, expected in examples:
        got = sol.maxProduct(nums, k, limit)
        if got != expected:
            print("Example mismatch:", nums, k, limit, "expected", expected, "got", got)
            return False

    random.seed(12345)
    for _ in range(200):
        n = random.randint(1, 12)
        nums = [random.randint(0, 12) for _ in range(n)]
        k = random.randint(-20, 20)
        limit = random.randint(1, 50)

        expected = _brute_force(nums, k, limit)
        got = sol.maxProduct(nums, k, limit)

        if expected != got:
            print("Mismatch:", nums, k, limit, "expected", expected, "got", got)
            return False

    return True


if __name__ == "__main__":
    if not _run_cross_check():
        raise SystemExit(1)