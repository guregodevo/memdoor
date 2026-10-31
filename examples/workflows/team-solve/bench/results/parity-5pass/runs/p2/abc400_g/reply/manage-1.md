### STATUS
done
### NEXT

### TASKS
- [done] Ideation: reduce to labeled selection, baseline top-2K, and parity correction with at most two operations.
- [done] Implement the complete Python solution using the fixed baseline parity-correction algorithm: choose p_i as any maximizing attribute, take top 2K by M_i, compute baseline sum and parity mask, build top-two selected/unselected/recolor/swap candidates, compute min single or conflict-free pair correction, and output baseline sum minus min loss.
- [done] Verify the implementation on the provided samples (passed) and review edge cases (ties in M_i, multiple maximizing attributes, K=N/2) in the algorithm.
