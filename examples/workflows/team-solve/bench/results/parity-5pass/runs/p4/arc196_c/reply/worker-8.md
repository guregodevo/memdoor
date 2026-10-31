[cut off at the reply limit]
### CODE
```python
import sys

MOD = 998244353
G = 3

DIRECT_M = 5000
BASE_T = 128
BUTTERFLY_COST = 3


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


def solve_direct(N, c_arr, r_arr, fact, invfact):
    mod = MOD
    M = len(c_arr)
    f = fact
    inv = invfact
    cc = c_arr
    rr = r_arr

    A = [0] * M
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


def solve_large(N, c_arr, r_arr, fact, invfact):
    mod = MOD
    M = len(c_arr)
    cc = c_arr
    rr = r_arr
    f = fact
    inv = invfact

    c0 = cc[0]
    max_need = (cc[-1] - c0 + 1) + (rr[-1] - c0 + 1) + 5
    max_n = 1
    while max_n < max_need:
        max_n <<= 1

    roots, iroots, inv_len = build_roots(max_n)

    A = [0] * M
    query = [0] * M

    sys.setrecursionlimit(1_000_000)

    def ntt(a, roots=roots, mod=mod):
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
                rng = range(1, half)
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

                    for j in rng:
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

    def intt(a, iroots=iroots, mod=mod):
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
                rng
