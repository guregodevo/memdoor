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
- **Testing:** The three provided examples produce 6, -1, and 9. A randomized exhaustive cross-check for n <= 10, nums[i] <= 5, limit <= 20, and k in [-10, 10] found no mismatches; no code changes were required.
- **Positive product DP:** pos0[p] and pos1[p] are bitsets of reachable alternating sums for non-empty zero-free subsequences with exact product p and even/odd length. Appending x to an even-length subsequence adds x, so left shift by x; appending to an odd-length subsequence subtracts x, so right shift by x.
- **Single element insertion:** A one-element subsequence [x] has odd length and sum x, so it is inserted into pos1[x]. For x = 1, this is done after snapshotting old states to avoid reusing the same 1 multiple times.
- **In-place update safety:** For x > 1, product strictly increases, so iterating p downward from limit // x to 1 prevents using the current element more than once. For x = 1, product does not increase, so full snapshots old0/old1 are required.
- **Limit pruning:** Positive factors greater than limit are skipped in the positive-product DP because any zero-free product containing them exceeds limit. They are still processed in the zero-product DP because a zero can make the final product 0.
- **Zero product DP:** no0/no1 track zero-free subsequences, with no0 initially containing the empty subsequence. z0/z1 track subsequences containing at least one zero. Positive x appends to both groups using the same parity transitions.
- **Zero transition:** Processing x = 0 flips parity without changing sum and moves no-zero states into the zero group: z0 becomes z0 | z1 | no1, and z1 becomes z1 | z0 | no0. No-zero states remain unchanged because a zero cannot be included in a zero-free subsequence.
- **Answer order:** Products are scanned from limit down to 1. The first positive product whose bitset contains the target sum is optimal. If none exists, return 0 iff the target sum is reachable in z0 | z1, otherwise -1.
- **Non-empty guarantee:** The empty subsequence is never returned. Positive DP has no empty state, and the zero-product check only inspects has-zero states.
- **Sum bitset bounds:** total = sum(nums), offset = total, and mask covers sums from -total to total. If abs(k) > total, no subsequence can have alternating sum k. Left shifts are masked; right shifts only move toward lower sums and cannot create valid out-of-range bits.
- **Complexity:** Positive DP performs O(sum(limit / x) over processed positive x) big-int operations, at most about 750k for limit = 5000 and n = 150. Zero DP is O(n) big-int operations. Bitsets have at most 3601 bits under the constraints.

### NEXT
none

### STATUS
solved
