# Plan — team-solve-v2 run 2026-11-02T203329

## Task summary

`solution.py`: read one string S (1 ≤ N ≤ 200000, alphabet {a, b, c}) from stdin,
print the number of non-empty substrings in which the counts of 'a', 'b' and 'c'
are all equal. Samples live in `tests/1.in`..`tests/5.in` with expected outputs
in the matching `.out` files; the deliverable is judged by running the program
on those pairs (check keyword: `samples`).

## Decomposition

1. **Solver** — write `solution.py`, single pass over S using a key built from
   the three running count differences; answer = Σ over unordered pairs of
   positions with equal keys. Verify against `tests/1..5`.
2. **Ideator A and Ideator B (independent)** — each proposes an approach with
   complexity, correctness argument, and Python-specific performance notes;
   insert at `<!-- proposal-a -->` / `<!-- proposal-b -->` in notes.md.
3. **Solver** — picks the better proposal, states the decision at
   `## Solver decision` in notes.md, implements and tests.
4. **Reviewer** — reads `solution.py` and the samples run, writes verdict at
   `## Review`; if anything fails, **Repair** fixes it and the reviewer
   re-checks.

## Candidate approaches

### A. Prefix-count-difference hashing (recommended)

Walk S once, keeping running counts (na, nb, nc). For each position i, the
substring S[i+1..j] is balanced iff counts over that window are equal, i.e.
(na[j]−na[i], nb[j]−nb[i], nc[j]−nc[i]) has all three equal — equivalently
na[j]−na[i] == nb[j]−nb[i] AND nb[j]−nb[i] == nc[j]−nc[i]. Define the key
k = (na−nb, nb−nc); positions with equal keys form balanced substrings between
them. Store k → occurrences in a dict, add the running count per position.

- Time O(N) expected, memory O(N).
- Correctness is direct: equal keys ⟺ the window between the two positions has
  equal a/b and b/c deltas ⟺ all three deltas equal ⟺ balanced.
- Non-empty guarantee: the empty window (same position twice) never happens
  because we add the count *before* processing the next position — seed the
  map with the key of the empty prefix (0, 0) and add the value *before*
  incrementing the map, or increment after querying; either ordering works as
  long as the same position is never paired with itself.
- Python notes: N = 2·10^5 is small; a plain dict with tuple keys or a single
  composite integer key (d1 · M + d2, with d1, d2 bounded by ±N and offset)
  is comfortably fast.

Tradeoff: none of substance — this is the standard, minimal solution.

### B. Per-position pair check with two dicts

Same single pass, but maintain two maps: (na−nb) → count and (nb−nc) → count,
and take the minimum of the two collision counts at each step.

Tradeoff: **incorrect** in general — equal na−nb and equal nb−nc must hold
*between the same pair of positions*, not independently, so this undercounts
(e.g. misses windows where a/b is locally balanced around a different pair).
Recorded here so ideators do not re-derive it.

### C. Brute force over all substrings (for reference only)

Enumerate all O(N^2) substrings and test balance.

Tradeoff: O(N^2) = 4·10^10 operations at N = 2·10^5 — far too slow, and even
at small N it is pointless given A. Reject.

## Instructions for independent ideators

Work alone; do not read the other ideator's entry in notes.md until yours is
written. Insert only at your own marker (`<!-- proposal-a -->` or
`<!-- proposal-b -->`) with a small patch — never rewrite the ledger. Include:
the core observation, the algorithm, complexity, why non-empty substrings are
never double counted or missed, one Python performance concern for N = 200000
and how you avoid it, and which of the four provided samples is the strongest
correctness check and why.

## Instructions for the solver

Choose between the ideators' proposals on merit (they may be equivalent); write
the choice and the reason at `## Solver decision`. Implement `solution.py`:
Python 3 standard library only, read all of stdin, strip the line, print only
the integer. Verify with the sample command in the Check line of task.md:

    for i in 1 2 3 4 5; do diff <(python3 solution.py < tests/$i.in) tests/$i.out || exit 1; done

All five must pass before handing to the reviewer.

## Check run on the current tree (baseline)

The sample loop was run once on the tree as it stands (no `solution.py` yet):
all five pairs fail with "can't open file ... solution.py: No such file or
directory" — the expected baseline before the solver writes the program.
The solver's first job is to make that loop exit silently for all five pairs.

## Instructions for the reviewer

Do not implement; check only. Confirm: (1) `solution.py` uses only the standard
library and prints exactly one integer; (2) no O(N^2) enumeration remains;
counts are aggregated in one pass with a hash map; (3) the empty-substring
edge case is handled (a single 'abc' counts once, not zero, and never as an
empty window); (4) the five sample pairs pass with the loop above; (5) the
tree was not otherwise modified. Write the verdict at `## Review` in
notes.md. On failure, hand to Repair, which patches `solution.py` only and
records what changed at `## Repair`.
