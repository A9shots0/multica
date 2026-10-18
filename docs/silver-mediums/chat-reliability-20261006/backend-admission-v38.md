# Backend review-handoff admission repair

## Current checkpoint - 2026-10-06 23:43 UTC

Drew still rejects both current designs. Neither is owner-ready. Technical checks, actual-shell reuse, independent package reopening and reviewer observations did not establish useful product outcomes. No new product brief or approval is required from Drew. The current priority is backend reliability and understanding the product/review failure, not another cosmetic iteration.

Backend v38 `5351e442d5ee14d67e45f01475b2603fafc37965` is pushed and active, image `sha256:8e891f95b2b2c66d5dfc227e64b058721e363a611d29592bfd334963adfd4ded`. It refuses ordinary tasks attempting a legacy squad handoff, and refuses missing explicit successor model/reasoning before advancing state or writing an event. Actual dispatch failures now log the event/source/target and reason. Existing completion recovery is preserved, not newly invented. The failed historical CRM event remains visible. Normal application live verification and health revision readback passed after verified backup `20261006T233800Z-8ac0c41b`; frontend/database container IDs and worker binaries preserved. No schema change in v38. Reboot untested.

Focused regression suite passed (35.826s), affected vet/build passed. The broad run was interrupted without verdict; the bounded combined run timed out at 180 seconds while moving through fixtures, with no assertion failure before timeout. Its completed cases and a separate green tail suite (25.340s) establish the individual workflow-control checks and two selected owner-route test groups, not an all-green combined suite. Exact logs/receipt are in the task artifact directory.

CRM final ZIP and Phone v3.2 final ZIP were independently downloaded, hash checked, extracted and reopened. The repaired Phone mobile menu round-trip was verified in both served candidate and freshly extracted archive. Candidate links/receipts remain in the structured record as rejected evidence. CRM ordinary independent review genuinely began but was cancelled by the newer rejection; no final visual verdict is claimed. Phone independent review was bounded to screenshots/source, with supervisor Browser interactions distinguished. The Mika follow-up's invented request for a new owner direction was corrected through Chat; a general summary-authority fix remains outstanding.

## Failure mechanism

A plain comment-triggered Builder task had no squad binding but was allowed to post an implementation_ready event to a legacy plan. The plan advanced to technical_review before task creation. Its reviewer had no explicit model, so sealed legacy assignment validation failed. Completion recovery joins task and plan by squad, so it could not see this ordinary source. The API returned pending_retry without a visible successor and previously discarded the exact enqueue error.

The fix prevents that mixed route at admission and validates explicit successor configuration before state mutation. It does not relax legacy tool restrictions or claim that a legacy isolated reviewer can use Browser. Ordinary independent delegation remains supported and was observed creating a real reviewer task. Qualitative product review, automatic route selection, and owner-action summary authority remain separate gaps.

Final v38 verification addendum: 27/27 workflow-control top-level tests have individual PASS evidence across the recorded runs; final three test-evidence cases passed separately in 26.958s. The single combined run still timed out. A one-shot normal-Chat negative probe did not reach the server because its prior inventory file was absent in the Chat task workspace. No runtime HTTP-409 pass is claimed. Read-only before/after counts are unchanged: three workflow events, technical_review revision4. This exposes evidence availability/route selection as remaining reusable-workflow gaps.

## Subsequent planning/review investigation

Read-only production observations: both SILV-93 and SILV-94 have acceptance_criteria=[]; CreateIssueRequest in handler/issue.go has no acceptance_criteria input, while sealed legacy assignment construction consumes issue.AcceptanceCriteria. Requirements also exist in issue descriptions/comments, so empty criteria alone does not prove the aesthetic failure. Trace the actual ordinary-review input and how current owner feedback becomes review obligations before proposing a schema/API change; do not mistake an unused field for a complete causal explanation.

Read-only existing Claude CLI auth status returned loggedIn=false on Windows, Mac Mini and Mac Studio. No credential files were read, auth changes made, or alternate API lanes invoked. Working Codex lanes remain available, so this is not a singular execution blocker and no owner authentication request is being imposed.
