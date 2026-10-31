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