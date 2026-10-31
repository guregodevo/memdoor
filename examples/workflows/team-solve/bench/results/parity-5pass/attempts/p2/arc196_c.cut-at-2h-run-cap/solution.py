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