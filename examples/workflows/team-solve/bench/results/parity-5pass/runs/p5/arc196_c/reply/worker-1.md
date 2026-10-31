### CODE
```python
import sys
from itertools import permutations, combinations
import random

MOD = 998244353


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
            if 1 <= b - 1 <= n - 1:
                h[b - 1] = w
    h[n] = n
    return h


def solve(n, s, fact=None, invfact=None):
    if n == 0:
        return 1
    if s[0] == 'W' or s[-1] == 'B':
        return 0
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
            r[j] = 0
            continue

        total = f[hj]
        for i in range(1, j):
            ri = r[i]
            if ri:
                total += ri * f[hj - i]

        r[j] = (-total % mod) * inv[hj - j] % mod

    return (-r[n]) % mod


def brute_count_direct(n, s):
    whites = [i for i, ch in enumerate(s, 1) if ch == 'W']
    blacks = [i for i, ch in enumerate(s, 1) if ch == 'B']
    cnt = 0

    for perm in permutations(whites):
        ok = True
        # perm[t] is the white vertex paired with blacks[t].
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
    samples = [
        (2, 'BWBW', 1),
        (4, 'BWWBWBWB', 0),
        (9, 'BWWBWBBBWWBWBBWWBW', 240792),
    ]

    for n, s, expected in samples:
        got = solve(n, s)
        if got != expected % MOD:
            print(f"FAIL sample n={n} s={s} got={got} expected={expected}")
            return

    # Exhaustive small strings: compare with direct prefix brute force.
    for n in range(1, 6):
        fact, invfact = prepare_fact(n)
        length = 2 * n
        for combo in combinations(range(length), n):
            arr = ['B'] * length
            for pos in combo:
                arr[pos] = 'W'
            s = ''.join(arr)

            expected = brute_count_direct(n, s)
            got = solve(n, s, fact, invfact)
            if expected != got:
                print(f"FAIL exhaustive n={n} s={s} brute={expected} solver={got}")
                return

    # Random slightly larger strings, still brute-forceable.
    random.seed(123456789)
    n = 6
    fact, invfact = prepare_fact(n)
    for _ in range(100):
        chars = ['W'] * n + ['B'] * n
        random.shuffle(chars)
        s = ''.join(chars)

        expected = brute_count_direct(n, s)
        got = solve(n, s, fact, invfact)
        if expected != got:
            print(f"FAIL random n={n} s={s} brute={expected} solver={got}")
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

    n = int(data[0])
    s = data[1].strip()
    print(solve(n, s))


if __name__ == '__main__':
    main()
```

### NOTES
- **Strong connectivity reduction:** The fixed edges form the directed path `1 -> 2 -> ... -> 2N`. The final graph is strongly connected iff every proper prefix `[1..k]` has at least one added edge from the suffix `{k+1..2N}` into the prefix. Necessity is immediate. Sufficiency: if some `i` cannot reach `j < i`, let `x` be the smallest vertex reachable from `i`. Since `i` reaches all larger vertices by the path, `x <= i`. If `x > j`, prefix `x-1` has no reachable vertex, but it must have an incoming edge `u -> v` with `u >= x` and `v < x`; then `u` is reachable and `v` becomes reachable, contradiction. Hence `x <= j` and `j` is reachable.

- **Pairing as permutation:** Order black vertices `b_1 < ... < b_N` and white vertices `w_1 < ... < w_N`. A pairing is a permutation `pi` where black `b_j` is paired with white `w_{pi(j)}`. A prefix containing `a` black vertices and `c` white vertices is closed iff `pi(1), ..., pi(a)` are all at most `c`, i.e. `M_a = max_{j<=a} pi(j) <= c`.

- **Boundary `h`:** For each `a = 1..N-1`, the hardest prefix with exactly `a` black vertices is just before the `(a+1)`-st black vertex. Define `h[a]` as the number of whites before that black vertex. The condition becomes `M_a > h[a]` for all `a = 1..N-1`. Also, if `S[0] == 'W'`, the first prefix has no black vertex and is impossible; if `S[-1] == 'B'`, a proper prefix contains all whites but not all blacks and is impossible. These are early exits. Set `h[N] = N` for the final inclusion-exclusion trick.

- **Inclusion-exclusion:** Let bad event `E_a` be `M_a <= h[a]`. For selected indices `a_1 < ... < a_m`, the number of permutations satisfying all selected bad events factorizes as `(N-a_m)! * product_{r=1..m} (h[a_r] - a_{r-1})_{a_r-a_{r-1}}`, where `a_0 = 0` and `(x)_k` is the falling factorial. This uses monotonicity of `h`. Define `R[0] = 1`, and let `R[j]` be the signed sum over selected sequences ending at `j` of the product part without the final `(N-j)!` factor.

- **Recurrence:** For `j >= 1`,
  `R[j] = - sum_{i=0}^{j-1} R[i] * fact[h[j]-i] / fact[h[j]-j]`,
  with `R[j] = 0` if `h[j] < j`. Equivalently,
  `R[j] = (-fact[h[j]] - sum_{i=1}^{j-1} R[i] * fact[h[j]-i]) * invfact[h[j]-j] mod M`.
  The good count is `sum_{j=0}^{N-1} R[j] * fact[N-j]`. With `h[N] = N`, one gets `R[N] = -sum_{i=0}^{N-1} R[i] * fact[N-i]`, so the answer is `(-R[N]) mod M`.

- **Current implementation:** The solver uses the direct `O(N^2)` recurrence. It precomputes factorials and inverse factorials, scans the string to build `h`, and computes `R[1..N]`. The inner sum is accumulated as a Python integer and reduced modulo once per `j`; this is fine for validation and small/medium `N`, but not for the full `2e5` constraint.

- **Validation included:** A self-test mode is included, triggered by `--selftest` as an argument or by `SELFTEST` as the first stdin token. It compares the solver against direct brute force over all matchings and all proper prefixes for all strings with `N = 1..5`, 100 random strings with `N = 6`, and the three samples. It is not run on normal judge input.

- **Remaining performance issue:** The recurrence is correct but quadratic. The term `sum_{i<j} R[i] * fact[h[j]-i]` is the bottleneck. Since `h[j]` is nondecreasing, this can be accelerated with a divide-and-conquer online convolution approach using a number-theoretic transform. Small blocks or sparse jumps in `h` can be handled directly.

### NEXT
- Replace the quadratic inner sum with a divide-and-conquer convolution method using a number-theoretic transform.
- Handle the nondecreasing `h[j]` values carefully: for each divide-and-conquer step, add contributions from the left half to the right half by convolving known `R[i]` values with factorial values over the needed `h` range.
- Add a direct-loop fallback for small blocks or sparse `h` ranges to avoid transform overhead.
- Stress-test the optimized solver against the current `O(N^2)` solver for random small and medium `N`.
- Submit the optimized full-constraint solver after validation.

### STATUS
continue
