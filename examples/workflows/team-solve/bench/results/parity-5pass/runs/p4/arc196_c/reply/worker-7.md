### CODE
```python
import sys

MOD = 998244353
G = 3
DIRECT_M = 2000


def extract_events(N, S):
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


def solve_large(N, c_arr, r_arr, fact, invfact):
    mod = MOD
    M = len(c_arr)

    c0 = c_arr[0]
    max_need_bound = (c_arr[-1] - c0 + 1) + (r_arr[-1] - c0 + 1) + 5
    max_n = 1
    while max_n < max_need_bound:
        max_n <<= 1

    roots, iroots, inv_len = build_roots(max_n)

    A = [0] * M
    query = [0] * M
    f = fact
    inv = invfact
    cc = c_arr
    rr_arr = r_arr

    T = 128
    DIRECT = 16384
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

    def intt(a, iroots=iroots, inv_len=inv_len, mod=mod):
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
        return [x * inv_n % mod for x in a]

    def add_convolution(l, mid, right,
                        f=f, cc=cc, rr_arr=rr_arr,
                        A=A, query=query,
                        ntt=ntt, intt=intt, mod=mod):
        c_l = cc[l]
        c_mid = cc[mid - 1]
        Clen = c_mid - c_l + 1

        min_diff = rr_arr[mid] - c_mid
        max_diff = rr_arr[right - 1] - c_l
        Flen = max_diff - min_diff + 1

        need = Clen + Flen - 1
        n = 1 << (need - 1).bit_length()

        fa = [0] * n
        for j in range(l, mid):
            fa[cc[j] - c_l] = A[j]

        fb = f[min_diff:min_diff + Flen]
        if len(fb) < n:
            fb += [0] * (n - len(fb))

        ntt(fa)
        ntt(fb)
        fa = [x * y % mod for x, y in zip(fa, fb)]
        del fb
        fa = intt(fa)

        base = -c_l - min_diff
        for i in range(mid, right):
            idx = rr_arr[i] + base
            val = query[i] + fa[idx]
            if val >= mod:
                val -= mod
            query[i] = val

    def cdq(l, right,
            A=A, query=query, f=f, inv=inv,
            cc=cc, rr_arr=rr_arr, mod=mod,
            T=T, DIRECT=DIRECT, add=add_convolution):
        if right - l <= T:
            for i in range(l, right):
                ri = rr_arr[i]
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

        if left_len * right_len <= DIRECT:
            for i in range(mid, right):
                ri = rr_arr[i]
                s = query[i]
                for j in range(l, mid):
                    s += A[j] * f[ri - cc[j]]
                s %= mod
                query[i] = s
        else:
            add(l, mid, right)

        cdq(mid, right)

    cdq(0, M)

    ans = f[N]
    for i in range(M):
        ans = (ans + A[i] * f[N - cc[i]]) % mod
    return ans


def should_use_direct(M, N, c_arr, r_arr):
    if M <= DIRECT_M:
        return True

    if M <= 4000:
        direct_ops = M * (M - 1) // 2
        span = (c_arr[-1] - c_arr[0]) + (r_arr[-1] - c_arr[0])
        base = max(M, span)
        ntt_ops = (base + M) * M.bit_length() * max(1, (2 * base + M).bit_length())
        if direct_ops * 2 <= ntt_ops:
            return True

    return False


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    N = int(data[0])
    S = data[1]

    if S[0] != 66 or S[-1] != 87:
        print(0)
        return

    c_arr, r_arr = extract_events(N, S)
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

    if should_use_direct(M, N, c_arr, r_arr):
        ans = solve_direct(N, c_arr, r_arr, fact, invfact)
    else:
        ans = solve_large(N, c_arr, r_arr, fact, invfact)

    print(ans % MOD)


if __name__ == "__main__":
    solve()
```

### NOTES
- **Endpoint condition:** Strong connectivity is impossible unless vertex 1 is black and vertex 2N is white. This is checked first and immediately returns 0 otherwise.
- **Cut characterization:** With the base path 1 -> 2 -> ... -> 2N, the graph is strongly connected iff every cut has at least one added edge from right to left. For a prefix containing c black vertices and w white vertices, this means at least one of the first c black vertices is paired with a white vertex after the prefix.
- **Event reduction:** For c = 1..N-1, let R[c] be the number of white vertices before black c+1. The hardest cut with c black vertices has w = R[c]. The bad event E_c is that the first c black vertices are paired only with the first R[c] white vertices. If R[c] < c, E_c is impossible. If R is constant over several c, only the first such c matters because later events are subsets of the earlier one.
- **Inclusion-exclusion DP:** For kept events (c_i, r_i), sorted increasingly, define A[i] as the signed inclusion-exclusion sum for subsets whose maximum kept index is i. Then  
  `A[i] = -(fact[r_i] + sum_{j<i} A[j] * fact[r_i - c_j]) / fact[r_i - c_i]`.  
  The answer is `fact[N] + sum_i A[i] * fact[N - c_i]`.
- **CDQ divide and conquer:** The recurrence is triangular. CDQ computes contributions from the left half to the right half. For a node, left terms are placed by actual c-coordinate, and a factorial segment is convolved to update all right-side query sums. This gives the needed shifted factorial convolution.
- **Direct threshold:** The direct O(M^2) solver is forced to be used only for M <= 2000 by `DIRECT_M = 2000`. A conservative adaptive fallback may still choose direct for M up to 4000 when the active c/r span is much smaller than N, but random/structured stress cases with N=3000..10000 and M>2000 use the large CDQ path.
- **Stress testing:** Compared the direct solver and the CDQ/NTT solver on random strings and structured strings (alternating, blocky, large jumps) in the requested N=3000..10000 range with active M>2000. No correctness mismatches were found.
- **NTT implementation:** Uses DIF forward and DIT inverse transforms with precomputed twiddle factors, avoiding per-butterfly twiddle updates. The transform size is bounded by the active c/r span, not always by 2N, which keeps memory and time lower for localized event ranges.
- **Indexing in convolution:** For a CDQ node, `min_diff = r[mid] - c[mid-1]` and `max_diff = r[right-1] - c[l]`. The convolution index for right event i is `r[i] - c[l] - min_diff`. This was verified against the direct recurrence on small and medium random cases.
- **Samples:** Sample 1 gives 1, sample 2 gives 0 by endpoint check, and sample 3 gives 240792 with the direct path.

### NEXT
- none

### STATUS
solved
