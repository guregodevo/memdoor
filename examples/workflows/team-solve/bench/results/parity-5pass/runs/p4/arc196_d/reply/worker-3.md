### CODE
```python
import sys


def solve():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    p = 0
    N = data[p]
    M = data[p + 1]
    Q = data[p + 2]
    p += 3

    L = [0] * M
    R = [0] * M
    pos = []
    neg = []

    for i in range(M):
        S = data[p]
        T = data[p + 1]
        p += 2
        if S < T:
            l = S - 1
            r = T - 1
            pos.append(i)
        else:
            l = T - 1
            r = S - 1
            neg.append(i)
        L[i] = l
        R[i] = r

    queries = data[p:p + 2 * Q]
    del data

    # f[j] = maximum i < j such that (i, j) is a bad pair.
    f = [-1] * M

    # Endpoint conflicts: same left endpoint or same right endpoint.
    last_l = [-1] * N
    last_r = [-1] * N
    for i in range(M):
        li = L[i]
        ri = R[i]
        v = last_l[li]
        if v > f[i]:
            f[i] = v
        v = last_r[ri]
        if v > f[i]:
            f[i] = v
        last_l[li] = i
        last_r[ri] = i
    del last_l, last_r

    # Timestamped iterative segment tree for point-max / range-max.
    size = 1
    while size < N:
        size <<= 1
    seg_val = [-1] * (2 * size)
    seg_tag = [0] * (2 * size)
    token = 0

    BASE = 128
    sys.setrecursionlimit(1_000_000)

    def cdq(ids, left, right, top=False):
        nonlocal token

        if right - left <= BASE:
            Larr = L
            Rarr = R
            farr = f

            block = ids[left:right]
            bl = [Larr[x] for x in block]
            br = [Rarr[x] for x in block]

            # Brute force strict crossings inside this small block.
            for a in range(len(block)):
                i = block[a]
                li = bl[a]
                ri = br[a]
                for b in range(a + 1, len(block)):
                    lj = bl[b]
                    if li < lj:
                        rj = br[b]
                        if lj < ri < rj:
                            j = block[b]
                            if i > farr[j]:
                                farr[j] = i
                    elif lj < li:
                        rj = br[b]
                        if li < rj < ri:
                            j = block[b]
                            if i > farr[j]:
                                farr[j] = i

            if top:
                return None

            by_l = block.copy()
            by_l.sort(key=Larr.__getitem__)
            by_r = block.copy()
            by_r.sort(key=Rarr.__getitem__, reverse=True)
            return by_l, by_r

        mid = (left + right) // 2
        left_l, left_r = cdq(ids, left, mid, False)
        right_l, right_r = cdq(ids, mid, right, False)

        Larr = L
        Rarr = R
        farr = f
        val_arr = seg_val
        tag_arr = seg_tag
        sz = size

        # Type A crossing: l_i < l_j < r_i < r_j.
        token += 1
        cur = token
        ptr = 0
        nleft = len(left_l)
        for j in right_l:
            lj = Larr[j]
            rj = Rarr[j]
            while ptr < nleft and Larr[left_l[ptr]] < lj:
                i = left_l[ptr]
                node = Rarr[i] + sz
                while node:
                    if tag_arr[node] != cur:
                        tag_arr[node] = cur
                        val_arr[node] = i
                    elif i > val_arr[node]:
                        val_arr[node] = i
                    node >>= 1
                ptr += 1

            if ptr:
                a = lj + 1 + sz
                b = rj - 1 + sz
                best = -1
                while a <= b:
                    if a & 1:
                        if tag_arr[a] == cur:
                            vv = val_arr[a]
                            if vv > best:
                                best = vv
                        a += 1
                    if not (b & 1):
                        if tag_arr[b] == cur:
                            vv = val_arr[b]
                            if vv > best:
                                best = vv
                        b -= 1
                    a >>= 1
                    b >>= 1

                if best > farr[j]:
                    farr[j] = best

        # Type B crossing: l_j < l_i < r_j < r_i.
        token += 1
        cur = token
        ptr = 0
        nleft = len(left_r)
        for j in right_r:
            rj = Rarr[j]
            lj = Larr[j]
            while ptr < nleft and Rarr[left_r[ptr]] > rj:
                i = left_r[ptr]
                node = Larr[i] + sz
                while node:
                    if tag_arr[node] != cur:
                        tag_arr[node] = cur
                        val_arr[node] = i
                    elif i > val_arr[node]:
                        val_arr[node] = i
                    node >>= 1
                ptr += 1

            if ptr:
                a = lj + 1 + sz
                b = rj - 1 + sz
                best = -1
                while a <= b:
                    if a & 1:
                        if tag_arr[a] == cur:
                            vv = val_arr[a]
                            if vv > best:
                                best = vv
                        a += 1
                    if not (b & 1):
                        if tag_arr[b] == cur:
                            vv = val_arr[b]
                            if vv > best:
                                best = vv
                        b -= 1
                    a >>= 1
                    b >>= 1

                if best > farr[j]:
                    farr[j] = best

        if top:
            return None

        # Merge sorted orders for the parent.
        la = len(left_l)
        lb = len(right_l)
        merged_l = [0] * (la + lb)
        i = j = k = 0
        while i < la and j < lb:
            a = left_l[i]
            b = right_l[j]
            if Larr[a] <= Larr[b]:
                merged_l[k] = a
                i += 1
            else:
                merged_l[k] = b
                j += 1
            k += 1
        if i < la:
            merged_l[k:] = left_l[i:]
        elif j < lb:
            merged_l[k:] = right_l[j:]

        merged_r = [0] * (la + lb)
        i = j = k = 0
        while i < la and j < lb:
            a = left_r[i]
            b = right_r[j]
            if Rarr[a] >= Rarr[b]:
                merged_r[k] = a
                i += 1
            else:
                merged_r[k] = b
                j += 1
            k += 1
        if i < la:
            merged_r[k:] = left_r[i:]
        elif j < lb:
            merged_r[k:] = right_r[j:]

        return merged_l, merged_r

    if len(pos) > 1:
        cdq(pos, 0, len(pos), True)
    if len(neg) > 1:
        cdq(neg, 0, len(neg), True)

    # Convert f to prefix maximum: f[R] = max_{j <= R} f[j].
    cur = -1
    for i in range(M):
        v = f[i]
        if v < cur:
            f[i] = cur
        else:
            cur = v

    out = []
    append = out.append
    for qi in range(0, 2 * Q, 2):
        Lq = queries[qi]
        Rq = queries[qi + 1]
        if Lq - 1 > f[Rq - 1]:
            append("Yes")
        else:
            append("No")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()
```

### NOTES
- **Prefix model:** For person `(S,T)`, set `l=min(S,T)-1`, `r=max(S,T)-1`, sign `+` if `S<T`, else `-`. The requirement is `P_l=P_r`, and every interior prefix `P_x` (`l<x<r`) is strictly greater for `+` and strictly smaller for `-`.
- **Bad-pair criterion:** A set is feasible iff it contains no pair with the same `l`, the same `r`, or the same sign and strictly crossing intervals. Same `l`/`r` creates a self-loop after contracting equalities; same-sign strict crossing creates a direct two-cycle. Without these, equality components are paths and the induced strict-inequality graph is acyclic, so integer values exist.
- **Query reduction:** Let `f[j]` be the maximum `i<j` forming a bad pair. After prefix-maximizing, `f[R-1]` is the largest bad-pair left index ending at or before `R`. A query `[L,R]` is feasible iff `L-1 > f[R-1]`.
- **Endpoint conflicts:** One scan over original person order with `last_l` and `last_r` catches all same-left/same-right pairs, regardless of sign.
- **CDQ crossing computation:** For each sign separately, process persons in original index order. CDQ over index order; base blocks of size 128 are brute-forced. Cross-half pairs are handled by two sweeps:
  - Type A: `l_i < l_j < r_i < r_j`. Sweep right half by increasing `l_j`, insert left intervals with `l_i < l_j` at coordinate `r_i`, query max in `[l_j+1, r_j-1]`.
  - Type B: `l_j < l_i < r_j < r_i`. Sweep right half by decreasing `r_j`, insert left intervals with `r_i > r_j` at coordinate `l_i`, query max in `[l_j+1, r_j-1]`.
- **Segment tree:** Iterative point-max/range-max over coordinates `0..N-1`. Each sweep uses a fresh timestamp; updates propagate maxima upward, and queries ignore nodes with old timestamps, avoiding explicit clearing.
- **Stress and edge checks:** Compared against an O(M^2) bad-pair oracle and a brute-force feasibility oracle on small random cases. Targeted cases included length-two intervals, touching endpoints, opposite-sign crossings/nesting, shared endpoints of different roles, all-same-sign laminar sets, reverse-direction duplicate intervals, and single-person queries. No mismatches were found.
- **Complexity:** `O(M log M log N + M*BASE + Q)` time and `O(N + M)` memory.

### NEXT
none

### STATUS
solved
