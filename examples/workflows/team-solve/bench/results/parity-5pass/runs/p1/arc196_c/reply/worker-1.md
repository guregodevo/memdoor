### CODE
```python
import sys
from itertools import permutations
from functools import lru_cache

MOD = 998244353


@lru_cache(maxsize=None)
def get_factorials(n):
    fact = [1] * (n + 1)
    for i in range(1, n + 1):
        fact[i] = fact[i - 1] * i % MOD

    invfact = [1] * (n + 1)
    invfact[n] = pow(fact[n], MOD - 2, MOD)
    for i in range(n, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    return fact, invfact


def extract_records(n, s):
    """
    Returns records (t_j, v_j):
      t_j = first black-prefix size k where p_k becomes a new value v_j
      v_j = p_k = number of white vertices before the (k+1)-th black
    Only records with v_j >= t_j are non-automatic.
    """
    records = []
    last_v = -1
    white_count = 0
    black_count = 0

    for ch in s:
        if ch == 'W':
            white_count += 1
        else:
            # Current black is black number black_count + 1.
            # If black_count >= 1, this is the (k+1)-th black with k = black_count.
            if black_count >= 1:
                k = black_count
                v = white_count
                if v > last_v:
                    last_v = v
                    if v >= k:
                        records.append((k, v))
            black_count += 1

    return records


def solve_dp(n, s):
    """
    Baseline direct O(m^2) DP, where m is the number of records.
    Intended for validation and small cases.
    """
    if n == 0:
        return 1
    if s[0] != 'B' or s[-1] != 'W':
        return 0

    records = extract_records(n, s)
    fact, invfact = get_factorials(n)

    m = len(records)
    t = [0] * (m + 1)
    v = [0] * (m + 1)
    for j, (kj, vj) in enumerate(records, 1):
        t[j] = kj
        v[j] = vj

    dp = [0] * (m + 1)
    dp[0] = 1

    for j in range(1, m + 1):
        vj = v[j]
        total = 0
        for i in range(j):
            total = (total + dp[i] * fact[vj - t[i]]) % MOD
        dp[j] = (-total * invfact[vj - t[j]]) % MOD

    ans = 0
    for i in range(m + 1):
        ans = (ans + dp[i] * fact[n - t[i]]) % MOD
    return ans


def brute_force(n, s):
    """
    Brute-force validator for n <= 8.
    Enumerates all bijections from black vertices to white vertices and checks
    whether every prefix cut 1..2n-1 is covered by at least one added edge
    from the right side to the left side.
    """
    if n == 0:
        return 1
    if s[0] != 'B' or s[-1] != 'W':
        return 0

    whites = []
    blacks = []
    for i, ch in enumerate(s, 1):
        if ch == 'W':
            whites.append(i)
        else:
            blacks.append(i)

    total_vertices = 2 * n
    full_mask = (1 << (total_vertices - 1)) - 1  # cuts 1..2n-1

    masks = []
    for b in blacks:
        row = []
        for w in whites:
            if w > b:
                # Edge w -> b covers cuts b, b+1, ..., w-1.
                row.append(((1 << (w - b)) - 1) << (b - 1))
            else:
                row.append(0)
        masks.append(row)

    count = 0
    for perm in permutations(range(n)):
        cover = 0
        for k, wi in enumerate(perm):
            cover |= masks[k][wi]
            if cover == full_mask:
                break
        if cover == full_mask:
            count += 1

    return count % MOD


def self_test():
    import random

    random.seed(123456789)

    # A few fixed small cases, validated by brute force.
    fixed = [
        (1, "BW"),
        (1, "WB"),
        (2, "BWBW"),
        (2, "BBWW"),
        (2, "WWBB"),
        (3, "BWWBWB"),
        (3, "BBWWBW"),
        (4, "BWWBWBWB"),
    ]
    for n, s in fixed:
        a = solve_dp(n, s)
        b = brute_force(n, s)
        if a != b:
            print(f"mismatch fixed: n={n} s={s} dp={a} brute={b}")
            return False

    # Random balanced strings.
    for n in range(1, 7):
        for _ in range(100):
            arr = ['B'] * (2 * n)
            for pos in random.sample(range(2 * n), n):
                arr[pos] = 'W'
            s = ''.join(arr)
            a = solve_dp(n, s)
            b = brute_force(n, s)
            if a != b:
                print(f"mismatch random: n={n} s={s} dp={a} brute={b}")
                return False

    for _ in range(20):
        n = 7
        arr = ['B'] * (2 * n)
        for pos in random.sample(range(2 * n), n):
            arr[pos] = 'W'
        s = ''.join(arr)
        a = solve_dp(n, s)
        b = brute_force(n, s)
        if a != b:
            print(f"mismatch random: n={n} s={s} dp={a} brute={b}")
            return False

    for _ in range(5):
        n = 8
        arr = ['B'] * (2 * n)
        for pos in random.sample(range(2 * n), n):
            arr[pos] = 'W'
        s = ''.join(arr)
        a = solve_dp(n, s)
        b = brute_force(n, s)
        if a != b:
            print(f"mismatch random: n={n} s={s} dp={a} brute={b}")
            return False

    print("self test ok")
    return True


def main():
    data = sys.stdin.read().split()
    if not data:
        return

    # Optional local validation mode. Not used by normal judge input.
    if data[0] == "SELFTEST":
        self_test()
        return

    n = int(data[0])
    s = data[1].strip()
    print(solve_dp(n, s))


if __name__ == "__main__":
    main()
```

### NOTES
- **Reduction:** The original path gives forward reachability. Strong connectivity is equivalent to every prefix cut `{1..i}` having at least one added edge from the right side to the left side. This immediately forces `S[0] == 'B'` and `S[-1] == 'W'`.
- **Permutation form:** Let black vertices be `B_1 < ... < B_N` and white vertices be `W_1 < ... < W_N`. For a pairing, let `π_k` be the white rank matched to the `k`-th black. For a prefix with `K` blacks and `Q` whites, the needed condition is `max(π_1..π_K) > Q`.
- **Prefix parameters:** For fixed `K`, the worst `Q` occurs just before the next black. Define `p_K = #white vertices before B_{K+1}` for `K=1..N-1`. The constraints are `max(π_1..π_K) >= p_K + 1`.
- **Record pruning:** `p_K` is nondecreasing. For each newly seen value `v`, only the earliest `K` matters. If `v < K`, the constraint is automatic because the maximum of `K` distinct positive integers is at least `K`. Thus records are `(t_j, v_j)` where `v_j` is a new `p` value and `v_j >= t_j`. Both `t_j` and `v_j` strictly increase.
- **Inclusion-exclusion DP:** Bad event `E_j` is “first `t_j` black ranks are all in `[1..v_j]`.” For a subset of records ending at `j`, the count factorizes into falling factorials. This yields `dp_0 = 1` and  
  `dp_j = -invfact[v_j - t_j] * sum_{i<j} dp_i * fact[v_j - t_i]`.  
  The answer is `sum_{i=0}^m dp_i * fact[N - t_i]`.
- **Baseline complexity:** The implemented DP is direct `O(m^2)`, with `m <= N`. It is correct but too slow for `N = 2e5`; it is intended for validation and small cases.
- **Sample 3 check:** For the sample 3 string, the records are `(1,2), (2,3), (5,5), (6,6), (8,8)`. The recurrence gives `240792`. A previous manual arithmetic slip omitted the `(6,6)` event and produced `242280`; including all records matches the sample.
- **Brute force:** For `N <= 8`, enumerate all permutations assigning white indices to black order. An added edge `w -> b` covers prefix cuts `b..w-1` iff `w > b`. Use a bitmask over cuts `1..2N-1`; a pairing is valid iff the OR of its edge masks equals the full mask.
- **Self-test:** `self_test()` compares `solve_dp` and `brute_force` on fixed small cases and random balanced strings. It is not run by default; it can be triggered by using a special first token in local testing.
- **Modulo:** All DP values are maintained modulo `998244353`. Factorials and inverse factorials up to `N` are precomputed once per `N` and cached.

### NEXT
- Implement CDQ divide-and-conquer with NTT to compute the triangular recurrence in near-linear time for `N` up to `2e5`.
- Choose a small-block threshold where direct convolution is faster than NTT.
- Optimize the Python NTT implementation for modulus `998244353`.
- Run broader random validation against brute force for small `N` and verify all samples.

### STATUS
continue
