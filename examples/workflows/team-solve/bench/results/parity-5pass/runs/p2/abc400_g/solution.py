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