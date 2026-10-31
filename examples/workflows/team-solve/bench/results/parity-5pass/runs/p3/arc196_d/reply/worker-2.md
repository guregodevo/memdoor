### CODE
```python
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
```

### NOTES
- **Model:** Use prefix potentials P_0..P_{N-1}, where town v is index v-1 and road strength is a difference of adjacent potentials. A person becomes an interval [l,r] with l=min(S,T)-1, r=max(S,T)-1 and sign + if S<T, - if S>T. Requirement: P_l=P_r, and every internal P_k is strictly greater (sign +) or strictly smaller (sign -) than P_l.
- **Feasibility oracle view:** Contract equal endpoint potentials. Each + interval gives directed edges endpoint-component -> internal-component; each - interval gives internal-component -> endpoint-component. The set is feasible iff this directed graph is acyclic, because topological ranks give integer potentials.
- **Bad pair lemma:** A set is infeasible iff it contains one of these pairs: same left endpoint with different right endpoint, same right endpoint with different left endpoint, same interval with opposite signs, or same-sign strict crossing l_i<l_j<r_i<r_j (or symmetric). Same-endpoint pairs force an endpoint to be internal to the other interval; same-sign crossing forces P_a<P_b<P_a or the reverse. Conversely, any directed cycle yields one of these pairs by taking the first equality/strict contradiction created while contracting endpoints.
- **Query reduction:** Let nxt[i] be the smallest j>i such that (i,j) is a bad pair. A query [L,R] is No iff some bad pair lies inside it, i.e. iff min_{i in [L,R]} nxt[i] <= R. Since nxt[i]>i, this is equivalent to suffix_min[L] <= R, where suffix_min[i]=min(nxt[i], suffix_min[i+1]).
- **Same endpoints:** Scan people backwards with last_l and last_r. For each i, the immediate next occurrence of l_i or r_i is the earliest same-endpoint bad partner. Store this in nxt and keep a copy ep for deletion scheduling.
- **Crossing sweeps:** There are four independent sweeps: sign + and sign -, and two orientations. For orientation l_i<l_j<r_i<r_j, process increasing i and maintain previous active same-sign intervals in a segment tree over l_j storing max r_j. Query l_j in (l_i,r_i) for max r_j > r_i. For orientation l_j<l_i<r_j<r_i, maintain a segment tree over r_j storing min l_j. Query r_j in (l_i,r_i) for min l_j < l_i.
- **Active deletion:** In each sweep, when current i finds active previous j, set nxt[j]=min(nxt[j],i) and delete j. Each interval is deleted at most once per sweep, so total reports are O(M).
- **Endpoint deletion events:** To avoid considering intervals that already have an earlier same-endpoint partner and to keep at most one active interval per coordinate, schedule deletion of j at time ep[j]. Use linked-list buckets head/ev_next over global index. The sweep scans all indices i, processes bucket i before querying, and only queries/inserts when sgn[i] matches the sweep sign. This works even if ep[j] has the opposite sign.
- **Segment tree find:** Use max_right/min_right style traversal to find the first leaf in a range with value > threshold or < threshold in O(log N). Point updates are O(log N) with early break when the parent aggregate does not change.
- **Complexity:** O((M+Q) log N) time and O(N+M) memory, with temporary O(N+M) arrays per sweep.
- **Edge cases:** |S-T|>1 guarantees at least one internal vertex. Strict crossing queries use l+1..r-1 and strict >/< thresholds. Duplicate same-sign intervals cannot occur because ordered pairs are distinct; same interval opposite sign is handled by same-endpoint ep.

### NEXT
none

### STATUS
solved
