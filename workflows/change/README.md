# change

Finds the change a checkout holds and writes it down once, for the workflows that review, describe or test it. They require its `diff` step; none carries a copy.

`diff` writes three files:

- `.memdoor/runs/change-<partition>.diff`: the unified diff.
- `.memdoor/runs/change-<partition>.stat`: the changed files with their line counts.
- `.memdoor/runs/change-<partition>.base`: the commit the diff is against.

What the change is depends on the checkout:

- In a GitHub pull request (`GITHUB_BASE_REF` set): the pull request against its base branch. The base is fetched, at depth 1 in a shallow checkout.
- On a branch ahead of the default branch: the branch against where it left it.
- Else: the last commit, with uncommitted edits. In a shallow checkout the parent is fetched; a first commit is compared with the empty tree.

Required by `pr-description`, `pr-review`, `pr-team-review`, `security-review` and `write-tests`:

```yaml
requires:
  - project_id: change
    table_pattern: diff
```

`memdoor workflow run change --wait` runs it alone and prints what the change is.
