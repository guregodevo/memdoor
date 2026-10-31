### CODE
```python
import sys

NEG = -(1 << 80)


def run_dp(base, items, NEG=NEG):
    a0 = 0
    a1 = a2 = a3 = a4 = a5 = a6 = a7 = NEG

    for x, y, z in items:
        w0 = x - base
        w1 = y - base
        w2 = z - base

        n0 = a0
        t = a1 + w0
        if t > n0:
            n0 = t
        t = a2 + w1
        if t > n0:
            n0 = t
        t = a4 + w2
        if t > n0:
            n0 = t

        n1 = a1
        t = a0 + w0
        if t > n1:
            n1 = t
        t = a3 + w1
        if t > n1:
            n1 = t
        t = a5 + w2
        if t > n1:
            n1 = t

        n2 = a2
        t = a3 + w0
        if t > n2:
            n2 = t
        t = a0 + w1
        if t > n2:
            n2 = t
        t = a6 + w2
        if t > n2:
            n2 = t

        n3 = a3
        t = a2 + w0
        if t > n3:
            n3 = t
        t = a1 + w1
        if t > n3:
            n3 = t
        t = a7 + w2
        if t > n3:
            n3 = t

        n4 = a4
        t = a5 + w0
        if t > n4:
            n4 = t
        t = a6 + w1
        if t > n4:
            n4 = t
        t = a0 + w2
        if t > n4:
            n4 = t

        n5 = a5
        t = a4 + w0
        if t > n5:
            n5 = t
        t = a7 + w1
        if t > n5:
            n5 = t
        t = a1 + w2
        if t > n5:
            n5 = t

        n6 = a6
        t = a7 + w0
        if t > n6:
            n6 = t
        t = a4 + w1
        if t > n6:
            n6 = t
        t = a2 + w2
        if t > n6:
            n6 = t

        n7 = a7
        t = a6 + w0
        if t > n7:
            n7 = t
        t = a5 + w1
        if t > n7:
            n7 = t
        t = a3 + w2
        if t > n7:
            n7 = t

        a0 = n0
        a1 = n1
        a2 = n2
        a3 = n3
        a4 = n4
        a5 = n5
        a6 = n6
        a7 = n7

    return a0


def main():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    idx = 0
    T = data[idx]
    idx += 1

    out = []
    out_append = out.append
    dp = run_dp

    for _ in range(T):
        N = data[idx]
        K = data[idx + 1]
        idx += 2

        if K == 1:
            t01 = t02 = t11 = t12 = t21 = t22 = 0
            for _ in range(N):
                x = data[idx]
                y = data[idx + 1]
                z = data[idx + 2]
                idx += 3

                if x > t01:
                    t02 = t01
                    t01 = x
                elif x > t02:
                    t02 = x

                if y > t11:
                    t12 = t11
                    t11 = y
                elif y > t12:
                    t12 = y

                if z > t21:
                    t22 = t21
                    t21 = z
                elif z > t22:
                    t22 = z

            ans = max(t01 + t02, t11 + t12, t21 + t22)
            out_append(str(ans))
            continue

        shift = N.bit_length()
        s = shift + 1
        items = [None] * N

        t01 = t02 = t11 = t12 = t21 = t22 = 0

        for i in range(N):
            x = data[idx]
            y = data[idx + 1]
            z = data[idx + 2]
            idx += 3

            if x > t01:
                t02 = t01
                t01 = x
            elif x > t02:
                t02 = x

            if y > t11:
                t12 = t11
                t11 = y
            elif y > t12:
                t12 = y

            if z > t21:
                t22 = t21
                t21 = z
            elif z > t22:
                t22 = z

            items[i] = (x << s) + 1, (y << s) + 1, (z << s) + 1

        max_pair = max(t01 + t02, t11 + t12, t21 + t22)

        if max_pair == 0:
            out_append("0")
            continue

        if K == N // 2:
            enc = dp(0, items)
            out_append(str(enc >> s))
            continue

        target = K << 1
        lo = 0
        hi = max_pair + 1
        mask = (1 << shift) - 1

        while lo + 1 < hi:
            mid = (lo + hi) >> 1
            if dp(mid << shift, items) & mask >= target:
                lo = mid
            else:
                hi = mid

        enc = dp(lo << shift, items)
        ans = (enc >> s) + lo * K
        out_append(str(ans))

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    main()
```

### NOTES
- **Reduction:** For each pair, choose one coordinate attaining the pair maximum and color both cakes with that coordinate. Then every color class has even size and the colored sum equals the pair price. Conversely, any selection of 2K cakes with even counts per color can be paired inside each color, and the true pair price is at least the colored sum. Hence the problem is exactly maximum-weight even-parity colored selection of 2K cakes.
- **Concavity:** The optimum as a function of the number of pairs is concave; it is the maximum-weight matching value in a complete graph with these edge weights, and such cardinality-value functions are concave. Therefore a Lagrangian penalty per pair can be binary-searched.
- **Penalty scaling:** Use an integer beta per pair. Double all coordinate values and subtract beta for every selected cake. The penalized score is 2 * colored_sum - beta * selected_count. If the optimal selected count is at least 2K at the largest feasible beta, the answer is penalized_score / 2 + beta * K.
- **8-state DP:** State is a 3-bit parity mask for X, Y, Z selected counts. For each cake, either skip it or select it as X/Y/Z, toggling bit 1/2/4. The transition is max-plus over 8 states. The implementation is fully unrolled for speed.
- **Tie-breaking and encoding:** Among equal penalized scores, the DP must maximize selected count. Encode (score, count) as score * 2^shift + count, where shift = N.bit_length(), so count < 2^shift. Precompute each item value as (2 * value << shift) + 1. For beta, base = beta << shift and item weight = precomputed - base. Final count is enc & ((1 << shift) - 1). Since final score is even, score / 2 is enc >> (shift + 1).
- **Search bounds:** beta = 0 is always valid because all values are nonnegative and the maximum feasible even count is at least 2K. An upper bound is max_pair + 1, where max_pair is the maximum single-pair price, computed as the maximum over colors of the top two values in that color. Every marginal pair value is at most max_pair, so beta = max_pair + 1 makes count 0 optimal.
- **Special cases:** K = 1 is answered directly by top two values per color. K = floor(N / 2) is answered by one DP pass at beta = 0, because the maximum feasible count is optimal when all values are nonnegative. If max_pair = 0, the answer is 0.
- **Performance:** Sum of N is 1e5. The general case uses at most about 31 DP passes, each with 8 unrolled states. Bit encoding avoids multiplication/division in the DP. Invalid states use NEG = -(1 << 80), safely below any valid encoded value.
- **Edge cases:** All-zero values, odd N with maximum K, negative intermediate penalized scores, and tie-breaking at zero marginal value are all handled by the parity DP and count tie-break.
- **Validation:** The logic matches the provided samples and agrees with brute-force matching/colored-selection checks on tiny random instances.

### NEXT
- none

### STATUS
solved
