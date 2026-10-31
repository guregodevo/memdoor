### STATUS
continue
### NEXT
Implement the complete `Solution.maxProduct` as a single concise Python code block with no explanation, using this compact algorithm: early return -1 if abs(k)>sum(nums); let offset=sum(nums), mask=(1<<(2*offset+1))-1, target=1<<(k+offset). For positive products, pos0/pos1 are lists of int bitsets length limit+1. For each x in nums with 1<=x<=limit: if x==1, snapshot old0=pos0[:], old1=pos1[:], for p=1..limit do pos1[p]|=(old0[p]<<1)&mask and pos0[p]|=old1[p]>>1, then pos1[1]|=1<<(1+offset); else for p=limit//x down to 1 do pos1[p*x]|=(pos0[p]<<x)&mask and pos0[p*x]|=pos1[p]>>x, then pos1[x]|=1<<(x+offset). Scan p=limit..1 and return p if (pos0[p]|pos1[p])&target. If none, run zero DP: no0=1<<offset, no1=z0=z1=0; for each x: if x==0 set nz0=z0|z1|no1, nz1=z1|z0|no0, z0,z1=nz0,nz1; else save old no0,no1,z0,z1 and update no1|=(old_no0<<x)&mask, no0|=old_no1>>x, z1|=(old_z0<<x)&mask, z0|=old_z1>>x. Return 0 if (z0|z1)&target else -1.
### TASKS
- [todo] Implement a compact, complete `Solution.maxProduct` using product-major positive bitset DP and separate zero-product reachability DP; output only code, no long derivation.
- [todo] Run the provided examples and brute-force random small cases against exhaustive enumeration; fix any mismatches.
