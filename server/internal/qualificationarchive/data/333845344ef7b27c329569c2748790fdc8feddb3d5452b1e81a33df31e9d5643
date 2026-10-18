# Item 10 — Inbox and Chat acceptance

Accepted owner contract: Inbox is the control center for existing work. Chat is
the place to form a new goal or understand a serious, unclear, or cross-issue
problem; it is not a workflow engine or a status-polling relay.

## The experience

```text
new or unclear goal -> Chat -> exactly one bound issue
                                  |
existing work -------> Inbox <----+
                         |
                         +-- Needs You: inspect, comment, approve or reject
                         +-- Incident: see consequence, direct repair, acknowledge
                         +-- Update: see only a meaningful change and next step
                         +-- Verified: see the proven outcome and acknowledge
```

Inbox excludes ordinary comments, agent/run starts, retries, heartbeats, raw
failures, and unclassified status changes. An item remains independent even
when another item belongs to the same issue. Selection and archive therefore
use the inbox-item id, not the issue id.

The bounded detail begins with what happened and the Current Picture state. It
may show the exact decision, recommendation, consequence, latest meaningful
progress, up to six compact evidence fields, direct evidence/UI links, and the
full-issue link. The owner can comment or correct and can issue distinct
revision, partial-acceptance, scope-change, or cancellation directions without
reading the full timeline. Those five directions use a typed, versioned
`issue_owner_directive` receipt: the exact owner text, issue revision, and
issue comment commit together. In-flight work is stopped before replacement
work can begin; cancellation also moves the issue to Cancelled. A v2
authorization decision alone exposes atomic Approve and Reject; other
directions never inherit that authority.

Chat displays its boundary in the empty state. Its Mika runtime prompt requires
a concrete request to bind one existing issue or create exactly one issue and
then move operational interaction to Inbox. Server guards additionally reject
Chat-originated comments on existing issues, issue mutation, and workflow
commands; Chat can perform intake/binding but cannot become the operational
workflow engine.

## Final review and correction receipt

The one final independent review returned NO-GO with five P1 integration
findings and no P0. One consolidated correction pass closed all five:

1. ordinary comment directives became typed, revision-checked, transactional
   receipts that stop active work;
2. authoritative Current Picture projections now create durable owner Inbox
   records for decisions, incidents, meaningful updates, and verified outcomes;
3. archive and restore became item-level across client and server;
4. list and unread counts now exclude raw notification noise; and
5. Chat's boundary became server-enforced rather than prompt-only.

No second reviewer loop was started. The corrected implementation passed the
focused backend/Core/Views gates, production build, and local browser proof.

## Evidence boundary

Focused tests prove filtering, independent archive/selection, decision packet
handling, bounded controls/evidence, typed issue-bound direction, cancellation,
and deterministic Chat guards. The local browser proves all four owner
categories, bounded controls, item-addressable navigation, Chat separation,
and an actual typed/versioned `correct` receipt on isolated `SILV-911`.
Presentation-category rows remain local fixtures. This does not prove hosted
migration, external notification, mobile parity, SMS, or Phone Farm. Those
remain later roadmap items; Item 11 was not started.
