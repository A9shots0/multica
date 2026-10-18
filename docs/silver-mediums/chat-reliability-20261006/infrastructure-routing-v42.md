# Ordinary Chat infrastructure repair, 2026-10-07

## Business failure

CRM SILV-95 and Phone Farm SILV-96 remain failed evaluations. The objective is reliable execution from ordinary conversation, not producing another design iteration. The monitor remains paused.

Production v41 was healthy, but three workflow defects were reproduced against its handler code in the task-owned isolated database:

1. Unassigned project intake silently selected the generic squad implementer and installed a legacy plan. Both trials ran Builder on Windows; the Phone Farm specialist was online on Mac Studio.
2. CRM's first correction arrived at 08:29:13Z, before its opening issue was linked at 08:30:15Z. Send-time target capture froze no issue, so the correction failed even though that same opening conversation subsequently created the issue.
3. A technically completed Phone Farm run left its issue blocked and reported inability to proceed. Delivery called the comment a reviewable result.

## Changes

- Chat intake requires an explicit suitable worker; it does not install a legacy workflow as a routing shortcut. Unassigned project issues cannot acquire a generic workflow through the driver's fallback either. The background driver also leaves ordinary Chat execution outside legacy squads, both before binding (exact Chat origin task) and after binding (committed intake receipt). It creates neither a late controller nor an owner question asking for one. `chat bind --agent-id` permits atomic assignment of an unassigned issue, invocation checks, receipt and enqueue. It cannot replace an existing assignee. Failure rolls back assignment and dispatch; matching retries replay once. This also avoids asking the owner to repair routing through an issue update that Chat itself cannot perform.
- An owner turn sent into a conversation with no issue records the exact pending opening task identities. Claim/submission may freeze a correction-only target for the sole issue created by those turns. Manual links, unrelated tasks, ambiguous conversations and decisions are excluded. Owner text stays unchanged; existing revision and permission checks still apply. Once frozen, the target never refreshes itself past a stale revision.
- Latest completed work on a blocked issue is displayed as stopped. Worker reports use neutral language. Custom blocked statuses are respected, attachments stay visible, and stable delivery IDs prevent duplicate historical updates. Historical receipt prose is normalized only for display; original database evidence is preserved. Historical kinds are not retroactively inferred from current issue state.
- Built-in Mika and CLI reference instructions explain these supported recovery paths. No workspace roster, model or identity contract was changed.

## Verification and limits

Evidence folder: `C:/Dev/_artifacts/multica-reset-20261007/`.

- `regressions-before.log`: three reproduced failures before changes.
- `regressions-final.log`: repaired routing, seven correction provenance/authority cases, blocked-report classification; includes refused invocation/admission rollback and replay protection.
- `regressions-broader.log`: surrounding existing correction, intake, delivery, acceptance and decision regressions, including custom blocked status/artifacts.
- `regressions-final-expanded.log`: final handler checks plus two actual driver passes for opening and admitted ordinary work. Existing non-Chat driver visibility cases still pass. The additional driver adoption path was found during surrounding-path inspection before activation.
- CLI HTTP test preserves explicit worker and multiline owner request. Service tests and Go vet passed. View tests: 21 tests passed; views typecheck passed.
- No schema migration, operational CRM/Phone data write, hardware restart or owner acceptance is part of this repair.

This is not hard capability/host routing enforcement. A coordinator can still explicitly choose an unsuitable worker. Role catalog rules and actual project source/host mappings must become machine-enforced routing evidence in subsequent work. Design source fidelity, distinct useful compositions, independent quality review and ordinary conversational success remain unproven. Reboot recovery and generic summary-authority reliability remain separate open gaps.

Release and normal-Chat observations must be recorded after activation; these local checks alone do not establish deployed behavior or business acceptance.
