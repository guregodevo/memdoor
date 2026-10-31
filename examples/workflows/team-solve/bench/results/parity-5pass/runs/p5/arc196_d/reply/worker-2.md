### CODE
```python
import sys

def main():
    data = list(map(int, sys.stdin.buffer.read().split()))
    if not data:
        return

    it = iter(data)
    N = next(it)
    M = next(it)
    Q = next(it)

    L = [0] * (M + 1)
    R = [0] * (M + 1)
    sign = [0] * (M + 1)

    for i in range(1, M + 1):
        s = next(it)
        t = next(it)
        if s < t:
            L[i] = s - 1
            R[i] = t - 1
            sign[i] = 1
        else:
            L[i] = t - 1
            R[i] = s - 1
            sign[i] = -1

    # Baseline O(M^2) computation of bad[j]:
    # bad[j] = max i < j such that persons i and j form a forbidden pair.
    bad = [0] * (M + 1)

    for j in range(1, M + 1):
        lj = L[j]
        rj = R[j]
        sj = sign[j]

        # Scan backwards so the first conflict found is the largest i.
        for i in range(j - 1, 0, -1):
            li = L[i]
            ri = R[i]

            if li == lj:
                # Same left endpoint:
                # - different right endpoints always conflict
                # - identical interval with opposite sign conflicts
                if ri != rj or sign[i] != sj:
                    bad[j] = i
                    break
            elif ri == rj:
                # Same right endpoint and different left endpoint:
                # always conflicts.
                bad[j] = i
                break
            else:
                # Strict crossing is forbidden only for the same sign.
                if sign[i] == sj:
                    if (li < lj < ri < rj) or (lj < li < rj < ri):
                        bad[j] = i
                        break

    # Prefix maximum of bad.
    pref = [0] * (M + 1)
    mx = 0
    for i in range(1, M + 1):
        if bad[i] > mx:
            mx = bad[i]
        pref[i] = mx

    out = []
    for _ in range(Q):
        lq = next(it)
        rq = next(it)
        # A query [L, R] is feasible iff no bad[j] for j <= R is >= L.
        out.append("Yes" if pref[rq] < lq else "No")

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    main()
```

### NOTES
- **Transformation:** Let P_i be the prefix sum of road strengths up to road i, with town t corresponding to P_{t-1}. A person (S,T) becomes l=min(S,T)-1, r=max(S,T)-1, and sign=+1 if S<T else -1. The requirement is P_l=P_r, and for every l<x<r, P_x>P_l if sign is +1, or P_x<P_l if sign is -1.
- **Forbidden pairs:** Under the adopted characterization, a set of people is feasible iff it contains no forbidden pair. Forbidden pairs are: identical (l,r) with opposite signs; same l with different r; same r with different l; and same-sign strict crossing, meaning l_i<l_j<r_i<r_j or l_j<l_i<r_j<r_i. Nested or touching intervals are not conflicts by themselves.
- **Baseline algorithm:** Store 1-indexed arrays L, R, sign. For each j, scan i from j-1 down to 1 and stop at the first conflict. That i is bad[j]. This gives the maximum conflicting previous index directly, but is O(M^2) worst-case.
- **Conflict test details:** If L_i==L_j, conflict iff R_i!=R_j or sign_i!=sign_j. Else if R_i==R_j, conflict because the left endpoints differ. Otherwise, conflict only when signs are equal and one of the two strict crossing chains holds.
- **Query answering:** Build pref[j]=max(pref[j-1], bad[j]). For query [L,R], answer Yes iff pref[R]<L. This works because bad[j]<j, so any prefix maximum >=L must come from a pair fully inside [L,R].
- **Sample traces:** Sample 1 gives bad[4]=3 from the crossing same-sign pair persons 3 and 4, so [1,3] is Yes and [2,4] is No. Sample 2 gives bad[3]=1, bad[4]=1, bad[5]=1, bad[6]=4, producing No, Yes, Yes.
- **Pitfalls:** Use town-1 prefix indices. Sign is the travel direction, not the left/right endpoint. Crossing comparisons must be strict. Identical same-sign intervals are allowed, but identical opposite-sign intervals are forbidden. Same-endpoint conflicts apply regardless of sign. The prefix-max query reduction depends on bad[j]<j.
- **Performance:** The current program is intentionally a baseline. M up to 2e5 makes the nested loop infeasible for full constraints. Parsing and array storage are fine; the pair-check loop is the bottleneck.
- **Next design notes:** Endpoint conflicts can be computed online with hash maps per endpoint, keeping top two distinct keys of the form (opposite endpoint, sign) to find the latest conflicting index. Same-sign crossing needs an offline method such as CDQ divide-and-conquer with Fenwick-tree sweeps, handling type A l_i<l_j<r_i<r_j and type B l_j<l_i<r_j<r_i separately for each sign.

### NEXT
- Replace the O(M^2) loop with an efficient computation of bad[j].
- Implement endpoint hash maps with top-two distinct keys for same-left and same-right conflicts.
- Implement CDQ/Fenwick or another offline 2D sweep for same-sign crossing pairs.
- Stress-test the efficient version against this O(M^2) baseline on small random cases.

### STATUS
continue
