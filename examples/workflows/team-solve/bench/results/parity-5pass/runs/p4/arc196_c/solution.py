import sys

MOD = 998244353
G = 3


def build_roots(max_n):
    mod = MOD
    max_log = max_n.bit_length() - 1
    roots = [None] * (max_log + 1)
    iroots = [None] * (max_log + 1)
    inv_len = [1] * (max_log + 1)
    inv2 = (mod + 1) // 2

    for k in range(1, max_log + 1):
        length = 1 << k
        half = length >> 1

        wlen = pow(G, (mod - 1) // length, mod)
        rw = [1] * half
        for i in range(1, half):
            rw[i] = rw[i - 1] * wlen % mod
        roots[k] = rw

        iwlen = pow(wlen, mod - 2, mod)
        irw = [1] * half
        for i in range(1, half):
            irw[i] = irw[i - 1] * iwlen % mod
        iroots[k] = irw

        inv_len[k] = inv_len[k - 1] * inv2 % mod

    return roots, iroots, inv_len


def ntt(a, roots, mod=MOD):
    n = len(a)
    if n == 1:
        return

    length = n
    k = n.bit_length() - 1
    while length > 1:
        half = length >> 1
        if half == 1:
            for i in range(0, n, 2):
                u = a[i]
                v = a[i + 1]
                x = u + v
                if x >= mod:
                    x -= mod
                y = u - v
                if y < 0:
                    y += mod
                a[i] = x
                a[i + 1] = y
        else:
            ws = roots[k]
            for i in range(0, n, length):
                p = i
                q = i + half
                u = a[p]
                v = a[q]
                x = u + v
                if x >= mod:
                    x -= mod
                y = u - v
                if y < 0:
                    y += mod
                a[p] = x
                a[q] = y

                for j in range(1, half):
                    p = i + j
                    q = p + half
                    u = a[p]
                    v = a[q]
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[p] = x
                    a[q] = y * ws[j] % mod

        length = half
        k -= 1


def intt(a, iroots, inv_len, mod=MOD):
    n = len(a)
    if n == 1:
        return a

    length = 2
    k = 1
    while length <= n:
        half = length >> 1
        if half == 1:
            for i in range(0, n, 2):
                u = a[i]
                v = a[i + 1]
                x = u + v
                if x >= mod:
                    x -= mod
                y = u - v
                if y < 0:
                    y += mod
                a[i] = x
                a[i + 1] = y
        else:
            ws = iroots[k]
            for i in range(0, n, length):
                p = i
                q = i + half
                u = a[p]
                v = a[q]
                x = u + v
                if x >= mod:
                    x -= mod
                y = u - v
                if y < 0:
                    y += mod
                a[p] = x
                a[q] = y

                for j in range(1, half):
                    p = i + j
                    q = p + half
                    u = a[p]
                    v = a[q] * ws[j] % mod
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[p] = x
                    a[q] = y

        length <<= 1
        k += 1

    inv_n = inv_len[n.bit_length() - 1]
    for i in range(n):
        a[i] = a[i] * inv_n % mod
    return a


def extract_events(S):
    c_arr = []
    r_arr = []
    prev = -1
    w = 0
    b = 0
    W = 87  # ord('W')

    for ch in S:
        if ch == W:
            w += 1
        else:
            b += 1
            if b >= 2:
                c = b - 1
                r = w
                if r > prev:
                    if r >= c:
                        c_arr.append(c)
                        r_arr.append(r)
                    prev = r

    return c_arr, r_arr


def solve_direct(N, c_arr, r_arr, fact, invfact):
    mod = MOD
    M = len(c_arr)
    A = [0] * M
    f = fact
    inv = invfact
    cc = c_arr
    rr = r_arr

    for i in range(M):
        ri = rr[i]
        s = 0
        for j in range(i):
            s += A[j] * f[ri - cc[j]]
        s %= mod

        val = f[ri] + s
        if val >= mod:
            val -= mod
        A[i] = (mod - val) * inv[ri - cc[i]] % mod

    ans = f[N]
    for i in range(M):
        ans = (ans + A[i] * f[N - cc[i]]) % mod
    return ans


def should_use_direct(M, c_arr, r_arr):
    if M <= 4000:
        return True
    if M > 6000:
        return False

    ops = M * (M - 1) // 2
    if ops <= 5_000_000:
        return True

    span = (c_arr[-1] - c_arr[0]) + (r_arr[-1] - c_arr[0])
    base = max(M, span)
    ntt_ops = (base + M) * M.bit_length() * max(1, (2 * base + M).bit_length())
    return ops * 2 <= ntt_ops


def solve_large(N, c_arr, r_arr, fact, invfact):
    mod = MOD
    M = len(c_arr)

    c0 = c_arr[0]
    max_need = (c_arr[-1] - c0 + 1) + (r_arr[-1] - c0 + 1) + 5
    max_n = 1
    while max_n < max_need:
        max_n <<= 1

    roots, iroots, inv_len = build_roots(max_n)

    A = [0] * M
    query = [0] * M
    f = fact
    inv = invfact
    cc = c_arr
    rr = r_arr

    sys.setrecursionlimit(1_000_000)

    T = 64
    DIRECT_OPS = 20_000
    DIRECT_CAP = 3_000_000

    def direct_add(l, mid, right,
                   A=A, query=query, f=f, cc=cc, rr=rr, mod=mod):
        left_len = mid - l
        if left_len == 1:
            aj = A[l]
            if aj == 0:
                return
            cj = cc[l]
            for i in range(mid, right):
                query[i] = (query[i] + aj * f[rr[i] - cj]) % mod
            return

        for i in range(mid, right):
            ri = rr[i]
            s = query[i]
            for j in range(l, mid):
                s += A[j] * f[ri - cc[j]]
            query[i] = s % mod

    def add_convolution(l, mid, right,
                        A=A, query=query, f=f, cc=cc, rr=rr,
                        roots=roots, iroots=iroots, inv_len=inv_len,
                        ntt=ntt, intt=intt, mod=mod):
        c_l = cc[l]
        c_mid = cc[mid - 1]
        Clen = c_mid - c_l + 1

        min_diff = rr[mid] - c_mid
        max_diff = rr[right - 1] - c_l
        Flen = max_diff - min_diff + 1

        need = Clen + Flen - 1
        n = 1 << (need - 1).bit_length()

        fa = [0] * n
        any_nonzero = False
        for j in range(l, mid):
            val = A[j]
            if val:
                fa[cc[j] - c_l] = val
                any_nonzero = True

        if not any_nonzero:
            return

        fb = f[min_diff:min_diff + Flen]
        if len(fb) < n:
            fb += [0] * (n - len(fb))

        ntt(fa, roots, mod)
        ntt(fb, roots, mod)

        for i in range(n):
            fa[i] = fa[i] * fb[i] % mod
        del fb

        intt(fa, iroots, inv_len, mod)

        base = -c_l - min_diff
        for i in range(mid, right):
            idx = rr[i] + base
            val = query[i] + fa[idx]
            if val >= mod:
                val -= mod
            query[i] = val

    def cdq(l, right,
            A=A, query=query, f=f, inv=inv, cc=cc, rr=rr, mod=mod,
            direct_add=direct_add, add_convolution=add_convolution):
        if right - l <= T:
            for i in range(l, right):
                ri = rr[i]
                s = query[i]
                for j in range(l, i):
                    s += A[j] * f[ri - cc[j]]
                s %= mod

                val = f[ri] + s
                if val >= mod:
                    val -= mod
                A[i] = (mod - val) * inv[ri - cc[i]] % mod
            return

        mid = (l + right) >> 1
        cdq(l, mid)

        left_len = mid - l
        right_len = right - mid
        ops = left_len * right_len

        use_direct = ops <= DIRECT_OPS
        if not use_direct:
            c_l = cc[l]
            c_mid = cc[mid - 1]
            r_mid = rr[mid]
            r_last = rr[right - 1]

            need = (c_mid - c_l + 1) + (r_last - c_l - (r_mid - c_mid) + 1) - 1
            n = 1 << (need - 1).bit_length()
            ntt_cost = n * (n.bit_length() - 1) * 3 // 2

            if ops <= DIRECT_CAP and ops * 6 <= ntt_cost:
                use_direct = True

        if use_direct:
            direct_add(l, mid, right)
        else:
            add_convolution(l, mid, right)

        cdq(mid, right)

    cdq(0, M)

    ans = f[N]
    for i in range(M):
        ans = (ans + A[i] * f[N - cc[i]]) % mod
    return ans


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    N = int(data[0])
    S = data[1]

    if S[0] != 66 or S[-1] != 87:  # 'B', 'W'
        print(0)
        return

    c_arr, r_arr = extract_events(S)
    M = len(c_arr)

    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD

    if M == 0:
        print(fact[N] % MOD)
        return

    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    if should_use_direct(M, c_arr, r_arr):
        ans = solve_direct(N, c_arr, r_arr, fact, invfact)
    else:
        ans = solve_large(N, c_arr, r_arr, fact, invfact)

    print(ans % MOD)


if __name__ == "__main__":
    solve()