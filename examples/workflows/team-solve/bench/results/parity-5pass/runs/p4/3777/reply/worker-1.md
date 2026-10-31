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
        target_bit = 1 << (k + offset)

        # pos0/pos1: bitsets for even/odd length positive-product subsequences.
        pos0 = [0] * (limit + 1)
        pos1 = [0] * (limit + 1)

        for x in nums:
            if 0 < x <= limit:
                old0 = pos0[:]
                old1 = pos1[:]
                max_p = limit // x
                for p in range(1, max_p + 1):
                    b0 = old0[p]
                    if b0:
                        pos1[p * x] |= (b0 << x) & mask
                    b1 = old1[p]
                    if b1:
                        pos0[p * x] |= (b1 >> x) & mask
                pos1[x] |= 1 << (offset + x)

        for p in range(limit, 0, -1):
            if (pos0[p] | pos1[p]) & target_bit:
                return p

        # no0/no1: no zero taken (no0 includes empty); z0/z1: at least one zero.
        no0 = 1 << offset
        no1 = 0
        z0 = 0
        z1 = 0

        for x in nums:
            old_no0 = no0
            old_no1 = no1
            old_z0 = z0
            old_z1 = z1

            if x > 0:
                no0 = old_no0 | ((old_no1 >> x) & mask)
                no1 = old_no1 | ((old_no0 << x) & mask)
                z0 = old_z0 | ((old_z1 >> x) & mask)
                z1 = old_z1 | ((old_z0 << x) & mask)
            else:
                z0 = (old_z0 | old_z1 | old_no1) & mask
                z1 = (old_z1 | old_z0 | old_no0) & mask

        if (z0 | z1) & target_bit:
            return 0
        return -1
```

### NOTES
- **Approach:** Split by product class. Since all nums are nonnegative, a subsequence product is either positive (no zeros) or zero (at least one zero). Positive products are maximized first; zero is only returned if no positive product within limit reaches k.
- **Sum bound:** The absolute alternating sum of any subsequence is at most the sum of its elements, hence at most total=sum(nums). Early return -1 if abs(k)>total. Use a bitset of length 2*total+1 with offset=total, so sum s is bit s+offset. target_bit=1<<(k+offset).
- **Positive DP:** pos0[p] and pos1[p] are bitsets of reachable alternating sums for non-empty positive-only subsequences with exact product p and even/odd length. Process nums in original order. For each positive x<=limit, snapshot pos0/pos1 to enforce using x at most once. From even length, append x with + sign: old0[p] << x into pos1[p*x]. From odd length, append x with - sign: old1[p] >> x into pos0[p*x]. Add the single-element subsequence [x] as odd length sum +x in pos1[x]. Products above limit are discarded because positive products never decrease when appending positive numbers.
- **Positive answer:** Scan p from limit down to 1. The first p where target_bit is set in pos0[p] | pos1[p] is the maximum valid positive product.
- **Zero DP:** Separate reachability DP for subsequences containing at least one zero. no0/no1 track no-zero states, with no0 initially containing the empty subsequence sum 0. z0/z1 track has-zero states. For x>0, apply the same parity transitions to both no and z groups: even -> odd with +x, odd -> even with -x. For x==0, no states only skip; zero states flip parity with no sum change, and no states can enter the zero group: z0=old_z0|old_z1|old_no1, z1=old_z1|old_z0|old_no0.
- **Zero answer:** If no positive product is found, return 0 iff target_bit is set in z0|z1, otherwise -1. Empty subsequence is excluded because positive DP has no empty state and zero DP only checks has-zero states.
- **Edge cases:** k=0 does not return empty product 1; positive DP starts empty and only adds non-empty single states. All-zero arrays return 0 for k=0. Elements x>limit are skipped in positive DP because any positive subsequence containing them exceeds limit, but they are still processed in zero DP because a zero can make the final product 0.
- **Complexity:** Positive DP does O(sum over x of limit/x) bitset operations, at most about 750k iterations for limit=5000 and n=150, with bitsets of at most 3601 bits. Zero DP is O(n) bitset operations. Memory is O(limit) list slots plus bitset integers, well within limits.
- **Examples:** Traced [1,2,3], k=2, limit=10 returns 6; [0,2,3], k=-5, limit=12 returns -1; [2,2,3,3], k=0, limit=9 returns 9 because product 36 is discarded and 9 is reachable.

### NEXT
none

### STATUS
solved
