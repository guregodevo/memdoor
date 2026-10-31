import sys
from bisect import bisect_left, bisect_right


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    vals = list(map(int, data))
    del data

    N, M, Q = vals[0], vals[1], vals[2]
    pos = 3

    l_arr = [0] * M
    r_arr = [0] * M
    idxs0 = []
    idxs1 = []

    for i in range(M):
        s = vals[pos]
        t = vals[pos + 1]
        pos += 2
        if s < t:
            l_arr[i] = s - 1
            r_arr[i] = t - 1
            idxs0.append(i)
        else:
            l_arr[i] = t - 1
            r_arr[i] = s - 1
            idxs1.append(i)

    # ans[j] = largest i < j such that persons i and j form a bad pair.
    ans = [0] * (M + 1)

    # Same left endpoint or same right endpoint is always a bad pair.
    last_l = [-1] * N
    last_r = [-1] * N
    for i in range(M):
        l = l_arr[i]
        r = r_arr[i]
        ai = ans[i + 1]

        prev = last_l[l]
        if prev > ai:
            ai = prev
        last_l[l] = i + 1

        prev = last_r[r]
        if prev > ai:
            ai = prev
        last_r[r] = i + 1

        ans[i + 1] = ai

    last_l = last_r = None
    sys.setrecursionlimit(1_000_000)

    # Bitset base block size. 4096 is a good balance for PyPy/CPython:
    # it keeps big-int blocks small while reducing CDQ levels significantly.
    BASE = 4096
    SHIFT = M.bit_length()
    size_global = 1 << (M - 1).bit_length() if M > 1 else 1
    tree = [0] * (2 * size_global)
    tmp = [0] * M
    stamp = 0

    blft = bisect_left
    brgt = bisect_right

    def base_bitset(idxs, nl, nr,
                    l_arr=l_arr, r_arr=r_arr, ans_arr=ans,
                    tmp_arr=tmp, blft=blft, brgt=brgt):
        b = nr - nl
        if b <= 1:
            return

        block = idxs[nl:nr]

        # If every right element already has a bad previous index at least as
        # large as the immediate previous element in this sign block, no
        # crossing inside this block can improve ans.
        need = False
        prev_x = block[0]
        for ii in range(1, b):
            x = block[ii]
            if ans_arr[x + 1] < prev_x + 1:
                need = True
                break
            prev_x = x
        if not need:
            return

        for p, x in enumerate(block):
            tmp_arr[x] = p

        all_mask = (1 << b) - 1

        order = block.copy()
        order.sort(key=l_arr.__getitem__)
        l_vals = [0] * b
        pref_l = [0] * (b + 1)
        mask = 0
        for t, x in enumerate(order):
            l_vals[t] = l_arr[x]
            mask |= 1 << tmp_arr[x]
            pref_l[t + 1] = mask
        del order

        order = block.copy()
        order.sort(key=r_arr.__getitem__)
        r_vals = [0] * b
        pref_r = [0] * (b + 1)
        mask = 0
        for t, x in enumerate(order):
            r_vals[t] = r_arr[x]
            mask |= 1 << tmp_arr[x]
            pref_r[t + 1] = mask
        del order

        unassigned = all_mask
        later_mask = 1 << (b - 1)

        pl = pref_l
        pr = pref_r
        lv = l_vals
        rv = r_vals

        # Process left endpoints in decreasing original index.
        # The first time a later position is selected, that left index is
        # the maximum previous crossing index inside this block.
        for i in range(b - 2, -1, -1):
            x = block[i]
            li = l_arr[x]
            ri = r_arr[x]

            # Type A: li < l_j < ri < r_j
            lo = brgt(lv, li)
            hi = blft(lv, ri)
            if lo < hi:
                posr = brgt(rv, ri)
                if posr < b:
                    maskA = (pl[hi] ^ pl[lo]) & (all_mask ^ pr[posr])
                else:
                    maskA = 0
            else:
                maskA = 0

            # Type B: l_j < li < r_j < ri
            lo = brgt(rv, li)
            hi = blft(rv, ri)
            if lo < hi:
                posl = blft(lv, li)
                if posl > 0:
                    maskB = (pr[hi] ^ pr[lo]) & pl[posl]
                else:
                    maskB = 0
            else:
                maskB = 0

            m = (maskA | maskB) & unassigned & later_mask
            if m:
                val = x + 1
                unassigned ^= m
                mm = m
                while mm:
                    lsb = mm & -mm
                    j = lsb.bit_length() - 1
                    jx = block[j]
                    if val > ans_arr[jx + 1]:
                        ans_arr[jx + 1] = val
                    mm ^= lsb

            later_mask |= 1 << i

    def cdq(idxs, nl, nr, bl, br,
            l_arr=l_arr, r_arr=r_arr, ans_arr=ans,
            tree_arr=tree, tmp_arr=tmp,
            blft=blft, brgt=brgt,
            BASE=BASE, SHIFT=SHIFT):
        nonlocal stamp

        k = nr - nl
        if k <= BASE:
            base_bitset(idxs, nl, nr)
            return

        mid = (nl + nr) >> 1
        threshold = idxs[mid]

        left_l = [x for x in bl if x < threshold]
        right_l = [x for x in bl if x >= threshold]
        left_r = [x for x in br if x < threshold]
        right_r = [x for x in br if x >= threshold]
        bl = br = None

        # Bottom-up: solve children first.  This lets parent sweeps be skipped
        # when ans already contains a previous index at least as large as the
        # whole left half.
        cdq(idxs, nl, mid, left_l, left_r)
        cdq(idxs, mid, nr, right_l, right_r)

        max_left = idxs[mid - 1] + 1
        need = False
        for ii in range(mid, nr):
            if ans_arr[idxs[ii] + 1] < max_left:
                need = True
                break
        if not need:
            return

        # Type A: l_i < l_j < r_i < r_j
        if left_l and right_l:
            if (l_arr[left_l[0]] < l_arr[right_l[-1]] and
                    r_arr[left_r[0]] < r_arr[right_r[-1]] and
                    r_arr[left_r[-1]] > l_arr[right_l[0]]):
                coords = []
                app = coords.append
                last = -1
                cpos = -1
                for x in right_l:
                    v = l_arr[x]
                    if v != last:
                        app(v)
                        last = v
                        cpos += 1
                    tmp_arr[x] = cpos

                clen = len(coords)
                sz = 1 << (clen - 1).bit_length() if clen > 1 else 1
                stamp += 1
                st_base = stamp << SHIFT
                c0 = coords[0]
                clast = coords[-1]

                p = 0
                llen = len(left_r)
                global_max = 0

                for j in right_r:
                    rj = r_arr[j]

                    while p < llen:
                        i = left_r[p]
                        if r_arr[i] >= rj:
                            break

                        li = l_arr[i]
                        ri = r_arr[i]
                        if li < clast and ri > c0:
                            if li < c0:
                                a = 0
                            else:
                                a = brgt(coords, li)
                            if ri > clast:
                                bnd = clen
                            else:
                                bnd = blft(coords, ri)

                            if a < bnd:
                                vv = i + 1
                                if vv > global_max:
                                    global_max = vv
                                newv = st_base + vv
                                ln = a + sz
                                rn = bnd + sz
                                while ln < rn:
                                    if ln & 1:
                                        if tree_arr[ln] < newv:
                                            tree_arr[ln] = newv
                                        ln += 1
                                    if rn & 1:
                                        rn -= 1
                                        if tree_arr[rn] < newv:
                                            tree_arr[rn] = newv
                                    ln >>= 1
                                    rn >>= 1
                        p += 1

                    cur = ans_arr[j + 1]
                    if global_max > cur:
                        node = tmp_arr[j] + sz
                        best = 0
                        while node:
                            pack = tree_arr[node]
                            if pack >= st_base:
                                vv = pack - st_base
                                if vv > best:
                                    best = vv
                                    if best == global_max:
                                        break
                            node >>= 1
                        if best > cur:
                            ans_arr[j + 1] = best

        # Type B: l_j < l_i < r_j < r_i
        if left_l and right_l:
            if (l_arr[left_l[-1]] > l_arr[right_l[0]] and
                    r_arr[left_r[-1]] > r_arr[right_r[0]] and
                    l_arr[left_l[0]] < r_arr[right_r[-1]]):
                coords = []
                app = coords.append
                last = -1
                cpos = -1
                for x in right_r:
                    v = r_arr[x]
                    if v != last:
                        app(v)
                        last = v
                        cpos += 1
                    tmp_arr[x] = cpos

                clen = len(coords)
                sz = 1 << (clen - 1).bit_length() if clen > 1 else 1
                stamp += 1
                st_base = stamp << SHIFT
                c0 = coords[0]
                clast = coords[-1]

                p = len(left_l) - 1
                global_max = 0

                for j in reversed(right_l):
                    lj = l_arr[j]

                    while p >= 0:
                        i = left_l[p]
                        if l_arr[i] <= lj:
                            break

                        li = l_arr[i]
                        ri = r_arr[i]
                        if li < clast and ri > c0:
                            if li < c0:
                                a = 0
                            else:
                                a = brgt(coords, li)
                            if ri > clast:
                                bnd = clen
                            else:
                                bnd = blft(coords, ri)

                            if a < bnd:
                                vv = i + 1
                                if vv > global_max:
                                    global_max = vv
                                newv = st_base + vv
                                ln = a + sz
                                rn = bnd + sz
                                while ln < rn:
                                    if ln & 1:
                                        if tree_arr[ln] < newv:
                                            tree_arr[ln] = newv
                                        ln += 1
                                    if rn & 1:
                                        rn -= 1
                                        if tree_arr[rn] < newv:
                                            tree_arr[rn] = newv
                                    ln >>= 1
                                    rn >>= 1
                        p -= 1

                    cur = ans_arr[j + 1]
                    if global_max > cur:
                        node = tmp_arr[j] + sz
                        best = 0
                        while node:
                            pack = tree_arr[node]
                            if pack >= st_base:
                                vv = pack - st_base
                                if vv > best:
                                    best = vv
                                    if best == global_max:
                                        break
                            node >>= 1
                        if best > cur:
                            ans_arr[j + 1] = best

    for idxs in (idxs0, idxs1):
        k = len(idxs)
        if k > 1:
            if k <= BASE:
                base_bitset(idxs, 0, k)
            else:
                bl = sorted(idxs, key=l_arr.__getitem__)
                br = sorted(idxs, key=r_arr.__getitem__)
                cdq(idxs, 0, k, bl, br)
            idxs.clear()

    # Prefix maximum: pref[R] = max_{j <= R} ans[j].
    for i in range(1, M + 1):
        if ans[i] < ans[i - 1]:
            ans[i] = ans[i - 1]

    # Release large structures before building output.
    base_bitset = cdq = None
    l_arr = r_arr = tree = tmp = None
    idxs0 = idxs1 = None

    out = []
    append = out.append
    for _ in range(Q):
        L = vals[pos]
        R = vals[pos + 1]
        pos += 2
        append("Yes" if ans[R] < L else "No")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()