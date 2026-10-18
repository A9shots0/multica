# Multica system scope review

**NON-AUTHORITATIVE / NO IMPLEMENTATION DECISIONS**

Advisory planning audit for the coordinator and Drew. Snapshot: 2026-09-09,
23:23 EDT. Evidence stage: local-only source/document review. The user has
paused feature implementation to understand the whole operating system. This
report proposes questions and proof obligations; it authorizes no feature,
architecture, agent change, release, migration, or external action.

## Assessment

The pilot has useful, narrowly demonstrated workflow mechanisms. It does not
yet establish a coherent company development experience. The largest gap is the
contract connecting human intent, executable scope, evidence, decisions,
progress, and every surface that displays or changes them. An Inbox redesign
cannot settle that contract by itself.

The earlier framing "controller = operational truth" needs qualification. The
controller can own recorded workflow state; it cannot make an agent claim true
or establish current production health merely by accepting an event. Evidence
has a producer, candidate, scope, observation time, freshness, and possible
contradictions. Issue, Inbox, board, and chat must preserve those distinctions.

The user's rejection of repetitive avatars and uninformative rows is a clear
experience constraint. Their short examples of previews and comments are not a
complete interaction specification. Likewise, the assistant's proposed Inbox
sections, pinned summaries, and role split are hypotheses requiring evaluation.

## Evidence and limits

- Read all 12 files present under `docs/silver-mediums` before this report was
  created, including the complete run journal, research, instruction packets,
  acceptance matrix, state contract, and controller pilot report.
- Read project `AGENTS.md` and `CLAUDE.md`; consulted the knowledge hub's
  governance/current-state navigation, `MULTICA_AGENT_OPERATING_BLUEPRINT.md`,
  and `MULTICA_SYSTEM_ROLE_COVERAGE.md`. The hub documents are dated context,
  not a refreshed inventory of hosted agents or company runtime.
- Inspected current reducer, workflow event/answer handlers, Inbox grouping,
  decision component, and answer/archive mutation. These are source findings,
  not fresh runtime reproductions.
- Checkout HEAD: `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`; branch
  `codex/selfhost-pilot-20260909`. Existing dirty source and documentation were
  preserved. Only this report was written by this audit lane.
- No tests, browser actions, daemon operations, hosted reads/writes, deployment,
  SMS, or Phone Farm technical work were performed. Previous test counts and
  runtime passes below are explicitly attributed to the pilot records.

## What exists, and what the evidence actually proves

| Capability | Classification | Boundary |
|---|---|---|
| Isolated local stack, account, workspace, profile, daemon, persistence across one restart | Previously proven locally | Journal evidence; no fresh health check in this audit. Not backup/restore or always-on readiness. |
| Squad ownership; worker and different reviewer; direct successful handoff | Previously proven locally | SILV-6 and SILV-11 recorded three runs, one Mika run, no remaining tasks. Fixed harmless scenarios. |
| Failed review, one correction, re-review; stop at retry ceiling | Previously proven locally | SILV-9 and SILV-10; no representative software repair or long-running recovery acceptance. |
| Workflow plan before first leader run | Previously proven locally | SILV-11 for configured Pilot Operations. Unconfigured squads intentionally retain stock behavior. |
| One structured choice and resumption of the same worker role/stored stage | Previously proven locally | SILV-13, four runs. Does not prove exact execution checkpoint, multi-question review, rejection semantics, interrupted side effects, or candidate-bound release approval. |
| State reducer for deploy, verify, rollback, dependency resolution, interruption | Implemented but not fully proven | State names/transitions exist. Actual adapters, evidence validation, health restoration, authorization, and recovery need separate proof. |
| Inbox grouping and relevant-comment navigation | Implemented; experience incomplete | Source groups by issue and preserves a decision representative. Navigation hooks already exist; the owner's scrolling complaint is not proof that all navigation capability is absent. |
| Rich decision review: interactive preview, annotations, attachments, revisions, partial acceptance | Proposed only / absent from inspected decision component | General issue comments may exist; they are not demonstrated as a durable decision-response contract. |
| Unified current picture and meaningful-change stream across chat/Inbox/issue/board | Proposed; no end-to-end acceptance | Prior Current Picture/Delivery State concepts exist in the hub. Their local synchronization and correctness are unproven. |
| Candidate-bound durable authorization, autonomous release and live regression gates | Proposed; enforcement not established | Reducer transitions do not prove release permission or actual checks. |
| Mobile private HTTPS, alert delivery and reply continuation | Planned; no local pilot proof | Journal explicitly says no SMS configured/sent. Native mobile is a separate client. |
| Interruption, host loss, controller outage, restart reconciliation, migration/restore | Unproven | The local database restart test covers only a small part of this. |
| Token policy and telemetry | Research plus measured baseline; efficiency failed | SILV-13 recorded 123,767 fresh input + 529,408 cached input + 5,192 output tokens. One small sample is not a reliability/cost distribution. |

## Source-supported problems to account for before extending the pilot

1. **An answer is currently treated as continuation, not a typed decision.**
   `PostIssueWorkflowOwnerDecision` accepts any nonempty answer, records it, and
   invokes `OwnerDecisionRecorded`; the reducer does not inspect answer meaning.
   For a reviewed-result owner gate, that can advance to Done or deployment even
   when the answer is `Reject`. The fallback wording says rejection leaves work
   waiting, which is inconsistent with this path. This is a source finding;
   SILV-13 does not establish safe approval/rejection behavior.
2. **The current request is not unambiguously bound to a candidate or question
   revision.** The handler verifies issue membership and accountable user, and
   for an explicit question compares the source agent with the suspended agent.
   It does not compare a persisted current-question ID to the supplied request
   ID. An older question from the same worker needs a deliberate stale-answer
   test. No candidate identity is part of this answer contract.
3. **A decision can have no reachable answer in this UI.** Packet validation
   permits zero options; the component renders only supplied option buttons.
   The API accepts text, but the component has no text response field.
4. **Answering and archiving are separate client requests.** The answer can
   commit before Inbox archival fails. The owner can then see an apparently
   unanswered card for work already resumed. Reconnect, retry, response loss,
   and answering from another surface need one settled-result contract.
5. **Urgent notification and owner decision are conflated in the event handler.**
   `NotifyOwner` creates a `workflow_owner_decision` and defaults to
   Approve/Reject if no decision packet exists. The reducer also sets that flag
   for production failure/rollback. An incident update is not inherently an
   approval request, and a rollback state cannot use the Needs Owner answer
   handler. This unproven release path needs separate event semantics.
6. **Stored labels exceed demonstrated evidence enforcement.** Reducer inputs
   do not contain a reproducible candidate, deployment receipt, check results,
   or freshness. `production_verified` can reduce to Live based on role/state.
   The handler's board fallback can show rolled-back work as In Progress and
   rollback as In Review. The current code alone does not meet the stronger
   meanings documented in the state contract.
7. **"Exact resume" has a limited meaning today.** Owner resume records a stage
   and agent; interruption reduction instead selects the implementer and Queued,
   even if the interrupted stage was verification or rollback. The handler's
   `workflowInput` does not supply `CanAutoRecover`. Durable checkpoints,
   operation receipts, and interruption-specific resumption are not proved.
8. **Per-issue grouping is not a complete attention model.** The helper selects
   one representative notification, preferring a decision, then sorts groups
   by representative creation time. It does not globally prioritize decisions
   above all newer issues, aggregate meaningful changes, or resolve multiple
   simultaneous requests. A pinned decision must not conceal an urgent incident.

These findings justify focused design and later regression cases. They are not
permission to resume implementation during the planning pause.

## System contracts that need to be worked through together

| Area | Scenarios and decisions that remain |
|---|---|
| Intent and intake | Distinguish discussion, investigation, accepted repair, new feature, incident, maintenance, and recurring monitoring. Link an additional chat message to the correct issue; handle ambiguous references, duplicates, scope additions, cancellation, and independent newly discovered work. An audit milestone must not close an accepted repair outcome. |
| Work structure | One business outcome may need parallel repos, multiple candidates, dependencies, and independently accepted sub-outcomes. The fixed implementer/reviewer pair is a useful fixture, not an established company task topology. Decide how internal work is represented without making Drew manage extra issues. |
| Authority | Bind permission to original owner evidence, action scope, candidate/target, and conditions. Define revocation and changed-scope behavior. Preserve existing valid authority through restarts and handoffs. Reconcile urgent employee recovery with UI direction/candidate rules instead of repeatedly asking or silently bypassing them. |
| Owner interaction | Decisions include factual input, UI feedback, annotation, choosing among alternatives, requesting changes, partial approval, scope/budget choices, authentication steps, external action, disagreement with findings, postponement, and cancellation. Decide which responses advance, revise, investigate, or remain pending. A comment is not automatically approval. |
| Progress | Distinguish process heartbeat, tool activity, meaningful progress, expected wait, and loss of visibility. Show what changed, what is known, what is running next, and when another signal is expected. A live process can be stuck; an absent update can be a disconnected observer. Do not invent percent completion or reset progress clocks from chatter. |
| Evidence and review | Bind review to the exact output and acceptance version; invalidate it on relevant changes. Different agent identity does not alone prove independence, sufficient access, or correct evidence. Handle disagreement, partial coverage, failing baseline tests, unavailable authenticated UI, and inconclusive results. |
| Release and recovery | Include build inputs/assets/config/migrations, target selection, concurrent releases, canary, partial rollout, install population, irreversible side effects, forward recovery, rollback failure, and restored health. A rollback can restore service while leaving the requested outcome unfinished. Permanent regression checks need owners, relevance rules, and ongoing maintenance. |
| Shared state and synchronization | Define owners for intent, scope, task state, evidence, current summary, decision, and read/acknowledgment state. Specify races, event ordering, idempotency, failed dispatch after committed state, lost websocket events, stale caches, and reconciliation. A direct issue status edit must not silently contradict the controller. |
| Roles and hosts | Inventory responsibilities, context, access, runtime capacity, and qualifications before adopting an org chart. Include specialist absence, reassignment, archived roles, shared credentials, and same-repo conflicts. Preserve Phone Farm's dedicated Mac-agent boundary; Windows only routes decisions and returned evidence. |
| Communications and resilience | Separate alert detection, sending, delivery receipt, reading, acknowledgment, and actual resolution. Define independent outage detection when Multica itself is down, duplicates across channels, escalation when Drew is unavailable, stale links, and which phone actions are possible privately and reliably. |
| Token and capacity economics | Measure total cost per accepted outcome including failed attempts, retrieval, review, corrections, supervision, and idle waits. Govern context size, concurrency, escalation, quota exhaustion, model substitution, and bounded retries across the whole issue, not just one stage. Summary generation must not become another repeated full-history run. |
| Operating the platform | Name responsibility for Multica updates, fork divergence, database/backups/uploads, restore testing, credentials, host outages, telemetry, and configuration drift. A pilot on an intermittently available workstation is not yet company infrastructure. Migration requires history/attachments/identity mapping, decision continuity, cutover ownership, and rollback. |

The dated role-coverage map already records Photobot and company email as
inactive, and recovery as a responsibility rather than another product/agent.
Do not recreate those previously corrected role gaps from old repository lists.

## Inbox, issue, board, and chat: questions before layouts

Their division of labor should be tested against the same scenarios. A useful
candidate allocation is below; it is not an approved design.

| Surface | Job to evaluate | Failure to avoid |
|---|---|---|
| Inbox | Understand changes since last attention; handle decisions and exceptions with enough context | Becoming a second task database, hiding active work when quiet, or mixing unread with unresolved |
| Issue | Preserve original intent, current scope, current state, evidence, discussion, decisions, and outcome | Making every visit a scroll through raw chronology or hiding the original request under rewritten summaries |
| Board | See overall workload, priorities, dependencies, waiting work, and delivery health | Making a status column substitute for progress evidence, or making Drew route every task |
| Chat | Explore, give direction, ask across issues, and discuss ambiguity while preserving issue linkage | Answering from a separate narrative, losing commitments, or treating every sentence as new executable work |
| Phone notification | Bring a timely, consequential change to Drew and open its current context | Equating sent with delivered, or treating an unbound SMS reply as authority for a different request |

Critical questions: Is Inbox the default home or an attention queue beside a
work overview? Where can Drew see work that is progressing but has no new
notification? Does opening an item only mark it read? Can it be snoozed without
hiding an unresolved decision? What remains visible after an answer while the
worker is still queued? Can Drew continue discussion next to a preview and see
the resulting scope change immediately? Can a chat answer cite the exact
current issue snapshot and distinguish fresh evidence from a stale summary?

The existing Company Guide/Mika split in the hub also needs reconciliation with
the proposed single conversational surface. The user should not have to infer
which agent to ask to obtain a truthful update. Whether two agents remain
internally is separate from what interaction Drew sees.

Evaluate minimum, typical, and maximum credible data at narrow, medium, and
wide viewports: empty day, one long issue, many simultaneous projects, a burst
of incident updates, multiple decisions, long titles, attachments, and stale
offline state. The removal of repetitive avatars should be assessed with the
replacement identity cues: project, plain task name, consequence, and action.

## Contradictions and documentation drift

- `HANDOFF.md` names SILV-5 as the latest checkpoint; the journal/acceptance
  record reaches SILV-13. `INITIATIVE.md` still sends the next agent toward the
  earlier efficiency slice; the journal's latest next step is deployment.
  Neither records the user's newest planning pause at this audit's first read.
- The pilot squad and worker instruction files still describe returning to
  Mika for completion/review, while newer controller documents replace those
  relays. A future handoff needs a clear distinction between historical test
  fixtures, installed instructions, and current intended policy.
- "One request, exactly one issue" in the pilot is stronger than the hub's
  "one business problem, one issue," which permits independently accepted
  outcomes to split. Chat exploration and large multi-project initiatives need
  an explicit rule rather than either slogan applied literally.
- The state contract says gates are declared before work begins; real evidence
  can reveal a UI change, new dependency, or expanded release impact later.
  Define how a plan evolves and invalidates prior approvals without resetting
  work or losing the original authorized outcome.
- Retry exhaustion is labeled Blocked, but Blocked is defined as an external
  dependency. The next meaningful actor/action must remain visible even if a
  simplified owner label groups several internal reasons.
- The pilot's automatic urgent recovery requirement and the hub's exact
  visible-product preview/release approval rules need a precedence model for
  unseen urgent repairs, restored previously approved behavior, and new UI
  direction. Do not resolve that conflict by silently broadening authority.
- The hub mentions 19 agents at its dated snapshot; a pilot document mentions
  an 18-agent production workspace. Neither is a current inventory in this
  audit. Counts and suggested model charts cannot establish the final team.

## Recommended planning deliverables

These are reviewable planning outputs, not a commitment to implement a feature
list. Complete them before choosing the next functional slice:

1. A reconciled evidence/decision register: user requirements with provenance;
   existing approvals; current source/runtime proof; hypotheses; unresolved
   questions; and explicit superseded instructions. Keep one resume pointer.
2. A scenario catalog grounded in real company work, with normal, exceptional,
   concurrent, interrupted, and owner-unavailable paths. Name expected owner
   experience, responsible actor, durable records, evidence, and next action.
3. A contract map for scope changes, authority, events, decisions, artifacts,
   summaries, progress clocks, and reconciliation across all surfaces. Mark
   what is prevented by code, detected afterward, or merely requested in a
   prompt.
4. A walkthrough of the same few representative issues through chat, Inbox,
   issue, board, and phone. Use interactive product previews later where
   interaction matters, and record what alternatives/tradeoffs were considered.
5. A dependency-ordered validation plan. Start with semantics and failure
   recovery that later UI/release controls rely on; record cost and clarity
   alongside correctness. A single happy-path issue cannot validate the whole
   system, even though it remains a necessary final integration exercise.

The coordinator can derive most technical proposals from evidence. Drew's
attention should be reserved for concrete product tradeoffs and business
authority that remain unresolved after that work, presented with examples and
recommendations. Asking him to enumerate every possibility would repeat the
same planning failure in a different form.

## Minimum scenarios for the next planning review

- A plain problem report becomes an investigation and then an already accepted
  repair; additional chat direction changes scope without duplicate work.
- A UI candidate receives text and screenshot feedback; revision B invalidates
  only the relevant acceptance; Reject does not authorize release.
- Two decisions exist around the same outcome; an old mobile tab answers the
  wrong revision while the current worker is waiting for another question.
- An owner answer commits but its response or notification archival is lost;
  all surfaces recover without duplicate execution or another approval demand.
- A worker is alive but making no progress; a different worker is progressing
  while the observer is disconnected; the owner sees the difference.
- An urgent repair has existing valid authority; deployment succeeds but the
  employee journey fails; rollback restores service but the repair remains
  open, with one clear incident update and an automatic next action.
- A release or external side effect completes just before daemon/controller
  failure; recovery proves what happened before retrying it.
- Two issues modify the same shared component; one candidate changes after
  review; only the current approved artifact reaches the target.
- Quota/runtime capacity disappears during review; work keeps its evidence and
  place, and an approved fallback or truthful wait resumes automatically.
- Multica is unavailable while an independent monitor detects an outage;
  delivery and later issue reconciliation are deduplicated and observable.
- Drew returns after several days and can understand current company work,
  consequential changes, unanswered decisions, and verified outcomes without
  scrolling histories or asking chat to reconstruct everything.

## Audit closeout

This audit wrote one advisory report and changed no implementation. Confidence
is high in the named source-path limitations and document drift; moderate in
the completeness of the planning map because the full current production
inventory, representative task corpus, stakeholder needs, and live behavior
were intentionally not inspected. Previous pilot successes remain valid only
within their recorded scenarios. The next coordinator action is to reconcile
this report with the maintained planning record and the owner's latest intent.
