from typing import List

class Solution:
    def maxProduct(self, nums: List[int], k: int, limit: int) -> int:
        total = sum(nums)
        if abs(k) > total:
            return -1

        offset = total
        mask = (1 << (2 * offset + 1)) - 1
        target = 1 << (k + offset)
        lim = limit

        pos0 = [0] * (lim + 1)
        pos1 = [0] * (lim + 1)

        for x in nums:
            if x == 0 or x > lim:
                continue

            if x == 1:
                old0 = pos0[:]
                old1 = pos1[:]
                for p in range(1, lim + 1):
                    a = old0[p]
                    if a:
                        pos1[p] |= (a << 1) & mask
                    b = old1[p]
                    if b:
                        pos0[p] |= b >> 1
                pos1[1] |= 1 << (1 + offset)
            else:
                for p in range(lim // x, 0, -1):
                    a = pos0[p]
                    if a:
                        pos1[p * x] |= (a << x) & mask
                    b = pos1[p]
                    if b:
                        pos0[p * x] |= b >> x
                pos1[x] |= 1 << (x + offset)

        for p in range(lim, 0, -1):
            if (pos0[p] | pos1[p]) & target:
                return p

        no0 = 1 << offset
        no1 = 0
        z0 = 0
        z1 = 0

        for x in nums:
            if x == 0:
                nz0 = z0 | z1 | no1
                nz1 = z1 | z0 | no0
                z0, z1 = nz0, nz1
            else:
                old_no0 = no0
                old_no1 = no1
                old_z0 = z0
                old_z1 = z1

                no1 |= (old_no0 << x) & mask
                no0 |= old_no1 >> x
                z1 |= (old_z0 << x) & mask
                z0 |= old_z1 >> x

        return 0 if ((z0 | z1) & target) else -1