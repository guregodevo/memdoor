### CODE
```python
import sys

MOD = 998244353
G = 3
B = 66  # ord('B')
W = 87  # ord('W')

ROOTS = [None, [1]]
IROOTS = [None, [1]]
INV_LEN = [1, (MOD + 1) // 2]
MAXLOG = 1
INV2 = (MOD + 1) // 2


def ensure_roots(length):
    global MAXLOG, ROOTS, IROOTS, INV_LEN
    target = length.bit_length() - 1
    while MAXLOG < target:
        MAXLOG += 1
        half = 1 << (MAXLOG - 1)
        wlen = pow(G, (MOD - 1) // (1 << MAXLOG), MOD)
        ws = [1] * half
        for i in range(1, half):
            ws[i] = ws[i - 1] * wlen % MOD
        ROOTS.append(ws)

        iwlen = pow(wlen, MOD - 2, MOD)
        iws = [1] * half
        for i in range(1, half):
            iws[i] = iws[i - 1] * iwlen % MOD
        IROOTS.append(iws)

        INV_LEN.append(INV_LEN[-1] * INV2 % MOD)


def ntt(a, invert, scale=True):
    n = len(a)
    mod = MOD

    if not invert:
        length = n
        lg = n.bit_length() - 1
        roots = ROOTS
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
                ws = roots[lg]
                for i in range(0, n, length):
                    j = i
                    h = i + half
                    for k in range(half):
                        u = a[j]
                        v = a[h]
                        x = u + v
                        if x >= mod:
                            x -= mod
                        y = u - v
                        if y < 0:
                            y += mod
                        a[j] = x
                        a[h] = y * ws[k] % mod
                        j += 1
                        h += 1
            length = half
            lg -= 1
    else:
        length = 2
        lg = 1
        iroots = IROOTS
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
                ws = iroots[lg]
                for i in range(0, n, length):
                    j = i
                    h = i + half
                    for k in range(half):
                        u = a[j]
                        v = a[h] * ws[k] % mod
                        x = u + v
                        if x >= mod:
                            x -= mod
                        y = u - v
                        if y < 0:
                            y += mod
                        a[j] = x
                        a[h] = y
                        j += 1
                        h += 1
            length <<= 1
            lg += 1

        if scale:
            inv_n = INV_LEN[lg - 1]
            for i in range(n):
                a[i] = a[i] * inv_n % mod


def poly_inv(P, n):
    mod = MOD
    h = [1]
    m = 1
    while m < n:
        m2 = m << 1
        if m2 > n:
            m2 = n

        # Direct extension while very small.
        if m < 128:
            h += [0] * (m2 - m)
            for k in range(m, m2):
                s = 0
                for i in range(1, k + 1):
                    s += P[i] * h[k - i]
                h[k] = (-s) % mod
            m = m2
            continue

        L = m << 1
        ensure_roots(L)
        logL = L.bit_length() - 1

        f = [0] * L
        f[:m2] = P[:m2]
        hpad = [0] * L
        hpad[:m] = h

        ntt(hpad, False)
        ntt(f, False)
        f = [x * y % mod for x, y in zip(f, hpad)]
        ntt(f, True, False)

        inv_n = INV_LEN[logL]
        q_len = m2 - m
        q = [0] * q_len
        base = m
        for i in range(q_len):
            q[i] = f[base + i] * inv_n % mod
        del f

        qpad = [0] * L
        qpad[:q_len] = q
        ntt(qpad, False)
        qpad = [x * y % mod for x, y in zip(qpad, hpad)]
        ntt(qpad, True, False)

        r = [0] * q_len
        for i in range(q_len):
            r[i] = qpad[i] * inv_n % mod
        del qpad

        h += [mod - x if x else 0 for x in r]
        m = m2

    return h


def alternating_direct(N, fact):
    mod = MOD
    if N <= 1:
        return fact[N]
    g = [0] * (N - 1)
    for i in range(N - 1):
        s = 0
        for j in range(i):
            s += g[j] * fact[i - j]
        val = fact[i + 1] + (s % mod)
        if val >= mod:
            val -= mod
        g[i] = mod - val if val else 0

    ans = fact[N]
    for i in range(N - 1):
        ans = (ans + g[i] * fact[N - 1 - i]) % mod
    return ans


def alternating_fast(N, fact):
    if N <= 2000:
        return alternating_direct(N, fact)

    P = [1] + fact[1:]
    H = poly_inv(P, N + 1)

    ans = 0
    mod = MOD
    for k in range(1, N + 1):
        ans = (ans + fact[k] * H[N - k]) % mod
    return ans


def parse_events(S, N):
    M = 2 * N
    i = 0
    cum_b = 0
    cum_w = 0
    b_list = []
    m_list = []

    while i < M:
        if S[i] == B:
            j = i + 1
            while j < M and S[j] == B:
                j += 1
            cum_b += j - i
            i = j
        else:
            j = i + 1
            while j < M and S[j] == W:
                j += 1
            cum_w += j - i
            i = j
            if i < M and cum_w >= cum_b:
                b_list.append(cum_w)
                m_list.append(cum_b)

    return b_list, m_list


def direct_dp_small(t, b, m, fact, invfact, N):
    mod = MOD
    g = [0] * t
    for i in range(t):
        bi = b[i]
        s = 0
        for j in range(i):
            s += g[j] * fact[bi - m[j]]
        val = fact[bi] + (s % mod)
        if val >= mod:
            val -= mod
        g[i] = (mod - val) * invfact[bi - m[i]] % mod

    ans = fact[N]
    for i in range(t):
        ans = (ans + g[i] * fact[N - m[i]]) % mod
    return ans


def cdq_solve(t, b, m, fact, invfact, N):
    mod = MOD
    g = [0] * t
    sacc = [0] * t

    BLOCK = 32
    DIRECT_LIMIT = 8192
    NTT_FACTOR = 4

    sys.setrecursionlimit(1_000_000)

    def cdq(l, r, b=b, m=m, g=g, sacc=sacc, fact=fact, invfact=invfact, mod=mod):
        if r - l + 1 <= BLOCK:
            for i in range(l, r + 1):
                bi = b[i]
                add = 0
                for j in range(l, i):
                    add += g[j] * fact[bi - m[j]]
                s = (sacc[i] + add) % mod
                val = fact[bi] + s
                if val >= mod:
                    val -= mod
                g[i] = (mod - val) * invfact[bi - m[i]] % mod
            return

        mid = (l + r) >> 1
        cdq(l, mid)

        ls = mid - l + 1
        rs = r - mid
        direct_cost = ls * rs

        m_l = m[l]
        m_mid = m[mid]
        b_mid1 = b[mid + 1]
        b_r = b[r]

        shift = b_mid1 - m_mid
        wf = m_mid - m_l
        wg = b_r - m_l - shift
        total = wf + wg + 1
        L = 1 << (total - 1).bit_length()
        logL = L.bit_length() - 1

        if direct_cost <= DIRECT_LIMIT or direct_cost <= NTT_FACTOR * L * logL:
            fl = fact
            ml = m
            gl = g
            sl = sacc
            bl = b
            for i in range(mid + 1, r + 1):
                bi = bl[i]
                s = 0
                for j in range(l, mid + 1):
                    s += gl[j] * fl[bi - ml[j]]
                sl[i] = (sl[i] + s) % mod
        else:
            ensure_roots(L)

            F = [0] * L
            ml = m
            gl = g
            for j in range(l, mid + 1):
                F[ml[j] - m_l] = gl[j]

            Ga = [0] * L
            Ga[:wg + 1] = fact[shift:shift + wg + 1]

            ntt(F, False)
            ntt(Ga, False)
            F = [x * y % mod for x, y in zip(F, Ga)]
            del Ga
            ntt(F, True, False)

            inv_n = INV_LEN[logL]
            base = -m_l - shift
            sl = sacc
            bl = b
            for i in range(mid + 1, r + 1):
                idx = bl[i] + base
                v = sl[i] + F[idx] * inv_n % mod
                if v >= mod:
                    v -= mod
                sl[i] = v

        cdq(mid + 1, r)

    cdq(0, t - 1)

    ans = fact[N]
    for i in range(t):
        ans = (ans + g[i] * fact[N - m[i]]) % mod
    return ans


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    N = int(data[0])
    S = data[1]

    if S[0] == W or S[-1] == B:
        print(0)
        return

    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD

    if S == b'BW' * N:
        print(alternating_fast(N, fact))
        return

    b_list, m_list = parse_events(S, N)
    t = len(b_list)

    if t == 0:
        print(fact[N])
        return

    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    if t * (t - 1) // 2 <= 2_000_000:
        ans = direct_dp_small(t, b_list, m_list, fact, invfact, N)
    else:
        ans = cdq_solve(t, b_list, m_list, fact, invfact, N)

    print(ans)


if __name__ == "__main__":
    main()
```

### NOTES
- **Reduction (verified):** Strong connectivity of the path graph 1→2→…→2N plus N white→black matching edges is equivalent to: (a) S[0] is not W (else vertex 1 has no incoming edge), (b) S[-1] is not B (else final prefix is impossible), and (c) for every prefix ending just before a black vertex, at least one added edge enters it.
- **Prefix condition:** For prefix {1..k} with B(k) blacks and W(k) whites, it is closed iff the first B(k) blacks are matched to whites among the first W(k) whites, i.e. max_{j≤B(k)} p_j ≤ W(k). It is automatically safe when W(k) < B(k). It suffices to check only prefixes ending just before black k+1 (k=1..N−1).
- **Events from blocks:** Parse the string into B/W runs. For each internal W-run (followed by a B-run), let b = cumulative whites after the run and m = cumulative blacks before the run. If b ≥ m, keep event (b,m). For each distinct b, the first such m dominates later k with the same b. Events have strictly increasing b and m.
- **Counting via inclusion-exclusion:** Forbidden event E_i: p_1..p_{m_i} ⊆ {1..b_i}. For a chain i_1<…<i_k, the number of permutations where all occur is ∏ (b_{i_j}−m_{i_{j−1}})!/(b_{i_j}−m_{i_j})! · (N−m_{i_k})!, with m_{i_0}=0. DP over chains:
  g[i] = −(fact[b_i] + Σ_{j<i} g[j]·fact[b_i−m_j]) / fact[b_i−m_i]
  answer = fact[N] + Σ_i g[i]·fact[N−m_i].
- **Hard part = online convolution:** S_i = Σ_{j<i} g[j]·fact[b_i−m_j]. This is a relaxed convolution of sparse updates at indices m_j with the factorial kernel, queried at b_i. Solved with CDQ divide-and-conquer: recurse left, add left→right contributions, recurse right.
- **CDQ NTT shift:** At node [l,r], mid: F[k]=g[j] at k=m_j−m_l for j∈[l,mid]; Ga[k]=fact[k+shift] with shift=b_{mid+1}−m_mid and WG=b_r−m_l−shift. Then (F*Ga)[b_i−m_l−shift] = Σ_{j∈[l,mid]} g[j]·fact[b_i−m_j]. NTT length is next power of two of WF+WG+1.
- **NTT implementation:** DIF forward / DIT inverse without bit-reversal. Twiddle factors for each stage are precomputed dynamically up to the largest length actually needed. Inverse NTT can skip global scaling; selected coefficients are scaled by inv_n when extracted, saving O(L) passes.
- **Tuning:** Small segments are computed directly. CDQ leaf blocks of size 32 avoid deep recursion and handle intra-block contributions directly. Internal direct/NTT choice uses both a fixed direct limit and an estimate comparing direct_cost with L·logL. Small polynomial inverse steps are done directly.
- **Alternating fast path:** For S = "BW" * N, events are (i,i), i=1..N−1. The answer is the number of indecomposable permutations of size N. Its ordinary generating function A(x) satisfies A = F/(1+F), where F(x)=Σ_{n≥1} n! x^n. For large N, compute H=1/(1+F) by Newton iteration using optimized NTT, then answer is [x^N] F·H. For small N, use direct O(N^2) recurrence.
- **Complexity:** General case is CDQ online convolution, near O(N log^2 N) in the worst dense case, but the maximum dense case is handled by the alternating fast path in O(N log N). Memory is dominated by factorial arrays, event arrays, and dynamically built NTT twiddle tables.
- **Edge cases handled:** N=1 ("BW"→1, "WB"→0); no events→fact[N]; S[0]=='W' or S[-1]=='B'→0; b_i≥m_i guaranteed for kept events; all factorial indices stay in [0,N].

### NEXT
- none

### STATUS
solved
