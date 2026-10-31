### CODE
```python
import sys
import random


def make_person(pair):
    S, T = pair
    if S < T:
        return (S - 1, T - 1, 1)
    else:
        return (T - 1, S - 1, -1)


def make_people(pairs):
    people = [make_person(p) for p in pairs]
    internals = [list(range(l + 1, r)) for l, r, _ in people]
    return people, internals


def is_bad_pair(a, b):
    l1, r1, s1 = a
    l2, r2, s2 = b

    if l1 == l2 and r1 != r2:
        return True
    if r1 == r2 and l1 != l2:
        return True
    if l1 == l2 and r1 == r2 and s1 != s2:
        return True
    if s1 == s2 and ((l1 < l2 < r1 < r2) or (l2 < l1 < r2 < r1)):
        return True
    return False


def direct_feasible(N, people, internals, mask):
    parent = list(range(N))

    def find(x):
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    M = len(people)

    for i in range(M):
        if mask & (1 << i):
            l, r, _ = people[i]
            a = find(l)
            b = find(r)
            if a != b:
                parent[b] = a

    for i in range(N):
        parent[i] = find(i)

    adj = [0] * N

    for i in range(M):
        if mask & (1 << i):
            l, r, s = people[i]
            cl = parent[l]

            if s == 1:
                for k in internals[i]:
                    ck = parent[k]
                    if cl == ck:
                        return False
                    adj[cl] |= 1 << ck
            else:
                for k in internals[i]:
                    ck = parent[k]
                    if cl == ck:
                        return False
                    adj[ck] |= 1 << cl

    reach = adj[:]
    for k in range(N):
        rk = reach[k]
        if rk == 0:
            continue
        for i in range(N):
            if (reach[i] >> k) & 1:
                reach[i] |= rk

    for i in range(N):
        if (reach[i] >> i) & 1:
            return False

    return True


def compute_bad_masks(people):
    M = len(people)
    bad = [0] * M

    for i in range(M):
        for j in range(i + 1, M):
            if is_bad_pair(people[i], people[j]):
                bad[i] |= 1 << j
                bad[j] |= 1 << i

    return bad


def bad_exists_dp(bad, M):
    size = 1 << M
    arr = [False] * size

    for mask in range(1, size):
        lsb = mask & -mask
        i = lsb.bit_length() - 1
        prev = mask ^ lsb
        arr[mask] = arr[prev] or ((bad[i] & prev) != 0)

    return arr


def report_mismatch(N, pairs, subset, direct, badflag):
    print("MISMATCH")
    print(f"N={N} M={len(pairs)}")
    for idx, (S, T) in enumerate(pairs):
        print(f"{idx + 1}: {S} {T}")
    print("subset:", [x + 1 for x in subset])
    print(f"direct_feasible={direct} bad_pair={badflag}")
    sys.exit(1)


def test_all_subsets(N, pairs, people, internals, bad, bad_arr):
    M = len(pairs)

    for mask in range(1, 1 << M):
        direct = direct_feasible(N, people, internals, mask)
        badflag = bad_arr[mask]

        if direct != (not badflag):
            subset = [i for i in range(M) if (mask >> i) & 1]
            report_mismatch(N, pairs, subset, direct, badflag)


def test_ranges(N, pairs, people, internals, bad):
    M = len(pairs)

    for L in range(M):
        mask = 0
        for R in range(L, M):
            mask |= 1 << R
            direct = direct_feasible(N, people, internals, mask)

            m = mask
            badflag = False
            while m:
                lsb = m & -m
                i = lsb.bit_length() - 1
                m ^= lsb
                if bad[i] & m:
                    badflag = True
                    break

            if direct != (not badflag):
                subset = list(range(L, R + 1))
                report_mismatch(N, pairs, subset, direct, badflag)


def all_pairs_for_N(N):
    return [(S, T) for S in range(1, N + 1) for T in range(1, N + 1) if abs(S - T) > 1]


def run_exhaustive(N):
    pairs = all_pairs_for_N(N)
    people, internals = make_people(pairs)
    bad = compute_bad_masks(people)
    bad_arr = bad_exists_dp(bad, len(pairs))
    test_all_subsets(N, pairs, people, internals, bad, bad_arr)


def run_random(rng, trials, maxN, maxM, mode):
    for _ in range(trials):
        N = rng.randint(3, maxN)
        all_pairs = all_pairs_for_N(N)
        M = rng.randint(1, min(maxM, len(all_pairs)))
        pairs = rng.sample(all_pairs, M)

        people, internals = make_people(pairs)
        bad = compute_bad_masks(people)

        if mode == "subsets":
            bad_arr = bad_exists_dp(bad, M)
            test_all_subsets(N, pairs, people, internals, bad, bad_arr)
        elif mode == "ranges":
            test_ranges(N, pairs, people, internals, bad)
        else:
            if M <= 7:
                bad_arr = bad_exists_dp(bad, M)
                test_all_subsets(N, pairs, people, internals, bad, bad_arr)
            else:
                test_ranges(N, pairs, people, internals, bad)


def run_nobad_stress(rng, trials, maxN, maxM):
    for _ in range(trials):
        N = rng.randint(3, maxN)
        all_pairs = all_pairs_for_N(N)
        rng.shuffle(all_pairs)

        chosen = []
        chosen_people = []

        for pair in all_pairs:
            if len(chosen) >= maxM:
                break
            p = make_person(pair)
            if all(not is_bad_pair(p, q) for q in chosen_people):
                chosen.append(pair)
                chosen_people.append(p)

        if len(chosen) < 2:
            continue

        people, internals = make_people(chosen)
        mask = (1 << len(chosen)) - 1
        bad = compute_bad_masks(people)

        if any(bad):
            direct = direct_feasible(N, people, internals, mask)
            report_mismatch(N, chosen, list(range(len(chosen))), direct, True)

        direct = direct_feasible(N, people, internals, mask)
        if not direct:
            report_mismatch(N, chosen, list(range(len(chosen))), direct, False)

        chosen_set = set(chosen)
        omitted = [p for p in all_pairs if p not in chosen_set]

        if omitted:
            extra = rng.choice(omitted)
            pairs2 = chosen + [extra]
            people2, internals2 = make_people(pairs2)
            bad2 = compute_bad_masks(people2)
            badflag = any(bad2)
            mask2 = (1 << len(pairs2)) - 1
            direct2 = direct_feasible(N, people2, internals2, mask2)

            if direct2 != (not badflag):
                report_mismatch(N, pairs2, list(range(len(pairs2))), direct2, badflag)


def main():
    data = sys.stdin.read().strip().split()

    seed = 1
    trials = 3000
    maxN = 8
    maxM = 8
    exhaustiveN = 5

    if 4 <= len(data) <= 5:
        try:
            seed = int(data[0])
            trials = int(data[1])
            maxN = int(data[2])
            maxM = int(data[3])
            exhaustiveN = int(data[4]) if len(data) == 5 else 5
        except Exception:
            pass

    trials = max(1, min(trials, 10000))
    maxN = max(3, min(maxN, 12))
    maxM = max(1, min(maxM, 10))
    exhaustiveN = max(3, min(exhaustiveN, 5))

    rng = random.Random(seed)

    for N in range(3, exhaustiveN + 1):
        run_exhaustive(N)

    run_random(rng, trials, maxN, min(maxM, 8), "subsets")
    run_random(rng, max(1, trials // 2), maxN, min(maxM, 10), "ranges")
    run_nobad_stress(rng, max(1, trials // 2), maxN, maxM)

    print("OK")


if __name__ == "__main__":
    main()
```

### NOTES
- **Model:** Use prefix potentials P_0..P_{N-1}. A person (S,T) becomes interval [l,r] with l=min(S,T)-1, r=max(S,T)-1, and sign + if S<T, - if S>T. The requirement is P_l=P_r, and for every internal k, P_k>P_l for sign + or P_k<P_l for sign -.
- **Oracle:** For a chosen subset, contract equal endpoint potentials with DSU. Then add directed edges for strict inequalities: sign + gives endpoint component -> internal component, sign - gives internal component -> endpoint component. A self-loop or directed cycle means infeasible; otherwise topological ranks give integer potentials. This is an exact feasibility oracle.
- **Bad pairs:** A pair is bad if it has the same left endpoint with different right endpoints, the same right endpoint with different left endpoints, the same interval with opposite signs, or same-sign strict crossing l_i<l_j<r_i<r_j or symmetric. Each bad pair creates an immediate equality/strict contradiction or a two-cycle, so bad implies infeasible.
- **Tester scope:** The program is a standalone validator. It exhaustively tests all subsets for N=3,4,5 over all valid ordered pairs. It also runs random instances, default 3000 trials, with N up to 8 and M up to 8 for subset checks, plus random range checks and a no-bad stress generator. Optional stdin config is seed trials maxN maxM [exhaustiveN], with caps to keep runtime small.
- **Output behavior:** On the first mismatch, it prints the instance, the subset indices, the direct oracle result, and the bad-pair result, then exits with code 1. If all tests pass, it prints OK.
- **Current status:** The oracle and bad-pair predicate are implemented. No counterexample is known. Running the script is the validation step; if it prints OK, the pairwise characterization is supported for the tested exhaustive and random cases.

### NEXT
- Run the tester with default settings and, if desired, larger seeds/trials within the caps.
- If a mismatch appears, analyze the printed counterexample and revise the forbidden-pair characterization.
- If it passes, proceed to implement the full query solution using earliest bad partners.

### STATUS
solved
