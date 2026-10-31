### STATUS
continue
### NEXT
Implement the complete Python solution using the pairwise-incompatibility reduction: normalize each person to (l,r,dir) with 0-based endpoints, compute endpoint prev with last_left/last_right, then for each sign run a CDQ over its index order (BRUTE~64) where cross steps use separate compressed left-endpoint and right-endpoint coordinates with versioned range-chmax/point-query segment trees to update prev for same-sign crossing pairs, and finally answer queries with an iterative range-max segment tree over prev (Yes iff max prev in [L,R] < L).
### TASKS
- [todo] Implement the complete Python solution using the pairwise-incompatibility reduction: normalize each person to (l,r,dir) with 0-based endpoints, compute endpoint prev with last_left/last_right, then for each sign run a CDQ over its index order (BRUTE~64) where cross steps use separate compressed left-endpoint and right-endpoint coordinates with versioned range-chmax/point-query segment trees to update prev for same-sign crossing pairs, and finally answer queries with an iterative range-max segment tree over prev (Yes iff max prev in [L,R] < L).
- [todo] Verify the implementation on the two samples and on small random cases against a brute-force DSU + directed-cycle feasibility checker, focusing on strict endpoint comparisons and same-sign crossing orientations.
