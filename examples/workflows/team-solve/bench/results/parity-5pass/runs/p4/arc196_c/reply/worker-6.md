### CODE
```python
import sys

MOD = 998244353
G = 3


def solve():
    input = sys.stdin.readline
    line = input().strip()
    if not line:
        return
    N = int(line)
    S = input().strip()

    # Necessary endpoint conditions for strong connectivity.
    if S[0] != 'B' or S[-1] != 'W':
        print(0)
        return

    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD

    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    # R[c] = number of white vertices before black vertex c+1, for c=1..N-1.
    # R[N] = N-1 (the last cut before the final white vertex).
    R = [0] * (N + 1)
    w = 0
    b = 0
    for ch in S:
        if ch == 'W':
            w += 1
        else:
            b += 1
            if b >= 2:
                R[b - 1] = w
    R[N] = N - 1

    # For each distinct value r of R, only the first c with R[c]=r matters.
    # The bad event is: first c black vertices are matched only to first r white vertices.
    c_arr = []
    r_arr = []
    prev = -1
    for c in range(1, N + 1):
        r = R[c]
        if r > prev:
            if r >= c:
                c_arr.append(c)
                r_arr.append(r)
            prev = r

    del R, S

    M = len(c_arr)
    ans = fact[N]
    if M == 0:
        print(ans % MOD)
        return

    mod = MOD
    f = fact
    inv = invfact
    c = c_arr
    r = r_arr

    # Small case: direct O(M^2) is faster and avoids NTT overhead.
    if M <= 4000:
        A = [0] * M
        for i in range(M):
            ri = r[i]
            s = 0
            for j in range(i):
                s += A[j] * f[ri - c[j]]
            s %= mod
            val = f[ri] + s
            if val >= mod:
                val -= mod
            A[i] = (mod - val) * inv[ri - c[i]] % mod

        ans = f[N]
        for i in range(M):
            ans = (ans + A[i] * f[N - c[i]]) % mod
        print(ans)
        return

    # NTT setup.  The largest needed convolution length is < 2N.
    max_n = 1
    while max_n < 2 * N + 5:
        max_n <<= 1
    max_log = max_n.bit_length() - 1

    root = [0] * (max_log + 1)
    iroot = [0] * (max_log + 1)
    inv_len = [1] * (max_log + 1)
    inv2 = (mod + 1) // 2
    for k in range(1, max_log + 1):
        length = 1 << k
        root[k] = pow(G, (mod - 1) // length, mod)
        iroot[k] = pow(root[k], mod - 2, mod)
        inv_len[k] = inv_len[k - 1] * inv2 % mod

    # DIF forward NTT: natural input, bit-reversed output.
    def ntt(a):
        n = len(a)
        if n == 1:
            return
        length = n
        k = n.bit_length() - 1
        while length > 1:
            half = length >> 1
            wlen = root[k]
            for i in range(0, n, length):
                w = 1
                j = i
                jj = i + half
                end = i + half
                while j < end:
                    u = a[j]
                    v = a[jj]
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[j] = x
                    a[jj] = y * w % mod
                    w = w * wlen % mod
                    j += 1
                    jj += 1
            length = half
            k -= 1

    # DIT inverse NTT: bit-reversed input, natural output.
    def intt(a):
        n = len(a)
        if n == 1:
            return
        length = 2
        k = 1
        while length <= n:
            half = length >> 1
            wlen = iroot[k]
            for i in range(0, n, length):
                w = 1
                j = i
                jj = i + half
                end = i + half
                while j < end:
                    u = a[j]
                    v = a[jj] * w % mod
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[j] = x
                    a[jj] = y
                    w = w * wlen % mod
                    j += 1
                    jj += 1
            length <<= 1
            k += 1
        inv_n = inv_len[n.bit_length() - 1]
        for i in range(n):
            a[i] = a[i] * inv_n % mod

    A = [0] * M
    query = [0] * M

    T = 128
    DIRECT = 16384
    sys.setrecursionlimit(1_000_000)

    def add_convolution(l, mid, rr):
        # Add sum_{j in [l,mid)} A[j] * fact[r[i]-c[j]] to query[i] for i in [mid,rr).
        c_l = c[l]
        c_mid = c[mid - 1]
        Clen = c_mid - c_l + 1

        min_diff = r[mid] - c_mid
        max_diff = r[rr - 1] - c_l
        Flen = max_diff - min_diff + 1

        need = Clen + Flen - 1
        n = 1 << (need - 1).bit_length()

        fa = [0] * n
        for j in range(l, mid):
            fa[c[j] - c_l] = A[j]

        fb = [0] * n
        fb[:Flen] = f[min_diff:min_diff + Flen]

        ntt(fa)
        ntt(fb)
        for i in range(n):
            fa[i] = fa[i] * fb[i] % mod
        del fb
        intt(fa)

        base = -c_l - min_diff
        for i in range(mid, rr):
            idx = r[i] + base
            val = query[i] + fa[idx]
            if val >= mod:
                val -= mod
            query[i] = val

    def cdq(l, rr):
        if rr - l <= T:
            for i in range(l, rr):
                ri = r[i]
                s = query[i]
                for j in range(l, i):
                    s += A[j] * f[ri - c[j]]
                s %= mod
                val = f[ri] + s
                if val >= mod:
                    val -= mod
                A[i] = (mod - val) * inv[ri - c[i]] % mod
            return

        mid = (l + rr) >> 1
        cdq(l, mid)

        left_len = mid - l
        right_len = rr - mid
        if left_len * right_len <= DIRECT:
            for i in range(mid, rr):
                ri = r[i]
                s = query[i]
                for j in range(l, mid):
                    s += A[j] * f[ri - c[j]]
                s %= mod
                query[i] = s
        else:
            add_convolution(l, mid, rr)

        cdq(mid, rr)

    cdq(0, M)

    ans = f[N]
    for i in range(M):
        ans = (ans + A[i] * f[N - c[i]]) % mod
    print(ans)


if __name__ == "__main__":
    solve()
```

### NOTES
- **Sample verification:** The program produces `1` for sample 1 (`N=2`, `BWBW`), `0` for sample 2 (`N=4`, `BWWBWBWB`, last character is `B`), and `240792` for sample 3 (`N=9`, `BWWBWBBBWWBWBBWWBW`).
- **Strong connectivity criterion:** With the base path `1 -> 2 -> ... -> 2N`, the graph is strongly connected iff for every cut `k`, there is at least one added edge from a vertex `> k` to a vertex `<= k`. Since added edges go only from white to black, this means for every prefix with `c` black vertices and `w` white vertices, at least one of the first `c` black vertices is matched to a white vertex after the prefix.
- **Reduction to events:** Let `R[c]` be the number of white vertices before black `c+1`, and set `R[N]=N-1`. For the `c` black vertices, the hardest cut in that block has `R[c]` whites. The bad event is `E_c`: the first `c` black vertices are matched only to the first `R[c]` white vertices. If `R[c]` is constant over a run of `c`, only the first `c` in that run matters, because later events imply it. Nonempty events require `R[c] >= c`.
- **Inclusion-exclusion DP:** Let the kept events be indexed by increasing `c_i, r_i`. For a subset whose maximum kept index is `i`, the number of matchings satisfying that subset is `fact[N-c_i] * product fact[r_j - c_prev] / fact[r_j - c_j]`. Define `A[i]` as the signed inclusion-exclusion sum for subsets ending at `i`. Then
  `A[i] = -(fact[r_i] + sum_{j<i} A[j] * fact[r_i - c_j]) / fact[r_i - c_i]`.
  The answer is `fact[N] + sum_i A[i] * fact[N - c_i]`.
- **CDQ/NTT computation:** The recurrence is triangular but the query sum is a shifted factorial convolution. CDQ divide-and-conquer is used: after the left half is solved, contributions from left indices to right indices are added. For a node, left terms are placed by their actual `c` coordinate, and the factorial segment is taken over the needed difference range `[min_diff, max_diff]`; one convolution gives all right-side query additions.
- **Implementation details:** Endpoint check `S[0]=='B'` and `S[-1]=='W'` is necessary. Small `M` uses direct O(M^2). For larger `M`, NTT is used with DIF forward and DIT inverse transforms, avoiding explicit bit-reversal. The maximum NTT size is bounded by `2N+5`.
- **Status:** Samples are verified; the algorithm is complete for the stated constraints, but large stress/performance testing is still the natural next step.

### NEXT
- Run large random stress tests against a brute force for small `N`.
- Benchmark worst-case strings such as alternating `BWBW...` and block strings.
- Tune `T`, `DIRECT`, and NTT constants if performance is insufficient.

### STATUS
solved
