import sys
import os

MOD = 998244353
G = 3

# Force the CDQ path in the normal solver.
SMALL_M = 0

# Heuristic thresholds for choosing direct O(left*right) work inside CDQ.
DIRECT_PAIRS = 16384
COST_FACTOR = 16

SUM_E = []
SUM_IE = []
RATE_INDEX = []
INV_LEN = []
_NTT_MAX_LOG = 0


def init_ntt(max_log):
    mod = MOD
    low = (mod - 1) & -(mod - 1)
    cnt2 = low.bit_length() - 1

    es = [0] * cnt2
    ies = [0] * cnt2
    e = pow(G, (mod - 1) >> cnt2, mod)
    ie = pow(e, mod - 2, mod)

    for i in range(cnt2, 1, -1):
        es[i - 2] = e
        ies[i - 2] = ie
        e = e * e % mod
        ie = ie * ie % mod

    sum_e = [1] * cnt2
    sum_ie = [1] * cnt2

    now = 1
    for i in range(cnt2 - 1):
        sum_e[i] = es[i] * now % mod
        now = now * ies[i] % mod

    now = 1
    for i in range(cnt2 - 1):
        sum_ie[i] = ies[i] * now % mod
        now = now * es[i] % mod

    max_w = 1 << max(0, max_log - 1)
    rate_index = [((i + 1) & -(i + 1)).bit_length() - 1 for i in range(max_w)]

    inv_len = [1] * (max_log + 1)
    inv2 = (mod + 1) // 2
    for i in range(1, max_log + 1):
        inv_len[i] = inv_len[i - 1] * inv2 % mod

    return sum_e, sum_ie, rate_index, inv_len


def ensure_ntt(max_log):
    global SUM_E, SUM_IE, RATE_INDEX, INV_LEN, _NTT_MAX_LOG
    if max_log > _NTT_MAX_LOG:
        SUM_E, SUM_IE, RATE_INDEX, INV_LEN = init_ntt(max_log)
        _NTT_MAX_LOG = max_log


def ntt(a):
    n = len(a)
    if n == 1:
        return

    mod = MOD
    se = SUM_E
    ri = RATE_INDEX
    h = n.bit_length() - 1

    for ph in range(1, h + 1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        shift = h - ph + 1
        now = 1

        for s in range(w):
            offset = s << shift
            end = offset + p

            if now == 1:
                for idx in range(offset, end):
                    j = idx + p
                    l = a[idx]
                    r = a[j]
                    x = l + r
                    if x >= mod:
                        x -= mod
                    y = l - r
                    if y < 0:
                        y += mod
                    a[idx] = x
                    a[j] = y
            else:
                for idx in range(offset, end):
                    j = idx + p
                    l = a[idx]
                    r = a[j] * now % mod
                    x = l + r
                    if x >= mod:
                        x -= mod
                    y = l - r
                    if y < 0:
                        y += mod
                    a[idx] = x
                    a[j] = y

            now = now * se[ri[s]] % mod


def intt(a):
    n = len(a)
    if n == 1:
        return

    mod = MOD
    sie = SUM_IE
    ri = RATE_INDEX
    inv_len = INV_LEN
    h = n.bit_length() - 1

    for ph in range(h, 0, -1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        shift = h - ph + 1
        inow = 1

        for s in range(w):
            offset = s << shift
            end = offset + p

            if inow == 1:
                for idx in range(offset, end):
                    j = idx + p
                    l = a[idx]
                    r = a[j]
                    x = l + r
                    if x >= mod:
                        x -= mod
                    y = l - r
                    if y < 0:
                        y += mod
                    a[idx] = x
                    a[j] = y
            else:
                for idx in range(offset, end):
                    j = idx + p
                    l = a[idx]
                    r = a[j]
                    x = l + r
                    if x >= mod:
                        x -= mod
                    y = l - r
                    if y < 0:
                        y += mod
                    a[idx] = x
                    a[j] = y * inow % mod

            inow = inow * sie[ri[s]] % mod

    inv_n = inv_len[h]
    for i in range(n):
        a[i] = a[i] * inv_n % mod


def get_factorials(n):
    mod = MOD
    fact = [1] * (n + 1)
    for i in range(1, n + 1):
        fact[i] = fact[i - 1] * i % mod

    invfact = [1] * (n + 1)
    invfact[n] = pow(fact[n], mod - 2, mod)
    for i in range(n, 0, -1):
        invfact[i - 1] = invfact[i] * i % mod

    return fact, invfact


def extract_records(s):
    # t[0] = v[0] = 0.  Records are 1-indexed: (t[j], v[j]).
    t = [0]
    v = [0]
    last_v = -1
    white_count = 0
    black_count = 0

    for ch in s:
        if ch == 87:  # 'W'
            white_count += 1
        else:
            if black_count:
                k = black_count
                val = white_count
                if val > last_v:
                    last_v = val
                    if val >= k:
                        t.append(k)
                        v.append(val)
            black_count += 1

    return t, v


def solve_direct(n, t, v, fact, invfact):
    m = len(t) - 1
    dp = [0] * (m + 1)
    dp[0] = 1
    mod = MOD

    for j in range(1, m + 1):
        vj = v[j]
        total = 0
        for i in range(j):
            total += dp[i] * fact[vj - t[i]]
        dp[j] = (-total * invfact[vj - t[j]]) % mod

    ans = 0
    for i in range(m + 1):
        ans = (ans + dp[i] * fact[n - t[i]]) % mod
    return ans


def solve_cdq(n, t, v, fact, invfact,
              direct_pairs=DIRECT_PAIRS, cost_factor=COST_FACTOR):
    m = len(t) - 1
    dp = [0] * (m + 1)
    add = [0] * (m + 1)
    dp[0] = 1

    max_log = (2 * n + 1).bit_length()
    mod = MOD
    sys.setrecursionlimit(1_000_000)

    def cdq(l, r, dp=dp, add=add, t=t, v=v, fact=fact, invfact=invfact,
            mod=mod, ntt_func=ntt, intt_func=intt,
            direct_pairs=direct_pairs, cost_factor=cost_factor,
            ensure=ensure_ntt, max_log=max_log):
        if r - l == 1:
            if l:
                dp[l] = (-add[l] * invfact[v[l] - t[l]]) % mod
            return

        mid = (l + r) // 2
        cdq(l, mid)

        left_len = mid - l
        right_len = r - mid
        pairs = left_len * right_len

        use_direct = pairs <= direct_pairs
        if not use_direct:
            tl = t[l]
            la = t[mid - 1] - tl + 1
            offset = v[mid] - t[mid - 1]
            lb = v[r - 1] - v[mid] + t[mid - 1] - tl + 1
            need = la + lb - 1
            ntt_len = 1 << (need - 1).bit_length()

            if pairs <= ntt_len * (ntt_len.bit_length() - 1) * cost_factor:
                use_direct = True

        if use_direct:
            if left_len == 1:
                dpi = dp[l]
                if dpi:
                    ti = t[l]
                    for j in range(mid, r):
                        add[j] = (add[j] + dpi * fact[v[j] - ti]) % mod
            else:
                for j in range(mid, r):
                    vj = v[j]
                    total = 0
                    for i in range(l, mid):
                        total += dp[i] * fact[vj - t[i]]
                    add[j] = (add[j] + total) % mod
        else:
            if ntt_len > 1:
                ensure(max_log)

            A = [0] * ntt_len
            for i in range(l, mid):
                A[t[i] - tl] = dp[i]

            B = fact[offset:offset + lb]
            B.extend([0] * (ntt_len - lb))

            ntt_func(A)
            ntt_func(B)

            for i in range(ntt_len):
                A[i] = A[i] * B[i] % mod
            del B

            intt_func(A)

            base = t[mid - 1] - tl
            vmid = v[mid]
            for j in range(mid, r):
                z = v[j] - vmid + base
                val = add[j] + A[z]
                if val >= mod:
                    val -= mod
                add[j] = val

            del A

        cdq(mid, r)

    cdq(0, m + 1)

    ans = 0
    for i in range(m + 1):
        ans = (ans + dp[i] * fact[n - t[i]]) % mod
    return ans


def solve_general(n, s, force_cdq=False, force_direct=False):
    # Vertex 1 must be black and vertex 2N must be white.
    if s[0] != 66 or s[-1] != 87:  # 'B', 'W'
        return 0

    t, v = extract_records(s)
    fact, invfact = get_factorials(n)
    m = len(t) - 1

    if force_cdq:
        return solve_cdq(n, t, v, fact, invfact)

    if m == 0:
        return fact[n]

    if force_direct:
        return solve_direct(n, t, v, fact, invfact)

    if m <= SMALL_M:
        return solve_direct(n, t, v, fact, invfact)

    return solve_cdq(n, t, v, fact, invfact)


def _reachable(adj, start, n):
    seen = [False] * n
    stack = [start]
    seen[start] = True
    while stack:
        v = stack.pop()
        for to in adj[v]:
            if not seen[to]:
                seen[to] = True
                stack.append(to)
    return all(seen)


def brute(n, s):
    # Direct graph check, used only for local self-testing.
    if s[0] != 66 or s[-1] != 87:
        return 0

    V = 2 * n
    whites = []
    blacks = []
    for i, ch in enumerate(s):
        if ch == 87:
            whites.append(i)
        else:
            blacks.append(i)

    base = [[] for _ in range(V)]
    rbase = [[] for _ in range(V)]
    for i in range(V - 1):
        base[i].append(i + 1)
        rbase[i + 1].append(i)

    from itertools import permutations

    count = 0
    for perm in permutations(whites):
        adj = [lst[:] for lst in base]
        radj = [lst[:] for lst in rbase]
        for b, w in zip(blacks, perm):
            adj[w].append(b)
            radj[b].append(w)

        if _reachable(adj, 0, V) and _reachable(radj, 0, V):
            count += 1

    return count


def selftest():
    import random

    random.seed(123456)

    for n in range(1, 6):
        for _ in range(30):
            if _ % 2 == 0:
                mid = [66] * (n - 1) + [87] * (n - 1)
                random.shuffle(mid)
                arr = [66] + mid + [87]
            else:
                arr = [66] * n + [87] * n
                random.shuffle(arr)

            s = bytes(arr)

            expected = brute(n, s)
            got_direct = solve_general(n, s, force_direct=True)
            got_cdq = solve_general(n, s, force_cdq=True)

            if s[0] != 66 or s[-1] != 87:
                got_forced_ntt = 0
            else:
                t, v = extract_records(s)
                fact, invfact = get_factorials(n)
                got_forced_ntt = solve_cdq(
                    n, t, v, fact, invfact,
                    direct_pairs=0, cost_factor=0
                )

            if expected != got_direct or expected != got_cdq or expected != got_forced_ntt:
                print("FAIL", n, s, expected, got_direct, got_cdq, got_forced_ntt)
                return

    print("SELFTEST OK")


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]
    print(solve_general(n, s))


if __name__ == "__main__":
    if os.environ.get("SELFTEST") == "1":
        selftest()
    else:
        main()