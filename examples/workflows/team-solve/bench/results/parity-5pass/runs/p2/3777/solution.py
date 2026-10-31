from typing import List
import random


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


def brute_force(nums: List[int], k: int, limit: int) -> int:
    n = len(nums)
    best = -1

    for mask in range(1, 1 << n):
        prod = 1
        s = 0
        parity = 0

        for i in range(n):
            if mask & (1 << i):
                if parity == 0:
                    s += nums[i]
                else:
                    s -= nums[i]
                prod *= nums[i]
                parity ^= 1

        if s == k and prod <= limit:
            if prod > best:
                best = prod

    return best


def main() -> None:
    sol = Solution()
    failures = []
    test_count = 0

    def check(nums: List[int], k: int, limit: int, expected=None) -> None:
        nonlocal test_count
        test_count += 1
        actual = sol.maxProduct(nums, k, limit)
        bf = brute_force(nums, k, limit)

        if expected is not None and actual != expected:
            failures.append((nums, k, limit, expected, actual, bf))
        if actual != bf:
            failures.append((nums, k, limit, bf, actual, bf))

    # Provided examples.
    check([1, 2, 3], 2, 10, 6)
    check([0, 2, 3], -5, 12, -1)
    check([2, 2, 3, 3], 0, 9, 9)

    # Targeted edge cases.
    targeted = [
        ([12, 12, 0, 12], 24, 5000),
        ([12, 12, 0, 12], -12, 100),
        ([12, 12, 0, 12], 0, 100),
        ([12, 12, 0, 12], 0, 200),
        ([2, 0, 3], 5, 1),
        ([2, 0, 3], -1, 5),
        ([2, 0, 3], -1, 6),
        ([0, 2, 3], 1, 1),
        ([2, 2, 3, 3], 0, 36),
        ([2, 2, 3, 3], 0, 5000),
        ([12, 12], 0, 100),
        ([12, 12], 0, 144),
        ([12, 12, 0], 0, 100),
        ([12, 12, 0], 0, 143),
        ([12, 12, 0], 0, 144),
        ([1, 1, 1, 1], 0, 1),
        ([1, 1, 1, 1], 1, 1),
        ([0, 1, 0], 0, 1),
        ([0, 1, 0], 1, 1),
        ([0, 1, 0], -1, 1),
    ]
    for nums, k, limit in targeted:
        check(nums, k, limit)

    # Deterministic edge sweeps.
    edge_nums = [
        [0],
        [0, 0],
        [1],
        [1, 1],
        [1, 1, 1],
        [2, 2],
        [2, 2, 3, 3],
        [12, 12],
        [12, 12, 0],
        [0, 2, 3],
        [2, 0, 3],
        [0, 0, 2],
    ]
    for nums in edge_nums:
        for k in range(-12, 13):
            for limit in (1, 2, 4, 9, 10, 12, 36, 100, 200, 5000):
                check(nums, k, limit)

    # Random tiny cases.
    random.seed(12345)

    for _ in range(1000):
        n = random.randint(1, 8)
        nums = [random.randint(0, 12) for _ in range(n)]
        k = random.randint(-30, 30)
        limit = random.randint(1, 50)
        check(nums, k, limit)

    for _ in range(1000):
        n = random.randint(1, 8)
        nums = [random.randint(0, 6) for _ in range(n)]
        if 0 not in nums:
            nums[random.randrange(n)] = 0
        k = random.randint(-10, 10)
        limit = random.randint(1, 10)
        check(nums, k, limit)

    for _ in range(500):
        n = random.randint(1, 8)
        nums = [random.randint(0, 1) for _ in range(n)]
        k = random.randint(-2, 2)
        limit = 1
        check(nums, k, limit)

    for _ in range(500):
        n = random.randint(1, 8)
        nums = [random.randint(0, 12) for _ in range(n)]
        k = 0
        limit = random.randint(1, 20)
        check(nums, k, limit)

    for _ in range(500):
        n = random.randint(1, 8)
        nums = [random.randint(0, 12) for _ in range(n)]
        k = random.randint(-10, 10)
        limit = 1
        check(nums, k, limit)

    for _ in range(500):
        n = random.randint(1, 8)
        nums = [random.randint(2, 12) for _ in range(n)]
        k = random.randint(-20, 20)
        limit = random.randint(1, 10)
        check(nums, k, limit)

    # Larger sanity calls, not brute-forced.
    sol.maxProduct([12] * 150, 0, 5000)
    sol.maxProduct([0] * 150, 0, 5000)
    sol.maxProduct([1] * 150, 0, 5000)
    sol.maxProduct([12, 0, 12, 0, 12], 0, 5000)

    if failures:
        print(f"FAILURES: {len(failures)}")
        for item in failures[:20]:
            print(item)
    else:
        print(f"No failing cases found in {test_count} tests.")


if __name__ == "__main__":
    main()