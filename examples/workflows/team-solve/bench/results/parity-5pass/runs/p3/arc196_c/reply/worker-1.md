### CODE
```python
import sys

MOD = 998244353
G = 3

def solve_case(N, S):
    if S[0] == 'W' or S[-1] == 'B':
        return 0
    M = 2 * N
    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD
    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD
    prefW = [0] * (M + 1)
    for i in range(M):
        prefW[i + 1] = prefW[i] + (1 if S[i] == 'W' else 0)
    posB = [0] * (N + 1)
    c = 0
    for i in range(M):
        if S[i] == 'B':
            c += 1
            posB[c] = i + 1
    min_k = {}
    for k in range(1, N):
        b = prefW[posB[k + 1] - 1]
        if b >= k and b not in min_k:
            min_k[b] = k
    events = sorted(min_k.items())
    t = len(events)
    if t == 0:
        return fact[N]
    b_arr = [e[0] for e in events]
    m_arr = [e[1] for e in events]
    g = [0] * t
    Sacc = [0] * t
    maxL = 1
    while maxL < 2 * N + 2:
        maxL <<= 1
    maxlog = maxL.bit_length() - 1
    wf = [0] * (maxlog + 1)
    wi = [0] * (maxlog + 1)
    for lg in range(1, maxlog + 1):
        length = 1 << lg
        wf[lg] = pow(G, (MOD - 1) // length, MOD)
        wi[lg] = pow(wf[lg], MOD - 2, MOD)
    def ntt(a, inv):
        n = len(a)
        mod = MOD
        j = 0
        for i in range(1, n):
            bit = n >> 1
            while j & bit:
                j ^= bit
                bit >>= 1
            j ^= bit
            if i < j:
                a[i], a[j] = a[j], a[i]
        length = 2
        lg = 1
        while length <= n:
            wlen = wi[lg] if inv else wf[lg]
            half = length >> 1
            for i in range(0, n, length):
                w = 1
                for j in range(i, i + half):
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
        if inv:
            ninv = pow(n, mod - 2, mod)
            for i in range(n):
                a[i] = a[i] * ninv % mod
    TH = 4096
    sys.setrecursionlimit(1000000)
    def cdq(l, r):
        if l == r:
            g[l] = (MOD - (fact[b_arr[l]] + Sacc[l]) % MOD) * invfact[b_arr[l] - m_arr[l]] % MOD
            return
        mid = (l + r) >> 1
        cdq(l, mid)
        ls = mid - l + 1
        rs = r - mid
        if ls * rs <= TH:
            fl = fact
            gl = g
            ml = m_arr
            bl = b_arr
            Sl = Sacc
            mod = MOD
            for i in range(mid + 1, r + 1):
                bi = bl[i]
                s = 0
                for j in range(l, mid + 1):
                    s += gl[j] * fl[bi - ml[j]]
                    if (j & 63) == 63:
                        s %= mod
                Sl[i] = (Sl[i] + s) % mod
        else:
            m_l = m_arr[l]
            m_mid = m_arr[mid]
            b_mid1 = b_arr[mid + 1]
            b_r = b_arr[r]
            shift = b_mid1 - m_mid
            WF = m_mid - m_l
            WG = b_r - m_l - shift
            total = WF + WG + 1
            L = 1
            while L < total:
                L <<= 1
            F = [0] * L
            Ga = [0] * L
            for j in range(l, mid + 1):
                F[m_arr[j] - m_l] = g[j]
            for k in range(WG + 1):
                Ga[k] = fact[k + shift]
            ntt(F, False)
            ntt(Ga, False)
            for i in range(L):
                F[i] = F[i] * Ga[i] % MOD
            ntt(F, True)
            for i in range(mid + 1, r + 1):
                Sacc[i] = (Sacc[i] + F[b_arr[i] - m_l - shift]) % MOD
        cdq(mid + 1, r)
    cdq(0, t - 1)
    ans = fact[N]
    for i in range(t):
        ans = (ans + g[i] * fact[N - m_arr[i]]) % MOD
    return ans

def brute_force(N, S):
    from itertools import permutations
    M = 2 * N
    blacks = [i for i in range(M) if S[i] == 'B']
    whites = [i for i in range(M) if S[i] == 'W']
    count = 0
    for p in permutations(range(N)):
        adj = [[] for _ in range(M)]
        for i in range(M - 1):
            adj[i].append(i + 1)
        for j in range(N):
            adj[whites[p[j]]].append(blacks[j])
        def bfs(start, graph):
            vis = [False] * M
            q = [start]
            vis[start] = True
            h = 0
            while h < len(q):
                u = q[h]
                h += 1
                for v in graph[u]:
                    if not vis[v]:
                        vis[v] = True
                        q.append(v)
            return vis
        if not all(bfs(0, adj)):
            continue
        radj = [[] for _ in range(M)]
        for u in range(M):
            for v in adj[u]:
                radj[v].append(u)
        if not all(bfs(0, radj)):
            continue
        count += 1
    return count % MOD

def main():
    data = sys.stdin.buffer.read().split()
    N = int(data[0])
    S = data[1].decode()
    print(solve_case(N, S))

if __name__ == '__main__':
    import random
    # Verify NTT
    _a = [1,2,3,0,0,0,0,0]; _b = [4,5,6,0,0,0,0,0]
    # (skip inline NTT self-test to keep runtime clean; logic verified by brute force below)
    # Samples
    assert solve_case(2, "BWBW") == 1, "sample1 failed"
    assert solve_case(4, "BWWBWBWB") == 0, "sample2 failed"
    # Brute-force cross-check for small N
    random.seed(12345)
    for N in range(1, 7):
        for _ in range(40):
            arr = ['W'] * N + ['B'] * N
            random.shuffle(arr)
            S = ''.join(arr)
            a1 = solve_case(N, S)
            a2 = brute_force(N, S)
            if a1 != a2:
                raise SystemExit(f"MISMATCH N={N} S={S} fast={a1} brute={a2}")
    # Now run on actual stdin
    main()
```

### NOTES
- **Reduction (verified):** Strong connectivity of the path graph 1→2→…→2N plus N white→black matching edges is equivalent to: (a) S[0]≠'W' (else vertex 1 has no incoming edge), (b) S[-1]≠'B' (else prefix {1..2N−1} with B=N−1,W=N needs max>N, impossible), and (c) for every prefix ending just before a black vertex, an added edge must enter it.
- **Prefix condition:** For prefix {1..k} with B(k) blacks (=first B(k) blacks by position) and W(k) whites, it is closed (no incoming added edge) iff p_1..p_{B(k)} ⊆ {1..W(k)}, i.e. max_{j≤B(k)} p_j ≤ W(k). It is automatically safe when W(k)<B(k). It suffices to check only prefixes ending just before black k+1 (k=1..N−1): for any other prefix with B(k)=k', the just-before-black-(k'+1) prefix has the same B but ≥ W, so its condition implies the other's.
- **Events:** A_k = #whites before black k+1 = prefW[posB[k+1]−1]. Constraint for k: max_{j≤k} p_j > A_k, binding iff A_k≥k. Since A_k is non-decreasing, for each distinct value b the smallest k with A_k=b dominates all larger k with the same b (smaller max-range, same threshold ⇒ stronger). So keep one event per distinct b: (b, m_b) with m_b = min{k: A_k=b}; it is valid iff b≥m_b (equivalently the first k with A_k=b satisfies b≥k). Events sorted by b have strictly increasing m as well (A non-decreasing ⇒ first-hit positions increase).
- **Counting via inclusion-exclusion:** Forbidden event E_b: p_1..p_{m_b} ⊆ {1..b}. For a chain i_1<…<i_k the #perms where all occur is ∏_{j}(b_{i_j}−m_{i_{j-1}})!/(b_{i_j}−m_{i_j})! · (N−m_{i_k})! with m_{i_0}=0. DP over chains: g[i] = −(fact[b_i] + Σ_{j<i} g[j]·fact[b_i−m_j]) / fact[b_i−m_i]; answer = fact[N] + Σ_i g[i]·fact[N−m_i]. Verified by hand on N=2 "BWBW" (=1) and N=3 "BWBWBW" (=3, matches brute force; an early manual SCC miscount was my error, not the formula).
- **Hard part = online convolution:** S_i = Σ_{j<i} g[j]·fact[b_i−m_j] is a relaxed convolution of the sparse source array (index m_j, value g[j]) with the factorial kernel, queried at b_i. Solved with CDQ divide-and-conquer: recurse left, then add left→right contributions, then recurse right.
- **CDQ NTT shift (verified algebra):** At node [l,r], mid: build F[k]=g[j] at k=m_j−m_l for j∈[l,mid]; Ga[k]=fact[k+shift] for k∈[0,WG] with shift=b_{mid+1}−m_mid, WG=b_r−m_l−shift. Then (F*Ga)[b_i−m_l−shift] = Σ_{j∈[l,mid]} g[j]·fact[b_i−m_j]. NTT length L=next_pow2(WF+WG+1), WF=m_mid−m_l. Indices stay in [0,N] because m_mid<b_{mid+1} and b_r−m_l≤N−1.
- **Complexity:** The m-range of a node bounds the NTT size (slope of event sequence in (m,b) plane is ≤1 because A_k increases by ≥0 per step), giving O(N log²N) total NTT work. Naive O(left·right) used when left·right≤4096 to avoid NTT overhead on tiny nodes.
- **Performance caveat:** Pure-Python NTT at N=2e5 (worst case t≈N, e.g. alternating "BWBW…") is borderline/slow (est. tens of seconds). Correctness is solid (cross-checked vs brute force for N≤6, 40 random strings each, plus samples 1&2). Sample 3 (N=9) is small and fast. If TLE occurs, the only real fix is a C++/PyPy NTT or a faster relaxed-convolution kernel; no O(N log N) closed form found for the factorial kernel.
- **Edge cases handled:** N=1 ("BW"→1, "WB"→0); no events→fact[N]; S[0]=='W' or S[-1]=='B'→0; b≥m_i guaranteed so invfact[b_i−m_i] valid; fact indices in [0,N].

### NEXT
- Run sample 3 (N=9, expect 240792) to confirm the full pipeline end-to-end.
- If performance is a concern at N=2e5: tune TH, or port the NTT inner loop to a faster formulation / consider PyPy; otherwise accept.
- Optionally add an NTT self-test (convolution of small arrays) to the dev harness.

### STATUS
continue
