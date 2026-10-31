### PLAN
First check that each character of S has a unique required target in T; if any S-letter maps to two different T-letters, the answer is -1. Compress the problem to at most 26 letter classes, forming a functional graph with an edge a -> b for each present S-letter a that must become b. Let C be the number of present letters whose target differs from themselves. A directed cycle of length at least 2 requires one extra operation only when no present letter outside that cycle maps into it; otherwise an incoming letter can serve as a buffer. The only additional impossibility is when S is not already T, all 26 letters appear in S, and no two present letters share the same target, because then no valid first operation exists. In all feasible cases the minimum is C plus the number of such isolated cycles.

### TASKS
- Read N, S, and T; build req[26] by setting req[S[i]] = T[i] for every position, and immediately mark the case impossible if a letter is assigned two different targets.
- Record the set of present letters and the number of present letters mapping to each target; compute C as the count of present letters a with req[a] != a, and return 0 if C == 0.
- Apply the feasibility rule: if C > 0, no letter is absent from S, and every target has at most one present preimage, output -1; otherwise continue.
- Find all directed cycles of length at least 2 in the graph a -> req[a] over present letters, and count a cycle as bad if every node in the cycle has indegree exactly 1 from present letters.
- Output C + (number of bad cycles).
