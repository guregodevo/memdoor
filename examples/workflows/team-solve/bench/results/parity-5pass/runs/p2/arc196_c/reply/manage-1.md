### STATUS
continue

### NEXT
Implement a complete Python 3 baseline solver (no NTT) for the problem using the active-cut inclusion-exclusion DP: read N,S; if S[0]!='B' or S[-1]!='W' print 0; scan the string to collect active pairs (u=i, t=C_i) for i=1..N-1 where C_i is the number of W before black i+1, C_i>C_{i-1}, and C_i>=i; precompute fact and invfact up to N modulo 998244353; use arrays u=[0]+active_u, t=[0]+active_t, dp=[1], and for i=1..K compute s=sum_{j=0}^{i-1} dp[j]*fact[t[i]-u[j]] mod, dp[i]=(-s)*invfact[t[i]-u[i]] mod; answer=sum_{j=0}^K dp[j]*fact[N-u[j]] mod; print answer. This O(K^2) version must be a standalone stdin/stdout program and pass samples 1-3; do not implement NTT yet.

### TASKS
- [todo] Implement baseline O(K^2) active-pair DP solver (no NTT) as a complete stdin/stdout program: precheck, active extraction, factorials/invfact, DP, and answer; pass samples 1-3.
- [todo] Verify baseline against brute force for all/random small N (N<=7) and edge cases (N=1, S[0]!='B' or S[-1]!='W', all B then all W, alternating small).
- [todo] Implement fast NTT convolution modulo 998244353 (iterative, power-of-two length, direct fallback for tiny sizes) and unit-test it against naive convolution on small random cases.
- [todo] Implement CDQ divide-and-conquer optimization for the active-pair DP: recursive solve(l,r), accumulated contribution array, left-to-right contributions via direct loops or NTT with a cost model, including dummy contribution and safe index bounds.
- [todo] Test optimized solution on samples, small brute-force cases, and large performance cases (N=2e5 alternating, all B then all W, random large sanity), fixing any failures or TLE.
