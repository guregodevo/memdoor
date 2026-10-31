### STATUS
continue
### NEXT
Implement the cell-state parity DSU solver: A cells get variables P (N/S) and Q (E/W), B cells get one variable R; store hvar/vvar and flip (1 for A, 0 for B); for each horizontal torus edge k->right unite hvar[k] with hvar[right] using constant flip[k], and for each vertical torus edge k->down unite vvar[k] with vvar[down] using constant flip[k]; output 0 on contradiction, otherwise pow(2, component_count, 998244353).
### TASKS
- [done] Ideation: formulate tile orientations as XOR constraints and choose parity DSU
- [todo] Implement the cell-state parity DSU solver: A cells get variables P (N/S) and Q (E/W), B cells get one variable R; store hvar/vvar and flip (1 for A, 0 for B); for each horizontal torus edge k->right unite hvar[k] with hvar[right] using constant flip[k], and for each vertical torus edge k->down unite vvar[k] with vvar[down] using constant flip[k]; output 0 on contradiction, otherwise pow(2, component_count, 998244353)
- [todo] Validate with brute force on tiny random grids (including H=2/W=2 and all-A odd cycle cases) and the sample
- [todo] If validation fails, implement dual edge-state XOR system as an independent cross-check/fallback
- [todo] Finalize fast I/O and memory-efficient implementation for total HW 1e6 and T up to 1e5
