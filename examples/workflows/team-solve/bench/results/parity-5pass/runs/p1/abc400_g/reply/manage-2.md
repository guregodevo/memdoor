### STATUS
done
### NEXT

### TASKS
- [done] Ideation: reduce the problem to selecting 2K cakes with 3 colors and even color counts, and prove that the optimum is baseline top-2K by max-attribute value plus at most two disjoint recolor/swap corrections.
- [done] Implement the exact baseline parity-exchange algorithm: compute b_i=max(X_i,Y_i,Z_i) and a base color attaining it, sort by b descending, take first 2K as selected T and rest U, baseline sum B and parity mask m; if m=0 output B; otherwise precompute top 5 candidates for each selected recolor role (base a to color c), selected swap-out role (base a), and unselected target role (color c), then evaluate all plans of one operation with mask m or two operations with the other two nonzero masks by brute-forcing distinct assignments of item ids to the <=4 roles and taking B - minimum loss.
- [done] Stress-test the current implementation against a brute-force matcher/colored-selection checker on random small cases (N<=8 or 10, all K, values 0..5 and large ties), including edge cases N=2, K=N/2, all equal values, and ties at the 2K boundary; confirm public samples pass and no mismatches remain.
