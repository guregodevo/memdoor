import sys
import gc
from bisect import bisect_left, bisect_right


def process_mode(ids, L, R, prev, idxs, N, mode, shift, mask):
    """
    mode 0:
      compressed coordinates = left endpoints l
      update interior l-positions in (l, r) with key = r
      query at l with threshold r
      finds earlier i with l_i < l_j < r_i < r_j

    mode 1:
      compressed coordinates = right endpoints r
      update interior r-positions in (l, r) with key = N - l
      query at r with threshold N - l
      finds earlier i with l_j < l_i < r_j < r_i
    """
    if len(ids) < 2:
        return

    if mode == 0:
        positions = sorted({L[pid] for pid in ids})
    else:
        positions = sorted({R[pid] for pid in ids})

    k = len(positions)
    if k < 2:
        return

    size = 1
    while size < k:
        size <<= 1

    tree = [None] * (size << 1)
    bl = bisect_left
    br = bisect_right

    for pid in ids:
        l = L[pid]
        r = R[pid]

        if mode == 0:
            qpos = bl(positions, l)
            x = r
        else:
            qpos = bl(positions, r)
            x = N - l

        base = x << shift
        idx = idxs[pid]

        # Point query: max index among records covering this point with key < x.
        q = qpos + size
        res = 0
        while q:
            lst = tree[q]
            if lst:
                if lst[-1] < base:
                    v = lst[-1] & mask
                elif lst[0] < base:
                    v = lst[bl(lst, base) - 1] & mask
                else:
                    v = 0
                if v > res:
                    res = v
            q >>= 1

        if res > prev[pid]:
            prev[pid] = res

        # Range update over compressed positions strictly between l and r.
        a = br(positions, l)
        b = bl(positions, r)
        if a < b:
            val = base | idx
            left = a + size
            right = b + size

            while left < right:
                if left & 1:
                    lst = tree[left]
                    if lst is None:
                        tree[left] = [val]
                    else:
                        while lst and lst[-1] >= base:
                            lst.pop()
                        lst.append(val)
                    left += 1

                if right & 1:
                    right -= 1
                    lst = tree[right]
                    if lst is None:
                        tree[right] = [val]
                    else:
                        while lst and lst[-1] >= base:
                            lst.pop()
                        lst.append(val)

                left >>= 1
                right >>= 1

    del tree, positions


def main():
    input = sys.stdin.buffer.readline
    line = input()
    if not line:
        return

    N, M, Q = map(int, line.split())

    L = [0] * M
    R = [0] * M
    Dir = bytearray(M)
    right = []
    left = []

    for i in range(M):
        s, t = map(int, input().split())
        if s < t:
            l, r, d = s, t, 0
            right.append(i)
        else:
            l, r, d = t, s, 1
            left.append(i)
        L[i] = l
        R[i] = r
        Dir[i] = d

    idxs = list(range(1, M + 1))
    prev = [0] * M

    shift = (M + 1).bit_length()
    mask = (1 << shift) - 1

    use_gc = sys.implementation.name == "cpython"
    if use_gc:
        gc.disable()

    # Endpoint conflicts:
    # - same direction, same l or same r
    # - opposite direction, same l or same r
    ll0 = [0] * (N + 2)
    lr0 = [0] * (N + 2)
    ll1 = [0] * (N + 2)
    lr1 = [0] * (N + 2)

    for pid in range(M):
        l = L[pid]
        r = R[pid]
        idx = idxs[pid]
        p = prev[pid]

        if Dir[pid] == 0:
            v = ll0[l]
            if v > p:
                p = v
            v = lr0[r]
            if v > p:
                p = v
            v = ll1[l]
            if v > p:
                p = v
            v = lr1[r]
            if v > p:
                p = v

            ll0[l] = idx
            lr0[r] = idx
        else:
            v = ll1[l]
            if v > p:
                p = v
            v = lr1[r]
            if v > p:
                p = v
            v = ll0[l]
            if v > p:
                p = v
            v = lr0[r]
            if v > p:
                p = v

            ll1[l] = idx
            lr1[r] = idx

        prev[pid] = p

    Dir = None
    ll0 = lr0 = ll1 = lr1 = None

    # Crossing same-direction conflicts.
    process_mode(right, L, R, prev, idxs, N, 0, shift, mask)
    process_mode(right, L, R, prev, idxs, N, 1, shift, mask)
    right = None

    process_mode(left, L, R, prev, idxs, N, 0, shift, mask)
    process_mode(left, L, R, prev, idxs, N, 1, shift, mask)
    left = None

    if use_gc:
        gc.enable()

    L = R = idxs = None

    # bad[x] = max prev[i] for i <= x
    bad = [0] * (M + 1)
    cur = 0
    for i in range(1, M + 1):
        p = prev[i - 1]
        if p > cur:
            cur = p
        bad[i] = cur

    prev = None

    out = []
    append = out.append
    for _ in range(Q):
        Lq, Rq = map(int, input().split())
        append("Yes" if bad[Rq] < Lq else "No")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    main()