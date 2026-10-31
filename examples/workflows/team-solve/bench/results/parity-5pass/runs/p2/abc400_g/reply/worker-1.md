### CODE
```python
import sys

INF = 10**30


def add_min(lst, val, idx):
    if not lst:
        lst.append((val, idx))
        return
    if len(lst) == 1:
        if lst[0][1] == idx:
            if val < lst[0][0]:
                lst[0] = (val, idx)
            return
        if val < lst[0][0]:
            lst.insert(0, (val, idx))
        else:
            lst.append((val, idx))
        return

    if lst[0][1] == idx:
        if val < lst[0][0]:
            lst[0] = (val, idx)
            if lst[0][0] > lst[1][0]:
                lst[0], lst[1] = lst[1], lst[0]
        return
    if lst[1][1] == idx:
        if val < lst[1][0]:
            lst[1] = (val, idx)
            if lst[0][0] > lst[1][0]:
                lst[0], lst[1] = lst[1], lst[0]
        return

    if val < lst[1][0]:
        if val < lst[0][0]:
            lst[1] = lst[0]
            lst[0] = (val, idx)
        else:
            lst[1] = (val, idx)


def add_max(lst, val, idx):
    if not lst:
        lst.append((val, idx))
        return
    if len(lst) == 1:
        if lst[0][1] == idx:
            if val > lst[0][0]:
                lst[0] = (val, idx)
            return
        if val > lst[0][0]:
            lst.insert(0, (val, idx))
        else:
            lst.append((val, idx))
        return

    if lst[0][1] == idx:
        if val > lst[0][0]:
            lst[0] = (val, idx)
            if lst[0][0] < lst[1][0]:
                lst[0], lst[1] = lst[1], lst[0]
        return
    if lst[1][1] == idx:
        if val > lst[1][0]:
            lst[1] = (val, idx)
            if lst[0][0] < lst[1][0]:
                lst[0], lst[1] = lst[1], lst[0]
        return

    if val > lst[1][0]:
        if val > lst[0][0]:
            lst[1] = lst[0]
            lst[0] = (val, idx)
        else:
            lst[1] = (val, idx)


def combine(a, b):
    if not a or not b:
        return INF
    if a[0][1] != b[0][1]:
        return a[0][0] + b[0][0]

    res = INF
    if len(a) >= 2:
        res = a[1][0] + b[0][0]
    if len(b) >= 2:
        v = a[0][0] + b[1][0]
        if v < res:
            res = v
    return res


def solve():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    pos = 0
    T = data[pos]
    pos += 1
    out = []

    DIR = {
        3: ((0, 1), (1, 0)),
        5: ((0, 2), (2, 0)),
        6: ((1, 2), (2, 1)),
    }

    for _ in range(T):
        N = data[pos]
        K = data[pos + 1]
        pos += 2

        X = [0] * N
        Y = [0] * N
        Z = [0] * N
        M = [0] * N
        P = [0] * N

        for i in range(N):
            x = data[pos]
            y = data[pos + 1]
            z = data[pos + 2]
            pos += 3

            X[i] = x
            Y[i] = y
            Z[i] = z

            m = x
            if y > m:
                m = y
            if z > m:
                m = z
            M[i] = m

            if x == m:
                p = 0
            elif y == m:
                p = 1
            else:
                p = 2
            P[i] = p

        order = list(range(N))
        order.sort(key=M.__getitem__, reverse=True)

        limit = 2 * K
        top2_selM = [[], [], []]
        recolor = [[] for _ in range(8)]

        B = 0
        Tmask = 0

        for idx in range(limit):
            i = order[idx]
            m = M[i]
            p = P[i]

            B += m
            Tmask ^= 1 << p
            add_min(top2_selM[p], m, i)

            if p != 0:
                add_min(recolor[(1 << p) ^ 1], m - X[i], i)
            if p != 1:
                add_min(recolor[(1 << p) ^ 2], m - Y[i], i)
            if p != 2:
                add_min(recolor[(1 << p) ^ 4], m - Z[i], i)

        if Tmask == 0:
            out.append(str(B))
            continue

        unsel = [[], [], []]
        for idx in range(limit, N):
            i = order[idx]
            add_max(unsel[0], X[i], i)
            add_max(unsel[1], Y[i], i)
            add_max(unsel[2], Z[i], i)

        swap = [[] for _ in range(8)]
        for mask in (3, 5, 6):
            for p, q in DIR[mask]:
                if not unsel[q]:
                    continue
                best_u = unsel[q][0][0]
                for m, i in top2_selM[p]:
                    add_min(swap[mask], m - best_u, i)

        loss = INF

        if recolor[Tmask]:
            loss = recolor[Tmask][0][0]
        if swap[Tmask] and swap[Tmask][0][0] < loss:
            loss = swap[Tmask][0][0]

        others = [m for m in (3, 5, 6) if m != Tmask]
        U = others[0]
        V = others[1]

        v = combine(recolor[U], recolor[V])
        if v < loss:
            loss = v

        v = combine(recolor[U], swap[V])
        if v < loss:
            loss = v

        v = combine(swap[U], recolor[V])
        if v < loss:
            loss = v

        for p1, q1 in DIR[U]:
            for p2, q2 in DIR[V]:
                if p1 == p2:
                    if len(top2_selM[p1]) < 2:
                        continue
                    sel = top2_selM[p1][0][0] + top2_selM[p1][1][0]
                else:
                    if not top2_selM[p1] or not top2_selM[p2]:
                        continue
                    sel = top2_selM[p1][0][0] + top2_selM[p2][0][0]

                if q1 == q2:
                    if len(unsel[q1]) < 2:
                        continue
                    uns = unsel[q1][0][0] + unsel[q1][1][0]
                else:
                    if not unsel[q1] or not unsel[q2]:
                        continue
                    if unsel[q1][0][1] != unsel[q2][0][1]:
                        uns = unsel[q1][0][0] + unsel[q2][0][0]
                    else:
                        uns = INF
                        if len(unsel[q1]) >= 2:
                            uns = unsel[q1][1][0] + unsel[q2][0][0]
                        if len(unsel[q2]) >= 2:
                            vv = unsel[q1][0][0] + unsel[q2][1][0]
                            if vv > uns:
                                uns = vv
                        if uns == INF:
                            continue

                cost = sel - uns
                if cost < loss:
                    loss = cost

        out.append(str(B - loss))

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()
```

### NOTES
- **Reduction:** A pairing can be represented by choosing 2K cakes and labeling each cake with one of the three attributes. Each label must appear an even number of times because it is used twice in each pair of that type. The total price equals the sum of labeled attribute values for the labeling obtained by choosing a maximizing attribute for each pair. Conversely, any even-count labeling can be paired inside labels and gives price at least the labeled sum. Thus the problem is exactly maximizing labeled sum under even label counts.
- **Baseline and parity:** Let M_i be the maximum attribute of cake i. Any labeled sum is at most the sum of M_i over chosen cakes, hence at most B, the sum of the largest 2K M_i. Take those 2K cakes as the baseline set S. Label each baseline cake by one maximizing attribute p_i, using first-max tie-breaking. Let Tmask be the xor of bits 1<<p_i over S. Since 2K is even, Tmask is either 0 or one of 3, 5, 6. If Tmask is 0, the baseline labeling is feasible and optimal, so answer is B.
- **Operations:** Any feasible final solution can be compared to the baseline. A kept cake may be recolored from p to q, with cost M_i - value_i(q) and mask bit(p) xor bit(q). A removed baseline cake i and added unselected cake j labeled q form a swap, with cost M_i - value_j(q) and mask bit(p_i) xor bit(q). All costs are nonnegative because every selected M_i is at least every unselected M_j, and values are at most M.
- **At most two operations:** The xor of all operation masks must equal Tmask. There are only three nonzero two-bit masks: 3, 5, 6. Canceling pairs of operations with the same nonzero mask preserves parity and does not increase cost. For Tmask nonzero, the remaining parity can be represented by either one operation of mask Tmask or two operations with the other two masks. Therefore the minimum loss is the minimum over these at-most-two-operation sets, with distinct selected cakes and distinct added unselected cakes.
- **Top-two structures:** For each mask, keep the two cheapest recolor operations with distinct selected ids. For each source label, keep the two selected cakes with smallest M. For each target label, keep the two unselected cakes with largest value for that label. For each mask, the two cheapest swap operations are obtained from the two smallest-M selected cakes for each source label and the best unselected value for the target label. Top-two lists are sufficient because when combining two operation lists, if the best ids conflict, one of the second-best ids is enough.
- **Combining two operations:** For two lists of operations, if their best ids differ, use both bests. If they are the same, try best of one plus second of the other, and second of one plus best of the other. This is used for recolor+recolor and recolor+swap in both directions.
- **Swap+swap details:** Enumerate the two directed label changes for each of the two masks. For selected sources: if source labels differ, use the best selected M from each; if they are the same, use the two smallest selected M from that label. For unselected targets: if target labels are the same, use the two largest values from that label; if different, use the best values unless their ids are the same, in which case take the maximum of replacing one side by its second-best value. This enforces distinct unselected cakes.
- **Edge cases:** If K = N/2, there are no unselected cakes, so all swap structures are empty and only recolors are used. Ties at the 2K boundary are safe: zero-cost swaps and zero-cost recolors among maximizing attributes handle alternative optimal baseline choices. Arbitrary tie-breaking in top-two lists is safe because any two optimal distinct ids are sufficient for conflict resolution.
- **Complexity:** Sorting dominates with O(N log N) per test case and O(N) memory. The total N over all test cases is at most 1e5, so the solution is easily fast enough.

### NEXT
- none

### STATUS
solved
