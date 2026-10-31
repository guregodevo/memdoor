import sys

MOD = 998244353
G = 3


def main():
    data = sys.stdin.read().split()
    if not data:
        return
    N = int(data[0])
    S = data[1]

    if S[0] != 'B' or S[-1] != 'W':
        print(0)
        return

    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD

    bs = []
    ms = []
    wb = 0
    mb = 0
    for i, ch in enumerate(S):
        if ch == 'W':
            wb += 1
        else:
            if i == 0 or S[i - 1] != 'B':
                if mb > 0 and wb >= mb:
                    bs.append(wb)
                    ms.append(mb)
            mb += 1

    K = len(bs)
    if K == 0:
        print(fact[N])
        return

    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    if K <= 2000:
        g = [0] * K
        mod = MOD
        for i in range(K):
            bi = bs[i]
            acc = 0
            for j in range(i):
                acc += g[j] * fact[bi - ms[j]]
            acc %= mod
            val = fact[bi] + acc
            if val >= mod:
                val -= mod
            g[i] = (mod - val) * invfact[bi - ms[i]] % mod

        ans = fact[N]
        for i in range(K):
            ans = (ans + g[i] * fact[N - ms[i]]) % mod
        print(ans)
        return

    try:
        from atcoder.convolution import convolution as conv
    except Exception:
        conv = None

    if conv is None or not callable(conv):
        max_n = 1
        while max_n <= 2 * N:
            max_n <<= 1
        max_log = max_n.bit_length() - 1

        root_pw = [0] * (max_log + 1)
        iroot_pw = [0] * (max_log + 1)
        inv_pow2 = [1] * (max_log + 1)

        for lg in range(1, max_log + 1):
            length = 1 << lg
            root_pw[lg] = pow(G, (MOD - 1) // length, MOD)
            iroot_pw[lg] = pow(root_pw[lg], MOD - 2, MOD)

        inv2 = (MOD + 1) // 2
        for lg in range(1, max_log + 1):
            inv_pow2[lg] = inv_pow2[lg - 1] * inv2 % MOD

        def ntt(a):
            n = len(a)
            mod = MOD
            roots = root_pw
            lg = (n - 1).bit_length()
            length = n
            while length > 1:
                half = length >> 1
                wlen = roots[lg]
                for i in range(0, n, length):
                    w = 1
                    end = i + half
                    for j in range(i, end):
                        u = a[j]
                        v = a[j + half]
                        x = u + v
                        if x >= mod:
                            x -= mod
                        y = u - v
                        if y < 0:
                            y += mod
                        a[j] = x
                        a[j + half] = y * w % mod
                        w = w * wlen % mod
                length = half
                lg -= 1

        def intt(a):
            n = len(a)
            mod = MOD
            roots = iroot_pw
            length = 2
            lg = 1
            while length <= n:
                half = length >> 1
                wlen = roots[lg]
                for i in range(0, n, length):
                    w = 1
                    end = i + half
                    for j in range(i, end):
                        u = a[j]
                        v = a[j + half] * w % mod
                        x = u + v
                        if x >= mod:
                            x -= mod
                        y = u - v
                        if y < 0:
                            y += mod
                        a[j] = x
                        a[j + half] = y
                        w = w * wlen % mod
                length <<= 1
                lg += 1

        def conv(a, b):
            la = len(a)
            lb = len(b)
            if la == 0 or lb == 0:
                return []
            need = la + lb - 1
            mod = MOD

            if la * lb <= 16384:
                res = [0] * need
                if la < lb:
                    for i, ai in enumerate(a):
                        if ai:
                            for j, bj in enumerate(b):
                                res[i + j] += ai * bj
                else:
                    for j, bj in enumerate(b):
                        if bj:
                            for i, ai in enumerate(a):
                                res[i + j] += ai * bj
                for i in range(need):
                    res[i] %= mod
                return res

            n = 1
            while n < need:
                n <<= 1

            fa = a + [0] * (n - la)
            fb = b + [0] * (n - lb)
            ntt(fa)
            ntt(fb)
            for i in range(n):
                fa[i] = fa[i] * fb[i] % mod
            del fb
            intt(fa)

            inv_n = inv_pow2[(n - 1).bit_length()]
            for i in range(need):
                fa[i] = fa[i] * inv_n % mod
            return fa[:need]

    sys.setrecursionlimit(1000000)

    g = [0] * K
    add = [0] * K
    DIRECT = 8192
    mod = MOD

    def cdq(l, r, bs=bs, ms=ms, fact=fact, invfact=invfact,
            g=g, add=add, conv=conv, mod=mod, DIRECT=DIRECT):
        if r - l == 1:
            i = l
            bi = bs[i]
            val = fact[bi] + add[i]
            if val >= mod:
                val -= mod
            g[i] = (mod - val) * invfact[bi - ms[i]] % mod
            return

        mid = (l + r) >> 1
        cdq(l, mid)

        left_len = mid - l
        right_len = r - mid

        if left_len * right_len <= DIRECT:
            for i in range(mid, r):
                bi = bs[i]
                acc = 0
                for j in range(l, mid):
                    acc += g[j] * fact[bi - ms[j]]
                add[i] = (add[i] + acc) % mod
        else:
            ml = ms[l]
            shift = bs[mid] - ms[mid - 1]
            lenA = ms[mid - 1] - ml + 1
            lenB = bs[r - 1] - ml - shift + 1

            A = [0] * lenA
            for idx in range(left_len):
                A[ms[l + idx] - ml] = g[l + idx]

            B = fact[shift:shift + lenB]
            C = conv(A, B)
            del A, B

            for i in range(mid, r):
                add[i] = (add[i] + C[bs[i] - ml - shift]) % mod
            del C

        cdq(mid, r)

    cdq(0, K)

    ans = fact[N]
    for i in range(K):
        ans = (ans + g[i] * fact[N - ms[i]]) % mod
    print(ans)


if __name__ == "__main__":
    main()