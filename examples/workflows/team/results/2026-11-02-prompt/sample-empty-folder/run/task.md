# Task

## Prompt

# Problem: Longest Balanced Substring Count

You are given a string S of length N (1 ≤ N ≤ 200000) over the alphabet {a, b, c}.
A substring is *balanced* if it contains the same number of 'a', 'b' and 'c'
(all three counts equal, possibly zero only for the empty substring, which does not count).

Print the number of non-empty balanced substrings of S.

## Input
A single line with the string S.

## Output
One integer: the number of balanced substrings.

## Constraints
N up to 200000, so the solution must run in O(N) or O(N log N) time.

## Examples
Input: abc       → Output: 1      (the whole string)
Input: abcabc    → Output: 5      (abc, bca, cab, abc, and abcabc itself)
Input: aabbcc    → Output: 1      (the whole string)
Input: aaa       → Output: 0

Write the program as solution.py reading stdin and printing the answer.

## Deliverable

<!-- plan: what the run leaves. "changes to this tree", or the new files by name (solution.py, answer.md, …) -->
The new file `solution.py` (Python standard library only, reads the string from stdin, prints the count alone on stdout).

## Check

<!-- plan: one command in backticks that exits 0 when the work is right (`go test ./...`, `npm test`, `make check`); the word samples when tests/N.in and tests/N.out are here; the word judge when nothing can be run -->
samples

## Scope

<!-- plan: the files and directories the work may touch; everything else stays as it is -->
- `solution.py` (create)
- `tests/` may gain additional generated input/output pairs for local verification, but the existing `tests/N.in` / `tests/N.out` files must not be modified.
- Nothing else in the tree may be changed.

## Done when

<!-- plan: what the reviewer and the judge check, one line each -->
- Reviewer: `solution.py` exists, uses only the Python standard library, reads stdin and prints only the integer answer, is O(N) or O(N log N) (no O(N^2) substring enumeration), and passes all five sample pairs.
- Judge: the word `samples` — every pair in `tests/` must agree when `solution.py` is run on `tests/N.in` and compared to `tests/N.out`.
