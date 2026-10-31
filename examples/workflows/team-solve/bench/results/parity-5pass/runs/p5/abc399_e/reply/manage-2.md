### STATUS
done
### NEXT

### TASKS
- [done] Implement initial Python solution using closed-cycle formula (passed public samples but open-cycle case incorrect).
- [done] Derive corrected formula: all cycles require +1; impossible only when cycles exist and no buffer exists.
- [done] Update the Python solution to the corrected formula: count every directed cycle of length >=2 in the functional graph of changed appearing letters; a buffer exists iff not both S and T use all 26 letters; if cycles exist and no buffer, print -1, otherwise print changed_count + cycle_count.
- [done] Validate corrected solution on samples and edge cases: open cycle with incoming edge, multiple cycles, full 26-letter permutations, full S but T not full, acyclic chains, inconsistent mapping.
