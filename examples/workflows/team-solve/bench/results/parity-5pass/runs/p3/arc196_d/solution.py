import sys

INF = 10**9


def find_max_first(tree, size, ql, qr, thr):
    """First position in [ql, qr] whose leaf value is > thr, or -1."""
    l = ql + size
    sm = -1
    while True:
        while (l & 1) == 0:
            l >>= 1
        v = sm
        tv = tree[l]
        if tv > v:
            v = tv
        if v > thr:
            while l < size:
                l <<= 1
                v = sm
                tv = tree[l]
                if tv > v:
                    v = tv
                if v <= thr:
                    sm = v
                    l += 1
            pos = l - size
            return pos if pos <= qr else -1
        sm = v
        l += 1
        if (l & -l) == l:
            break
    return -1


def find_min_first(tree, size, ql, qr, thr, inf=INF):
    """First position in [ql, qr] whose leaf value is < thr, or -1."""
    l = ql + size
    sm = inf
    while True:
        while (l & 1) == 0:
            l >>= 1
        v = sm
        tv = tree[l]
        if tv < v:
            v = tv
        if v < thr:
            while l < size:
                l <<= 1
                v = sm
                tv = tree[l]
                if tv < v:
                    v = tv
                if v >= thr:
                    sm = v
                    l += 1
            pos = l - size
            return pos if pos <= qr else -1
        sm = v
        l += 1
        if (l & -l) == l:
            break
    return -1


def update_max(tree, size, pos, val):
    p = size + pos
    tree[p] = val
    p >>= 1
    while p:
        nv = tree[p << 1]
        rv = tree[(p << 1) | 1]
        if rv > nv:
            nv = rv
        if tree[p] == nv:
            break
        tree[p] = nv
        p >>= 1


def update_min(tree, size, pos, val):
    p = size + pos
    tree[p] = val
    p >>= 1
    while p:
        nv = tree[p << 1]
        rv = tree[(p << 1) | 1]
        if rv < nv:
            nv = rv
        if tree[p] == nv:
            break
        tree[p] = nv
        p >>= 1


def process_max(l, r, sgn, ep, nxt, M, N, size, sign):
    # Previous interval j crosses current i as: l_i < l_j < r_i < r_j.
    # Tree over l_j stores max r_j.
    tree = [-1] * (2 * size)
    idx_at = [-1] * N
    active = bytearray(M)

    head = [-1] * M
    ev_next = [-1] * M
    for i in range(M):
        if sgn[i] == sign:
            e = ep[i]
            if e != INF:
                ev_next[i] = head[e]
                head[e] = i

    upd = update_max
    find = find_max_first

    for i in range(M):
        j = head[i]
        while j != -1:
            if active[j]:
                active[j] = 0
                pos = l[j]
                if idx_at[pos] == j:
                    idx_at[pos] = -1
                    upd(tree, size, pos, -1)
            j = ev_next[j]

        if sgn[i] != sign:
            continue

        ql = l[i] + 1
        qr = r[i] - 1
        if ql <= qr:
            thr = r[i]
            while True:
                pos = find(tree, size, ql, qr, thr)
                if pos < 0:
                    break
                j = idx_at[pos]
                if j != -1 and active[j]:
                    if i < nxt[j]:
                        nxt[j] = i
                    active[j] = 0
                    idx_at[pos] = -1
                    upd(tree, size, pos, -1)
                else:
                    idx_at[pos] = -1
                    upd(tree, size, pos, -1)

        pos = l[i]
        active[i] = 1
        idx_at[pos] = i
        upd(tree, size, pos, r[i])


def process_min(l, r, sgn, ep, nxt, M, N, size, sign):
    # Previous interval j crosses current i as: l_j < l_i < r_j < r_i.
    # Tree over r_j stores min l_j.
    inf = INF
    tree = [inf] * (2 * size)
    idx_at = [-1] * N
    active = bytearray(M)

    head = [-1] * M
    ev_next = [-1] * M
    for i in range(M):
        if sgn[i] == sign:
            e = ep[i]
            if e != INF:
                ev_next[i] = head[e]
                head[e] = i

    upd = update_min
    find = find_min_first

    for i in range(M):
        j = head[i]
        while j != -1:
            if active[j]:
                active[j] = 0
                pos = r[j]
                if idx_at[pos] == j:
                    idx_at[pos] = -1
                    upd(tree, size, pos, inf)
            j = ev_next[j]

        if sgn[i] != sign:
            continue

        ql = l[i] + 1
        qr = r[i] - 1
        if ql <= qr:
            thr = l[i]
            while True:
                pos = find(tree, size, ql, qr, thr)
                if pos < 0:
                    break
                j = idx_at[pos]
                if j != -1 and active[j]:
                    if i < nxt[j]:
                        nxt[j] = i
                    active[j] = 0
                    idx_at[pos] = -1
                    upd(tree, size, pos, inf)
                else:
                    idx_at[pos] = -1
                    upd(tree, size, pos, inf)

        pos = r[i]
        active[i] = 1
        idx_at[pos] = i
        upd(tree, size, pos, l[i])


def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    it = iter(data)

    N = int(next(it))
    M = int(next(it))
    Q = int(next(it))

    l = [0] * M
    r = [0] * M
    sgn = bytearray(M)  # 1 for S < T, 0 for S > T

    for i in range(M):
        S = int(next(it))
        T = int(next(it))
        if S < T:
            l[i] = S - 1
            r[i] = T - 1
            sgn[i] = 1
        else:
            l[i] = T - 1
            r[i] = S - 1
            sgn[i] = 0

    ql = [0] * Q
    qr = [0] * Q
    for k in range(Q):
        ql[k] = int(next(it)) - 1
        qr[k] = int(next(it)) - 1

    del data, it

    size = 1
    while size < N:
        size <<= 1

    # nxt[i] = earliest later index j that forms a bad pair with i.
    nxt = [INF] * M
    last_l = [-1] * N
    last_r = [-1] * N

    for i in range(M - 1, -1, -1):
        li = l[i]
        ri = r[i]
        best = INF

        j = last_l[li]
        if j != -1:
            best = j

        j = last_r[ri]
        if j != -1 and j < best:
            best = j

        nxt[i] = best
        last_l[li] = i
        last_r[ri] = i

    # Same-endpoint partners are fixed; crossing sweeps may improve nxt.
    ep = nxt[:]

    process_max(l, r, sgn, ep, nxt, M, N, size, 1)
    process_max(l, r, sgn, ep, nxt, M, N, size, 0)
    process_min(l, r, sgn, ep, nxt, M, N, size, 1)
    process_min(l, r, sgn, ep, nxt, M, N, size, 0)

    # Suffix minimum of nxt. Since nxt[i] > i, suffix min from L is enough.
    suf = [INF] * (M + 1)
    for i in range(M - 1, -1, -1):
        v = nxt[i]
        if suf[i + 1] < v:
            v = suf[i + 1]
        suf[i] = v

    out = []
    for k in range(Q):
        if suf[ql[k]] <= qr[k]:
            out.append("No")
        else:
            out.append("Yes")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    main()