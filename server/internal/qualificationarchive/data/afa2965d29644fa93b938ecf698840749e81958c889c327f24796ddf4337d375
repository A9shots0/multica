# Hosted Multica workflow and interaction audit

Status: evidence baseline for the self-hosted overhaul
Snapshot window: 2026-09-10 03:54-04:02 UTC
Workspace: Silver Mediums HQ
Method: read-only hosted API and CLI reads plus two independent Astra review lanes

## Why this audit exists

The problem is not merely that Mika's wording is annoying. The owner currently
has to act as the workflow controller: detect that work stopped, find the real
issue, correct Mika's facts, repeat authority, ask whether a release is live,
and discover regressions. This audit compares:

`owner request -> Mika reply -> issue/run evidence -> owner correction -> action -> outcome`

The resulting requirements are inputs to the self-hosted system design. They
do not authorize hosted changes, production changes, Phone Farm access, or a
final UI design.

## Evidence boundary

- All 33 hosted issue records, run histories, active-run lists, usage records,
  status timelines, and 310 root-thread summaries were retrieved.
- 37 selected issue threads returned 636 comments; 222 relevant full comment
  bodies were semantically examined. This is a complete issue census but not a
  word-by-word reading of every historical issue comment.
- Ten active hosted chat histories contained 515 messages at the final chat
  snapshot: 258 owner messages and 257 Mika replies. The conversation audit
  reads the actual messages, not chat summaries. Attachment contents and
  suggested quick-action buttons are outside the count.
- Hosted records can prove what Multica stored and what agents claimed. They do
  not independently prove current production, SMS delivery, employee behavior,
  or Phone Farm state.
- No hosted data, issue, chat, daemon, repository, deployment, or Phone Farm
  system was changed by this audit.

## Executive finding

The current system does not provide a dependable answer to the owner's basic
questions: what is actually happening, what changed, whether anything is
running, what needs the owner, and whether the requested business result is
working now.

The failure is architectural. Mika is being used as conversation interface,
requirements interpreter, approval memory, dispatcher, state machine, status
reporter, and recovery controller. Those roles drift apart. Polishing Mika's
tone without moving workflow truth into deterministic records would leave the
main failure intact.

## Current issue truth

The 33 issue statuses were unchanged between two snapshots five minutes apart:

| Board state | Count |
|---|---:|
| Working | 6 |
| In progress without a named owner state | 2 |
| In Review | 10 |
| Blocked | 7 |
| Needs You | 4 |
| Live | 1 |
| Cancelled | 3 |

Only TASK-47 and TASK-57 had active runs. Six of eight progress-labelled cards
had no active execution: TASK-39, TASK-46, TASK-48, TASK-53, TASK-54, and
TASK-56. A legitimate external wait is possible, but the board does not say
that; it presents all six as active work.

Six of ten generic In Review cards conflict with explicit historical closeout
evidence: TASK-25, TASK-26, TASK-28, TASK-29, TASK-32, and TASK-35. Their stored
status changed in a tight batch under a shared member identity, so the writer
cannot be attributed safely to Drew or to a particular automation.

At least three owner-facing requests are stale or misleading:

- TASK-33 says Needs You while its own metadata says nothing waits on Drew and
  the requested token-removal control was already declined.
- TASK-52 asks about a rollback target that is superseded by later cross-issue
  evidence of a newer Phone Farm release. Current release truth must be
  reconciled before the owner is asked to decide.
- TASK-55 asks Drew to choose among Report 8 options while TASK-57 explicitly
  treats that process as historical and continues a separately approved design
  studio.

Other important mismatches:

- TASK-39 says Working with no run; the stored verification request references
  a CRM revision later identified elsewhere as a rollback target.
- TASK-46 says Working with no run; the reviewed release was routed to the
  wrong host and then stopped on an internal capability problem.
- TASK-48 says Working with no run; the latest recorded provider attempt failed
  and implementation is waiting on an owner preview request.
- TASK-53 and TASK-54 are finished concurrency probes stranded In progress.
- TASK-56 says Working with no run after an independently reviewed candidate;
  the remaining release/receipt step is not represented honestly.

## Inbox truth

At the read snapshot, the active Inbox contained one unread raw assignment for
TASK-47. Archive contained 23 items:

| Notification type | Count |
|---|---:|
| Task failed | 14 |
| Mentioned | 6 |
| Assignee changed | 2 |
| Quick-create completed | 1 |

The Inbox therefore failed as an owner attention surface. It exposed internal
mechanics while four cards claimed Needs You, multiple releases/regressions
needed reconciliation, and several Working cards had no worker. No observed
active or archived type expressed a structured owner decision, business
consequence, regression, rollback, verified outcome, or next meaningful
action.

## Interaction audit

### Measured conversation load

| Chat | Messages | Owner / Mika | Recorded span |
|---|---:|---:|---|
| Audit and Redesign Weekly Reports Page | 114 | 57 / 57 | Sep 8-Sep 10 |
| CRM Outreach DM Notification via Phone Farm | 105 | 53 / 52 | Sep 7-Sep 10 |
| Outstanding Issues and Required Actions | 24 | 12 / 12 | Sep 8-Sep 10 |
| Audit Linkhub and Phone Farm Setup | 98 | 49 / 49 | Sep 7-Sep 10 |
| Cool Phone Farm Issue Resolution Failure | 30 | 15 / 15 | Sep 8-Sep 10 |
| Audit and Remove Legacy Phone Farm Macros | 8 | 4 / 4 | Sep 9-Sep 10 |
| Audit Gallery Discrepancies and Posting Media Limit | 46 | 23 / 23 | Sep 9 |
| Task 31 Direction Issues | 30 | 15 / 15 | Sep 7-Sep 9 |
| Phone Farm Automation Failures | 38 | 19 / 19 | Sep 7-Sep 8 |
| Audit Data Accuracy and Discrepancies | 22 | 11 / 11 | Sep 7-Sep 8 |
| **Total** | **515** | **258 / 257** |  |

In the independently counted five-chat half alone:

- The owner corrected Mika's understanding or behavior in 5/5 chats.
- Mika made a substantive claim later corrected or retracted in 4/5 chats.
- 13 of 72 replies opened with "You're right."
- 29 of 72 replies contained owner-action, decision, approval, or Needs You
  language.
- 23 of 72 replies discussed previews or mockups across all five chats.
- 20 of 72 replies explicitly invoked named workflow or knowledge rules,
  concentrated in two chats.

These are lexical counts, not automatic quality judgments. Their importance is
the interaction sequence: the acknowledgments and process language commonly
followed an avoidable wrong claim, unnecessary gate, or lack of execution.

The 98-message LinkHub history makes the amplification especially concrete.
Twenty-two Mika replies contain a clear admission, reversal, or material
correction. At least 24 of Drew's 49 turns were spent correcting facts,
rejecting unnecessary scope, restating an existing requirement, demanding a
direct answer, or recovering lost authority. Across roughly 56.5 hours, the
conversation moved through a wrong wildcard-domain blocker, an unsuitable bulk
UI, an admitted idle task previously described as running, a fake 200-link
test, revived legacy work the owner had intentionally removed, an inaccessible
Mac-localhost preview, a Save fix that passed 557 checks but immediately failed
its central requirement, and a release halted for approval already given. The
recorded end state was one successful LinkHub handoff, no successful Instagram
completion, a device-assignment failure for LH145, and the approved Koala test
still pending. One direct execution request received only "Stopped."

### Repeating failure loops

#### 1. Owner asks for a result; Mika reports the owner's evidence back

In CRM Outreach, the owner supplied proof that a DM was received and replied
to. Mika restated outbound delivery and the empty Inbox before revealing, only
after challenge, that the inbound check had never started because an obsolete
outbound check occupied the workspace. The answer that mattered was hidden
behind two turns of information the owner already knew.

Required behavior: answer from the current issue/run/evidence record first.
State only new information, the evidence boundary, and the next automatic or
owner action.

#### 2. Mika claims progress before execution exists

This appears across all ten chats. Examples include a two-hour queued issue
with no activity, a release lane that had done nothing for roughly 40 minutes,
an issue described as actively being fixed after its last run ended, and six
current Working/In-progress cards with no active execution.

Required behavior: a promise is not progress. Working requires a live lease,
worker, expected external event, or scheduled retry. Every dispatch must have
an action receipt. Missing execution must trigger controller recovery without
waiting for Drew to notice.

#### 3. Mika guesses, then agrees after the owner corrects it

Examples include incorrect September report history, incorrect staff roles and
daily/weekly volume, invented Phone Farm stuck-state and Instagram-rejection
diagnoses, wrong CRM project routing, and changing KPI definitions presented as
verified. In one chat Mika explicitly admitted, "Everything else I said was
invention."

Required behavior: distinguish owner statement, stored record, observed
mechanism, and independently verified business result. Unsupported hypotheses
must never be phrased as findings. A correction must update durable scope and
invalidate dependent conclusions, not merely produce another apology.

#### 4. Approved work repeatedly returns to Drew

Gallery release authority was given, delayed for idle windows, given again,
then requested again through a Mac-localhost preview that Drew could not access
from Windows. The assistant later admitted losing both context and approval.
The CRM Outreach history contains repeated rediscovery of clear ship
authorization. Macro removal also required the owner to correct who creates
tasks and reject an unnecessary removal preview.

Required behavior: authorization must be a candidate- and scope-bound durable
receipt with revocation and supersession rules. Handoffs and retries inherit a
still-valid receipt. The owner is asked again only when the candidate, scope,
audience, risk, or business choice materially changes.

#### 5. Internal review is turned into owner review

Mika repeatedly asks Drew to approve mockups, repair plans, or routine technical
steps before independent internal review has converged. In the report-design
chat, the owner became the effective requirements keeper and visual QA agent,
catching constraints that multiple internal rounds allowed through.

Required behavior: internal reviewer failures create structured correction
packets and bounded retries. Drew receives a decision only for actual direction,
business acceptance, external authority, or irreducible tradeoff. UI feedback
must support comments, attachments, annotations, revision requests, partial
acceptance, and comparison; it cannot be reduced to one Approve button.

#### 6. Status vocabulary obscures employee reality

The gallery conversation alternated among running for employees, deployed, not
verified Live, and Live. Drew needed three increasingly direct questions to
learn what employees were actually using. A production failure then returned a
one-word "Stopped" response, followed later by rollback disclosures.

Required behavior: separately store and display candidate built, reviewed,
deployed, exposed population, runtime check, business check, degraded, rolled
back, and requested outcome still open. "Live" is reserved for current target
revision plus fresh runtime and business acceptance evidence.

#### 7. Audit completion is mistaken for problem completion

Phone Farm failures were described as fixed and Live after one scrolling test;
later Mika admitted the issue had been closed after diagnosis without the
scrolling and Account Status fixes. The reporting audit similarly closed with
nothing coded. This is the source of the repeated "then do something" exchange.

Required behavior: each issue keeps a durable business outcome and explicit
milestones. Finishing investigation, implementation, review, release, or one
verification slice cannot close a broader outcome.

#### 8. The owner must poll because follow-through is not durable

Mika promised proactive follow-up, then later said it only sees new comments
when Drew messages and told him to ping it. Across the histories, Drew repeatedly
asks for updates, opens issues, checks board state, and supplies screenshots to
discover stalled work or regressions.

Required behavior: an event-driven controller owns timers, retries, expected
signals, and stale-work detection. Chat and Inbox read the same event stream.
No status-check LLM loop should be required to know whether work advanced.

## Run and token amplification

- 2,827 issue runs were recorded: 2,304 completed turns, 468 failed, 53
  cancelled, and 2 running at T1. A completed turn is not a completed issue.
- 416 of 468 failures, 88.9%, were quota or capacity related across ten issues.
- TASK-52 alone recorded 155 weekly-limit failures. TASK-47 recorded 158 total
  failures, including 134 weekly-limit failures.
- Report tasks TASK-50, TASK-55, and TASK-57 consumed 161 runs, 4.85 million
  input/output tokens, and 325.81 million cache tokens while the accepted
  reusable report remained unfinished.
- Across 1,679 metered task records: 46,493,569 input tokens; 18,844,032 output;
  5,746,265,366 cache-read; and 103,829,683 cache-write. Monetary ticks were
  zero while tokens were marked uncosted, so dollar cost is unknown, not free.

The waste is not primarily one expensive answer. It is amplification: Mika
status turns, worker turns, reviewer turns, retries, polling, re-reading full
history, lost decisions, misrouting, and owner correction loops compound.

## Requirements forced by this evidence

1. The deterministic controller, not Mika, owns transitions, dispatch,
   retries, timers, leases, and reconciliation.
2. One current-picture record binds original outcome, accepted scope, current
   milestone, candidate/revision, evidence freshness, active actor, next event,
   owner decision, and superseded facts.
3. Every owner decision and release authorization is durable, typed,
   candidate-bound, scope-bound, attributable, revocable, and inherited across
   valid handoffs.
4. Every promised action has a receipt. If no worker starts or a lease dies,
   the controller retries or escalates automatically within a bounded policy.
5. Internal review produces structured verdicts and correction packets. Mika
   is not woken for routine worker/reviewer transitions.
6. Board state is a projection of controller truth and cannot be silently
   overwritten by chat prose, a shared member token, or a stale automation.
7. Inbox is a prioritized consequence-and-action queue, not a raw event feed.
   It groups meaningful changes by issue while keeping unresolved decisions and
   urgent incidents visible.
8. Issue view opens on the current picture and newest meaningful activity,
   while preserving full chronology and the original request.
9. Chat is the conversational interface for intent, ambiguity, direction, and
   cross-issue questions. It reads the same current picture; it does not invent
   a parallel narrative or become the polling engine.
10. Notifications distinguish detected, sent, delivered, read, acknowledged,
    and resolved. Urgent regression, rollback, failed release, employee block,
    and true owner decision have explicit phone/SMS policies.
11. Release state separates source, reviewed candidate, deployed revision,
    runtime health, employee exposure, business result, regression, rollback,
    and outcome recovery. Permanent regression checks travel with the feature.
12. Token governance measures cost per accepted business outcome, not per
    successful model call. Context packets are bounded; retry ceilings and
    quota-aware model fallback are deterministic.
13. Owner responses support more than Approve/Reject: comments, questions,
    annotations, attachments, revision requests, partial acceptance,
    postponement, cancellation, factual input, and explicit rejection.
14. All surfaces use the same semantics, but have different jobs: Board for
    portfolio position, Issue for durable truth/evidence, Inbox for attention,
    Chat for conversation, and phone for interruption-worthy events.

## What this does and does not settle

This audit is strong enough to reject the hosted interaction model as the
design baseline and to require deterministic workflow control. It is not a
complete design for the replacement. The exact Inbox information architecture,
issue current-picture layout, UI-review interaction, notification thresholds,
release adapters, recovery policies, and permanent agent/squad roster still
need scenario design and acceptance testing.

The next planning step is not another agent roster or another happy-path pilot.
It is to turn these observed failure loops into a scenario catalog and one
conflict-free contract spanning Board, Issue, Inbox, Chat, phone, controller,
review, release, recovery, and evidence.
