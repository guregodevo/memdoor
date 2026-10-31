### PLAN
Model each tile by binary state variables: for type A use P for the north endpoint and Q for the east endpoint, so south and west are forced to 1-P and 1-Q; for type B use R for the north/south endpoints, so east/west are forced to 1-R.  
The no-dead-end condition on the torus is exactly equality of endpoint bits on every shared edge, which becomes a set of xor equations between variables of adjacent cells.  
Build a parity constraint graph whose nodes are these state variables and whose edges are those equations, with constants determined by the A/B types of the two cells.  
Use a weighted union-find with xor parity to detect contradictions and count the connected components of this graph.  
If consistent, each component has one free bit, so the answer is 2^{components} modulo 998244353; otherwise it is 0.

### TASKS
- Read all test cases and, for each cell, assign variable IDs: two IDs (P,Q) for A and one ID (R) for B, storing enough information to retrieve the correct variable for each direction.
- Initialize a parity DSU with one node per variable and a component counter equal to the number of variables.
- For every horizontal torus edge from (i,j) to (i,(j+1) mod W), add the equation: A-A Q⊕Q=1, A-B Q⊕R=1, B-A R⊕Q=0, B-B R⊕R=0.
- For every vertical torus edge from (i,j) to ((i+1) mod H,j), add the equation: A-A P⊕P=1, A-B P⊕R=1, B-A R⊕P=0, B-B R⊕R=0.
- After all equations, output 0 if any contradiction was found; otherwise output pow(2, component_count, 998244353) for that test case.
