### CODE
```python
import sys

def solve():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    t = data[0]
    pos = 1
    out = []
    INF = 10**30

    for _ in range(t):
        n = data[pos]
        k = data[pos + 1]
        pos += 2
        s = 2 * k

        items = []
        for i in range(n):
            x = data[pos]
            y = data[pos + 1]
            z = data[pos + 2]
            pos += 3

            if x >= y and x >= z:
                m = x
                c = 0
            elif y >= x and y >= z:
                m = y
                c = 1
            else:
                m = z
                c = 2

            items.append((m, x, y, z, c, i))

        items.sort(key=lambda e: e[0], reverse=True)

        selected = items[:s]
        unselected = items[s:]

        total = 0
        parity = 0
        for e in selected:
            total += e[0]
            parity ^= 1 << e[4]

        if parity == 0:
            out.append(str(total))
            continue

        # Top 3 selected cakes of each baseline color, by smallest m.
        sel_by_base = [[], [], []]
        for e in selected:
            sel_by_base[e[4]].append((e[0], e[5]))
        for a in range(3):
            if len(sel_by_base[a]) > 3:
                sel_by_base[a].sort()
                del sel_by_base[a][3:]

        # Top 3 selected cakes for each ordered recolor a -> b, by smallest loss.
        recolor = [[[] for _ in range(3)] for __ in range(3)]
        for m, x, y, z, c, i in selected:
            if c != 0:
                recolor[c][0].append((m - x, i))
            if c != 1:
                recolor[c][1].append((m - y, i))
            if c != 2:
                recolor[c][2].append((m - z, i))

        for a in range(3):
            for b in range(3):
                if a == b:
                    continue
                lst = recolor[a][b]
                if len(lst) > 3:
                    lst.sort()
                    del lst[3:]

        # Top 3 unselected cakes for each target color b, by largest value.
        uns_by_target = [[], [], []]
        for m, x, y, z, c, i in unselected:
            uns_by_target[0].append((x, i))
            uns_by_target[1].append((y, i))
            uns_by_target[2].append((z, i))

        for b in range(3):
            lst = uns_by_target[b]
            if len(lst) > 3:
                lst.sort(reverse=True)
                del lst[3:]

        # Generate candidate operations:
        # (mask, cost, selected_id, unselected_id)
        ops = []

        # Recolor operations.
        for a in range(3):
            bita = 1 << a
            for b in range(3):
                if a == b:
                    continue
                mask = bita ^ (1 << b)
                for loss, i in recolor[a][b]:
                    ops.append((mask, loss, i, -1))

        # Swap operations.
        for a in range(3):
            bita = 1 << a
            for b in range(3):
                if a == b:
                    continue
                mask = bita ^ (1 << b)
                for m_i, i in sel_by_base[a]:
                    for val_j, j in uns_by_target[b]:
                        ops.append((mask, m_i - val_j, i, j))

        best = INF

        # One operation fixing the parity.
        for mask, cost, si, uj in ops:
            if mask == parity and cost < best:
                best = cost

        # Two operations whose masks XOR to the needed parity.
        L = len(ops)
        for i in range(L):
            m1, c1, s1, u1 = ops[i]
            for j in range(i + 1, L):
                m2, c2, s2, u2 = ops[j]
                if (m1 ^ m2) != parity:
                    continue

                cost = c1 + c2
                if cost >= best:
                    continue

                # The two operations must use disjoint cakes.
                if s1 == s2:
                    continue
                if u1 != -1 and u1 == u2:
                    continue

                best = cost

        out.append(str(total - best))

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    solve()
```

### NOTES
- **Validation:** Cross-checked against an exhaustive pairing brute force for N <= 8, all valid K, values 0..3, tie-heavy cases, all-zero values, N=2K, and K=1. No mismatch was found, so the top-3 parity-correction implementation is kept as is.
- **Reduction:** A pairing can be converted to a coloring by coloring both cakes in each pair with a coordinate attaining the pair maximum. Each color count is even, and the coloring score equals the pair total. Conversely, any even-count coloring can be paired inside each color, giving pair price at least the colored sum. Thus the problem equals maximizing colored sum under even color counts.
- **Baseline upper bound:** Let m_i be the maximum of the three values. Any feasible solution uses 2K cakes and scores at most the sum of their m_i, so the top 2K m_i sum W0 is a global upper bound. Color each selected cake by one maximizing attribute as the baseline.
- **Parity masks:** Use bits 1,2,4 for X,Y,Z. Since 2K is even, a nonzero parity mask has exactly two bits set. A recolor a->b or a swap removing baseline color a and adding target color b toggles mask bit(a)^bit(b).
- **At most two operations:** Any feasible final solution can be represented from the baseline by recolors of kept selected cakes and swaps pairing removed selected cakes with added unselected cakes. All operation costs are nonnegative. The XOR of all operation masks is the baseline parity p. In the 3-color triangle, any multiset of edge masks with XOR p contains a submultiset of size 1 or 2 with the same XOR. Taking that submultiset keeps cakes disjoint and does not increase loss, so the optimum loss is achieved by one or two operations.
- **Top-3 sufficiency:** At most two operations are used. A selected-base list, recolor list, or unselected-target list can be involved in at most two needed cakes, and a different operation can conflict with at most one candidate in that list. Keeping top 3 by the relevant criterion (smallest m for swaps out, smallest recolor loss, largest value for swaps in) is therefore enough.
- **Enumeration:** Recolor candidates come from top-3 ordered recolor lists. Swap candidates are Cartesian products of top-3 selected-by-base and top-3 unselected-by-target lists. One operation is valid if its mask equals p; two operations are valid if their masks XOR to p and they do not share a selected or unselected cake.
- **Ties and cutoffs:** Arbitrary top-2K selection among equal m is safe. Zero-cost recolors for tied maximizing attributes and zero-cost swaps among equal-cutoff cakes are included, so if another top-2K set or color choice attains W0 with even parity, the minimum loss becomes 0.
- **Edge cases:** N=2K leaves no unselected cakes, so only recolors are used. K=1, all-zero values, and large totals are handled by Python integers. Complexity is O(N log N) per test case plus constant enumeration; sum N is at most 1e5.

### NEXT
- none

### STATUS
solved
