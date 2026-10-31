### STATUS
continue
### NEXT
Implement the full Python solution using the derived reduction: for each test case, maintain column A-parity and row prefix; if any row has odd A count mark invalid; for each B cell union row node i and column node H+j with parity w = 1 ^ col_parity[j] ^ row_prefix; after all rows require all column parities zero and no DSU contradiction; output 0 if invalid else pow(2, components, 998244353), where components is the number of DSU components among H+W nodes; then run the sample tests to verify.
### TASKS
- [done] Derive GF(2) reduction: even A count in every row/column; B cells impose row-variable xor column-variable = 1 xor A-above xor A-left; count is 2^{connected components} if consistent.
- [todo] Implement fast Python program with parity DSU over H+W nodes, row/column parity checks, and modular power output.
- [todo] Verify implementation on sample input and small brute-force cases for convention/off-by-one errors.
