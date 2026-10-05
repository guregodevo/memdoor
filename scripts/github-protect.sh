#!/bin/sh
# github-protect.sh — the repository's rules on GitHub, so only the maintainer
# can merge and nothing lands without CI (Greg, 2026-10-05: "none except me
# can approve PR", "no one can take control"). Idempotent; run it after the
# repo goes public (branch protection is a paid feature on a private one):
#
#   make github-protect
#
# What it sets: squash-only merges; main cannot be force-pushed or deleted;
# every change to main comes through a pull request that CI passed and the
# code owner (@guregodevo, .github/CODEOWNERS) approved, with stale approvals
# dismissed on new commits; nobody else can push to main; workflow runs from
# a fork wait for approval; the Actions token is read-only.
set -e
REPO=${REPO:-guregodevo/memdoor}
export PATH=/usr/bin:$PATH

gh api -X PATCH "repos/$REPO" \
  -F allow_merge_commit=false -F allow_rebase_merge=false -F allow_squash_merge=true \
  -F delete_branch_on_merge=true -F has_projects=false -F allow_update_branch=true >/dev/null

gh api -X PUT "repos/$REPO/branches/main/protection" --input - <<'JSON' >/dev/null
{
  "required_status_checks": { "strict": true, "contexts": ["Build, vet & test"] },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": true,
    "required_approving_review_count": 1
  },
  "restrictions": { "users": ["guregodevo"], "teams": [], "apps": [] },
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON

gh api -X PUT "repos/$REPO/actions/permissions/fork-pr-contributor-approval" \
  -f approval_policy=all_external_contributors >/dev/null
gh api -X PUT "repos/$REPO/actions/permissions/workflow" \
  -f default_workflow_permissions=read -F can_approve_pull_request_reviews=false >/dev/null

echo "protected: $(gh api "repos/$REPO/branches/main/protection" --jq '"status checks \(.required_status_checks.contexts), code-owner review \(.required_pull_request_reviews.require_code_owner_reviews), push restricted to \(.restrictions.users[].login)"')"
