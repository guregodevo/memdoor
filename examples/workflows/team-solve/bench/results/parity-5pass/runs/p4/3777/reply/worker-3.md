### CODE
```python
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
```

### NOTES
- **Approach:** Split by product class. Since all nums are nonnegative, a subsequence product is positive iff it contains no zero, and zero iff it contains at least one zero. Positive products are checked in descending order first; zero is returned only if no valid positive product exists.
- **Sum bitset:** Let total=sum(nums). If abs(k)>total, return -1 immediately. Use offset=total, mask=(1<<(2*offset+1))-1, and target=1<<(k+offset). Bit i represents alternating sum i-offset. Left shift by x adds x, right shift by x subtracts x, and mask truncates impossible high sums.
- **Positive DP:** pos0[p] and pos1[p] are bitsets of reachable alternating sums for non-empty positive-only subsequences with exact product p and even/odd length. For x=1, snapshot old0/old1 before updating all p, because product does not increase and in-place updates would reuse the same 1 multiple times. For x>1, iterate p from limit//x down to 1 so source product p is not yet contaminated by the current x. Transitions are even length plus x to odd length, and odd length minus x to even length. Single-element subsequences [x] are added to pos1[x].
- **Positive answer:** Scan p from limit down to 1. The first p where target is set in pos0[p] | pos1[p] is the maximum valid positive product.
- **Zero DP:** no0/no1 track no-zero subsequences, with no0 initially containing the empty subsequence sum 0. z0/z1 track subsequences containing at least one zero. For x>0, save old states and apply the same parity transitions to both no and z groups. For x=0, zero states flip parity with no sum change, and no states can enter the zero group: z0 becomes old z0 | old z1 | old no1, and z1 becomes old z1 | old z0 | old no0. No states remain unchanged when a zero is processed.
- **Zero answer:** After the positive scan, return 0 iff target is set in z0 | z1, otherwise -1. The empty subsequence is never returned because positive DP has no empty state and zero DP only checks has-zero states.
- **Edge cases:** k=0 does not return empty product 1. All-zero arrays return 0 for k=0. Elements greater than limit are skipped in positive DP because any positive product containing them exceeds limit, but they are still processed in zero DP because a zero can make the final product 0. Positive products above limit are discarded safely because appending positive numbers cannot reduce the product.
- **Complexity:** Positive DP performs O(sum over positive x<=limit of limit/x) bitset operations, at most about 750k for limit=5000 and n=150. Bitsets have at most 2*total+1 bits, and total<=1800 under constraints. Zero DP is O(n) bitset operations. Memory is O(limit) integer bitsets.

### NEXT
- none

### STATUS
solved
