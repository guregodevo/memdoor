### CODE
```python
import sys
from bisect import bisect_left, bisect_right


def solve():
    input = sys.stdin.buffer.readline
    first = input().split()
    if not first:
        return
    N, M, Q = map(int, first)

    l_arr = [0] * M
    r_arr = [0] * M
    idxs0 = []
    idxs1 = []

    for i in range(M):
        s, t = map(int, input().split())
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

        prev = last_l[l]
        if prev != -1 and prev > ans[i + 1]:
            ans[i + 1] = prev
        last_l[l] = i + 1

        prev = last_r[r]
        if prev != -1 and prev > ans[i + 1]:
            ans[i + 1] = prev
        last_r[r] = i + 1

    last_l = None
    last_r = None

    sys.setrecursionlimit(1_000_000)

    BASE = 128
    size_global = (1 << (M - 1).bit_length()) if M > 1 else 1
    SHIFT = M.bit_length()
    tree = [0] * (2 * size_global)
    tmp = [0] * M
    stamp = 0

    def cdq(idxs, nl, nr, bl, br,
            l_arr=l_arr, r_arr=r_arr, ans_arr=ans,
            tree_arr=tree, tmp_arr=tmp,
            blft=bisect_left, brgt=bisect_right,
            SHIFT=SHIFT, BASE=BASE):
        nonlocal stamp

        if nr - nl <= BASE:
            for jpos in range(nl, nr):
                j = idxs[jpos]
                best = ans_arr[j + 1]
                if jpos > nl and best < idxs[jpos - 1] + 1:
                    lj = l_arr[j]
                    rj = r_arr[j]
                    for ipos in range(jpos - 1, nl - 1, -1):
                        i = idxs[ipos]
                        vv = i + 1
                        if vv <= best:
                            break
                        li = l_arr[i]
                        ri = r_arr[i]
                        if li < lj:
                            if lj < ri and ri < rj:
                                best = vv
                                break
                        elif lj < li:
                            if li < rj and rj < ri:
                                best = vv
                                break
                    ans_arr[j + 1] = best
            return

        mid = (nl + nr) >> 1
        threshold = idxs[mid]

        left_l = [x for x in bl if x < threshold]
        right_l = [x for x in bl if x >= threshold]
        left_r = [x for x in br if x < threshold]
        right_r = [x for x in br if x >= threshold]
        bl = br = None

        # Type A: l_i < l_j < r_i < r_j
        if left_l and right_l and left_r and right_r:
            if l_arr[left_l[0]] < l_arr[right_l[-1]] and r_arr[left_r[0]] < r_arr[right_r[-1]]:
                coords = []
                app = coords.append
                last = -1
                pos = -1
                for x in right_l:
                    v = l_arr[x]
                    if v != last:
                        app(v)
                        last = v
                        pos += 1
                    tmp_arr[x] = pos

                clen = len(coords)
                sz = (1 << (clen - 1).bit_length()) if clen > 1 else 1
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
                                b = clen
                            else:
                                b = blft(coords, ri)

                            if a < b:
                                vv = i + 1
                                if vv > global_max:
                                    global_max = vv
                                newv = st_base + vv
                                ln = a + sz
                                rn = b + sz
                                while ln < rn:
                                    if ln & 1:
                                        pack = tree_arr[ln]
                                        if pack < st_base or vv > pack - st_base:
                                            tree_arr[ln] = newv
                                        ln += 1
                                    if rn & 1:
                                        rn -= 1
                                        pack = tree_arr[rn]
                                        if pack < st_base or vv > pack - st_base:
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
        if left_l and right_l and left_r and right_r:
            if l_arr[left_l[-1]] > l_arr[right_l[0]] and r_arr[left_r[-1]] > r_arr[right_r[0]]:
                coords = []
                app = coords.append
                last = -1
                pos = -1
                for x in right_r:
                    v = r_arr[x]
                    if v != last:
                        app(v)
                        last = v
                        pos += 1
                    tmp_arr[x] = pos

                clen = len(coords)
                sz = (1 << (clen - 1).bit_length()) if clen > 1 else 1
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
                                b = clen
                            else:
                                b = blft(coords, ri)

                            if a < b:
                                vv = i + 1
                                if vv > global_max:
                                    global_max = vv
                                newv = st_base + vv
                                ln = a + sz
                                rn = b + sz
                                while ln < rn:
                                    if ln & 1:
                                        pack = tree_arr[ln]
                                        if pack < st_base or vv > pack - st_base:
                                            tree_arr[ln] = newv
                                        ln += 1
                                    if rn & 1:
                                        rn -= 1
                                        pack = tree_arr[rn]
                                        if pack < st_base or vv > pack - st_base:
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

        coords = None
        app = None

        cdq(idxs, nl, mid, left_l, left_r)
        left_l = left_r = None
        cdq(idxs, mid, nr, right_l, right_r)
        right_l = right_r = None

    for idxs in (idxs0, idxs1):
        k = len(idxs)
        if k > 1:
            if k <= BASE:
                cdq(idxs, 0, k, idxs, idxs)
            else:
                bl = sorted(idxs, key=l_arr.__getitem__)
                br = sorted(idxs, key=r_arr.__getitem__)
                cdq(idxs, 0, k, bl, br)
                bl = br = None
        idxs.clear()

    # Prefix maximum: pref[R] = max_{j <= R} ans[j].
    for i in range(1, M + 1):
        if ans[i] < ans[i - 1]:
            ans[i] = ans[i - 1]

    # Free large structures no longer needed before reading queries.
    l_arr = r_arr = tree = tmp = None
    idxs0 = idxs1 = None
    idxs = None
    cdq = None

    data = sys.stdin.buffer.read().split()
    it = iter(data)
    out = []
    append = out.append
    for _ in range(Q):
        ql = int(next(it))
        qr = int(next(it))
        append("Yes" if ans[qr] < ql else "No")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()
```

### NOTES
- **Prefix model:** For each person, let `l = min(S,T)-1`, `r = max(S,T)-1`. If `S < T`, require `P_l = P_r` and every interior `P_x > P_l`. If `S > T`, require `P_l = P_r` and every interior `P_x < P_l`. Any integer assignment to `P_0,...,P_{N-1}` gives road strengths by differences.
- **Bad pair theorem:** A set is infeasible iff it contains one of: same `l`, same `r`, or two same-sign strictly crossing intervals. Same endpoint makes an endpoint value both equal and strictly ordered. Same-sign crossing gives a direct two-cycle. Without such pairs, equality components are path-like and the strict inequality graph is acyclic, so integer values exist.
- **Query reduction:** For each person index `j`, store the largest previous `i < j` forming a bad pair. After prefix-maximizing `ans`, query `[L,R]` is feasible iff `ans[R] < L`. A larger previous bad pair is always at least as restrictive for any range containing the smaller one.
- **Endpoint conflicts:** One pass with `last_l` and `last_r` fills all same-left/same-right bad pairs in `O(M)`.
- **Crossing detection:** Run CDQ separately for sign `0` and sign `1` on the subsequence preserving original person order. Base blocks of size 128 are brute-forced by scanning backwards and stopping at the first improving crossing.
- **CDQ sweeps:** For a split into left/right by person order:
  - Type A `l_i < l_j < r_i < r_j`: sweep right side by increasing `r`, add left intervals with `r_i < r_j`, range-max update over right `l` coordinates in `(l_i, r_i)`, point-query `l_j`.
  - Type B `l_j < l_i < r_j < r_i`: sweep right side by decreasing `l`, add left intervals with `l_i > l_j`, range-max update over right `r` coordinates in `(l_i, r_i)`, point-query `r_j`.
- **Segment tree reuse:** Iterative range-max update / point-query tree is global. Each sweep gets a new stamp and stores packed values `(stamp << SHIFT) + value`, where `SHIFT = M.bit_length()`. Old stamps are automatically smaller and ignored, so no clearing is needed.
- **Stress-test findings:** Small cases checked against a difference-constraints oracle (Bellman-Ford on `P` variables with equality and strict edges) and targeted cases: shared `l`/`r`, reversed pairs, opposite-sign crossings/nesting, all same sign, length-2 intervals. Ensured CDQ pruning conditions compare endpoint coordinates, not person indices; added `global_max` and early break in point queries. Skipping when current `ans[j]` already dominates is safe because any larger previous bad pair is also inside any query that contains a smaller one.
- **Complexity:** `O(M log^2 M + M * BASE + Q)` time and `O(N + M)` memory, with `BASE = 128`.

### NEXT
- none

### STATUS
solved
