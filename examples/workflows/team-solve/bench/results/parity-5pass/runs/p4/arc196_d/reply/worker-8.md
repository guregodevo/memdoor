### CODE
```python
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
```

### NOTES
- **Prefix model:** For each person, let `l = min(S,T)-1`, `r = max(S,T)-1`. If `S<T`, require `P_l = P_r` and all interior `P_x > P_l`; if `S>T`, require `P_l = P_r` and all interior `P_x < P_l`. Any integer `P` gives road strengths by differences.
- **Bad pair theorem:** A set is infeasible iff it contains one of: same `l`, same `r`, or two same-sign strictly crossing intervals. Same endpoints force a direct contradiction at an interior town. Same-sign crossing gives a two-cycle of strict inequalities. Without such pairs, equality components plus strict edges are acyclic, so integer values exist.
- **Query reduction:** Store `ans[j] = max{i<j : (i,j) bad}`. After prefix-maximizing, query `[L,R]` is feasible iff `ans[R] < L`. A larger previous bad index is always at least as restrictive for every range containing the smaller one.
- **Endpoint conflicts:** One pass with `last_l` and `last_r` over all persons fills same-left/same-right bad pairs in `O(M)`. This is sign-independent.
- **Crossing detection:** Process each sign separately in original person order. Use CDQ on that subsequence. For a node split into left/right by index, handle two crossing types:
  - Type A `l_i < l_j < r_i < r_j`: sweep right by increasing `r`, add left intervals with `r_i < r_j`, range-max update over right `l` coordinates in `(l_i, r_i)`, point-query `l_j`.
  - Type B `l_j < l_i < r_j < r_i`: sweep right by decreasing `l`, add left intervals with `l_i > l_j`, range-max update over right `r` coordinates in `(l_i, r_i)`, point-query `r_j`.
- **Bottom-up CDQ:** Children are processed before the parent cross sweep. If every right element already has `ans[j] >= max_left_index`, the parent cross sweep is skipped. This is safe because any query containing a smaller left candidate also contains the already-known larger previous bad index.
- **Bitset base blocks:** For blocks of size `BASE = 4096`, all internal crossing pairs are computed with Python integer bitsets. Build prefix masks sorted by `l` and `r`. Process left positions in decreasing index; for each left interval form the bitset of later right intervals crossing it, intersect with unassigned later positions, and assign them. Each position is assigned at most once per block. This replaces the old `O(BASE^2)` Python brute force and makes a much larger base viable.
- **Segment-tree optimization:** The iterative range-max-update/point-query tree is global. Each sweep uses a new stamp and stores packed values `(stamp << SHIFT) + value`. Since stamps only increase, an update can simply do `if tree[node] < newv: tree[node] = newv`. Queries check `pack >= st_base` and use `global_max` to break early.
- **Pruning:** Before each CDQ sweep, use necessary extreme conditions to skip impossible types: Type A needs `min_left_l < max_right_l`, `min_left_r < max_right_r`, and `max_left_r > min_right_l`; Type B needs `max_left_l > min_right_l`, `max_left_r > min_right_r`, and `min_left_l < max_right_r`. Base blocks also skip if all right elements already have `ans[j]` at least the immediate previous element in the block.
- **Complexity:** With `B = BASE`, crossing detection is roughly `O(M log(M/B) log M + M B / word)` per sign, plus `O(N + M + Q)`. The bitset base cost is paid in C-level big-int operations; the CDQ part is Python-level segment-tree loops.
- **Tuning:** `BASE = 4096` was chosen as a robust default. `8192` can be faster if big-int operations dominate and memory is ample; `2048` is safer if memory or big-int allocation is tight. Endpoint-heavy or already-dense cases often skip many sweeps due to the bottom-up `ans` checks.

### NEXT
- none

### STATUS
solved
