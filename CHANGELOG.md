# Changelog

## v1.512.0-wiki — 2026-10-06

### Added

- A `sweep` workflow that reads this repository against what Memdoor is today, writes every finding with the stale sentence and its exact replacement to a sweep report, stops at a gate for you to approve, and then applies the must-fix items, building and testing before it commits.

### Fixed

- `memdoor logout` now tells you that remote control and the hosted workflow state are off here while workflows keep running in this project, instead of claiming workflows are off on this Mac.
