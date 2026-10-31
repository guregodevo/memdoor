### CODE
```python
import sys
import random


def all_pairs(N):
    res = []
    for s in range(1, N + 1):
        for t in range(1, N + 1):
            if abs(s - t) > 1:
                res.append((s, t))
    return res


def make_data(pairs, N):
    m = len(pairs)
    L = [0] * m
    R = [0] * m
    D = [0] * m
    edges = []

    for i, (s, t) in enumerate(pairs):
        if s < t:
            l, r, d = s, t, 1
        else:
            l, r, d = t, s, -1

        L[i] = l
        R[i] = r
        D[i] = d

        # Directed strict inequality edges: u -> v means H_u < H_v.
        if d == 1:
            edges.append([(l, x) for x in range(l + 1, r)])
        else:
            edges.append([(x, l) for x in range(l + 1, r)])

    return L, R, D, edges


def feasible_indices(indices, N, L, R, edges):
    # Equality contraction by transitive closure on the endpoint equality graph.
    # This is equivalent to DSU contraction for N <= 8.
    eq = [0] * (N + 1)
    for i in indices:
        a = L[i]
        b = R[i]
        eq[a] |= 1 << b
        eq[b] |= 1 << a

    towns = range(1, N + 1)

    for k in towns:
        bitk = 1 << k
        eqk = eq[k]
        if eqk:
            for u in towns:
                if eq[u] & bitk:
                    eq[u] |= eqk

    comp = [0] * (N + 1)
    for u in towns:
        if comp[u] == 0:
            for v in towns:
                if v == u or ((eq[u] >> v) & 1):
                    comp[v] = u

    # Build strict inequality graph on contracted components.
    adj = [0] * (N + 1)
    for i in indices:
        for u, v in edges[i]:
            cu = comp[u]
            cv = comp[v]
            if cu == cv:
                return False
            adj[cu] |= 1 << cv

    # Directed cycle detection by transitive closure.
    for k in towns:
        bitk = 1 << k
        adjk = adj[k]
        if adjk:
            for u in towns:
                if adj[u] & bitk:
                    adj[u] |= adjk

    for u in towns:
        if adj[u] & (1 << u):
            return False

    return True


def pairwise_indices(indices, N, L, R, D):
    arr = list(indices)
    m = len(arr)

    for a in range(m):
        i = arr[a]
        li = L[i]
        ri = R[i]
        di = D[i]

        for b in range(a + 1, m):
            j = arr[b]

            if di == D[j]:
                lj = L[j]
                rj = R[j]

                # Open intervals overlap as real intervals.
                if (li if li > lj else lj) < (ri if ri < rj else rj):
                    # Strict containment is compatible.
                    if (li < lj and rj < ri) or (lj < li and ri < rj):
                        continue
                    return False
            else:
                # Opposite directions conflict iff they share a normalized endpoint.
                if li == L[j] or ri == R[j]:
                    return False

    return True


def check_instance(pairs, N, mismatches, total, subset_limit):
    L, R, D, edges = make_data(pairs, N)
    M = len(pairs)

    # Check all contiguous ranges.
    for a in range(M):
        for b in range(a, M):
            inds = range(a, b + 1)
            brute = feasible_indices(inds, N, L, R, edges)
            pairwise = pairwise_indices(inds, N, L, R, D)
            total[0] += 1
            if brute != pairwise:
                mismatches.append((N, pairs, list(inds), brute, pairwise))
                return False

    # Check all subsets when small enough.
    if M <= subset_limit:
        for mask in range(1 << M):
            inds = [i for i in range(M) if (mask >> i) & 1]
            brute = feasible_indices(inds, N, L, R, edges)
            pairwise = pairwise_indices(inds, N, L, R, D)
            total[0] += 1
            if brute != pairwise:
                mismatches.append((N, pairs, inds, brute, pairwise))
                return False

    return True


def main():
    data = sys.stdin.read().split()
    seed = 12345
    if data:
        try:
            seed = int(data[0])
        except ValueError:
            pass

    rng = random.Random(seed)
    mismatches = []
    total = [0]

    ok = True

    # Exhaustive small N: all subsets and all ranges of the full valid pair set.
    for N in (3, 4, 5):
        if not check_instance(all_pairs(N), N, mismatches, total, 12):
            ok = False
            break

    # Full pair sets for larger N: all contiguous ranges.
    if ok:
        for N in (6, 7, 8):
            if not check_instance(all_pairs(N), N, mismatches, total, 12):
                ok = False
                break

    # Random instances.
    if ok:
        for _ in range(200):
            N = rng.randint(3, 8)
            pool = all_pairs(N)
            M = rng.randint(1, len(pool))
            pairs = rng.sample(pool, M)
            if not check_instance(pairs, N, mismatches, total, 10):
                ok = False
                break

    print(f"Total checks: {total[0]}")

    if not ok or mismatches:
        print("FAILED: mismatches found")
        for N, pairs, inds, brute, pairwise in mismatches:
            print(
                f"MISMATCH N={N} pairs={pairs} indices={inds} "
                f"brute={brute} pairwise={pairwise}"
            )
        sys.exit(1)

    print("PASSED: pairwise rule matches brute force on all checked ranges/subsets.")


if __name__ == "__main__":
    main()
```

### NOTES
- **Model:** Use prefix heights H_1..H_N, with H_1 arbitrary. A person (S,T) becomes normalized interval [l,r] and direction d. If S<T (right), constraints are H_l=H_r and H_x>H_l for every interior town x. If S>T (left), constraints are H_l=H_r and H_x<H_l for every interior town x. Integer road strengths exist exactly when such integer H exists.
- **Brute oracle:** For each tested set of people, contract endpoint equalities and then check strict inequalities. The harness implements equality contraction by bitmask transitive closure on the endpoint equality graph, which is equivalent to DSU contraction for N<=8. Each interior constraint becomes a directed edge u->v meaning H_u<H_v. The set is infeasible iff an edge becomes a self-loop after contraction or the contracted inequality graph has a directed cycle. Cycle detection is done by bitmask Warshall closure.
- **Pairwise rule under test:** A set is feasible iff it contains no incompatible pair. Same direction: conflict iff open intervals overlap, i.e. max(l1,l2)<min(r1,r2), and neither interval strictly contains the other. Strict containment requires both endpoint inequalities to be strict, so shared left or shared right endpoints are conflicts. Crossing intervals with no common integer interior town are still conflicts because the open real intervals overlap. Opposite direction: conflict iff the normalized left endpoints are equal or the normalized right endpoints are equal. Opposite-side touching, such as r1=l2 or r2=l1, is compatible.
- **Harness scope:** The program exhaustively checks all subsets and all ranges for the full valid directed pair sets for N=3,4,5. It checks all contiguous ranges for the full valid pair sets for N=6,7,8. It also runs 200 random instances with N in [3,8], M uniformly chosen from 1 to the number of valid directed pairs, and a random order. For every random instance it checks all contiguous ranges, and it checks all subsets when M<=10. A fixed default seed is used; an optional seed can be supplied as the first stdin token.
- **Pitfalls:** Open-interval overlap must be interpreted as real interval overlap, not as sharing an integer interior town. Strict containment must be strict on both endpoints; otherwise endpoint-sharing same-direction intervals would be incorrectly treated as compatible. Identical same-direction intervals are absent because (S_i,T_i) are unique; if duplicates were allowed, the stated pairwise rule would need a special case. Length-1 intervals are excluded by the original constraints, but the oracle would handle them naturally by having no interior inequality edges.
- **Validation status:** The harness prints the number of checked sets and either a passed message or the first mismatch with N, pairs, indices, brute-force result, and pairwise result. It does not answer the original Q queries; it only validates the combinatorial pairwise feasibility rule.
- **Next algorithmic step:** Once the rule is trusted, compute for each person j the latest earlier incompatible index prev[j]. Endpoint conflicts can be handled by arrays storing latest indices per normalized endpoint and direction. Same-direction crossing conflicts can be handled by 2D range-max/point-query structures in (l,r) and reversed coordinates. Then a query [L,R] is Yes iff L > max_{j<=R} prev[j].

### NEXT
- Run the harness and confirm it reports passed.
- If a mismatch appears, inspect the printed counterexample and revise the pairwise rule or oracle.
- Implement the final query solver using prev[j] and 2D range-max/point-query structures.
- Test the final solver on samples and on random small instances against the brute-force oracle.

### STATUS
solved
