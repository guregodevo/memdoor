### CODE
```python
import sys

MOD = 998244353
G = 3

DIRECT_M = 2000
BASE = 64
DIRECT_ADD = 8192
DIRECT_FORCE = 50000
RUN_MIN = 2048


def precompute_fact(n):
    fact = [1] * (n + 1)
    for i in range(1, n + 1):
        fact[i] = fact[i - 1] * i % MOD
    invfact = [1] * (n + 1)
    invfact[n] = pow(fact[n], MOD - 2, MOD)
    for i in range(n, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD
    return fact, invfact


_roots_cache = {}
_inv_n_cache = {}


def _get_roots(length, invert):
    key = (length, invert)
    roots = _roots_cache.get(key)
    if roots is None:
        wlen = pow(G, (MOD - 1) // length, MOD)
        if invert:
            wlen = pow(wlen, MOD - 2, MOD)
        half = length >> 1
        roots = [1] * half
        w = 1
        for i in range(1, half):
            w = w * wlen % MOD
            roots[i] = w
        _roots_cache[key] = roots
    return roots


def _get_inv_n(n):
    v = _inv_n_cache.get(n)
    if v is None:
        v = pow(n, MOD - 2, MOD)
        _inv_n_cache[n] = v
    return v


def ntt_forward(a):
    n = len(a)
    if n == 1:
        return
    mod = MOD
    length = n
    while length > 1:
        half = length >> 1
        roots = _get_roots(length, False)
        for i in range(0, n, length):
            k = 0
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
                a[j + half] = y * roots[k] % mod
                k += 1
        length = half


def ntt_inverse(a):
    n = len(a)
    if n == 1:
        return
    mod = MOD
    length = 2
    while length <= n:
        half = length >> 1
        roots = _get_roots(length, True)
        if length == n:
            inv_n = _get_inv_n(n)
            for i in range(0, n, length):
                k = 0
                end = i + half
                for j in range(i, end):
                    u = a[j]
                    v = a[j + half] * roots[k] % mod
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[j] = x * inv_n % mod
                    a[j + half] = y * inv_n % mod
                    k += 1
        else:
            for i in range(0, n, length):
                k = 0
                end = i + half
                for j in range(i, end):
                    u = a[j]
                    v = a[j + half] * roots[k] % mod
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[j] = x
                    a[j + half] = y
                    k += 1
        length <<= 1


def convolution(a, b):
    if not a or not b:
        return []
    la, lb = len(a), len(b)
    need = la + lb - 1

    if la * lb <= 20000:
        res = [0] * need
        if la <= lb:
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
            res[i] %= MOD
        return res

    n = 1
    while n < need:
        n <<= 1

    fa = a + [0] * (n - la)
    fb = b + [0] * (n - lb)

    ntt_forward(fa)
    ntt_forward(fb)
    for i in range(n):
        fa[i] = fa[i] * fb[i] % MOD
    ntt_inverse(fa)
    return fa[:need]


def poly_inv_fast(p, n):
    """Inverse of p modulo x^n.  p[0] must be invertible (here p[0]=1)."""
    if n == 0:
        return []
    q = [pow(p[0], MOD - 2, MOD)]
    m = 1
    while m < n:
        m2 = m << 1

        p0 = p[:m]
        p1 = p[m:m2]

        c1 = convolution(p0, q)
        c2 = convolution(p1, q) if p1 else []

        len_c1 = len(c1)
        len_c2 = len(c2)
        e_high = [0] * m
        for i in range(m):
            val = 0
            idx = m + i
            if idx < len_c1:
                val += c1[idx]
            if i < len_c2:
                val += c2[i]
            if val >= MOD:
                val -= MOD
            if val:
                e_high[i] = MOD - val

        c3 = convolution(q, e_high)
        q = q + c3[:m]
        m = m2

    return q[:n]


def solve_direct(N, at, ar, fact, invfact):
    m = len(at)
    H = [0] * m
    H[0] = 1
    nz = [0]
    ans = fact[N]

    for i in range(1, m):
        ri = ar[i]
        s = 0
        for j in nz:
            s += H[j] * fact[ri - at[j]]
        h = (-s % MOD) * invfact[ri - at[i]] % MOD
        H[i] = h
        if h:
            nz.append(i)
        ans = (ans + h * fact[N - at[i]]) % MOD

    return ans


def solve_large(N, at, ar, fact, invfact):
    m = len(at)
    H = [0] * m
    contrib = [0] * m
    H[0] = 1

    bal = [0] * m
    for i in range(1, m):
        bal[i] = ar[i] - at[i]

    def add_contrib(l, mid, r):
        nz = []
        for j in range(l, mid + 1):
            hj = H[j]
            if hj:
                nz.append(j)
        if not nz:
            return

        cnt = len(nz)
        right_len = r - mid

        if cnt <= 8 or cnt * right_len <= DIRECT_ADD:
            for i in range(mid + 1, r + 1):
                ri = ar[i]
                total = 0
                for j in nz:
                    total += H[j] * fact[ri - at[j]]
                total %= MOD
                v = contrib[i] + total
                if v >= MOD:
                    v -= MOD
                contrib[i] = v
            return

        first = nz[0]
        last = nz[-1]
        t_min = at[first]
        t_max = at[last]
        wlen = t_max - t_min + 1

        k_min = ar[mid + 1] - t_min
        k_max = ar[r] - t_min
        j0 = k_min - wlen + 1
        if j0 < 0:
            j0 = 0

        left = [0] * wlen
        for j in nz:
            left[at[j] - t_min] = H[j]

        b_arr = fact[j0:k_max + 1]
        c = convolution(left, b_arr)

        for i in range(mid + 1, r + 1):
            idx = ar[i] - t_min - j0
            if 0 <= idx < len(c):
                v = contrib[i] + c[idx]
                if v >= MOD:
                    v -= MOD
                contrib[i] = v

    def cdq(l, r):
        if r - l + 1 <= BASE:
            for i in range(l, r + 1):
                if i == 0:
                    H[i] = 1
                    continue
                s = contrib[i]
                ri = ar[i]
                for j in range(l, i):
                    hj = H[j]
                    if hj:
                        s += hj * fact[ri - at[j]]
                H[i] = (-s % MOD) * invfact[ri - at[i]] % MOD
            return

        mid = (l + r) >> 1
        cdq(l, mid)
        add_contrib(l, mid, r)
        cdq(mid + 1, r)

    def make_P(c, L):
        P = [1] * L
        if L > 1:
            invfc = invfact[c]
            for k in range(1, L):
                P[k] = fact[c + k] * invfc % MOD
        return P

    def compute_forcing(p, T0, c, L):
        # previous active indices are 0 .. p-1
        if p == 1:
            return fact[c + T0:c + T0 + L]

        nz = []
        for j in range(p):
            if H[j]:
                nz.append(j)
        if not nz:
            return [0] * L

        if len(nz) == 1:
            j0 = nz[0]
            h0 = H[j0]
            base = c + T0 - at[j0]
            return [(h0 * fact[base + n]) % MOD for n in range(L)]

        if len(nz) <= 8 or len(nz) * L <= DIRECT_FORCE:
            T = [0] * L
            for n in range(L):
                base = c + T0 + n
                total = 0
                for j in nz:
                    total += H[j] * fact[base - at[j]]
                T[n] = total % MOD
            return T

        first = nz[0]
        last = nz[-1]
        t_min = at[first]
        t_max = at[last]
        wlen = t_max - t_min + 1

        k_min = c + T0 - t_min
        k_max = c + T0 + L - 1 - t_min
        j0 = k_min - wlen + 1
        if j0 < 0:
            j0 = 0

        left = [0] * wlen
        for j in nz:
            left[at[j] - t_min] = H[j]

        b_arr = fact[j0:k_max + 1]
        c = convolution(left, b_arr)

        start = k_min - j0
        T = [0] * L
        for n in range(L):
            idx = start + n
            if idx < len(c):
                T[n] = c[idx]
        return T

    # Long constant-balance suffix: solve prefix by CDQ, suffix by one inverse.
    Ls = 1
    for i in range(m - 1, 1, -1):
        if at[i] == at[i - 1] + 1 and bal[i] == bal[i - 1]:
            Ls += 1
        else:
            break

    if Ls >= RUN_MIN and Ls * 3 >= m:
        p = m - Ls
        if p > 1:
            cdq(0, p - 1)

        c = bal[p]
        T = compute_forcing(p, at[p], c, Ls)
        Q = poly_inv_fast(make_P(c, Ls), Ls)
        convTQ = convolution(T, Q)
        invfc = invfact[c]

        ans = fact[N]
        for j in range(1, p):
            hj = H[j]
            if hj:
                ans = (ans + hj * fact[N - at[j]]) % MOD
        t0 = at[p]
        for n in range(Ls):
            h = (MOD - convTQ[n] * invfc % MOD) % MOD
            if h:
                ans = (ans + h * fact[N - (t0 + n)]) % MOD
        return ans

    # Peel very long constant-balance prefixes, then CDQ the rest.
    start = 0
    bound = 0
    while start + 1 < m:
        remaining = m - 1 - start
        s = start + 1
        L = 1
        while s + L < m and at[s + L] == at[s + L - 1] + 1 and bal[s + L] == bal[s + L - 1]:
            L += 1

        if L >= RUN_MIN and L * 3 >= remaining:
            c = bal[s]
            T = compute_forcing(s, at[s], c, L)
            Q = poly_inv_fast(make_P(c, L), L)
            convTQ = convolution(T, Q)
            invfc = invfact[c]

            for n in range(L):
                H[s + n] = (MOD - convTQ[n] * invfc % MOD) % MOD

            new_end = s + L - 1
            if new_end < m - 1:
                add_contrib(bound, new_end, m - 1)
                bound = new_end + 1
            start = new_end
        else:
            break

    if start + 1 < m:
        if bound == 0:
            cdq(0, m - 1)
        else:
            cdq(bound, m - 1)

    ans = fact[N]
    for j in range(1, m):
        hj = H[j]
        if hj:
            ans = (ans + hj * fact[N - at[j]]) % MOD
    return ans


def solve_problem(N, S):
    if N == 1:
        return 1 if S == "BW" else 0
    if S[0] != 'B' or S[-1] != 'W':
        return 0

    fact, invfact = precompute_fact(N)

    at = [0]
    ar = [0]
    w = 0
    b = 0
    prev_r = 0

    for ch in S:
        if ch == 'W':
            w += 1
        else:
            b += 1
            if b >= 2:
                r = w
                t = b - 1
                if r >= t and r > prev_r:
                    at.append(t)
                    ar.append(r)
                prev_r = r

    m = len(at)
    if m == 1:
        return fact[N]

    if m <= DIRECT_M:
        return solve_direct(N, at, ar, fact, invfact)

    return solve_large(N, at, ar, fact, invfact)


def main():
    data = sys.stdin.read().split()
    if not data:
        return
    N = int(data[0])
    S = data[1].strip()
    print(solve_problem(N, S) % MOD)


if __name__ == "__main__":
    main()
```

### NOTES
- **Strong connectivity reduction:** With the original path edges, forward reachability is automatic. Strong connectivity is equivalent to the last vertex reaching the first. This forces `S[0]='B'` and `S[-1]='W'`, and for every cut before black `t+1` (`t=1..N-1`) with `R[t]` whites on the left, the matching must have at least one white on the right matched to a black on the left. In permutation terms, if `q_j` is the white rank matched to black rank `j`, validity is `max(q_1..q_t) > R[t]` for all `t`.
- **Inclusion-exclusion recurrence:** Let bad event `E_t` be `max(q_1..q_t) <= R[t]`. Only the first occurrence of each `R` value can contribute. Define `H[0]=1` and, for active first-occurrence cuts `t`,
  `H[t] = - invfact[R[t]-t] * sum_{s<t} H[s] * fact[R[t]-s]`.
  The answer is `N! + sum_{active t} H[t] * fact[N-t]`.
- **First-occurrence simplification:** If `R[t] == R[t-1]`, then `H[t]=0`. More generally, for a first occurrence of value `r`, the sum over `s<t` can be taken over all previously active cuts because non-active `H` values are zero. This makes the recurrence depend only on the active list `(t, R[t])`.
- **Active list compression:** Build only active cuts `t` where `R[t] >= t` and `R[t] > R[t-1]`. Index 0 is the artificial base `(0,0)` with `H[0]=1`. The recurrence is then a triangular online convolution over this sparse list.
- **CDQ/NTT solver:** The active-list CDQ adds contributions from a left half of active cuts to a right half. For a left block, pack `H` by black index `t` into an array over `[t_min, t_max]`, and convolve with a shifted factorial slice. The shift is chosen as `j0 = k_min - W + 1`, where `k = R - t_min`, so only needed factorial coefficients are used. Small blocks use direct summation; very sparse left blocks also use direct summation.
- **NTT implementation:** Uses DIF forward and DIT inverse transforms, avoiding explicit bit-reversal. Roots are cached per length and direction. Inverse scaling is fused into the final stage. Convolution uses direct multiplication for tiny products and NTT otherwise.
- **Constant-balance shortcut:** If a long run of active cuts has constant `c = R[t]-t` and consecutive `t`, the recurrence becomes a linear convolution with kernel `fact[c+k]`. Let `P_c(x)=1+invfact[c]*sum_{k>=1} fact[c+k]x^k`. The run values are obtained from a forcing series `T(x)` by `H_run = -invfact[c] * T * P_c^{-1}`. `T` is the contribution from all earlier cuts. This reduces long alternating / long initial-W-run cases to one polynomial inverse plus a few convolutions.
- **Polynomial inverse:** A Newton iteration specialized to avoid full-size `4m` transforms: at each doubling, compute `e_high = high(2 - P*Q)` from two convolutions of size `2m`, then one more convolution `Q*e_high`. This keeps the largest NTT size at `2m` instead of `4m` for the final step.
- **Solver selection:** Endpoints invalid return 0. If there are no active cuts, answer is `N!`. For small active count (`<=2000`), use direct O(m^2). For large cases, first try a long constant-balance suffix (CDQ prefix + polynomial-inverse suffix). Otherwise peel very long constant-balance prefixes (each at least about one third of the remaining active suffix), then run CDQ on the rest. This targets the worst benchmark shapes: alternating `BW*N`, long initial W-run followed by alternating `WB`, and random.
- **Benchmarking:** The intended benchmark cases are `BW*N`, a long initial W-run followed by alternating `WB`, and random strings at `N=200000`. The constant-balance shortcuts handle the first two with a small number of large NTTs. Random cases use the active-list CDQ; active cuts are a proper subset of all cuts, and direct summation handles small/sparse nodes. The constants `BASE=64`, `DIRECT_ADD=8192`, `DIRECT_FORCE=50000`, and `RUN_MIN=2048` were chosen to balance Python direct-loop cost against NTT overhead.
- **Pitfalls:** `R` is indexed by black count `t=1..N-1`; `R[0]` is unused. Factorial indices are nonnegative for active cuts. The answer sum uses `fact[N-t]`, not `fact[N-r]`. In the constant-balance shortcut, the forcing term includes the artificial `H[0]` term, so for a run starting at `a` with balance `c`, `T_n` starts from `fact[a+c+n]` when only `H[0]` is present.

### NEXT
- Run the actual timed benchmark on `N=200000` for alternating, long-initial-W-run, and random strings.
- If random CDQ is still too slow, tune `BASE`, `DIRECT_ADD`, and NTT direct thresholds, or add a block-online convolution variant.
- Optionally add a small self-test comparing brute force for `N<=5` and random small cases.

### STATUS
continue
