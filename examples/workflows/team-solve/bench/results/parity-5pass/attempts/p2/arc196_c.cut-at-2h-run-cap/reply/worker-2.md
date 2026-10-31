### CODE
```python
import sys

MOD = 998244353

def main():
    data = sys.stdin.read().split()
    if not data:
        return

    N = int(data[0])
    S = data[1].strip()

    # Vertex 1 must be black and vertex 2N must be white.
    if S[0] != 'B' or S[-1] != 'W':
        print(0)
        return

    # Factorials and inverse factorials up to N.
    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD

    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    # Collect active cuts.
    # For i = 1..N-1, c_i = number of W before black i+1.
    # Keep cuts where c_i strictly increases and c_i >= i.
    active_u = []
    active_t = []

    white = 0
    black = 0
    prev_c = 0

    for ch in S:
        if ch == 'W':
            white += 1
        else:  # 'B'
            black += 1
            if black >= 2:
                i = black - 1
                c = white
                if c > prev_c and c >= i:
                    active_u.append(i)
                    active_t.append(c)
            prev_c = white

    K = len(active_u)
    u = [0] + active_u
    t = [0] + active_t

    dp = [0] * (K + 1)
    dp[0] = 1

    f = fact
    inv = invfact
    uu = u
    tt = t
    d = dp
    mod = MOD

    # O(K^2) baseline DP.
    for i in range(1, K + 1):
        ti = tt[i]
        s = 0
        for j in range(i):
            s += d[j] * f[ti - uu[j]]
        s %= mod
        d[i] = (-s * inv[ti - uu[i]]) % mod

    ans = 0
    for j in range(K + 1):
        ans += d[j] * f[N - uu[j]]

    print(ans % mod)

if __name__ == "__main__":
    main()
```

### NOTES
- **Strong connectivity criterion:** The original path edges only go from smaller to larger vertices. Therefore the final graph is strongly connected iff every prefix cut k=1..2N-1 has at least one added edge from a white vertex >k to a black vertex <=k. This immediately forces S[0]='B' and S[-1]='W'; otherwise the answer is 0.
- **Relevant cuts:** It is enough to inspect cuts just before black vertices. For i=1..N-1, let c_i be the number of white vertices before black i+1. A cut with c_i < i is automatically covered because the left side has more black vertices than white vertices, so some left black must be matched to a right white.
- **Active cut selection:** c_i is nondecreasing. Within a run where c_i is constant, later cuts are nested in the inclusion-exclusion sense and only the first cut of the run contributes. The scan keeps exactly the first non-forced cut of each run: c_i > c_{i-1} and c_i >= i. Stored pairs are (u=i, t=c_i).
- **DP recurrence:** With u[0]=t[0]=0 and dp[0]=1, for i=1..K compute s = sum_{j=0}^{i-1} dp[j] * fact[t[i]-u[j]] modulo MOD, then dp[i] = -s * invfact[t[i]-u[i]] modulo MOD. The final answer is sum_{j=0}^{K} dp[j] * fact[N-u[j]] modulo MOD. This is the compressed inclusion-exclusion/Möbius form over active cuts.
- **Implementation details:** Precompute fact and invfact up to N using Fermat inverse. In the O(K^2) loop, accumulate the inner sum as a Python integer and take modulo once per i to reduce overhead. All indices t[i]-u[j] and N-u[j] are nonnegative for the active list.
- **Sample verification:** Sample 1 gives 1, sample 2 gives 0, and sample 3 gives 240792 with this recurrence.
- **Complexity:** Time O(N + K^2), memory O(N + K), where K <= N-1. This baseline is correct but too slow for worst-case N=2e5; NTT/CDQ is still needed.
- **Edge cases:** N=1 works: BW gives 1, WB is rejected by the endpoint check. If there are no active cuts, the answer is N!, which matches cases like all black vertices before all white vertices.

### NEXT
- Replace the O(K^2) recurrence with CDQ divide-and-conquer plus NTT modulo 998244353.
- Use factorial convolution to add contributions from solved left halves to right halves in O(N log^2 N) or O(N log N).
- Add randomized brute-force tests for small N against exhaustive matchings.
- Run performance tests near N=2e5 after the NTT implementation.

### STATUS
continue
