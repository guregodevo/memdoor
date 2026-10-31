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