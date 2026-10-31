### CODE
```python
import sys


def main():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    it = iter(data)
    T = next(it)
    out_lines = []

    bits = (1, 2, 4)
    other_pair = {3: (5, 6), 5: (3, 6), 6: (3, 5)}

    for _ in range(T):
        N = next(it)
        K = next(it)
        M = 2 * K

        items = []
        for i in range(N):
            x = next(it)
            y = next(it)
            z = next(it)
            b = max(x, y, z)
            if x == b:
                a = 0
            elif y == b:
                a = 1
            else:
                a = 2
            items.append((b, x, y, z, a, i))

        items.sort(key=lambda t: t[0], reverse=True)

        selected = items[:M]
        unselected = items[M:]

        B = 0
        p0 = p1 = p2 = 0
        for b, x, y, z, a, i in selected:
            B += b
            if a == 0:
                p0 ^= 1
            elif a == 1:
                p1 ^= 1
            else:
                p2 ^= 1

        m = p0 * 1 + p1 * 2 + p2 * 4

        if m == 0:
            out_lines.append(str(B))
            continue

        recolor = [[[] for _ in range(3)] for __ in range(3)]
        swapout = [[] for _ in range(3)]
        swapin = [[] for _ in range(3)]

        for b, x, y, z, a, i in selected:
            swapout[a].append((b, i))
            if a != 0:
                recolor[a][0].append((b - x, i))
            if a != 1:
                recolor[a][1].append((b - y, i))
            if a != 2:
                recolor[a][2].append((b - z, i))

        for b, x, y, z, a, i in unselected:
            swapin[0].append((x, i))
            swapin[1].append((y, i))
            swapin[2].append((z, i))

        for a in range(3):
            for c in range(3):
                if a != c:
                    lst = recolor[a][c]
                    lst.sort(key=lambda p: p[0])
                    if len(lst) > 5:
                        del lst[5:]

            lst = swapout[a]
            lst.sort(key=lambda p: p[0])
            if len(lst) > 5:
                del lst[5:]

        for c in range(3):
            lst = swapin[c]
            lst.sort(key=lambda p: p[0], reverse=True)
            if len(lst) > 5:
                del lst[5:]

        ops = {3: [], 5: [], 6: []}

        for a in range(3):
            for c in range(3):
                if a == c:
                    continue
                d = bits[a] ^ bits[c]

                for cost, idx in recolor[a][c]:
                    ops[d].append((cost, (idx,)))

                out_list = swapout[a]
                in_list = swapin[c]
                if out_list and in_list:
                    for b_out, idx_out in out_list:
                        for val_in, idx_in in in_list:
                            ops[d].append((b_out - val_in, (idx_out, idx_in)))

        INF = 10**30
        best = INF

        for cost, _ in ops[m]:
            if cost < best:
                best = cost

        if best != 0:
            d1, d2 = other_pair[m]
            list1 = ops[d1]
            list2 = ops[d2]

            for cost1, ids1 in list1:
                if len(ids1) == 1:
                    id1 = ids1[0]
                    for cost2, ids2 in list2:
                        if len(ids2) == 1:
                            if id1 != ids2[0]:
                                total = cost1 + cost2
                                if total < best:
                                    best = total
                        else:
                            if id1 != ids2[0] and id1 != ids2[1]:
                                total = cost1 + cost2
                                if total < best:
                                    best = total
                else:
                    id1a, id1b = ids1
                    for cost2, ids2 in list2:
                        if len(ids2) == 1:
                            if id1a != ids2[0] and id1b != ids2[0]:
                                total = cost1 + cost2
                                if total < best:
                                    best = total
                        else:
                            id2a, id2b = ids2
                            if (id1a != id2a and id1a != id2b and
                                id1b != id2a and id1b != id2b):
                                total = cost1 + cost2
                                if total < best:
                                    best = total

        out_lines.append(str(B - best))

    sys.stdout.write("\n".join(out_lines))


if __name__ == "__main__":
    main()
```

### NOTES
- **Reduction:** A pair can be charged to the attribute attaining its maximum, coloring both cakes with that attribute. Thus a feasible solution is exactly 2K distinct cakes colored 0/1/2 with each color count even, maximizing the sum of chosen attribute values. Pairing same-colored cakes gives price at least that sum, and charging an optimal pair solution gives equality.
- **Baseline:** Let b_i be the maximum attribute value of cake i and a_i one maximizing attribute. The sum of the largest 2K b_i values is a global upper bound B. Take those cakes as selected S and the rest as U. Coloring S by a_i gives sum B and a parity mask m, where bit c is set if the number of selected cakes with base color c is odd. If m is zero, B is achievable and optimal.
- **Operation model:** Any other feasible colored selection differs from the baseline by recoloring selected cakes and swapping selected cakes out with unselected cakes in. A recolor of selected i from base a to color c has loss b_i - value_i_c and parity delta bit(a) xor bit(c). A swap of selected i out and unselected j in with color c has loss b_i - value_j_c and the same delta. All losses are nonnegative because selected b values dominate unselected b values and every value is at most its b.
- **Two-operation lemma:** The nonzero deltas are 3, 5, and 6, forming a 2-bit vector space. For any multiset of operations whose xor is m, the parity of operation counts has either one or two odd deltas, so a subcollection of one or two operations has the same xor. Dropping the remaining operations cannot increase loss, and operations coming from one valid solution are disjoint. Therefore an optimal correction uses at most two disjoint operations. For nonzero m, the two-operation masks are the other two nonzero masks.
- **Candidate truncation:** For one operation, the best recolor is the minimum loss in its directed role, and the best swap is the minimum selected b minus the maximum unselected value in the target color. For two operations, each role can conflict with at most one item id from the other operation: one selected id for recolor or swap-out roles, and one unselected id for swap-in roles. Keeping the top 5 items per role is safe: if an optimal item is outside the top 5, at least four top items are available and no more expensive. Top 2 would already suffice, and 5 is a safe margin. All top swap-out/swap-in pairs are generated, so the optimal disjoint pair is present.
- **Stress checks:** Compared against a brute-force colored-selection checker on small N up to 8/10, all K, values 0..5, large tied values, N=2, K=N/2, all equal values, and ties at the 2K boundary. No mismatches were found. Targeted cases include N=2 with cakes (100,0,99) and (0,100,99), where the optimum 198 requires two recolors to the third color, and boundary-tie cases where zero-cost swaps change the selected set.
- **Edge cases:** If U is empty, swap lists are empty and only recolors are used. If m is zero, the answer is B immediately. For nonzero m, one operation always exists by recoloring one odd base color to the other odd base color, so best is finite. Ties in b or in the maximizing attribute are handled by zero-loss swaps/recolors; arbitrary tie-breaking is safe.
- **Complexity:** Sorting dominates: O(N log N) per test case and O(sum N log N) overall. Candidate generation and two-operation evaluation are constant per test case, with at most a few dozen operations per mask and a few thousand pair checks. Python integers safely handle totals up to about 1e14.

### NEXT
none

### STATUS
solved
