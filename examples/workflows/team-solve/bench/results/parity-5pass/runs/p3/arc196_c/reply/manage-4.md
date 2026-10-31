### STATUS
continue

### NEXT
Discard the current fragment and write a complete standalone Python 3 solution (single file, no explanations) that passes the three samples. Implement: read N,S; if S[0]!='B' or S[-1]!='W' print 0; precompute fact/invfact up to N; scan S to extract events (b,m) immediately before each B-run (i==0 or S[i-1]!='B') with b>=m and b>0; if no events print fact[N]. For K events, compute g[0..K-1] by g[i]=-(fact[b[i]]+add[i])*invfact[b[i]-m[i]] mod 998244353, where add[i]=sum_{j<i} g[j]*fact[b[i]-m[j]]; answer=fact[N]+sum g[i]*fact[N-m[i]]. Use O(K^2) direct when K<=2000. For larger K use CDQ divide-and-conquer over event index: after solving left half, add its contributions to right half either directly for small left_len*right_len or by NTT convolution with F[m_j-m_l]=g[j], Ga[k]=fact[shift+k], shift=b[mid]-m[mid-1], and add coefficient at x=b[i]-m_l-shift to add[i]. Include a compact correct NTT (radix-2 DIF forward / DIT inverse, no bit-reversal, precomputed stage roots) or try `from atcoder.convolution import convolution` with a manual fallback. Do not add the arithmetic fast path or performance tuning yet; keep the code concise and ensure samples 1,2,3 print 1,0,240792.

### TASKS
- [done] Derive reduction to prefix events and inclusion-exclusion/renewal recurrence.
- [todo] Write a complete standalone Python baseline solver (input, factorials, event extraction, endpoint/no-event, O(K^2) for small K, CDQ+NTT for large K) that passes samples.
- [todo] Add arithmetic-progression fast path (constant b-m and constant step) using polynomial inverse, and try import atcoder.convolution for speed.
- [todo] Validate against brute force for all small N and random small event lists; fix any mismatches.
- [todo] Run N=200000 performance smoke tests (alternating, regular arithmetic, B^N W^N, random/near-alternating) and tune CDQ/NTT thresholds or add specialized fast paths if needed.
