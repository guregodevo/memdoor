### CODE
```python
import sys

MOD = 998244353
G = 3

SUM_E = []
SUM_IE = []
RATE_INDEX = []
INV_LEN = []

SMALL_M = 3000
DIRECT_PAIRS = 4096


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

    dp_l = dp
    fact_l = fact
    invfact_l = invfact
    t_l = t
    v_l = v

    for j in range(1, m + 1):
        vj = v_l[j]
        total = 0
        for i in range(j):
            total += dp_l[i] * fact_l[vj - t_l[i]]
        dp_l[j] = (-total * invfact_l[vj - t_l[j]]) % mod

    ans = 0
    for i in range(m + 1):
        ans = (ans + dp_l[i] * fact_l[n - t_l[i]]) % mod
    return ans


def solve_cdq(n, t, v, fact, invfact):
    m = len(t) - 1
    dp = [0] * (m + 1)
    add = [0] * (m + 1)
    dp[0] = 1

    # Maximum convolution length is at most 2N - 1.
    max_log = (2 * n + 1).bit_length()
    global SUM_E, SUM_IE, RATE_INDEX, INV_LEN
    SUM_E, SUM_IE, RATE_INDEX, INV_LEN = init_ntt(max_log)

    mod = MOD
    sys.setrecursionlimit(1_000_000)

    def cdq(l, r, dp=dp, add=add, t=t, v=v, fact=fact, invfact=invfact,
            mod=mod, ntt_func=ntt, intt_func=intt, DIRECT=DIRECT_PAIRS):
        if r - l == 1:
            if l:
                dp[l] = (-add[l] * invfact[v[l] - t[l]]) % mod
            return

        mid = (l + r) // 2
        cdq(l, mid)

        left_len = mid - l
        right_len = r - mid

        if left_len * right_len <= DIRECT:
            if left_len == 1:
                dpi = dp[l]
                if dpi:
                    ti = t[l]
                    for j in range(mid, r):
                        add[j] = (add[j] + dpi * fact[v[j] - ti]) % mod
            else:
                idxs = range(l, mid)
                for j in range(mid, r):
                    vj = v[j]
                    total = 0
                    for i in idxs:
                        total += dp[i] * fact[vj - t[i]]
                    add[j] = (add[j] + total) % mod
        else:
            tl = t[l]
            la = t[mid - 1] - tl + 1
            offset = v[mid] - t[mid - 1]
            lb = v[r - 1] - v[mid] + t[mid - 1] - tl + 1

            need = la + lb - 1
            ntt_len = 1 << (need - 1).bit_length()

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


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]

    # Vertex 1 must be black and vertex 2N must be white.
    if s[0] != 66 or s[-1] != 87:  # 'B', 'W'
        print(0)
        return

    t, v = extract_records(s)
    fact, invfact = get_factorials(n)

    m = len(t) - 1
    if m == 0:
        print(fact[n])
        return

    if m <= SMALL_M:
        print(solve_direct(n, t, v, fact, invfact))
    else:
        print(solve_cdq(n, t, v, fact, invfact))


if __name__ == "__main__":
    main()
```

### NOTES
- **Strong connectivity reduction:** The base edges form a directed path, so forward reachability is automatic. Strong connectivity is equivalent to every prefix cut `{1..i}` having at least one added edge from the right side to the left side. This immediately forces `S_1 = B` and `S_{2N} = W`.
- **Prefix condition:** For a prefix containing `K` black vertices and `Q` white vertices, we need some black among the first `K` to be paired with a white of rank `> Q`. Thus `max(pi_1..pi_K) > Q`. The worst `Q` for fixed `K` is `p_K`, the number of whites before the `(K+1)`-th black.
- **Record pruning:** `p_K` is nondecreasing. For each newly seen value `v`, only the earliest `K` matters because later constraints with the same `v` are weaker. If `v < K`, the constraint is automatic since the maximum of `K` distinct positive ranks is at least `K`. The remaining records `(t_j, v_j)` have strictly increasing `t_j` and `v_j`, and `v_j >= t_j`.
- **Inclusion-exclusion DP:** Bad event `E_j` is “the first `t_j` white ranks are all in `[1..v_j]`.” For a selected chain `i_1 < ... < i_s`, the number of permutations satisfying all events is  
  `fact[v_i1] invfact[v_i1-t_i1] * fact[v_i2-t_i1] invfact[v_i2-t_i2] * ... * fact[N-t_is]`.  
  Define `dp_0 = 1` and  
  `dp_j = -invfact[v_j-t_j] * sum_{i<j} dp_i * fact[v_j-t_i]`.  
  The answer is `sum_{i=0}^m dp_i * fact[N-t_i]`.
- **CDQ structure:** Maintain `add[j]` as the accumulated sum over all already computed `i < j`. In `solve(l, r)`, compute the left half, add its contributions to the right half, then compute the right half. At a leaf `l > 0`, set `dp[l] = -add[l] * invfact[v_l-t_l]`.
- **Convolution mapping:** For a node `[l, r)` with `mid`, set `x_i = t_i - t_l`, `y_j = v_j - v_mid`, and `c = v_mid - t_{mid-1}`. Then  
  `v_j - t_i = c + y_j + (t_{mid-1} - t_l - x_i)`.  
  Use `A[x_i] = dp_i`, `B[b] = fact[c+b]`, compute `C = A * B`, and add `C[y_j + t_{mid-1} - t_l]` to `add[j]`.
- **Array sizes:** `A` length is `t[mid-1] - t[l] + 1`. `B` length is `v[r-1] - v[mid] + t[mid-1] - t[l] + 1`. All factorial indices stay in `[0, N]`, and the convolution length is at most `2N - 1`.
- **NTT implementation:** Uses the ACL butterfly transform for modulus `998244353` with primitive root `3`. It precomputes `sum_e`, `sum_ie`, and a `rate_index` table for least-zero-bit updates. Forward is DIF, inverse is DIT, and inverse multiplies by `inv(2^h)`. This avoids explicit bit-reversal and keeps root updates per block rather than per butterfly.
- **Thresholds and fallbacks:** If `m <= 3000`, use the direct `O(m^2)` DP. Inside CDQ, if `left_len * right_len <= 4096`, use direct loops with one modulo per target `j`; otherwise use NTT. This keeps small segments cheap and avoids many tiny NTT calls.
- **Edge cases:** If there are no records, the answer is `N!`. If `S_1 != B` or `S_{2N} != W`, the answer is `0`. The case `v_j = t_j` is valid and uses `invfact[0]`.
- **Validation:** The direct DP matches brute-force permutation enumeration on small random balanced strings and the provided samples. The CDQ solver uses the same recurrence, and the convolution index mapping was checked algebraically.

### NEXT
- none

### STATUS
solved
