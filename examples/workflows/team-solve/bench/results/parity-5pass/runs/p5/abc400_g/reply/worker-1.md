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
- **Reduction:** For every pair, choose the coordinate that attains the pair maximum and color both cakes with that coordinate. Each color then appears an even number of times. Conversely, any selection of 2K cakes with even color counts can be paired inside each color, and each pair's true price is at least the sum of that color, so the original problem equals maximizing the sum of chosen color values under even color counts.
- **Baseline upper bound:** Let m_i be the max of the three values. Any feasible solution uses 2K cakes and gives at most the sum of their m_i, so the top 2K m_i sum W0 is a global upper bound. Take those 2K cakes as baseline and color each by one maximizing attribute. If the parity mask of these colors is zero, W0 is achievable.
- **Parity masks:** Use bits 1,2,4 for X,Y,Z. Because 2K is even, a nonzero mask has exactly two bits set. An operation that changes one cake's color from a to b toggles mask bit(a)^bit(b).
- **Elementary corrections:** Recolor selected cake i from its baseline color a to b: cost m_i - value_i[b]. Swap selected cake i (baseline a) with unselected cake j colored b: cost m_i - value_j[b]. Both costs are nonnegative because selected m_i is at least unselected m_j and values are at most m.
- **At most two operations:** Any feasible final solution can be reached from the baseline by such operations with total cost W0 - final_value. The XOR of their masks is the baseline parity p. In a triangle of three colors, any multiset of edge masks with XOR p contains a submultiset of size 1 (the edge p) or size 2 (the other two edges) with the same XOR. Taking that submultiset keeps resources disjoint and does not increase cost, so the optimum loss is the minimum over one or two non-conflicting operations.
- **Candidate lists:** Keep top 3 selected cakes of each baseline color by smallest m (for swaps out), top 3 selected cakes for each ordered recolor a->b by smallest m-value[b], and top 3 unselected cakes for each target b by largest value[b]. This is sufficient because at most two operations are used; any one list can contribute at most two needed cakes, and a conflict can eliminate at most one candidate from that list. Ties can be broken arbitrarily.
- **Enumeration:** Generate all recolor operations from the recolor lists and all swap operations by combining selected-base and unselected-target lists. A one-operation candidate is valid if its mask equals p. A two-operation candidate is valid if the two masks XOR to p and the operations do not share a selected id or an unselected id. Answer is W0 - minimum cost.
- **Ties and cutoffs:** The arbitrary top-2K selection is safe. Zero-cost recolors to tied maximizing attributes and zero-cost swaps among cakes with equal cutoff m are included, so if another top-2K set or color choice attains W0 with even parity, the minimum loss becomes 0.
- **Edge cases:** N=2K leaves no unselected cakes, so swap lists are empty and recolors alone fix parity. K=1, all-zero values, and large totals are handled by Python ints. Complexity is O(N log N) per test case plus constant enumeration; sum N is 1e5.

### NEXT
none

### STATUS
solved
