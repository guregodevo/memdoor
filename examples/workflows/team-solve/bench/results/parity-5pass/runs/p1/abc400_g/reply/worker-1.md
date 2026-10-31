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
- **Reduction:** Charge each pair to one attribute attaining its maximum. Both cakes in that pair receive that color. Thus a solution is exactly 2K cakes colored with 3 colors, each color count even, maximizing the sum of the chosen color’s value. Pairing inside colors gives price at least that sum, and the reverse direction gives equality.
- **Baseline:** Let b_i = max(X_i, Y_i, Z_i), and choose a base color a_i attaining b_i. The global upper bound B is the sum of the largest 2K b_i. Take those cakes as selected S, the rest U. The baseline colored sum is B, and parity mask m has bit c if the number of selected base-color c cakes is odd. If m = 0, B is feasible and optimal.
- **Local loss operations:** Any feasible solution differs from the baseline by recolors and swaps. Recolor selected i from a to c: loss b_i - v_{i,c}, delta bit(a) xor bit(c). Swap selected i (base a) out and unselected j (colored c) in: loss b_i - v_{j,c}, same delta. All losses are nonnegative because selected b values dominate unselected b values and v <= b.
- **At most two operations:** Nonzero deltas are 3, 5, 6, forming a 2-dimensional F2 space. The XOR of all operation deltas must equal m. Looking at parity of how many times each delta appears, a subcollection of one or two operations has the same XOR. Dropping the rest cannot increase loss, so an optimal correction uses 0, 1, or 2 disjoint operations. For m nonzero, the two-operation masks are the other two nonzero masks.
- **Candidate pools:** Keep top 5 selected items for each directed recolor role (a, c) by loss, top 5 selected items for each swap-out base a by smallest b, and top 5 unselected items for each swap-in color c by largest value. Top 5 is safe because in any two-operation plan each role can conflict with at most one item id from another role; even if the same unselected item is top for two colors, one replacement per role is enough. Expand each swap role pair into all out/in pairs to form operation candidates.
- **Evaluation:** Build ops[mask] from recolor candidates and swap candidate pairs. The answer is B minus the minimum cost of one op with mask m or two disjoint ops with the other two masks. Disjointness is checked by original item ids.
- **Edge cases:** If U is empty, swap lists are empty and only recolors are used. Ties in b at the 2K boundary or ties for the maximum attribute are handled by zero-loss swaps/recolors; arbitrary tie-breaking is fine. Since 2K is even, m is 0 or one of 3, 5, 6. Python ints handle totals up to about 1e14.
- **Complexity:** Sorting dominates: O(N log N) per test case, total O(sum N log N); candidate evaluation is a small constant per test case.

### NEXT
none

### STATUS
solved
