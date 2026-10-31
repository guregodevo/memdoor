### NOTES
The core difficulty is that each tile has only a few local rotations, but the “no dead ends on a torus” condition couples every neighboring pair of cells and also wraps around both directions. A direct enumeration over all local rotations is exponential, so the problem needs to be converted into a global counting problem with a much smaller state space.

A very useful way to look at the grid is to put a binary variable on every shared edge midpoint: 1 means a line endpoint is present there, 0 means it is not. The no-dead-end condition says that the two cells sharing an edge midpoint must agree on that variable. Then each tile type becomes a small set of XOR constraints on its four incident edge-midpoint variables:

- For an A tile, exactly one horizontal edge and exactly one vertical edge are used. This means the two horizontal variables differ, and the two vertical variables differ.
- For a B tile, either both horizontal edges are used or both vertical edges are used. This means the two horizontal variables are equal, the two vertical variables are equal, and one horizontal variable XOR one vertical variable equals 1.

Thus the whole problem is a system of binary linear equations over GF(2). If the system is inconsistent, the answer is 0. If it is consistent, the number of solutions is a power of 2, because each connected component of the constraint graph contributes one free bit. The main challenge is to exploit the grid structure so that the constraint graph can be processed in near-linear time.

A strong structural observation is that the horizontal constraints in each row form a cycle around the torus, and the vertical constraints in each column form a cycle. Summing the horizontal equations in a row shows that the row must contain an even number of A tiles; if so, all horizontal variables in that row are determined by one free row bit plus prefix parities of A tiles. Similarly, each column must contain an even number of A tiles, and all vertical variables in that column are determined by one free column bit plus prefix parities. After this reduction, A tiles impose no further constraints, while each B tile imposes one XOR equation between its row bit and its column bit. The label of that equation depends on the parity of A tiles to the left and above the B tile. The remaining problem is therefore a weighted XOR graph on H row nodes and W column nodes.

Candidate approaches:

1. Full edge-midpoint XOR graph.
   Create one variable for every shared edge midpoint, for a total of 2HW variables. For each cell, add the local XOR equations for its type. Then use a parity DSU, also called weighted union-find, to check consistency and count connected components. If consistent, the answer is 2 raised to the number of components.
   Pitfalls: This is conceptually simple but uses the largest graph: up to 2e6 variables and up to 3e6 equations in total. In Python, many union operations may be close to the time limit. Indexing the torus wrap-around must be done carefully. It is a good fallback or brute-force checker, but not necessarily the most efficient final approach.

2. Row/column gauge reduction to a row-column XOR graph.
   First check that every row and every column has an even number of A tiles. If not, output 0. If yes, introduce one free bit per row and one free bit per column. Horizontal edge variables are expressed using the row bit and the prefix parity of A tiles in that row; vertical edge variables are expressed using the column bit and the prefix parity of A tiles in that column. Each B tile then gives one XOR equation between its row bit and column bit. Use a parity DSU on H+W nodes, adding one edge per B tile. If consistent, the answer is 2 raised to the number of DSU components.
   Pitfalls: The most likely bugs are off-by-one errors in prefix parities and in the label of the B-tile equation. The column prefix parity must represent A tiles strictly above the current row when processing top to bottom. The row prefix parity must represent A tiles strictly to the left of the current column. The final column parities must be checked after processing all rows. Isolated row and column nodes must be counted as free components; otherwise all-A cases will be undercounted. The parity DSU implementation must correctly maintain XOR-to-root values and update them when attaching roots.

3. Orientation-bit XOR graph.
   Instead of edge-midpoint variables, introduce variables directly for tile orientations: an A tile needs two independent bits, one for choosing left/right and one for choosing top/bottom; a B tile needs one bit, choosing horizontal pair versus vertical pair. The no-dead-end condition becomes equality of affine functions of neighboring orientation bits, which can again be written as XOR equations. Then use a parity DSU or BFS to count consistent assignments.
   Pitfalls: This is closer to the original tile description, but the affine constants are easy to get wrong because A tiles have complemented endpoint expressions while B tiles do not. It also creates more variables and more boundary equations than the row/column reduction, so it is likely slower. It can be useful as an independent prototype for small grids.

4. Transfer-matrix or dynamic programming over one dimension.
   Process rows one by one and keep the vector of vertical edge states across the W columns as the DP state. For each row, the top vertical bits determine the bottom vertical bits through the vertical tile equations, and the horizontal equations plus B-tile cross constraints decide whether the row is valid and how many horizontal choices exist. The torus condition requires the final bottom state to match the initial top state.
   Pitfalls: The state size is exponential in W, or exponential in H if processing columns. This is only feasible when the smaller dimension is very small. It is useful for verification on tiny random cases, but not for the full constraints. The torus wrap-around and the multiplicative weight for rows with no B tiles must be handled carefully.

5. General sparse linear algebra, Pfaffian/FKT, or tensor-network contraction.
   View the problem as counting solutions to a sparse GF(2) linear system, or as counting constrained 2-factors/Eulerian subgraphs on the torus grid. One could attempt sparse rank computation, determinant/Pfaffian methods, or tensor-network contraction.
   Pitfalls: These methods are much heavier and less natural here. Sparse GF(2) rank computation is hard to implement efficiently at this scale. FKT/Pfaffian methods are designed mainly for perfect matchings on planar graphs, and the torus plus mixed A/B vertex constraints make a direct application nontrivial. Tensor-network contraction has exponential treewidth in the grid width. These approaches are overkill unless the simpler XOR-DSU reductions fail.

### NEXT
- Try the row/column reduction with a parity DSU on H+W nodes as the primary implementation: stream rows, maintain column prefix parities, add one weighted edge per B tile, and check row/column parity.
- Implement the full edge-midpoint parity DSU as a simpler fallback and as a small-grid cross-checker, using 2HW variables and all local XOR equations.
- Prototype an orientation-bit XOR graph, with two bits per A tile and one bit per B tile, to independently verify the local endpoint-to-XOR translation on small cases.
- For very small min(H,W), build a row-transfer DP over vertical edge states to validate the linear-algebra result, including torus wrap-around and weights for free row choices.
- Keep general sparse GF(2) rank, Pfaffian/FKT, and tensor-network methods only as conceptual alternatives; do not pursue them unless the XOR-DSU reductions reveal a hidden nonlinearity or inconsistency.
