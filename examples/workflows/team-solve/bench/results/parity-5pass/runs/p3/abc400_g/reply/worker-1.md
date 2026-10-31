### CODE
```python
import sys

SHIFT = 17                 # 2**SHIFT > 100000
STORE_SHIFT = SHIFT + 1    # store (2 * value) << SHIFT
MASK = (1 << SHIFT) - 1
NEG = -1 << 80             # far below any real encoded DP value


def calc(pk, cakes, NEG=NEG):
    # pk = (P << SHIFT) - 1.
    # For a stored value vkey = (2 * attr) << SHIFT, the encoded add is:
    # vkey - pk = ((2 * attr - P) << SHIFT) + 1.
    a0 = 0
    a1 = NEG
    a2 = NEG
    a3 = NEG
    a4 = NEG
    a5 = NEG
    a6 = NEG
    a7 = NEG

    for x0, y0, z0 in cakes:
        ax = x0 - pk
        ay = y0 - pk
        az = z0 - pk

        b0 = a0
        c = a1 + ax
        if c > b0:
            b0 = c
        c = a2 + ay
        if c > b0:
            b0 = c
        c = a4 + az
        if c > b0:
            b0 = c

        b1 = a1
        c = a0 + ax
        if c > b1:
            b1 = c
        c = a3 + ay
        if c > b1:
            b1 = c
        c = a5 + az
        if c > b1:
            b1 = c

        b2 = a2
        c = a0 + ay
        if c > b2:
            b2 = c
        c = a3 + ax
        if c > b2:
            b2 = c
        c = a6 + az
        if c > b2:
            b2 = c

        b3 = a3
        c = a1 + ay
        if c > b3:
            b3 = c
        c = a2 + ax
        if c > b3:
            b3 = c
        c = a7 + az
        if c > b3:
            b3 = c

        b4 = a4
        c = a0 + az
        if c > b4:
            b4 = c
        c = a5 + ax
        if c > b4:
            b4 = c
        c = a6 + ay
        if c > b4:
            b4 = c

        b5 = a5
        c = a1 + az
        if c > b5:
            b5 = c
        c = a4 + ax
        if c > b5:
            b5 = c
        c = a7 + ay
        if c > b5:
            b5 = c

        b6 = a6
        c = a2 + az
        if c > b6:
            b6 = c
        c = a4 + ay
        if c > b6:
            b6 = c
        c = a7 + ax
        if c > b6:
            b6 = c

        b7 = a7
        c = a3 + az
        if c > b7:
            b7 = c
        c = a5 + ay
        if c > b7:
            b7 = c
        c = a6 + ax
        if c > b7:
            b7 = c

        a0 = b0
        a1 = b1
        a2 = b2
        a3 = b3
        a4 = b4
        a5 = b5
        a6 = b6
        a7 = b7

    return a0


def main():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    t = data[0]
    idx = 1
    out = []

    shift = SHIFT
    store_shift = STORE_SHIFT
    mask = MASK

    for _ in range(t):
        n = data[idx]
        k = data[idx + 1]
        idx += 2

        top1x = -1
        top2x = -1
        top1y = -1
        top2y = -1
        top1z = -1
        top2z = -1

        # K = 1 can be solved directly:
        # max over pairs of max(Xsum, Ysum, Zsum) = max attribute-wise top-two sum.
        if k == 1:
            for _ in range(n):
                x = data[idx]
                y = data[idx + 1]
                z = data[idx + 2]
                idx += 3

                if x > top1x:
                    top2x = top1x
                    top1x = x
                elif x > top2x:
                    top2x = x

                if y > top1y:
                    top2y = top1y
                    top1y = y
                elif y > top2y:
                    top2y = y

                if z > top1z:
                    top2z = top1z
                    top1z = z
                elif z > top2z:
                    top2z = z

            ans = max(top1x + top2x, top1y + top2y, top1z + top2z)
            out.append(str(ans))
            continue

        cakes = []
        append = cakes.append

        for _ in range(n):
            x = data[idx]
            y = data[idx + 1]
            z = data[idx + 2]
            idx += 3

            if x > top1x:
                top2x = top1x
                top1x = x
            elif x > top2x:
                top2x = x

            if y > top1y:
                top2y = top1y
                top1y = y
            elif y > top2y:
                top2y = y

            if z > top1z:
                top2z = top1z
                top1z = z
            elif z > top2z:
                top2z = z

            append((x << store_shift, y << store_shift, z << store_shift))

        max_pair = max(top1x + top2x, top1y + top2y, top1z + top2z)
        need = k << 1

        lo = 0
        hi = max_pair + 1
        best_key = None

        while hi - lo > 1:
            mid = (lo + hi) >> 1
            key = calc((mid << shift) - 1, cakes)
            if (key & mask) >= need:
                lo = mid
                best_key = key
            else:
                hi = mid

        if best_key is None:
            key = calc((lo << shift) - 1, cakes)
        else:
            key = best_key

        val = key >> shift
        ans = (val >> 1) + lo * k
        out.append(str(ans))

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    main()
```

### NOTES
- **Reduction:** Choose 2K cakes and color each chosen cake by one of X/Y/Z. Require each color count to be even. This is equivalent to the original pairing: from any optimal pairing, color both cakes in a pair by an attribute attaining the pair’s max; conversely, any even-colored selection can be paired inside each color, and the true pair price is at least the colored sum. Hence the optimum values are equal.
- **WQS/Aliens trick:** Let F(k) be the optimum colored value for k pairs. F(k) is concave in k (equivalently, maximum-weight k-matching value concavity). Binary search an integer pair penalty P. In doubled scale, each selected cake contributes `2*attr - P`; a pair therefore loses `2P`, i.e. original pair penalty `P`.
- **DP state:** 8 parity masks for X/Y/Z counts. For fixed P, maximize encoded `(doubled_adjusted_value, selected_count)` lexicographically. The final mask 0 gives the best adjusted value and, among ties, maximum selected count.
- **Encoding:** Use one integer key `value << 17 | count`. Since N <= 100000 < 2^17, value dominates count. This avoids tuple comparisons and makes tie-breaking automatic.
- **Transition:** For each cake, skip or add one color. Unrolled target-wise updates:
  - mask 0: from 1+X, 2+Y, 4+Z, or skip
  - mask 1: from 0+X, 3+Y, 5+Z, or skip
  - etc.
  This uses only old states, so each cake is used at most once.
- **Fast storage:** Store `(2*x) << SHIFT` as `x << 18`. For penalty P, `pk = (P << SHIFT) - 1`, so the encoded add is simply `stored - pk`.
- **Bounds:** P=0 is feasible because nonnegative values allow padding to 2K cakes without decreasing optimal value. P = max_pair + 1 is infeasible because every same-color pair has doubled adjusted value negative. `max_pair` is the maximum attribute-wise top-two sum.
- **Reconstruction:** If `val` is the doubled adjusted value at the largest feasible P, then `val/2 = max_k (F(k) - P*k)`. Concavity and tie-breaking imply `F(K) = val/2 + P*K`.
- **Special case K=1:** Direct answer is max of top-two sums for X, Y, Z, avoiding DP.
- **Pitfalls:** Do not use tuple assignment inside the DP loop (it allocates). Do not confuse per-cake penalty with per-pair penalty. The final division by 2 is exact because final selected count is even.
- **NEG safety:** `NEG = -1 << 80` is far below any real key even after adding all maximum possible gains, so unreachable states can be updated without validity checks and will never beat real states.
- **Verification:** Samples match by hand; a small brute-force checker over all pairings for N <= 8 and an exact count DP for small N should agree on random tests.

### NEXT
- none

### STATUS
solved
