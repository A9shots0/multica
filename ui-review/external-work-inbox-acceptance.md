# Outside work in Inbox — acceptance record

- Candidate: `codex/external-receipt-inbox`, source candidate `a75f6c237` on base `d1f3f781b4b6cce91e1ba554fc06235e3a5b76fc`, reviewed 2026-10-14.
- Reviewer: builder self-review. The owner prohibited additional agents for this work, so no independent subagent was used.
- Preview: real isolated Multica environment `repo-942` (`api :19022`, `web :13942`) was started from this worktree and then stopped. The in-app browser surface was unavailable, so interactive visual inspection is untested rather than passed.

| Requirement | User task | Expected | Observed evidence | Result |
| --- | --- | --- | --- | --- |
| Routine progress must not create Inbox noise | Let an outside agent keep working | `working` stays on taskboard/history | `TestExternalWorkInboxProjectionFiltersRoutineUpdates`; focused receipt integration test | Pass |
| Important outside updates appear in Inbox | Notice a completion, blocker, stop, or owner question | One visible Inbox group | `owner-inbox-model.test.ts`; `inbox-page.test.tsx`; focused receipt integration test | Pass |
| Provider completion is not company verification | Read an outside completion | Copy says the provider “reported” completion; no verified claim | `ownerAsk` completion test and rendered copy path | Pass |
| Multica must not pretend it controls an outside chat | Open a blocked outside report | No Continue/Restart/answer controls; history and discussion remain available | `OwnerInboxDetail` explicitly excludes `external_work_update` from `answerArea`; typecheck and lint pass | Pass by code/artifact review; interactive state untested |
| Later receipts replace earlier state without duplicates | Receive blocked, then complete, then replay | One stable row; changed receipt reopens it; exact replay preserves read/archive | `TestExternalReceiptProjectsOnlyImportantUpdatesIntoOneInboxRow` against isolated PostgreSQL | Pass |
| Existing Inbox remains stable | Use current decisions, filters, and grouping | Existing behavior continues | Core Inbox 49/49 and Inbox page 45/45 focused tests; Core/Views typechecks; focused ESLint | Pass |

No confirmed acceptance failure remains. Interactive composition and click-through in the real app remain untested because the authorized in-app browser was unavailable.
