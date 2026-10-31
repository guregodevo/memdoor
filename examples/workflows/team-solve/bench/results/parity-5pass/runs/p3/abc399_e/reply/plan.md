### PLAN
First check consistency: for every letter c that appears in S, all positions i with S[i]=c must have the same T[i]; otherwise print -1. Let f(c) be that required target and build a directed graph on 26 letters with an edge c -> f(c) only for present c with f(c) != c. The number of these edges is the base cost, since each such letter must be chosen as x at least once. In this outdegree-at-most-one graph, every directed cycle of length at least two needs a temporary letter; if some non-cycle changed letter has an edge into the cycle, its required operation can provide the temporary, otherwise the cycle contributes one extra operation. If all 26 letters appear in S and there is no non-cycle changed letter but some changed letter exists, the required mapping is a non-identity permutation and cannot be obtained by any non-empty sequence of replacements, so print -1. Otherwise the minimum number of operations is the number of changed edges plus the number of such isolated cycles.

### TASKS
- Read the input, mark which 26 letters appear in S, and compute map[c] for each appearing c; if any c maps to two different letters in T, output -1 and stop.
- Build edge[c] = map[c] for appearing c with map[c] != c, count changed = number of such edges, and detect all directed cycles in this functional graph, marking which vertices are in cycles.
- For each detected cycle, determine whether any vertex outside the cycle has an edge to a vertex in that cycle; count cycles with no such incoming edge as isolated.
- Set free_exists to true if any letter is absent from S and tree_exists to true if any changed vertex is not in a cycle; if not (free_exists or tree_exists) and changed > 0, output -1.
- Otherwise output changed + isolated_cycles.
