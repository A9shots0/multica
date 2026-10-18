# Shared-brain release handoff

Observed 2026-09-24 UTC. This is a local integration handoff, not authorization
to push, merge, deploy Multica, apply production migrations, or approve its UI.

## Exact targets

- Canonical repository: `https://github.com/A9shots0/silver-mediums-agent-control-plane.git`.
- Canonical Git destination: `origin/main`. Fresh `git ls-remote --symref` returned
  HEAD -> main and SHA `8fac9a705dac91b600b26c14f2f00f4490f6218b`.
- Existing recovery branch `recovery/2026-09-22-overnight-snapshot` is the saved
  snapshot at `8080a0e4422ad7adf6d00461fc8e5bbeb44aaa3e`, not the canonical default.
  Its occupied checkout contains an owner modification to `auth.go`; preserve it.
- Brain source handoff: `codex/brain-integration-20260924`,
  `C:/Dev/_work/multica-brain-integration-20260924`,
  `5c0bac36e20184b9b9803e823acb4767d3f5af53`.
- Combined local integration candidate already exists:
  `codex/brain-owner-review-20260924`,
  `C:/Dev/_work/multica-brain-owner-review-20260924`.
  Implementation/review checkpoint `f0d6d9a5f2b0685b46679cb48c01f566c5be5486`.
  It includes owner Chat `a568d6daf65d3505e7b736a74f43413c926f080a` as an ancestor.
  All five brain source commits are patch-equivalent here (`git cherry` reports
  all five with `-`), so replaying them would duplicate the integration.

The owner preview remains `codex/mika-chat-local-review` at
`C:/Dev/multica-mika-chat-local-review`, ports3220/3221 and database55438.
Its approval is separate. No branch has been designated a newly approved
production release by this handoff. Do not publish the brain source branch as a
complete latest-Chat candidate: it branched before the owner's final UI polish.

## Existing release workflow and remaining gates

The repository's private release workflow is receipt-based:
`scripts/release-candidate.py`, `scripts/release-qualify.py`,
`scripts/release-recovery-rehearsal.py`, and the existing controlled release
scripts. `.github/workflows/release-candidate-images.yml` builds an exact pinned
commit from `.github/release-build/request.json`; the request presently points
to an older release and must not be treated as this candidate. The separate
`release.yml` tag workflow publishes versioned releases. Neither was triggered.

For the next authorized Multica release:

1. Obtain owner acceptance of the latest local Chat UI before including that UI
   in a release. If a backend-only release is desired sooner, explicitly scope
   and verify that slice against the actual deployed base; the brain commits
   depend on existing Chat target/task changes and are not proven independent
   of the local recovery/Chat baseline.
2. Refresh remote main, deployed source identity, occupied worktrees and current
   migration ledger. Review the complete delta from main/deployed base, including
   the recovery snapshot; this handoff proves the brain slice, not every earlier
   unpublished change. Production source identity and recovery health were not
   re-established in this branch-identification task.
3. Pin the accepted combined source, regenerate its source fingerprint and build
   receipts, run the existing changed-package qualification, and verify migration
   ordering/compatibility. Chat migration582 and brain migrations583-586 have only
   been exercised on isolated databases here. Protect active tasks and uploads.
4. Establish fresh backup, isolated restore/rollback rehearsal and off-machine
   recovery readback before production changes. Configure the protected real
   workspace/project mapping and compatible server/daemon versions; do not copy
   the disposable review mapping or claim credentials into production.
5. After applicable publication/deployment authorization, publish/merge to main
   and deploy the exact qualified artifact, then verify the real owner Chat flow
   and runtime identity separately. A Git merge, worker completion, or shared
   handoff does not establish UI acceptance or business success.

## Completed evidence reused

See [independent review](shared-brain-owner-review.md): scoped context and
immutable reads, selected-issue project binding, receipt validation, durable
replay, history/evidence retrieval, actual Codex-adapter use and independent
Drew Mini readback all passed. No extra provider run is needed to repeat them.
Temporary task credentials were revoked; retained evidence is under
`C:/Dev/_work/brain-completion-evidence`.

Brain integration coordinator: task `01a0d16b-c68e-7f30-afd4-a4884610dfa8`.
Multica Chat/release continuation stays with the owner task and existing review
candidate. The remaining dependency is UI acceptance plus the separately
authorized release qualification above, not more shared-brain implementation.
