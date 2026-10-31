import sys
from array import array
from itertools import accumulate
from bisect import bisect_right


def build_sparse(vals, is_max, pow2):
    m = len(vals)
    st = [array('I', vals)]
    j = 1
    while pow2[j] <= m:
        half = pow2[j - 1]
        length = m - pow2[j] + 1
        prev = st[-1]
        curr = array('I', [0]) * length
        if is_max:
            for i in range(length):
                a = prev[i]
                b = prev[i + half]
                curr[i] = a if a >= b else b
        else:
            for i in range(length):
                a = prev[i]
                b = prev[i + half]
                curr[i] = a if a <= b else b
        st.append(curr)
        j += 1
    return st


def process_brute(ids, L, R, prev):
    K = len(ids)
    for a in range(K):
        i = ids[a]
        li = L[i]
        ri = R[i]
        cur = prev[i]
        for b in range(a - 1, -1, -1):
            j = ids[b]
            if j <= cur:
                break
            lj = L[j]
            rj = R[j]
            if (lj <= li and rj <= ri and rj > li) or (lj >= li and lj < ri and rj >= ri):
                prev[i] = j
                break


def process_block(ids, L, R, prev, N, B, lg, pow2):
    K = len(ids)
    nb = (K + B - 1) // B

    starts = [0] * nb
    ends = [0] * nb
    ms = [0] * nb
    firsts = [0] * nb
    lasts = [0] * nb

    for b in range(nb):
        st = b * B
        en = st + B
        if en > K:
            en = K
        starts[b] = st
        ends[b] = en
        m = en - st
        ms[b] = m
        firsts[b] = ids[st]
        lasts[b] = ids[en - 1]

    pls = [None] * nb
    prs = [None] * nb
    Lcnts = [None] * nb
    Rcnts = [None] * nb
    masks = [0] * nb
    stmax_l = [None] * nb
    stmin_r = [None] * nb

    delta = [0] * (N + 1)
    acc = accumulate

    for b in range(nb):
        st = starts[b]
        en = ends[b]
        m = ms[b]
        masks[b] = (1 << m) - 1

        l_vals = [0] * m
        r_vals = [0] * m
        for p in range(m):
            i = ids[st + p]
            l_vals[p] = L[i]
            r_vals[p] = R[i]

        order_l = sorted(range(m), key=l_vals.__getitem__)
        pl = [0] * (m + 1)
        r_by_l = [0] * m
        bits = 0
        for t, p in enumerate(order_l, 1):
            bits |= 1 << p
            pl[t] = bits
            r_by_l[t - 1] = r_vals[p]
        pls[b] = pl
        stmax_l[b] = build_sparse(r_by_l, True, pow2)

        order_r = sorted(range(m), key=r_vals.__getitem__)
        pr = [0] * (m + 1)
        l_by_r = [0] * m
        bits = 0
        for t, p in enumerate(order_r, 1):
            bits |= 1 << p
            pr[t] = bits
            l_by_r[t - 1] = l_vals[p]
        prs[b] = pr
        stmin_r[b] = build_sparse(l_by_r, False, pow2)

        touched = []
        for v in l_vals:
            pos = v + 1
            if delta[pos] == 0:
                touched.append(pos)
            delta[pos] += 1
        Lcnts[b] = array('H', acc(delta))
        for pos in touched:
            delta[pos] = 0

        touched = []
        for v in r_vals:
            pos = v + 1
            if delta[pos] == 0:
                touched.append(pos)
            delta[pos] += 1
        Rcnts[b] = array('H', acc(delta))
        for pos in touched:
            delta[pos] = 0

    for b in range(nb):
        st = starts[b]
        m = ms[b]

        pl_b = pls[b]
        pr_b = prs[b]
        Lc_b = Lcnts[b]
        Rc_b = Rcnts[b]
        mask_b = masks[b]
        stmax_b = stmax_l[b]
        stmin_b = stmin_r[b]

        maskp = 0
        for p in range(m):
            i = ids[st + p]
            li = L[i]
            ri = R[i]
            li1 = li + 1
            ri1 = ri + 1
            found = False

            if maskp:
                c3 = Rc_b[li1]
                c2 = Rc_b[ri1]
                existsA = False
                if c2 > c3:
                    length = c2 - c3
                    k = lg[length]
                    row = stmin_b[k]
                    v1 = row[c3]
                    v2 = row[c2 - pow2[k]]
                    if (v1 if v1 < v2 else v2) <= li:
                        existsA = True

                c4 = Lc_b[li]
                c5 = Lc_b[ri]
                existsB = False
                if c5 > c4:
                    length = c5 - c4
                    k = lg[length]
                    row = stmax_b[k]
                    v1 = row[c4]
                    v2 = row[c5 - pow2[k]]
                    if (v1 if v1 >= v2 else v2) >= ri:
                        existsB = True

                if existsA or existsB:
                    valid = 0
                    if existsA:
                        c1 = Lc_b[li1]
                        valid = pl_b[c1] & (pr_b[c2] ^ pr_b[c3])
                    if existsB:
                        c6 = Rc_b[ri]
                        vb = (pl_b[c5] ^ pl_b[c4]) & (mask_b ^ pr_b[c6])
                        if valid:
                            valid |= vb
                        else:
                            valid = vb

                    if valid:
                        valid &= maskp
                        if valid:
                            j = ids[st + valid.bit_length() - 1]
                            if j > prev[i]:
                                prev[i] = j
                            found = True

            if not found:
                cur = prev[i]
                for bb in range(b - 1, -1, -1):
                    if lasts[bb] <= cur:
                        break

                    Lc = Lcnts[bb]
                    Rc = Rcnts[bb]

                    c3 = Rc[li1]
                    c2 = Rc[ri1]
                    existsA = False
                    if c2 > c3:
                        length = c2 - c3
                        k = lg[length]
                        row = stmin_r[bb][k]
                        v1 = row[c3]
                        v2 = row[c2 - pow2[k]]
                        if (v1 if v1 < v2 else v2) <= li:
                            existsA = True

                    c4 = Lc[li]
                    c5 = Lc[ri]
                    existsB = False
                    if c5 > c4:
                        length = c5 - c4
                        k = lg[length]
                        row = stmax_l[bb][k]
                        v1 = row[c4]
                        v2 = row[c5 - pow2[k]]
                        if (v1 if v1 >= v2 else v2) >= ri:
                            existsB = True

                    if existsA or existsB:
                        pl_bb = pls[bb]
                        pr_bb = prs[bb]
                        mask_bb = masks[bb]
                        valid = 0

                        if existsA:
                            c1 = Lc[li1]
                            valid = pl_bb[c1] & (pr_bb[c2] ^ pr_bb[c3])
                        if existsB:
                            c6 = Rc[ri]
                            vb = (pl_bb[c5] ^ pl_bb[c4]) & (mask_bb ^ pr_bb[c6])
                            if valid:
                                valid |= vb
                            else:
                                valid = vb

                        if valid:
                            if firsts[bb] <= cur:
                                abs_pos = bisect_right(ids, cur, starts[bb], ends[bb])
                                pos = abs_pos - starts[bb]
                                if pos < ms[bb]:
                                    valid &= mask_bb ^ ((1 << pos) - 1)
                                else:
                                    valid = 0

                            if valid:
                                j = ids[starts[bb] + valid.bit_length() - 1]
                                if j > cur:
                                    cur = j
                                    prev[i] = cur
                                break

                            if firsts[bb] <= cur:
                                break

            maskp = (maskp << 1) | 1


def solve():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    idx = 0
    N = data[idx]
    M = data[idx + 1]
    Q = data[idx + 2]
    idx += 3

    L = [0] * M
    R = [0] * M
    plus = []
    minus = []

    for i in range(M):
        s = data[idx]
        t = data[idx + 1]
        idx += 2
        if s < t:
            l = s - 1
            r = t - 1
            plus.append(i)
        else:
            l = t - 1
            r = s - 1
            minus.append(i)
        L[i] = l
        R[i] = r

    ql = [0] * Q
    qr = [0] * Q
    for k in range(Q):
        ql[k] = data[idx] - 1
        qr[k] = data[idx + 1] - 1
        idx += 2
    del data

    prev = [-1] * M
    last_l = [-1] * N
    last_r = [-1] * N

    for i in range(M):
        l = L[i]
        r = R[i]
        p = last_l[l]
        q = last_r[r]
        if q > p:
            p = q
        prev[i] = p
        last_l[l] = i
        last_r[r] = i

    del last_l, last_r

    B = 3072
    BRUTE = 2000

    lg = [0] * (B + 1)
    for i in range(2, B + 1):
        lg[i] = lg[i >> 1] + 1
    max_log = lg[B]
    pow2 = [1 << i for i in range(max_log + 2)]

    if len(plus) >= 2:
        if len(plus) <= BRUTE:
            process_brute(plus, L, R, prev)
        else:
            process_block(plus, L, R, prev, N, B, lg, pow2)
    del plus

    if len(minus) >= 2:
        if len(minus) <= BRUTE:
            process_brute(minus, L, R, prev)
        else:
            process_block(minus, L, R, prev, N, B, lg, pow2)
    del minus, L, R

    size = 1
    while size < M:
        size <<= 1

    seg = [-1] * (2 * size)
    seg[size:size + M] = prev
    for i in range(size - 1, 0, -1):
        a = seg[i << 1]
        b = seg[i << 1 | 1]
        seg[i] = a if a >= b else b

    out = []
    for k in range(Q):
        lq = ql[k]
        rq = qr[k]
        l = lq + size
        r = rq + size
        res = -1

        while l <= r:
            if l & 1:
                v = seg[l]
                if v > res:
                    res = v
                l += 1
            if not (r & 1):
                v = seg[r]
                if v > res:
                    res = v
                r -= 1
            l >>= 1
            r >>= 1

        out.append('Yes' if res < lq else 'No')

    sys.stdout.write('\n'.join(out))


if __name__ == '__main__':
    solve()