import sys

NEG = -(1 << 80)


def run_dp(base, items, NEG=NEG):
    a0 = 0
    a1 = a2 = a3 = a4 = a5 = a6 = a7 = NEG

    for x, y, z in items:
        w0 = x - base
        w1 = y - base
        w2 = z - base

        n0 = a0
        t = a1 + w0
        if t > n0:
            n0 = t
        t = a2 + w1
        if t > n0:
            n0 = t
        t = a4 + w2
        if t > n0:
            n0 = t

        n1 = a1
        t = a0 + w0
        if t > n1:
            n1 = t
        t = a3 + w1
        if t > n1:
            n1 = t
        t = a5 + w2
        if t > n1:
            n1 = t

        n2 = a2
        t = a3 + w0
        if t > n2:
            n2 = t
        t = a0 + w1
        if t > n2:
            n2 = t
        t = a6 + w2
        if t > n2:
            n2 = t

        n3 = a3
        t = a2 + w0
        if t > n3:
            n3 = t
        t = a1 + w1
        if t > n3:
            n3 = t
        t = a7 + w2
        if t > n3:
            n3 = t

        n4 = a4
        t = a5 + w0
        if t > n4:
            n4 = t
        t = a6 + w1
        if t > n4:
            n4 = t
        t = a0 + w2
        if t > n4:
            n4 = t

        n5 = a5
        t = a4 + w0
        if t > n5:
            n5 = t
        t = a7 + w1
        if t > n5:
            n5 = t
        t = a1 + w2
        if t > n5:
            n5 = t

        n6 = a6
        t = a7 + w0
        if t > n6:
            n6 = t
        t = a4 + w1
        if t > n6:
            n6 = t
        t = a2 + w2
        if t > n6:
            n6 = t

        n7 = a7
        t = a6 + w0
        if t > n7:
            n7 = t
        t = a5 + w1
        if t > n7:
            n7 = t
        t = a3 + w2
        if t > n7:
            n7 = t

        a0 = n0
        a1 = n1
        a2 = n2
        a3 = n3
        a4 = n4
        a5 = n5
        a6 = n6
        a7 = n7

    return a0


def main():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    idx = 0
    T = data[idx]
    idx += 1

    out = []
    out_append = out.append
    dp = run_dp

    for _ in range(T):
        N = data[idx]
        K = data[idx + 1]
        idx += 2

        if K == 1:
            t01 = t02 = t11 = t12 = t21 = t22 = 0
            for _ in range(N):
                x = data[idx]
                y = data[idx + 1]
                z = data[idx + 2]
                idx += 3

                if x > t01:
                    t02 = t01
                    t01 = x
                elif x > t02:
                    t02 = x

                if y > t11:
                    t12 = t11
                    t11 = y
                elif y > t12:
                    t12 = y

                if z > t21:
                    t22 = t21
                    t21 = z
                elif z > t22:
                    t22 = z

            ans = max(t01 + t02, t11 + t12, t21 + t22)
            out_append(str(ans))
            continue

        shift = N.bit_length()
        s = shift + 1
        items = [None] * N

        t01 = t02 = t11 = t12 = t21 = t22 = 0

        for i in range(N):
            x = data[idx]
            y = data[idx + 1]
            z = data[idx + 2]
            idx += 3

            if x > t01:
                t02 = t01
                t01 = x
            elif x > t02:
                t02 = x

            if y > t11:
                t12 = t11
                t11 = y
            elif y > t12:
                t12 = y

            if z > t21:
                t22 = t21
                t21 = z
            elif z > t22:
                t22 = z

            items[i] = (x << s) + 1, (y << s) + 1, (z << s) + 1

        max_pair = max(t01 + t02, t11 + t12, t21 + t22)

        if max_pair == 0:
            out_append("0")
            continue

        if K == N // 2:
            enc = dp(0, items)
            out_append(str(enc >> s))
            continue

        target = K << 1
        lo = 0
        hi = max_pair + 1
        mask = (1 << shift) - 1

        while lo + 1 < hi:
            mid = (lo + hi) >> 1
            if dp(mid << shift, items) & mask >= target:
                lo = mid
            else:
                hi = mid

        enc = dp(lo << shift, items)
        ans = (enc >> s) + lo * K
        out_append(str(ans))

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    main()