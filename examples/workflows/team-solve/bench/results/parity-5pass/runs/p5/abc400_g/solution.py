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