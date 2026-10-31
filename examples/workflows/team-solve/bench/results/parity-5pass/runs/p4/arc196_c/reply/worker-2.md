### CODE
```python
import sys
import random
import itertools

sys.setrecursionlimit(1 << 20)

MOD = 998244353
G = 3


def precompute_fact(n):
    fact = [1] * (n + 1)
    for i in range(1, n + 1):
        fact[i] = fact[i - 1] * i % MOD
    invfact = [1] * (n + 1)
    invfact[n] = pow(fact[n], MOD - 2, MOD)
    for i in range(n, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD
    return fact, invfact


def parse_D(N, S):
    """D[k] = number of black vertices before white k (1-indexed whites)."""
    D = [0] * (N + 1)
    b = 0
    w = 0
    for ch in S:
        if ch == 'B':
            b += 1
        else:
            w += 1
            D[w] = b
    return D


def parse_R(N, S):
    """R[t] = number of white vertices before black (t+1), for t=1..N-1."""
    R = [0] * N
    b = 0
    w = 0
    for ch in S:
        if ch == 'W':
            w += 1
        else:
            b += 1
            if b >= 2:
                R[b - 1] = w
    return R


def brute_force_count(N, S):
    """
    Enumerate all perfect white->black pairings and simulate reachability
    from the last vertex to the first vertex.  With the original path edges,
    this is equivalent to strong connectivity.
    """
    if N == 1:
        return 1 if S == "BW" else 0
    if S[0] != 'B' or S[-1] != 'W':
        return 0

    whites = []
    blacks = []
    for i, ch in enumerate(S):
        if ch == 'W':
            whites.append(i)
        else:
            blacks.append(i)

    last = 2 * N - 1
    ans = 0

    for perm in itertools.permutations(range(N)):
        m = last
        while True:
            nm = m
            for i, wpos in enumerate(whites):
                if wpos >= m:
                    bpos = blacks[perm[i]]
                    if bpos < nm:
                        nm = bpos
            if nm == m:
                break
            m = nm
            if m == 0:
                break
        if m == 0:
            ans += 1
    return ans % MOD


def dp_open_arc_naive(N, S, fact=None, invfact=None):
    """
    Baseline O(N^2) open-arc DP.

    S_j[m] = number of ways to assign whites j..N to distinct black ranks
             all >= m, while satisfying all cut constraints that are already
             forced.  Answer is S_2[1].
    """
    if N == 1:
        return 1 if S == "BW" else 0
    if S[0] != 'B' or S[-1] != 'W':
        return 0

    if fact is None or invfact is None or len(fact) < N + 1:
        fact, invfact = precompute_fact(N)

    D = parse_D(N, S)
    dN = D[N - 1]
    if dN <= 0:
        return 0

    dp = [0] * (dN + 1)
    for m in range(1, dN + 1):
        dp[m] = (dN - m + 1) % MOD

    for j in range(N - 1, 1, -1):
        d = D[j - 1]
        newD = d if d < j else j
        if newD <= 0:
            return 0

        C = 0
        if d < j and d + 1 < len(dp):
            C = (j - d) * dp[d + 1] % MOD

        newdp = [0] * (newD + 1)
        for m in range(1, newD + 1):
            newdp[m] = ((j - m + 1) * dp[m] - C) % MOD
        dp = newdp

    return dp[1] % MOD if len(dp) >= 2 else 0


def dp_open_arc_count(N, S, fact=None, invfact=None):
    """
    Same open-arc DP, but zero-subtraction runs are compressed using
    precomputed factorials.  Still worst-case O(N^2), but fast for medium N.
    """
    if N == 1:
        return 1 if S == "BW" else 0
    if S[0] != 'B' or S[-1] != 'W':
        return 0

    if fact is None or invfact is None or len(fact) < N + 1:
        fact, invfact = precompute_fact(N)

    D = parse_D(N, S)
    d = [0] * (N + 1)
    for r in range(2, N + 1):
        d[r] = D[r - 1]

    dN = d[N]
    if dN <= 0:
        return 0

    dp = [0] * (dN + 1)
    for m in range(1, dN + 1):
        dp[m] = (dN - m + 1) % MOD

    events = []
    for r in range(N - 1, 1, -1):
        if d[r] < r and d[r] < d[r + 1]:
            events.append(r)

    current_r = N - 1

    for r_e in events:
        if r_e > current_r:
            continue

        # Zero-subtraction run: steps current_r, current_r-1, ..., r_e+1.
        if current_r >= r_e + 1:
            D_end = d[r_e + 1] if d[r_e + 1] < r_e + 1 else r_e + 1
            if D_end <= 0:
                return 0
            for m in range(1, D_end + 1):
                dp[m] = dp[m] * fact[current_r - m + 1] % MOD
                dp[m] = dp[m] * invfact[r_e + 1 - m] % MOD
            if len(dp) > D_end + 1:
                dp = dp[:D_end + 1]
        else:
            D_end = d[r_e + 1] if d[r_e + 1] < r_e + 1 else r_e + 1
            if len(dp) > D_end + 1:
                dp = dp[:D_end + 1]

        # Event step r_e: deadline drops, so a boundary term appears.
        dval = d[r_e]
        if dval <= 0:
            return 0

        C = 0
        if dval + 1 < len(dp):
            C = (r_e - dval) * dp[dval + 1] % MOD

        for m in range(1, dval + 1):
            dp[m] = ((r_e - m + 1) * dp[m] - C) % MOD

        if len(dp) > dval + 1:
            dp = dp[:dval + 1]

        current_r = r_e - 1

    # Final zero-subtraction run down to step 2.
    if current_r >= 2:
        D_end = d[2] if d[2] < 2 else 2
        if D_end <= 0:
            return 0
        for m in range(1, D_end + 1):
            dp[m] = dp[m] * fact[current_r - m + 1] % MOD
            dp[m] = dp[m] * invfact[2 - m] % MOD
        if len(dp) > D_end + 1:
            dp = dp[:D_end + 1]

    return dp[1] % MOD if len(dp) >= 2 else 0


def dp_inclusion_naive(N, S, fact=None, invfact=None):
    """
    O(N^2) inclusion-exclusion DP.
    H[0] = 1.
    H[t] = - invfact[R[t]-t] * sum_{s<t} H[s] * fact[R[t]-s]
           if R[t] >= t, else 0.
    Answer = N! + sum_{t=1}^{N-1} H[t] * (N-t)!.
    """
    if N == 1:
        return 1 if S == "BW" else 0
    if S[0] != 'B' or S[-1] != 'W':
        return 0

    if fact is None or invfact is None or len(fact) < N + 1:
        fact, invfact = precompute_fact(N)

    R = parse_R(N, S)
    H = [0] * N
    H[0] = 1

    for t in range(1, N):
        rt = R[t]
        if rt < t:
            H[t] = 0
            continue
        s = 0
        for prev in range(t):
            hv = H[prev]
            if hv:
                s += hv * fact[rt - prev]
        H[t] = (-s % MOD) * invfact[rt - t] % MOD

    ans = fact[N]
    for t in range(1, N):
        if H[t]:
            ans = (ans + H[t] * fact[N - t]) % MOD
    return ans % MOD


# ---------------- NTT / CDQ optimized solver ----------------

_roots_cache = {}
_inv_n_cache = {}
_rev_cache = {}


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


def _get_rev(n):
    rev = _rev_cache.get(n)
    if rev is None:
        if n == 1:
            rev = [0]
        else:
            logn = n.bit_length() - 1
            rev = [0] * n
            for i in range(1, n):
                rev[i] = (rev[i >> 1] >> 1) | ((i & 1) << (logn - 1))
        _rev_cache[n] = rev
    return rev


def _ntt_inv_n(n):
    v = _inv_n_cache.get(n)
    if v is None:
        v = pow(n, MOD - 2, MOD)
        _inv_n_cache[n] = v
    return v


def ntt(a, invert):
    n = len(a)
    if n == 1:
        return

    rev = _get_rev(n)
    for i in range(n):
        j = rev[i]
        if i < j:
            a[i], a[j] = a[j], a[i]

    length = 2
    while length <= n:
        half = length >> 1
        roots = _get_roots(length, invert)
        for i in range(0, n, length):
            k = 0
            end = i + half
            for j in range(i, end):
                u = a[j]
                v = a[j + half] * roots[k] % MOD
                x = u + v
                if x >= MOD:
                    x -= MOD
                y = u - v
                if y < 0:
                    y += MOD
                a[j] = x
                a[j + half] = y
                k += 1
        length <<= 1

    if invert:
        inv_n = _ntt_inv_n(n)
        for i in range(n):
            a[i] = a[i] * inv_n % MOD


def convolution(a, b):
    if not a or not b:
        return []

    la, lb = len(a), len(b)
    need = la + lb - 1

    # Small cases: plain multiplication.
    if min(la, lb) <= 8 or la * lb <= 20000:
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

    ntt(fa, False)
    ntt(fb, False)
    for i in range(n):
        fa[i] = fa[i] * fb[i] % MOD
    ntt(fa, True)

    return fa[:need]


def poly_inv(p, n):
    """Inverse of polynomial p modulo x^n, assuming p[0] != 0."""
    q = [pow(p[0], MOD - 2, MOD)]
    m = 1
    while m < n:
        m2 = m << 1
        if m2 > n:
            m2 = n

        t = convolution(p[:m2], q)
        if len(t) < m2:
            t += [0] * (m2 - len(t))
        t = t[:m2]

        u = [0] * m2
        u[0] = (2 - t[0]) % MOD
        for i in range(1, m2):
            u[i] = (-t[i]) % MOD

        q = convolution(q, u)
        if len(q) < m2:
            q += [0] * (m2 - len(q))
        q = q[:m2]

        m = m2

    return q


def solve_diagonal_ntt(N, fact, invfact):
    """
    Special case R[t] = t for all t = 1..N-1 (alternating BWBW...).
    Then H satisfies H = 1 / (1 + F), where F[k] = k! for k >= 1.
    """
    P = [1] + fact[1:N]
    Q = poly_inv(P, N)

    ans = fact[N]
    for t in range(1, N):
        if Q[t]:
            ans = (ans + Q[t] * fact[N - t]) % MOD
    return ans % MOD


def solve_cdq_ntt(N, S, fact=None, invfact=None, base=32, naive_thresh=8192, R=None):
    """
    CDQ divide-and-conquer + NTT for the inclusion-exclusion recurrence.
    """
    if N == 1:
        return 1 if S == "BW" else 0
    if S[0] != 'B' or S[-1] != 'W':
        return 0

    if fact is None or invfact is None or len(fact) < N + 1:
        fact, invfact = precompute_fact(N)

    if R is None:
        R = parse_R(N, S)

    good = [False] * N
    any_good = False
    for t in range(1, N):
        if R[t] >= t:
            good[t] = True
            any_good = True

    if not any_good:
        return fact[N]

    H = [0] * N
    contrib = [0] * N
    H[0] = 1

    def add_contrib(l, mid, r):
        # Add contributions from s in [l, mid] to t in [mid+1, r].
        needed = []
        k_min = 10**30
        k_max = -1

        for t in range(mid + 1, r + 1):
            if good[t]:
                needed.append(t)
                k = R[t] - l
                if k < k_min:
                    k_min = k
                if k > k_max:
                    k_max = k

        if not needed:
            return

        first = l
        while first <= mid and H[first] == 0:
            first += 1
        if first > mid:
            return

        last = mid
        while last >= first and H[last] == 0:
            last -= 1

        offset = first
        L = last - first + 1
        left = H[first:last + 1]

        # Shift k values from offset l to offset 'first'.
        shift = l - offset
        k_min += shift
        k_max += shift

        if L * len(needed) <= naive_thresh:
            for t in needed:
                k = R[t] - offset
                total = 0
                for i, hv in enumerate(left):
                    if hv:
                        idx = k - i
                        if idx >= 0:
                            total += hv * fact[idx]
                contrib[t] = (contrib[t] + total) % MOD
            return

        j0 = k_min - L + 1
        if j0 < 0:
            j0 = 0

        B = fact[j0:k_max + 1]
        C = convolution(left, B)

        for t in needed:
            k = R[t] - offset
            idx = k - j0
            if 0 <= idx < len(C):
                contrib[t] = (contrib[t] + C[idx]) % MOD

    def cdq(l, r):
        if r - l + 1 <= base:
            for t in range(l, r + 1):
                if t == 0:
                    H[0] = 1
                    continue
                if not good[t]:
                    H[t] = 0
                    continue

                s = contrib[t]
                rt = R[t]
                for prev in range(l, t):
                    hv = H[prev]
                    if hv:
                        idx = rt - prev
                        if idx >= 0:
                            s += hv * fact[idx]

                H[t] = (-s % MOD) * invfact[rt - t] % MOD
            return

        mid = (l + r) // 2
        cdq(l, mid)
        add_contrib(l, mid, r)
        cdq(mid + 1, r)

    cdq(0, N - 1)

    ans = fact[N]
    for t in range(1, N):
        if H[t]:
            ans = (ans + H[t] * fact[N - t]) % MOD
    return ans % MOD


def solve_problem(N, S):
    if N == 1:
        return 1 if S == "BW" else 0

    if S[0] != 'B' or S[-1] != 'W':
        return 0

    fact, invfact = precompute_fact(N)
    R = parse_R(N, S)

    any_good = False
    for t in range(1, N):
        if R[t] >= t:
            any_good = True
            break

    if not any_good:
        return fact[N]

    DP_LIMIT = 2000
    if N <= DP_LIMIT:
        return dp_open_arc_count(N, S, fact, invfact)

    # Very common worst case for CDQ: R[t] = t for all t.
    diagonal = True
    for t in range(1, N):
        if R[t] != t:
            diagonal = False
            break
    if diagonal:
        return solve_diagonal_ntt(N, fact, invfact)

    return solve_cdq_ntt(N, S, fact, invfact, base=32, naive_thresh=8192, R=R)


def self_test():
    max_fact = 1200
    fact, invfact = precompute_fact(max_fact)
    checked = 0

    def check(N, S, cdq_base=4, cdq_thresh=0, use_brute=False, use_inclusion=True, use_diag=True):
        nonlocal checked
        results = []

        if use_brute:
            results.append(('brute', brute_force_count(N, S)))

        results.append(('dp_naive', dp_open_arc_naive(N, S, fact, invfact)))
        results.append(('dp_comp', dp_open_arc_count(N, S, fact, invfact)))

        if use_inclusion:
            results.append(('incl', dp_inclusion_naive(N, S, fact, invfact)))

        results.append(('cdq', solve_cdq_ntt(N, S, fact, invfact,
                                             base=cdq_base, naive_thresh=cdq_thresh)))

        if use_diag and N > 1 and S[0] == 'B' and S[-1] == 'W':
            R = parse_R(N, S)
            diag = True
            for t in range(1, N):
                if R[t] != t:
                    diag = False
                    break
            if diag:
                results.append(('diag', solve_diagonal_ntt(N, fact, invfact)))

        first = results[0][1]
        for name, val in results[1:]:
            if val != first:
                raise AssertionError(f"Mismatch N={N} S={S}: {results}")

        checked += 1

    # Exhaustive small cases.
    for N in range(1, 6):
        for bpos in itertools.combinations(range(2 * N), N):
            arr = ['W'] * (2 * N)
            for p in bpos:
                arr[p] = 'B'
            S = ''.join(arr)
            check(N, S, use_brute=True, use_inclusion=True)

    # Random slightly larger cases with brute force.
    rng = random.Random(12345)
    for N, trials in [(6, 50), (7, 20)]:
        for _ in range(trials):
            bpos = set(rng.sample(range(2 * N), N))
            S = ''.join('B' if i in bpos else 'W' for i in range(2 * N))
            check(N, S, use_brute=True, use_inclusion=True)

    # Required random N=8..20, forcing CDQ recursion and convolution paths.
    for N in range(8, 21):
        for _ in range(20):
            bpos = set(rng.sample(range(2 * N), N))
            S = ''.join('B' if i in bpos else 'W' for i in range(2 * N))
            check(N, S, cdq_base=4, cdq_thresh=0, use_brute=False, use_inclusion=True)

    # Larger random cases to exercise NTT more seriously.
    for N in [100, 200, 300, 500, 1000]:
        for _ in range(2):
            bpos = set(rng.sample(range(2 * N), N))
            S = ''.join('B' if i in bpos else 'W' for i in range(2 * N))
            check(N, S, cdq_base=16, cdq_thresh=4096,
                  use_brute=False, use_inclusion=(N <= 1000))

    # Explicit diagonal cases to test the polynomial-inverse shortcut.
    for N in [100, 300, 500, 1000]:
        S = 'BW' * N
        check(N, S, cdq_base=16, cdq_thresh=4096,
              use_brute=False, use_inclusion=(N <= 300))

    print(f"selftest ok: {checked} cases")


def main():
    if "--selftest" in sys.argv:
        self_test()
        return

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
- **Strong connectivity reduction:** The original path edges make forward reachability automatic. The graph is strongly connected iff the last vertex can reach the first vertex. This is equivalent to: `S[0] == 'B'`, `S[-1] == 'W'`, and for every cut before black `t+1` (`t=1..N-1`), at least one of the first `t` black vertices is matched to a white after the cut. In permutation terms, if `q_j` is the white rank matched to black rank `j`, validity is `max(q_1..q_t) > R[t]` for all `t`, where `R[t]` is the number of whites before black `t+1`.
- **R/D conjugate form:** `D[k]` is the number of black vertices before white `k`. The open-arc deadline after assigning whites `j..N` is `d_j = D[j-1]` (capped at `N`). The state support is `m <= min(d_j, j)`.
- **Open-arc DP:** `S_j[m]` counts assignments of whites `j..N` to distinct black ranks all `>= m`, satisfying already forced cuts. Initial state is `S_N[m] = D[N-1] - m + 1`. Transition is `S_j[m] = (j-m+1) S_{j+1}[m] - (j-d_j) S_{j+1}[d_j+1]` when `d_j < j`, with the second term absent when it is zero. Answer is `S_2[1]`. The compressed version batches consecutive zero-subtraction steps using factorial ratios: product of `(r-m+1)` becomes `fact[current_r-m+1] * invfact[r_e+1-m]`.
- **Inclusion-exclusion DP:** For bad cuts `E_t`, the intersection weights give `H[0]=1` and `H[t] = -invfact[R[t]-t] * sum_{s<t} H[s] fact[R[t]-s]` when `R[t] >= t`, else `0`. The answer is `N! + sum_{t=1}^{N-1} H[t] (N-t)!`.
- **CDQ/NTT solver:** Computes the inclusion-exclusion recurrence online. In `add_contrib`, trim leading/trailing zero `H` values in the left half, shift query indices to the first nonzero offset, and use direct summation for small products. Otherwise convolve the left `H` slice with the needed factorial slice `fact[j0:k_max+1]`, where `j0 = k_min - L + 1`. This index mapping was checked carefully and is exercised by self-test.
- **NTT implementation:** Standard DIT NTT with cached bit-reversal arrays and cached root powers for each length/invert flag. Convolution uses plain multiplication for very small inputs and NTT otherwise. Modulus is `998244353`, primitive root `3`.
- **Diagonal shortcut:** If `R[t] = t` for all `t=1..N-1` (the alternating `BWBW...` case), the recurrence becomes `H = 1 / (1 + F)` with `F[k] = k!`. This is solved by Newton polynomial inversion using the same convolution routine. It is used for large `N` to avoid the general CDQ cost in this common worst case.
- **Solver selection:** Invalid endpoints return `0`. If no `R[t] >= t`, every cut is automatically crossed and the answer is `N!`. For `N <= 2000`, use the compressed open-arc DP. For larger `N`, use the diagonal polynomial-inverse shortcut when applicable, otherwise CDQ/NTT.
- **Self-test coverage:** Exhaustive all strings up to `N=5` with brute force; random `N=6,7` with brute force; required random `N=8..20` comparing compressed DP, inclusion DP, and CDQ with small base/threshold to force CDQ logic; larger random `N=100..1000` and explicit diagonal cases to exercise NTT and the polynomial-inverse shortcut.
- **Pitfalls:** `S[0]` must be `B` and `S[-1]` must be `W`; `R` has length `N` with `R[0]` unused; initial open-arc state uses `D[N-1]`, not `D[N]`; zero-run factorial division uses `invfact[r_e+1-m]`; final zero run goes down to step `2`; CDQ `good[t]` guarantees factorial indices are nonnegative; answer sum includes `t=1..N-1` only.
- **Performance notes:** Pure Python NTT is the main risk. The program uses DP for medium `N`, caches NTT data, direct multiplication for tiny convolutions, and a diagonal shortcut. General large non-diagonal cases use CDQ/NTT with `base=32` and a direct/NTT threshold.

### NEXT
- none

### STATUS
solved
