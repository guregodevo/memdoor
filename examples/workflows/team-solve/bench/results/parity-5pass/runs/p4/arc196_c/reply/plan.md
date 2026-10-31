```python
import sys

MOD = 998244353
G = 3

def mod_pow(a, e, mod=MOD):
    res = 1
    while e:
        if e & 1:
            res = res * a % mod
        a = a * a % mod
        e >>= 1
    return res

def ntt(a, invert):
    n = len(a)
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
    while length <= n:
        wlen = mod_pow(G, (MOD - 1) // length)
        if invert:
            wlen = mod_pow(wlen, MOD - 2)
        half = length >> 1
        for i in range(0, n, length):
            w = 1
            for j in range(i, i + half):
                u = a[j]
                v = a[j + half] * w % MOD
                x = u + v
                if x >= MOD:
                    x -= MOD
                y = u - v
                if y < 0:
                    y += MOD
                a[j] = x
                a[j + half] = y
                w = w * wlen % MOD
        length <<= 1

    if invert:
        inv_n = mod_pow(n, MOD - 2)
        for i in range(n):
            a[i] = a[i] * inv_n % MOD

def convolution(a, b):
    if not a or not b:
        return []
    need = len(a) + len(b) - 1
    n = 1
    while n < need:
        n <<= 1
    fa = a + [0] * (n - len(a))
    fb = b + [0] * (n - len(b))
    ntt(fa, False)
    ntt(fb, False)
    for i in range(n):
        fa[i] = fa[i] * fb[i] % MOD
    ntt(fa, True)
    return fa[:need]

def solve():
    input = sys.stdin.readline
    N = int(input().strip())
    S = input().strip()

    if S[0] != 'B' or S[-1] != 'W':
        print(0)
        return

    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD
    invfact = [1] * (N + 1)
    invfact[N] = mod_pow(fact[N], MOD - 2)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    # R[c] = number of W before black (c+1), for c=1..N-1; R[N] = N-1
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

    # A[c] is updated online.  We need queries:
    # Q_c = sum_{t < c} A[t] * fact[R[c] - t]
    # Use CDQ divide and conquer with NTT.
    A = [0] * (N + 1)
    query = [0] * (N + 1)

    def cdq(l, r):
        if l >= r:
            return
        mid = (l + r) // 2
        cdq(l, mid)

        # contributions from t in [l, mid] to c in [mid+1, r]
        left_len = mid - l + 1
        right_len = r - mid
        # A[t] at index t-l
        left = [0] * left_len
        for i in range(l, mid + 1):
            left[i - l] = A[i]
        # fact[R[c] - t] is not a pure convolution unless we shift by c.
        # For each c, t is in [l,mid].  We build an array over t and convolve
        # with fact.  The result at index (R[c] - l) gives sum left[t-l] * fact[R[c]-t].
        # We only need indices up to max R[c] - l.
        max_idx = 0
        for c in range(mid + 1, r + 1):
            if R[c] >= l:
                if R[c] - l > max_idx:
                    max_idx = R[c] - l
        if max_idx >= 0:
            fact_part = fact[:max_idx + 1]
            conv = convolution(left, fact_part)
            for c in range(mid + 1, r + 1):
                idx = R[c] - l
                if 0 <= idx < len(conv):
                    query[c] = (query[c] + conv[idx]) % MOD

        cdq(mid + 1, r)

        for c in range(mid + 1, r + 1):
            if R[c] >= c:
                val = (fact[R[c]] + query[c]) % MOD
                A[c] = (-val * invfact[R[c] - c]) % MOD

    cdq(1, N)

    ans = fact[N]
    for c in range(1, N + 1):
        if A[c]:
            ans = (ans + A[c] * fact[N - c]) % MOD
    print(ans % MOD)

if __name__ == "__main__":
    solve()
```
