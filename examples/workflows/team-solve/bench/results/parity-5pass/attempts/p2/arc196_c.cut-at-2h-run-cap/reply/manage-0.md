### STATUS
continue
### NEXT
Implement the complete Python solution using the recurrence H[b] = (fact[e[b]] - sum_{c<b} H[c]*fact[e[b]-c]) * invfact[e[b]-b] mod 998244353, where e[b] is the number of W before the (b+1)-th B with e[N]=N, and H[b]=0 unless b>0, e[b]>=b, and e[b]>e[b-1]. Early-exit with 0 if S[0]!='B' or S[-1]!='W'. Compute H[0..N] by CDQ divide-and-conquer over b with a leaf threshold (e.g. 64) and a global sorted list of nonzero H indices. In apply(l,m,ql,qr), add contributions only to valid b in [ql,qr]; use direct loops when nonzero_count*valid_count is small, otherwise NTT-convolve H[c0..c1] with fact[t_min..t_max], where c0/c1 are the first/last nonzero H in [l,m], t_min=max(0, e[first_valid_b]-c0-(c1-c0)), t_max=e[last_valid_b]-c0, and query conv[e[b]-c0-t_min]. Include a correct fast NTT for 998244353 and print H[N].
### TASKS
- [todo] Implement the full CDQ+NTT solution: precheck, e/valid/valid_pos, factorials/invfact, CDQ with leaf threshold, global nonzero H list, apply helper with direct/NTT cost model, and output H[N].
- [todo] Implement a fast NTT convolution modulo 998244353 (power-of-two length, direct fallback for tiny sizes) and unit-test it against naive convolution on small random cases.
- [todo] Ensure all index bounds are safe: valid[b] implies e[b]-b>=0 and e[b]-c>=0 for c<b; NTT B slice stays within fact[0..N]; convolution query indices are valid.
- [todo] Test on samples 1-3 and edge cases (N=1, all B then all W, precheck failures, small random brute-force verification) and fix any failures.
