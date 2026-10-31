### CODE
```python
import sys
import random
import gc
from itertools import combinations, permutations

MOD = 998244353
G = 3


def prepare_fact(n):
    fact = [1] * (n + 1)
    for i in range(1, n + 1):
        fact[i] = fact[i - 1] * i % MOD
    invfact = [1] * (n + 1)
    invfact[n] = pow(fact[n], MOD - 2, MOD)
    for i in range(n, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD
    return fact, invfact


def compute_h(n, s):
    h = [0] * (n + 1)
    w = 0
    b = 0
    for ch in s:
        if ch == 'W':
            w += 1
        else:
            b += 1
            a = b - 1
            if 1 <= a <= n - 1:
                h[a] = w
    h[n] = n
    return h


def solve_slow(n, s, fact=None, invfact=None):
    if n == 0:
        return 1
    if s[0] == 'W' or s[-1] == 'B':
        return 0
    if n == 1:
        return 1
    if fact is None:
        fact, invfact = prepare_fact(n)

    h = compute_h(n, s)
    r = [0] * (n + 1)
    mod = MOD
    f = fact
    inv = invfact

    for j in range(1, n + 1):
        hj = h[j]
        if hj < j:
            continue
        total = f[hj]
        for i in range(1, j):
            ri = r[i]
            if ri:
                total += ri * f[hj - i]
        r[j] = (-total % mod) * inv[hj - j] % mod

    return (-r[n]) % mod


def prepare_roots(max_size):
    max_log = max_size.bit_length() - 1
    f_roots = [None] * (max_log + 1)
    i_roots = [None] * (max_log + 1)
    inv_pow2 = [1] * (max_log + 1)
    inv2 = (MOD + 1) // 2
    mod = MOD

    for k in range(1, max_log + 1):
        inv_pow2[k] = inv_pow2[k - 1] * inv2 % mod

    for k in range(1, max_log + 1):
        length = 1 << k
        half = length >> 1
        wlen = pow(G, (mod - 1) // length, mod)
        iwlen = pow(wlen, mod - 2, mod)
        roots = [1] * half
        iroots = [1] * half
        for j in range(1, half):
            roots[j] = roots[j - 1] * wlen % mod
            iroots[j] = iroots[j - 1] * iwlen % mod
        f_roots[k] = roots
        i_roots[k] = iroots

    return f_roots, i_roots, inv_pow2


def solve_fast(n, s, fact=None, invfact=None, roots=None,
               force_ntt=False, direct_threshold=20000):
    if n == 0:
        return 1
    if s[0] == 'W' or s[-1] == 'B':
        return 0
    if n == 1:
        return 1

    if fact is None:
        fact, invfact = prepare_fact(n)

    h = compute_h(n, s)

    T = [0] * (n + 1)
    is_good = bytearray(n + 1)
    good_pref = [0] * (n + 2)

    for j in range(1, n + 1):
        hj = h[j]
        if hj >= j:
            is_good[j] = 1
            T[j] = fact[hj]
        good_pref[j + 1] = good_pref[j] + is_good[j]

    # If no proper prefix can be closed, every pairing works.
    if good_pref[n] == 0:
        return fact[n] % MOD

    if roots is None:
        max_size = 1 << (2 * n - 1).bit_length()
        f_roots, i_roots, inv_pow2 = prepare_roots(max_size)
    else:
        f_roots, i_roots, inv_pow2 = roots
        max_size = 1 << (2 * n - 1).bit_length()
        cur_size = 1 << (len(f_roots) - 1)
        if cur_size < max_size:
            f_roots, i_roots, inv_pow2 = prepare_roots(max_size)

    next_good = [n + 1] * (n + 2)
    nxt = n + 1
    for j in range(n, 0, -1):
        if is_good[j]:
            nxt = j
        next_good[j] = nxt

    prev_good = [0] * (n + 1)
    prv = 0
    for j in range(1, n + 1):
        if is_good[j]:
            prv = j
        prev_good[j] = prv

    R = [0] * (n + 1)
    mod = MOD

    def ntt(a, f_roots=f_roots, mod=MOD):
        ntt_n = len(a)
        length = ntt_n
        k = ntt_n.bit_length() - 1
        while length > 1:
            half = length >> 1
            roots = f_roots[k]
            for i in range(0, ntt_n, length):
                base = i
                for j in range(half):
                    idx = base + j
                    u = a[idx]
                    v = a[idx + half]
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[idx] = x
                    a[idx + half] = (y * roots[j]) % mod
            length = half
            k -= 1

    def intt(a, i_roots=i_roots, inv_pow2=inv_pow2, mod=MOD):
        intt_n = len(a)
        length = 2
        k = 1
        while length <= intt_n:
            half = length >> 1
            roots = i_roots[k]
            for i in range(0, intt_n, length):
                base = i
                for j in range(half):
                    idx = base + j
                    u = a[idx]
                    v = (a[idx + half] * roots[j]) % mod
                    x = u + v
                    if x >= mod:
                        x -= mod
                    y = u - v
                    if y < 0:
                        y += mod
                    a[idx] = x
                    a[idx + half] = y
            length <<= 1
            k += 1

        inv_n = inv_pow2[intt_n.bit_length() - 1]
        if inv_n != 1:
            for i in range(intt_n):
                a[i] = (a[i] * inv_n) % mod

    def apply(l, mid, r, h=h, T=T, R=R, fact=fact, is_good=is_good,
              good_pref=good_pref, next_good=next_good, prev_good=prev_good,
              mod=MOD, force_ntt=force_ntt, direct_threshold=direct_threshold,
              ntt=ntt, intt=intt):
        first = next_good[mid + 1]
        if first > r:
            return
        if good_pref[mid + 1] - good_pref[l] == 0:
            return

        last = prev_good[r]
        Hmin = h[first]
        Hmax = h[last]
        good_count = good_pref[r + 1] - good_pref[mid + 1]

        nz_count = 0
        for i in range(l, mid + 1):
            if R[i]:
                nz_count += 1
        if nz_count == 0:
            return

        L = mid - l + 1
        Hspan = Hmax - Hmin

        if Hspan == 0 and not force_ntt:
            val = 0
            for i in range(l, mid + 1):
                ri = R[i]
                if ri:
                    val += ri * fact[Hmin - i]
            val %= mod
            if val:
                for j in range(mid + 1, r + 1):
                    if is_good[j]:
                        x = T[j] + val
                        if x >= mod:
                            x -= mod
                        T[j] = x
            return

        cost_j = nz_count * good_count
        cost_h = nz_count * (Hspan + 1)
        direct_cost = (cost_j if cost_j <= cost_h else cost_h) + (r - mid)
        use_j = cost_j <= cost_h

        conv_len = 2 * L + Hspan - 1
        ntt_len = 1 << (conv_len - 1).bit_length()

        if not force_ntt:
            ntt_cost = ntt_len * (ntt_len.bit_length() - 1) * 3
            if direct_cost <= direct_threshold or direct_cost * 2 <= ntt_cost:
                nz = []
                for i in range(l, mid + 1):
                    ri = R[i]
                    if ri:
                        nz.append((i, ri))

                if use_j:
                    if nz_count == 1:
                        i0, ri0 = nz[0]
                        for j in range(mid + 1, r + 1):
                            if is_good[j]:
                                add = ri0 * fact[h[j] - i0] % mod
                                x = T[j] + add
                                if x >= mod:
                                    x -= mod
                                T[j] = x
                    else:
                        for j in range(mid + 1, r + 1):
                            if is_good[j]:
                                hj = h[j]
                                total = 0
                                for i, ri in nz:
                                    total += ri * fact[hj - i]
                                T[j] = (T[j] + total) % mod
                else:
                    hspan1 = Hspan + 1
                    vals = [0] * hspan1
                    for i, ri in nz:
                        base = Hmin - i
                        for d in range(hspan1):
                            vals[d] += ri * fact[base + d]
                    for d in range(hspan1):
                        vals[d] %= mod
                    for j in range(mid + 1, r + 1):
                        if is_good[j]:
                            add = vals[h[j] - Hmin]
                            if add:
                                x = T[j] + add
                                if x >= mod:
                                    x -= mod
                                T[j] = x
                return

        A = R[l:mid + 1] + [0] * (ntt_len - L)
        start = Hmin - mid
        length_b = Hspan + L
        B = fact[start:start + length_b] + [0] * (ntt_len - length_b)

        ntt(A)
        ntt(B)
        for i in range(ntt_len):
            A[i] = (A[i] * B[i]) % mod
        intt(A)
        del B

        base_idx = L - 1
        for j in range(mid + 1, r + 1):
            if is_good[j]:
                add = A[base_idx + h[j] - Hmin]
                if add:
                    x = T[j] + add
                    if x >= mod:
                        x -= mod
                    T[j] = x

    def cdq(l, r, good_pref=good_pref, h=h, T=T, R=R,
            invfact=invfact, mod=MOD, apply=apply):
        if good_pref[r + 1] - good_pref[l] == 0:
            return
        if l == r:
            hj = h[l]
            R[l] = (-T[l] * invfact[hj - l]) % mod
            return

        mid = (l + r) >> 1
        cdq(l, mid)
        apply(l, mid, r)
        cdq(mid + 1, r)

    cdq(1, n)
    return (-R[n]) % mod


def brute_count_direct(n, s):
    whites = [i for i, ch in enumerate(s, 1) if ch == 'W']
    blacks = [i for i, ch in enumerate(s, 1) if ch == 'B']
    cnt = 0
    for perm in permutations(whites):
        ok = True
        for k in range(1, 2 * n):
            found = False
            for t, b in enumerate(blacks):
                if b <= k < perm[t]:
                    found = True
                    break
            if not found:
                ok = False
                break
        if ok:
            cnt += 1
    return cnt % MOD


def run_selftest():
    sys.setrecursionlimit(1_000_000)
    gc.disable()

    samples = [
        (2, 'BWBW', 1),
        (4, 'BWWBWBWB', 0),
        (9, 'BWWBWBBBWWBWBBWWBW', 240792),
    ]
    for n, s, expected in samples:
        slow = solve_slow(n, s)
        fast = solve_fast(n, s)
        if slow != expected % MOD or fast != expected % MOD:
            print(f"FAIL sample n={n} s={s} slow={slow} fast={fast} expected={expected}")
            return

    for n in range(1, 6):
        fact, invfact = prepare_fact(n)
        max_size = 1 << (2 * n - 1).bit_length()
        roots = prepare_roots(max_size)
        length = 2 * n
        for combo in combinations(range(length), n):
            arr = ['B'] * length
            for pos in combo:
                arr[pos] = 'W'
            s = ''.join(arr)
            brute = brute_count_direct(n, s)
            slow = solve_slow(n, s, fact, invfact)
            fast = solve_fast(n, s, fact, invfact, roots, force_ntt=True)
            if brute != slow % MOD or brute != fast % MOD:
                print(f"FAIL exhaustive n={n} s={s} brute={brute} slow={slow} fast={fast}")
                return

    random.seed(123456789)
    stress_cases = [
        (8, 30, True),
        (16, 20, True),
        (32, 10, True),
        (64, 10, False),
        (128, 5, False),
        (256, 3, False),
        (512, 2, False),
        (1024, 1, False),
    ]
    for n, trials, force in stress_cases:
        fact, invfact = prepare_fact(n)
        max_size = 1 << (2 * n - 1).bit_length()
        roots = prepare_roots(max_size)
        for _ in range(trials):
            while True:
                chars = ['W'] * n + ['B'] * n
                random.shuffle(chars)
                s = ''.join(chars)
                if s[0] == 'B' and s[-1] == 'W':
                    break
            slow = solve_slow(n, s, fact, invfact)
            fast = solve_fast(n, s, fact, invfact, roots, force_ntt=force)
            if slow != fast % MOD:
                print(f"FAIL random n={n} s={s} slow={slow} fast={fast}")
                return

    print("SELFTEST PASSED")


def main():
    if len(sys.argv) > 1 and sys.argv[1] == '--selftest':
        run_selftest()
        return

    data = sys.stdin.read().split()
    if not data:
        return

    if data[0] == 'SELFTEST':
        run_selftest()
        return

    sys.setrecursionlimit(1_000_000)
    gc.disable()

    n = int(data[0])
    s = data[1].strip()
    print(solve_fast(n, s))


if __name__ == '__main__':
    main()
```

### NOTES
- **Strong connectivity reduction:** The fixed path edges go forward. A proper prefix is closed iff no added edge enters it. Since added edges go white to black, an entering edge exists iff some black in the prefix is paired with a white outside the prefix.
- **Boundary `h`:** For `a=1..N-1`, `h[a]` is the number of whites before the `(a+1)`-st black. A prefix with `a` blacks is closed iff `max(pi(1..a)) <= h[a]`. Strong connectivity is equivalent to `max(pi(1..a)) > h[a]` for all `a=1..N-1`, plus early exits: first vertex white or last vertex black gives 0.
- **Inclusion-exclusion recurrence:** Let `R[j]` be the signed inclusion-exclusion contribution for subsets whose maximum selected event is `j`, normalized by `(N-j)!` for `j<N`, and `R[N]` itself for the sentinel `h[N]=N`. The answer is `-R[N] mod MOD`. For good `j` (`h[j]>=j`):
  `R[j] = -(fact[h[j]] + sum_{i<j} R[i] fact[h[j]-i]) * invfact[h[j]-j]`.
  If `h[j] < j`, then `R[j]=0`.
- **Automatic case:** If no `j=1..N-1` is good, no proper prefix can be closed, so the answer is `N!`. This early return avoids unnecessary NTT setup.
- **CDQ structure:** `T[j]` accumulates `fact[h[j]] + sum_{i<j} R[i] fact[h[j]-i]`. CDQ solves left half, adds left-to-right contributions, then solves right half. At a leaf, `R[j]` is computed from `T[j]`.
- **Contribution mapping:** For node `[l,r]`, `mid`, left length `L=mid-l+1`, and good right indices with `Hmin=h[first_good]`, `Hmax=h[last_good]`, use
  `A[p]=R[l+p]`, `B[q]=fact[Hmin-mid+q]` for `q=0..Hspan+L-1`.
  The needed contribution for right `j` is convolution coefficient `L-1 + (h[j]-Hmin)`. This was re-verified algebraically and against the sample 3 recurrence by hand.
- **Off-by-one fix:** `good_pref` now has length `n+2` because the loop writes `good_pref[j+1]` for `j=n`. Audited lengths: `next_good` length `n+2`, `prev_good` length `n+1`, `T/R/h` length `n+1`, `fact/invfact` length `n+1`. In NTT, `B` slice end is `Hmax-l+1 <= N`, so it stays inside `fact`.
- **Good-index helpers:** `good_pref[k]` counts good indices `1..k-1`. `next_good[x]` is the first good index `>=x`, `prev_good[x]` is the last good index `<=x`. CDQ skips intervals with no good indices, and apply skips if the left half has no good indices or the right half has no good indices.
- **NTT implementation:** Modulo `998244353`, primitive root `3`. Forward transform is decimation-in-frequency, inverse is decimation-in-time, so the paired transforms need no explicit bit reversal. Root tables are precomputed up to the next power of two at least `2N-1`, which is safely above the maximum convolution length.
- **Hybrid direct/NTT:** For each CDQ apply, if `Hspan==0` use a scalar sum. Otherwise estimate direct cost by iterating over good right indices or over the `H` span, and use direct for small/sparse cases; otherwise use NTT. This avoids many tiny NTTs and handles sparse/gapped boundaries.
- **Selftest:** The program supports `--selftest` or first stdin token `SELFTEST`. It checks the three samples, exhaustive small `N<=5` against brute force, and random medium `N` against the O(N^2) solver. Small stress cases use `force_ntt=True` to exercise the NTT path.

### NEXT
none

### STATUS
solved
