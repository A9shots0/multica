# Controlled completion correction: review resolution

Updated: 2026-09-15T20:42:22.818734+00:00

Scope: native qualified-broker reply parents and one supported requesting-member operator completion command, followed by bounded native publication acceptance. The original owner/release/A: worktrees and all historical SILV-38 tasks remain preserved. Source correction runs in `completion-20260915`, branch `codex/workflow-completion-20260915`.

## One independent review and consolidated correction

The independent reviewer examined 29 frozen application/acceptance/release files. Report SHA256: `f6892978758c17249eb725149146f6630fa6e529b76aba033cce78ac574cab28`; source manifest SHA256: `7a9e50456b93a2924e20d4f796b3ac38c8cf5f8fee39afcbec8483b58e8e4c91`. Original review artifacts are retained under `.qualification-evidence/completion-correction-20260915/independent-review/`.

| Finding | Consolidated correction | Verification |
| --- | --- | --- |
| Pending owner decision bypass | Explicit eligible legacy states; reject suspensions, unanswered native decision events and active normalized decisions | Supported dynamic owner question remains actionable with policy flag false; completion returns 409 with no writes |
| Plan/issue lock cycle | Exclusive issue admission plus a dedicated `FOR UPDATE NOWAIT` plan read | Deterministic overlap with actual native event, failure projection and owner-decision writers; completion yields 409 and native writer commits |
| Captured evidence drift | Fixed evidence SHA256 before parsing/provider use and again during artifact publication | Changed JSON is rejected; candidate digest remains fixed |
| CLI/helper digest enforcement | Check all binaries against the bound build receipt at each sensitive stage; verify local reviewed source around build | Modified CLI or authority helper is rejected before use |
| Mixed release evidence paths | Use `production-completion-20260915` consistently, retaining old release evidence | Offline release/rehearsal path check |
| Old image verifier in rehearsal | Use successor verifier with the same final release identity | Synthetic OCI archive accepted only for the exact candidate/rollback set; missing image rejected |

Additional completion checks preserve historical failure rows, bind exact replay to its own completion event, reject stale/elevated/machine/normalized/deployment inputs, roll back when Inbox admission fails, and commit one completion under concurrent replay. The supported CLI submits one human-authenticated request and does not retry a conflict automatically. Core receipt validation rejects malformed responses.

## Evidence boundaries

The wider suite has preserved baseline failures: two architecture-guard direct legacy writes; two unrelated Windows CLI assertions/timeouts; one missing-dispatch recovery expectation. Each was reproduced on original 3228 source. The new changes do not modify those legacy writers. The missing-dispatch test expects a leader recovery task after a deliberately absent successor; the bounded acceptance must establish a real successful native successor instead.

Review-to-candidate binding for SILV-38 is an explicit requesting-member attestation. The real completed independent review, actual publication and fixed artifact digest remain distinct evidence. The assisted completion event preserves that provenance and the prior failed picture. Native automatic handoff is a separate new-issue acceptance and is not inferred from assisted closure.

Local synthetic Browser reproduction was observed on loopback 13005: Done property, technical-stop Now and Blocked board. Corrected CHECK-1 acceptance then passed with explicit operator assistance and unchanged failed-task history. This independent review does not establish later production or provider acceptance.

## Subsequent live-data follow-up

The first candidate was deployed after matching recovery gates. Its supported SILV-38 completion returned HTTP 409 because the genuine direct requesting-member review retained a NULL squad ID. The rejected transaction produced no completion event or workflow mutation; the real owner digest attestation is retained.

The targeted follow-up accepts that case only when the original human originator and immutable trigger-comment identity match the requesting member, issue and workspace, the comment precedes review completion, and the current plan still names the same independent reviewer. A different non-null squad remains invalid. Historical task provenance is not rewritten.

The positive real-data shape was reproduced before the change. Nine focused cases and 102 affected handler test events passed after it. Second candidate `60fea34b9770-local-9eaa711d97b1` built successfully and CHECK-2 passed the supported command plus built-in Browser acceptance across Issue, Now, Inbox and Board, including reloads. Only three application files changed between the two candidates; all other build inputs retained their hashes. This follow-up was verified locally and is not described as a second independent review. Actual second deployment and native acceptance are recorded separately in `COMPLETION-CORRECTION-RESULT-20260915.md`.
