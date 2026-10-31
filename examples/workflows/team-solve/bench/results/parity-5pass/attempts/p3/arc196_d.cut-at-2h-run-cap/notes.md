
## ideation
The road strengths can be eliminated by using prefix sums. Let P_1 = 0 and P_{x+1} = P_x + w_x. For a person whose route covers the interval [l, r] with l = min(S_i, T_i) and r = max(S_i, T_i):

- If S_i < T_i, the stamina at town x is P_x - P_l. The requirement becomes P_l = P_r and P_x > P_l for every l < x < r.
- If S_i > T_i, the stamina at town x is P_l - P_x. The requirement becomes P_l = P_r and P_x < P_l for every l < x < r.

Thus each person is an interval with a sign: “up” means the interior must be strictly above the endpoint value, and “down” means the interior must be strictly below the endpoint value.

For one fixed set of people, the feasibility problem is a system of equalities and strict inequalities over integer prefix values. Contract towns that are forced equal by endpoint equalities. Each person then imposes strict inequalities between equality components: an “up” person makes every internal component greater than its endpoint component, and a “down” person makes every internal component smaller. Because the values are integers, such a system is feasible exactly when the resulting directed graph of strict inequalities is acyclic; a topological order can be turned into integer prefix values, and shifting makes P_1 = 0.

The real difficulty is answering many interval queries over the people. A per-query construction of the equality/inequality graph is far too slow. The promising reduction is that infeasibility may be detectable by forbidden pairs of people. The natural forbidden pairs are:

1. The same interval with opposite signs.
2. Two intervals with the same left endpoint but different right endpoints.
3. Two intervals with the same right endpoint but different left endpoints.
4. Two same-sign intervals that strictly cross.

If this pairwise characterization is valid, then a query [L, R] is bad exactly when it contains at least one forbidden pair. Then define p[j] as the largest index i < j such that i and j form a forbidden pair. A query [L, R] is Yes iff max p[j] over j in [L, R] is less than L. This reduces the problem to computing p[j] for all j and answering static range-maximum queries.

Computing endpoint-based conflicts is easy with last-occurrence maps. The hard part is same-sign crossing conflicts. For each j, we need the maximum earlier i with the same sign and either:

- l_i < l_j < r_i < r_j, or
- l_j < l_i < r_j < r_i.

This is a 3D dominance / range-maximum problem because of the index constraint i < j plus the two endpoint coordinates.

A major risk is the pairwise characterization itself. In principle, a set could be infeasible because of a directed cycle of length three or more even if every pair is feasible. Any pair-based approach should be validated by proof or small random brute-force checks. If the characterization fails, the whole reduction to forbidden pairs is invalid, and one would need a much heavier general partial-order feasibility method.
