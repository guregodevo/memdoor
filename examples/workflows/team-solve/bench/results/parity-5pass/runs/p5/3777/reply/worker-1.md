### CODE
```python
from typing import List

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

        # Z0/Z1: zero-product subsequences by parity and sum.
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

                old_active = active_products[:]
                for p in old_active:
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
```

### NOTES
- **State representation:** F0/F1 are Python int bitsets over sums [-12n,12n] with offset=12n; bit offset+s set iff some subsequence of the processed prefix has alternating sum s and even/odd length. P0[p]/P1[p] are bitsets for non-empty subsequences with exact positive product p (1..limit). Z0/Z1 are bitsets for zero-product subsequences.
- **Why F is needed:** a zero can turn any previous subsequence into product 0, even if that previous positive product already exceeded limit. F keeps uncapped sum/parity feasibility so each zero can seed Z from all old prefixes.
- **Why exact product:** for the same sum and parity, a smaller positive product may remain under limit after future multiplications while a larger one cannot, so keeping only the maximum product per sum/parity is unsafe.
- **Positive transition:** for x>0, skip keeps old states. Take flips parity: old parity0 shifts left by x (sum +x) to parity1, old parity1 shifts right by x (sum -x) to parity0. P adds singleton [x] at sum x, parity1, if x<=limit. For each old active product p, if p*x<=limit, OR old P0[p]<<x into new P1[p*x] and old P1[p]>>x into new P0[p*x]. If x>limit, no positive subsequence can take x and stay within limit, so P is unchanged.
- **Zero transition:** for x=0, P is unchanged. Z is seeded from old F: oldF0 goes to Z1 and oldF1 goes to Z0 (taking the zero), and old Z states also skip or take with parity flip and no sum change. F becomes oldF0|oldF1 in both parities because zero can be skipped or taken.
- **Masking and range:** max_sum=12n, size=2*max_sum+1, mask=(1<<size)-1. Left shifts are masked; right shifts drop out-of-range bits. If abs(k)>max_sum, return -1 immediately.
- **Non-empty handling:** P is seeded only by explicit singletons and extensions from non-empty P; Z is seeded only by taking a zero. The empty subsequence (sum 0, product 1) is never returned.
- **Answer extraction:** target_bit=1<<(offset+k). Scan p from limit down to 1; the first p with target bit in P0[p] or P1[p] is the maximum valid positive product. If none, return 0 if target bit is in Z0 or Z1, else -1.
- **Complexity:** bitset length <=3601, active products <=limit<=5000. Each positive element iterates old active products and does a few big-int shifts/ORs; memory is O(limit * bitset_words + bitset_words), well within constraints.
- **Checked cases:** the three examples; zero after an over-limit positive prefix; zero before positive elements; k=0 with only empty subsequence; repeated ones; x>limit elements ignored for P but kept in F/Z; multiple zeros.

### NEXT
none

### STATUS
solved
