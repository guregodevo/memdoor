### CODE
```python
import sys

MOD = 998244353
G = 3
INV2 = (MOD + 1) // 2

ROOT_LOG = 1
FWD_ROOTS = [None, [1]]
INV_ROOTS = [None, [1]]
INV_SIZE = [1, INV2]


def ensure_roots(size):
    global ROOT_LOG
    log = size.bit_length() - 1
    if log <= ROOT_LOG:
        return

    while len(FWD_ROOTS) <= log:
        FWD_ROOTS.append(None)
    while len(INV_ROOTS) <= log:
        INV_ROOTS.append(None)
    while len(INV_SIZE) <= log:
        INV_SIZE.append(INV_SIZE[-1] * INV2 % MOD)

    for l in range(ROOT_LOG + 1, log + 1):
        half = 1 << (l - 1)
        wlen = pow(G, (MOD - 1) >> l, MOD)
        iwlen = pow(wlen, MOD - 2, MOD)

        r = [1] * half
        ir = [1] * half
        for i in range(1, half):
            r[i] = r[i - 1] * wlen % MOD
            ir[i] = ir[i - 1] * iwlen % MOD

        FWD_ROOTS[l] = r
        INV_ROOTS[l] = ir

    ROOT_LOG = log


def ntt(a, invert=False):
    mod = MOD
    n = len(a)

    if not invert:
        length = n
        log = n.bit_length() - 1
        roots = FWD_ROOTS

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

            elif half == 2:
                w1 = roots[log][1]
                for i in range(0, n, 4):
                    u = a[i]
                    v = a[i + 2]
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[i] = x
                    a[i + 2] = y

                    u = a[i + 1]
                    v = a[i + 3]
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[i + 1] = x
                    a[i + 3] = y * w1 % mod

            else:
                r = roots[log]
                for i in range(0, n, length):
                    j = i
                    for w in r:
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
                        j += 1

            length = half
            log -= 1

    else:
        length = 2
        log = 1
        roots = INV_ROOTS

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

            elif half == 2:
                w1 = roots[log][1]
                for i in range(0, n, 4):
                    u = a[i]
                    v = a[i + 2]
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[i] = x
                    a[i + 2] = y

                    u = a[i + 1]
                    v = a[i + 3] * w1 % mod
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[i + 1] = x
                    a[i + 3] = y

            else:
                r = roots[log]
                for i in range(0, n, length):
                    j = i
                    for w in r:
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
                        j += 1

            length <<= 1
            log += 1

        inv_n = INV_SIZE[n.bit_length() - 1]
        for i in range(n):
            a[i] = a[i] * inv_n % mod


def convolution(a, b):
    n = len(a)
    m = len(b)
    if n == 0 or m == 0:
        return []

    if n > m:
        a, b = b, a
        n, m = m, n

    if n * m <= 200000:
        res = [0] * (n + m - 1)
        for i, ai in enumerate(a):
            if ai:
                for j, bj in enumerate(b):
                    res[i + j] += ai * bj
        return [x % MOD for x in res]

    size = 1 << ((n + m - 2).bit_length())
    ensure_roots(size)

    fa = a + [0] * (size - n)
    fb = b + [0] * (size - m)

    ntt(fa, False)
    ntt(fb, False)

    mod = MOD
    for i in range(size):
        fa[i] = fa[i] * fb[i] % mod

    ntt(fa, True)

    need = n + m - 1
    del fa[need:]
    return fa


def fps_inv(f, n):
    mod = MOD
    if n <= 0:
        return []

    limit = min(n, 256)
    g = [0] * limit
    g[0] = 1

    for i in range(1, limit):
        s = f[i]
        for j in range(1, i):
            s += f[j] * g[i - j]
        g[i] = (-s) % mod

    if n <= limit:
        return g

    old = limit
    while old < n:
        new = min(old * 2, n)
        size = 1 << ((2 * new - 1).bit_length())
        ensure_roots(size)

        F = f[:new] + [0] * (size - new)
        G = g + [0] * (size - old)

        ntt(F, False)
        ntt(G, False)

        for i in range(size):
            x = F[i] * G[i] % mod
            y = 2 - x
            if y < 0:
                y += mod
            G[i] = G[i] * y % mod

        ntt(G, True)

        del G[new:]
        g = G
        old = new

    g[0] = 1
    return g


def precompute_fact(n):
    fact = [1] * (n + 1)
    mod = MOD
    for i in range(1, n + 1):
        fact[i] = fact[i - 1] * i % mod
    return fact


def precompute_invfact(fact, n):
    invfact = [1] * (n + 1)
    invfact[n] = pow(fact[n], MOD - 2, MOD)
    mod = MOD
    for i in range(n, 0, -1):
        invfact[i - 1] = invfact[i] * i % mod
    return invfact


def build_active(n, s):
    active = []
    kval = []

    b = 0
    w = 0
    prev_h = 0

    for c in s:
        if c == 66:  # 'B'
            b += 1
            if b >= 2:
                a = b - 1
                ha = w
                if ha >= a and (a == 1 or ha != prev_h):
                    active.append(a)
                    kval.append(ha)
                prev_h = ha
        else:
            w += 1

    return active, kval


def solve_alternating(n, fact):
    # For S = BWBW...BWBW, A_i are coefficients of 1 / sum_{j>=0} j! x^j.
    b = fps_inv(fact, n)
    s = 0
    for i in range(1, n):
        s += b[i] * fact[n - i]
    return (fact[n] + s) % MOD


def solve_direct_small(n, active, kval, fact, invfact):
    m = len(active)
    A = [0] * m
    mod = MOD

    for i in range(m):
        ki = kval[i]
        total = fact[ki]
        for p in range(i):
            total += A[p] * fact[ki - active[p]]
        total %= mod
        A[i] = (mod - total) * invfact[ki - active[i]] % mod

    s = 0
    for i, a in enumerate(active):
        s += A[i] * fact[n - a]
    return (fact[n] + s) % mod


def try_special_consecutive(n, active, kval, fact, invfact):
    m = len(active)
    if m < 512:
        return None

    d = active[0] - 1
    c = kval[0] - active[0]
    if c < 0:
        return None

    ai = d + 1
    for i in range(m):
        if active[i] != ai or kval[i] != ai + c:
            return None
        ai += 1

    L = d + m
    if L + c > n:
        return None

    mod = MOD

    if c == 0:
        f = fact
        f_owned = False
    else:
        f = [0] * (L + 1)
        f[0] = 1
        invc = invfact[c]
        for j in range(1, L + 1):
            f[j] = fact[j + c] * invc % mod
        f_owned = True

    B = fps_inv(f, L + 1)

    ans = fact[n]
    s = 0

    if d == 0:
        # A_i = B_i
        for i in range(1, m + 1):
            s += B[i] * fact[n - i]
        if f_owned:
            del f
    else:
        P = f[:d + 1]
        if f_owned:
            del f

        if d <= 32:
            A_vals = [0] * m
            Bv = B
            Av = A_vals
            for j, pj in enumerate(P):
                if pj:
                    off = d - j + 1
                    for i in range(m):
                        Av[i] += pj * Bv[off + i]
            for i in range(m):
                Av[i] %= mod
                s += Av[i] * fact[n - d - 1 - i]
        else:
            C = convolution(P, B)
            start = d + 1
            for i in range(m):
                s += C[start + i] * fact[n - d - 1 - i]

    return (ans + s) % mod


def solve_case(n, s):
    # If vertex 1 is white, no edge can enter prefix {1}.
    # If vertex 2N is black, it has no outgoing edge.
    if s[0] == 87 or s[-1] == 66:  # 'W' or 'B'
        return 0

    fact = precompute_fact(n)

    if n == 1:
        return 1

    # Full alternating case: solve by one FPS inverse.
    if s == b'BW' * n:
        return solve_alternating(n, fact)

    active, kval = build_active(n, s)
    if not active:
        return fact[n]

    invfact = precompute_invfact(fact, n)
    m = len(active)

    if m <= 512:
        return solve_direct_small(n, active, kval, fact, invfact)

    special = try_special_consecutive(n, active, kval, fact, invfact)
    if special is not None:
        return special

    A = [0] * m
    add = [0] * m
    mod = MOD

    sys.setrecursionlimit(1_000_000)

    def cdq(l, r,
            a=active, K=kval, A=A, add=add,
            fact=fact, invfact=invfact,
            ensure=ensure_roots, ntt_func=ntt,
            mod=MOD):
        if r - l == 1:
            ai = a[l]
            ki = K[l]
            val = fact[ki] + add[l]
            if val >= mod:
                val -= mod
            A[l] = (mod - val) * invfact[ki - ai] % mod
            return

        mid = (l + r) >> 1
        cdq(l, mid)

        left_len = mid - l
        right_len = r - mid
        prod = left_len * right_len

        use_ntt = False
        offset = la = Rmin = Rlen = size = 0

        if prod > 16384:
            offset = a[l]
            la = a[mid - 1] - offset + 1
            dmin = K[mid] - offset
            dmax = K[r - 1] - offset
            Rmin = dmin - la + 1
            Rlen = dmax - Rmin + 1
            conv_len = la + Rlen - 1
            size = 1 << ((conv_len - 1).bit_length())
            logsize = size.bit_length() - 1

            # Direct multiplication is competitive for small effective sizes.
            if prod > size * logsize * 8:
                use_ntt = True

        if not use_ntt:
            left_range = range(l, mid)
            for i in range(mid, r):
                ki = K[i]
                total = 0
                for p in left_range:
                    total += A[p] * fact[ki - a[p]]
                add[i] = (add[i] + total) % mod
        else:
            ensure(size)

            L = [0] * size
            for p in range(l, mid):
                L[a[p] - offset] = A[p]

            R = [0] * size
            R[:Rlen] = fact[Rmin:Rmin + Rlen]

            ntt_func(L, False)
            ntt_func(R, False)

            for i in range(size):
                L[i] = L[i] * R[i] % mod

            ntt_func(L, True)
            del R

            for i in range(mid, r):
                add[i] = (add[i] + L[K[i] - offset - Rmin]) % mod

            del L

        cdq(mid, r)

    cdq(0, m)

    s = 0
    for i, a in enumerate(active):
        s += A[i] * fact[n - a]
    return (fact[n] + s) % mod


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    n = int(data[0])
    s = data[1].strip()
    print(solve_case(n, s))


if __name__ == "__main__":
    main()
```

### NOTES
- **Model:** Pairings are permutations `pi` from black order to white order. With path edges only forward, strong connectivity is equivalent to every proper prefix having an added edge entering it. For prefix before the `(a+1)`-st black, let `h[a]` be the number of whites before that black. Bad event `E_a`: `max(pi(1..a)) <= h[a]`.
- **Redundant events:** If `h[a] < a`, `E_a` is empty. If `h[a] = h[a-1]`, then `E_a` is contained in `E_{a-1}`. Keep only active `a` with `h[a] >= a` and `h[a] != h[a-1]` (or `a=1`).
- **Inclusion-exclusion recurrence:** For active indices `a_i` and `K_i=h[a_i]`, define signed grouped sums `A_i` by  
  `A_i = -invfact[K_i-a_i] * (fact[K_i] + sum_{p<i} A_p * fact[K_i-a_p])`.  
  Answer is `fact[N] + sum_i A_i * fact[N-a_i]`.
- **Active construction:** Scan the string once. When the `b`-th black is seen (`b>=2`), `a=b-1`, `h[a]=current white count`. Keep active if above condition holds. This avoids storing the full `h` array.
- **Early exits:** If `S[0]=='W'` or `S[-1]=='B'`, answer is `0`. For `N=1`, only `BW` survives and answer is `1`.
- **Full alternating fast path:** If `S == "BW"*N`, then `a_i=i`, `K_i=i`. The recurrence becomes the inverse of `F(x)=sum i! x^i`. Compute `B=1/F` up to `N` by Newton FPS and answer `fact[N] + sum_{i=1}^{N-1} B[i] fact[N-i]`.
- **Generalized consecutive fast path:** If active indices are consecutive `a_i = d+i` and `K_i = a_i+c` for constant `c`, recurrence is solvable with one FPS inverse of `F_j = fact[j+c]/fact[c]`. For `d=0`, `A_i=B_i`. For `d>0`, `A_i` is a short convolution of `F[0..d]` with `B`, computed directly if `d` is small or by NTT otherwise. This covers near-alternating worst cases.
- **CDQ convolution:** For general active lists, compute `A` online. In a node `[l,r)`, after solving left half, add to right half: `sum_{p in left} A[p] fact[K[i]-a[p]]`. This is a convolution with a shifted factorial array. Use direct loops for small products; otherwise NTT.
- **NTT details:** Mod `998244353`, primitive root `3`. Use in-place DIT/DIF pair without bit reversal, precomputed twiddle roots for all needed powers, and special-cased half=1/2 stages for speed.
- **Complexity:** Alternating and consecutive-constant cases are `O(N log N)`. General case is CDQ with NTT, roughly `O(M log^2 M)`, with adaptive direct cutoffs to reduce small-transform overhead.
- **Modulo safety:** Direct sums are accumulated as Python integers and reduced once per affected entry; values stay within a few hundred bits, which is safe and faster than per-addition modulo.

### NEXT
- none

### STATUS
solved
