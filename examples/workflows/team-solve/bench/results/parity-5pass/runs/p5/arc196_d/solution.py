import sys
import heapq
from bisect import bisect_left, bisect_right


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    p = 0
    N = int(data[p]); p += 1
    M = int(data[p]); p += 1
    Q = int(data[p]); p += 1

    L = [0] * M
    R = [0] * M
    sign = bytearray(M)  # 0: left-to-right, 1: right-to-left

    for i in range(M):
        s = int(data[p])
        t = int(data[p + 1])
        p += 2
        if s < t:
            L[i] = s - 1
            R[i] = t - 1
        else:
            L[i] = t - 1
            R[i] = s - 1
            sign[i] = 1

    ql = [0] * Q
    qr = [0] * Q
    for k in range(Q):
        ql[k] = int(data[p]) - 1
        qr[k] = int(data[p + 1]) - 1
        p += 2

    del data

    bad = [-1] * M

    # Endpoint conflicts:
    # same L with different (R, sign), same R with different (L, sign).
    shift_e = (2 * N).bit_length()
    mask_e = (1 << shift_e) - 1

    left1 = [-1] * N
    left2 = [-1] * N
    right1 = [-1] * N
    right2 = [-1] * N

    for j in range(M):
        lj = L[j]
        rj = R[j]
        sj = sign[j]
        bj = bad[j]
        enc = j << shift_e

        # same left endpoint
        k = (rj << 1) | sj
        v = left1[lj]
        if v != -1:
            vk = v & mask_e
            if vk != k:
                cand = v >> shift_e
            else:
                v2 = left2[lj]
                cand = v2 >> shift_e if v2 != -1 else -1
            if cand > bj:
                bj = cand

            if vk == k:
                left1[lj] = enc | k
            else:
                left2[lj] = v
                left1[lj] = enc | k
        else:
            left1[lj] = enc | k

        # same right endpoint
        k = (lj << 1) | sj
        v = right1[rj]
        if v != -1:
            vk = v & mask_e
            if vk != k:
                cand = v >> shift_e
            else:
                v2 = right2[rj]
                cand = v2 >> shift_e if v2 != -1 else -1
            if cand > bj:
                bj = cand

            if vk == k:
                right1[rj] = enc | k
            else:
                right2[rj] = v
                right1[rj] = enc | k
        else:
            right1[rj] = enc | k

        bad[j] = bj

    del left1, left2, right1, right2

    # Same-sign strict crossing conflicts.
    shift = M.bit_length()
    mask = (1 << shift) - 1
    N1 = N - 1
    INF = N

    def process_sign(s):
        setL = set()
        setR = set()
        Lloc = L
        Rloc = R
        signloc = sign
        Mloc = M

        for i in range(Mloc):
            if signloc[i] == s:
                setL.add(Lloc[i])
                setR.add(Rloc[i])

        if not setL:
            return

        uniqueL = sorted(setL)
        uniqueR = sorted(setR)

        mapL = {v: i for i, v in enumerate(uniqueL)}
        mapR = {v: i for i, v in enumerate(uniqueR)}

        compL = [-1] * Mloc
        compR = [-1] * Mloc
        for i in range(Mloc):
            if signloc[i] == s:
                compL[i] = mapL[Lloc[i]]
                compR[i] = mapR[Rloc[i]]

        del mapL, mapR, setL, setR

        U = len(uniqueL)
        V = len(uniqueR)

        S_A = 1 << (U - 1).bit_length()
        S_B = 1 << (V - 1).bit_length()

        # A: keyed by L, stores max R.
        segA = [-1] * (2 * S_A)
        # B: keyed by R, stores min L.
        segB = [INF] * (2 * S_B)

        heapsA = [[] for _ in range(U)]
        heapsB = [[] for _ in range(V)]
        dead = bytearray(Mloc)

        badloc = bad
        heappush = heapq.heappush
        heappop = heapq.heappop
        bl = bisect_left
        br = bisect_right

        mask_loc = mask
        shift_loc = shift
        N1_loc = N1
        INF_loc = INF

        def set_A(pos, val, segA=segA, S_A=S_A):
            p = pos + S_A
            if segA[p] == val:
                return
            segA[p] = val
            p >>= 1
            while p:
                left = segA[p << 1]
                right = segA[(p << 1) | 1]
                nv = left if left >= right else right
                if segA[p] == nv:
                    break
                segA[p] = nv
                p >>= 1

        def set_B(pos, val, segB=segB, S_B=S_B):
            p = pos + S_B
            if segB[p] == val:
                return
            segB[p] = val
            p >>= 1
            while p:
                left = segB[p << 1]
                right = segB[(p << 1) | 1]
                nv = left if left <= right else right
                if segB[p] == nv:
                    break
                segB[p] = nv
                p >>= 1

        stack = []

        for i in range(Mloc - 1, -1, -1):
            if signloc[i] != s:
                continue

            li = Lloc[i]
            ri = Rloc[i]

            # Type A: L_i < L_j < R_i < R_j.
            ql = br(uniqueL, li)
            qr = bl(uniqueL, ri)
            if ql < qr:
                l = ql + S_A
                r = qr + S_A
                stack.clear()

                while l < r:
                    if l & 1:
                        if segA[l] > ri:
                            stack.append(l)
                        l += 1
                    if r & 1:
                        r -= 1
                        if segA[r] > ri:
                            stack.append(r)
                    l >>= 1
                    r >>= 1

                while stack:
                    node = stack.pop()
                    if segA[node] <= ri:
                        continue

                    if node >= S_A:
                        pos = node - S_A
                        heap = heapsA[pos]

                        while heap:
                            key = heap[0]
                            idx = key & mask_loc
                            if dead[idx]:
                                heappop(heap)
                                continue

                            Rj = N1_loc - (key >> shift_loc)
                            if Rj <= ri:
                                break

                            heappop(heap)
                            dead[idx] = 1
                            if badloc[idx] < i:
                                badloc[idx] = i

                            posB = compR[idx]
                            hb = heapsB[posB]
                            while hb and dead[hb[0] & mask_loc]:
                                heappop(hb)
                            valB = hb[0] >> shift_loc if hb else INF_loc
                            set_B(posB, valB)

                        while heap and dead[heap[0] & mask_loc]:
                            heappop(heap)
                        valA = N1_loc - (heap[0] >> shift_loc) if heap else -1
                        set_A(pos, valA)
                    else:
                        lc = node << 1
                        rc = lc | 1
                        if segA[rc] > ri:
                            stack.append(rc)
                        if segA[lc] > ri:
                            stack.append(lc)

            # Type B: L_j < L_i < R_j < R_i.
            ql = br(uniqueR, li)
            qr = bl(uniqueR, ri)
            if ql < qr:
                l = ql + S_B
                r = qr + S_B
                stack.clear()

                while l < r:
                    if l & 1:
                        if segB[l] < li:
                            stack.append(l)
                        l += 1
                    if r & 1:
                        r -= 1
                        if segB[r] < li:
                            stack.append(r)
                    l >>= 1
                    r >>= 1

                while stack:
                    node = stack.pop()
                    if segB[node] >= li:
                        continue

                    if node >= S_B:
                        pos = node - S_B
                        heap = heapsB[pos]

                        while heap:
                            key = heap[0]
                            idx = key & mask_loc
                            if dead[idx]:
                                heappop(heap)
                                continue

                            Lj = key >> shift_loc
                            if Lj >= li:
                                break

                            heappop(heap)
                            dead[idx] = 1
                            if badloc[idx] < i:
                                badloc[idx] = i

                            posA = compL[idx]
                            ha = heapsA[posA]
                            while ha and dead[ha[0] & mask_loc]:
                                heappop(ha)
                            valA = N1_loc - (ha[0] >> shift_loc) if ha else -1
                            set_A(posA, valA)

                        while heap and dead[heap[0] & mask_loc]:
                            heappop(heap)
                        valB = heap[0] >> shift_loc if heap else INF_loc
                        set_B(pos, valB)
                    else:
                        lc = node << 1
                        rc = lc | 1
                        if segB[rc] < li:
                            stack.append(rc)
                        if segB[lc] < li:
                            stack.append(lc)

            # Insert current interval into both structures.
            posA = compL[i]
            keyA = ((N1_loc - Rloc[i]) << shift_loc) | i
            heappush(heapsA[posA], keyA)
            if Rloc[i] > segA[posA + S_A]:
                set_A(posA, Rloc[i])

            posB = compR[i]
            keyB = (Lloc[i] << shift_loc) | i
            heappush(heapsB[posB], keyB)
            if Lloc[i] < segB[posB + S_B]:
                set_B(posB, Lloc[i])

    process_sign(0)
    process_sign(1)

    # Prefix maximum of bad[j].
    for i in range(1, M):
        if bad[i] < bad[i - 1]:
            bad[i] = bad[i - 1]

    out = ["Yes" if bad[r] < l else "No" for l, r in zip(ql, qr)]
    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()