### PLAN
Build the required character mapping from S to T; if one source character maps to two different targets, answer is -1. Count present letters and the base cost B, the number of present letters whose required target differs from themselves. Build the functional graph a -> f(a) on present letters and find directed cycles of length at least 2. A cycle is closed if no present letter outside the cycle maps into any of its nodes (equivalently every node in the cycle has indegree 1); only closed cycles need an extra operation. A spare letter is available if some lowercase letter is absent from S or some target letter has at least two present preimages; such a spare can be made empty to break closed cycles. If there is at least one closed cycle and no spare, print -1; otherwise print B plus the number of closed cycles.

### TASKS
- Read N, S, and T; initialize mapping, present, and indegree arrays for the 26 lowercase letters.
- Scan S and T to fill the mapping; if a source letter is assigned two different targets, output -1; otherwise mark present letters and, after the scan, increment indegree[map[a]] once for each present a.
- Compute B as the count of present letters a with map[a] != a.
- Detect cycles in the 26-node functional graph restricted to present letters; for each cycle of length > 1, count it as closed if indegree[v] == 1 for every v in that cycle.
- Determine spare = (number of present letters < 26) or (any indegree[d] >= 2); if closed_cycles > 0 and not spare, print -1, otherwise print B + closed_cycles.
