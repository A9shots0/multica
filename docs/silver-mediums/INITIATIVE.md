# Silver Mediums Multica self-hosting pilot

Status: active
Last updated: 2026-09-13 America/New_York
Coordinator: current Codex task
Resume here: `CURRENT-CONTINUATION.md`

> **Current-status notice — 2026-09-13:** The business outcome and durable
> owner requirements in this file remain authoritative. The older `Current
> phase` and `Next action` prose below preserves implementation chronology and
> is superseded for continuation by `CURRENT-CONTINUATION.md` and
> `ITEM-14-ROLE-BUILD-PROGRESS.md`. Items 1-12 are accepted locally. Item 13's
> local backup/restore path is verified, while OneDrive provider readback and
> separate phone key custody remain unproved. Item 14 role and squad
> qualification is active; zero production configurations are qualified. The
> current pipeline uses Google Drive only. Telegram/Photobot is retired history,
> not a current role, requirement, test, or system edge.

## Business outcome

Create an isolated, self-hosted Multica pilot that Drew can evaluate without
changing, migrating, or interrupting the existing hosted Multica workspace.
The pilot must prove that Silver Mediums can control the workflow behavior that
is currently failing: truthful issue state, dependable coordination, internal
review, owner alerts, concise owner communication, and token-efficient agent
routing.

This is not a second owner-facing board. Multica remains the board. The pilot is
a replacement candidate for the hosted Multica installation only if it earns
trust through the acceptance tests below.

## Current phase

Phase 1 infrastructure and Phase 2 isolated runtime are running locally on the
Alienware using the official self-hosted release. The local pilot has narrowly
proved automatic squad plan installation, direct worker-to-reviewer routing,
one bounded correction cycle, a retry ceiling, and one structured owner
decision/resume path through `SILV-13`. Those proofs remain local and their
token cost is unacceptable.

The operating requirements, owner walkthrough, notification policy, scenario
catalog, and implementation plan are approved. Local implementation is active.
The baseline, workflow contract v2, normalized ledgers and Current Picture,
versioned Current Picture API, authoritative causal commands, compatibility
guards, durable correction execution/recovery, typed correction results,
immutable replacement candidates, deterministic independent rereview, and
truthful non-deployment Done without Mika, and typed owner authorization with
truthful owner-attention/resume/replan behavior are accepted locally after
coordinator gates and independent review. The current implementation slice is
general review-to-release-to-production-verification-to-regression-to-rollback
authority. Item 8A release-attempt versus observed-target truth and fail-closed
Current Picture v3 release status, item 8B exact authoritative admission, and
item 8C fake-provider deploy/expose plus independently authenticated runtime and
business verification, and item 8D regression containment, new-generation
rollback, and independently authenticated restoration are accepted locally.
Real provider adapters remain deferred. Item 9's server-owned typed owner
summary, strict Go/Core Current Picture v3 alignment, batch/realtime read path,
and browser-verified Board/Issue surfaces are accepted locally. Item 10's
owner-relevant Inbox projection, typed/versioned directions, item-level archive
and unread semantics, bounded evidence controls, and deterministically separated
Chat intake are also accepted locally after one independent review and one
consolidated correction pass. Items 11 and 12 are accepted locally. Item 13's
local recovery path is verified, while its two external custody gates remain
open. Item 14 role and squad qualification is active after benchmark discovery
closed. Permanent production agents and squads remain unselected. Do not
migrate production workspace data or reconnect/reconfigure the existing cloud
daemon.

A read-only hosted-workspace census now grounds that design in the actual
failure loops rather than screenshots alone. See
`HOSTED-WORKFLOW-INTERACTION-AUDIT-20260910.md`: 33 issues, 2,827 runs, 515
messages across ten active chats, six progress-labelled cards without active
execution, stale owner decisions, historical closeouts moved back to In Review,
and repeated prompt/correction/re-authorization loops. These are requirements
inputs, not permission to repair the hosted workspace during this planning
phase.

## Source and environment

- Pilot checkout: `C:\Dev\SMK-multica-selfhost-pilot`
- Branch: `codex/selfhost-pilot-20260909`
- Official upstream: `https://github.com/multica-ai/multica.git`
- Checkout source observed 2026-09-09: `9fab6da917953c6f1a967a17beef045aff3d0ce5`
- Pilot image version: `v0.4.42` (published tag; must be set explicitly)
- Host: Drew's Alienware, Windows 11 Pro, i7-13700HX, 32 GB RAM
- Docker: Desktop 4.68.0 and engine 29.3.1 are running
- Existing hosted Multica: preserve unchanged
- Existing hosted CLI/daemon configuration: preserve unchanged until an
  isolated connection method is proven
- Existing control-plane checkout: `C:\Dev\SMK-multica-control-plane`, occupied
  and dirty from the paused Astra task; do not edit or reuse it for this pilot
- Older `C:\Dev\Multica-Pilot-Sandbox`: dirty owner work; preserve untouched

## Non-negotiable owner requirements

1. Mika coordinates. Drew describes the business problem; Mika determines the
   technical route from evidence and does not simply agree or invent facts.
2. Work belongs to squads. Individual agents may perform bounded work inside a
   squad, but no issue is left owned by one unsupported agent.
3. Audit comes before a fix when the cause or scope is unknown. Findings must be
   explained to Drew before a materially different repair is authorized.
4. Review means independent internal review. Drew is not the code reviewer.
   Owner approval is a separate state used only for a real business decision.
5. An urgent employee-blocking correction proceeds through build, independent
   review, release, and production verification without waiting for Drew to say
   "deploy" again, provided it stays within the already-approved recovery scope.
6. Statuses must describe reality. `Queued` requires a real resource or timer
   and automatic continuation. A stale or failed run cannot masquerade as
   progress.
7. Mika must report conclusions once, after checking. Do not send a preliminary
   sentence followed by a second repetitive answer after tool use.
8. Drew must receive a meaningful alert for urgent regressions, rollbacks,
   failed releases, employee-blocking incidents, and decisions that truly need
   him. Routine agent chatter and assignments are not owner alerts.
9. The inbox must summarize the actual consequence and next action; it must not
   be a feed of raw assignment/run events.
10. Token use must be governed by task complexity, bounded retries, compact
    evidence, and explicit model selection. No endless status-check loops or
    repeated full-history reads.
11. Claims remain separated as local, deployed, runtime-verified, and
    business-verified. Unknown is acceptable; confident guessing is not.
12. Existing cloud work, dirty repositories, credentials, active services, and
    Phone Farm's dedicated Mac-only boundary must be preserved.

## Phases and gates

### Phase 1 - Local infrastructure

- [x] Create a clean checkout separate from all occupied work.
- [x] Record official source revision and published release tag.
- [x] Record the initiative, constraints, and evidence boundaries.
- [x] Create a secret-bearing local `.env` without printing or committing it.
- [x] Start PostgreSQL, backend, and frontend with Docker.
- [x] Verify container health, backend health, migrations, frontend response,
      and persistent volumes.
- [x] Prove restarting the stack preserves the pilot database.

### Phase 2 - Isolated login and runtime

- [x] Create the first local account during a bounded signup window, then lock
      public signup and verify it is disabled.
- [x] Create a disposable Silver Mediums pilot workspace.
- [x] Connect an isolated test daemon without overwriting the current cloud
      daemon/configuration.
- [x] Prove one test agent can receive and finish a harmless squad-owned issue.

### Phase 3 - Workflow behavior

- [ ] Encode Mika's evidence and response contract.
- [x] Encode squad ownership and independent review stages.
- [ ] Define truthful owner-facing states and transitions.
- [x] Define urgency/release behavior and rollback notification rules as local
      policy; provider implementation and delivery proof remain Phase 4 work.
- [ ] Add token/model budgets and bounded retry/heartbeat behavior.
- [x] Distill the supplied model research into a testable routing and telemetry
      policy without treating unverified prices or benchmarks as facts.
- [ ] Prove one complete harmless pilot issue from chat intake through verified
      result without manual issue polling.

### Phase 4 - Mobile and notifications

- [x] Add private HTTPS access for mobile web through private Tailscale Serve.
- [x] Verify the phone-sized web workflow in the built-in Browser compact layout.
- [x] Implement deterministic alert policy with dry-run and separately gated
      live-test/live delivery modes; prove one bounded provider delivery.
- [ ] Prove urgent failure, rollback, owner-decision, and completion alerts.
- [ ] Decide later whether a custom-signed native iOS build adds enough value.

### Phase 5 - Hosting and migration decision

- [ ] Compare the validated pilot against hosted Multica using the same test
      scenario and measured token use.
- [ ] Select the permanent always-on host only after the pilot passes.
- [ ] Design a staged migration because hosted Multica has no established
      complete export/import path.
- [ ] Move one noncritical cohort first; retain rollback and cloud archive.

## Completion definition

The initiative is not complete because containers run. It is complete only when
Drew can report a problem once, see understandable progress, receive only
meaningful alerts, answer a true decision from phone, and see squad-owned work
advance through independent review and verified outcome without chasing issue
comments or repeatedly asking Mika for status.

## Current risks and unknowns

- Accepted workflow foundations and the correction-to-rereview-to-non-live-Done
  path are local-only. They have not proved deployment, production verification,
  rollback, owner-device behavior, notifications, or a real business outcome.
- `issue_workflow_plan` is temporary compatibility state, so an explicit
  deployment requirement used for completion must be frozen into normalized
  authoritative command/state before `CompleteNonLive`; a mutable legacy row or
  its default cannot be final authority.
- The local containers and disposable PostgreSQL test database are development
  evidence only. One labeled SMS proved Telnyx acceptance and delivery for the
  live-test lane; public ingress, real decision/resume, real business outcome,
  outage monitoring, deployed revision, recovery time, and business acceptance
  remain unproved.
- The local instance does not have an email provider configured. Login codes
  are therefore generated but not delivered by email; the first login was
  completed using only the newest code from the local backend log. Do not
  expose full logs or reuse production email credentials for the pilot.
- Stock Mika initially created and completed `SILV-1` entirely from its chat
  run, leaving the issue unassigned with zero issue runs. This does not satisfy
  the ownership workflow even though the requested comment and Done status
  were produced.
- After adding the `Pilot Operations` squad and pilot instructions, `SILV-2`
  retained squad ownership, started a real issue run, produced the exact
  required comment, and completed successfully. The trivial request still used
  two agent runs and took about 90 seconds end to end; efficient intake remains
  an open Phase 3 requirement.
- `SILV-3` proved the full internal path without owner intervention: the squad
  retained ownership, Mika moved the issue to Working, Pilot Worker returned
  its result, Mika moved it to In Review, Pilot Reviewer independently passed
  it, and Mika marked it Done. The issue reached Done in 2 minutes 59 seconds.
- That correct path required five complete agent runs for a trivial arithmetic
  check: three Mika runs, one worker run, and one reviewer run. Together they
  consumed 65,611 new input tokens, 5,544 output tokens, and 864,000 cached
  input tokens. Correctness passed; efficiency failed.
- `SILV-4` exposed a direct instruction conflict after the model mix changed:
  all technical work and review passed, but Luna obeyed stock Multica's rule to
  leave Done to a human while the pilot squad rule required Mika to finish.
  `SILV-3` had passed only because Sol ignored the conflicting built-in rule.
- The local backend now distinguishes technical completion from owner
  acceptance. A different squad member's independent review can authorize Done
  for routine technical work; a real business/owner acceptance decision remains
  In Review. Focused contract tests passed against an isolated test database.
- `SILV-5` runtime-verified the corrected rule with Mika=Luna/low, Worker=Luna/low,
  and Reviewer=Terra/medium. It reached Done automatically in 2 minutes 3
  seconds. Usage was 79,277 fresh input, 4,131 output, and 362,496 cached input
  tokens. That is about 52% fewer reported tokens than SILV-3, but still five
  complete runs; repeatability and deterministic handoff reduction remain open.
- Native mobile self-host selection is not currently available in the stock
  app; Phase 4 starts with mobile web.
- No workspace migration has been attempted or authorized.
- One explicitly authorized, clearly labeled live-test SMS was provider-
  confirmed delivered. Unrestricted live mode remains disabled and the pilot
  is back in dry-run.
- The model-routing research now favors per-stage capability selection,
  deterministic workflow transitions, compact escalation packets, and measured
  cost per successful task. The inspected stock execution path selects one
  model per assigned agent and exposes no task-stage override; no broad
  agent-model reassignment has been made.
- Later outside feedback on the proposed organization has been recorded as
  research, not policy. It supports static role tiers, deterministic release
  machinery, compact review packets, and a responsibility-first agent audit.
  Its suggested agent chart and model assignments remain unapproved hypotheses.

## Next action

Items 1–12 are accepted locally. Inbox is the owner control center for
existing work; Chat is limited to intake, brainstorming, cross-issue questions,
and explanation. Concrete chat requests bind one existing issue or create one,
then operational interaction moves to Inbox. Phone-sized web and mobile code
now preserve those same meanings. Item 12 adds a deterministic, durable,
privacy-preserving SMS interruption path and secure Inbox receipt. One bounded
live-test was provider-confirmed delivered, and the pilot was returned to
dry-run. The final Item 12 review found three P1s and one important P2; one
consolidated correction pass closed them without a reviewer loop.

Item 13 now has an encrypted atomic local backup, a real isolated restore,
external health receipts, hidden scheduled tasks, and an owner-only Inbox
recovery projection. The latest corrected drill matched the exact backup run,
migration identity, representative database counts, all upload checksums, and
137 restored user tables, then proved zero disposable resources remained. Its
single final review found five P1 gaps; one consolidated pass closed them
without a reviewer loop. The built-in Browser visibly proved the truthful
`Secondary copy needed` state and its exact next action. The owner selected the
separate Samsung A: SSD, encrypted personal OneDrive storage, and the phone
password manager for independent key custody. The newest scheduled artifact
and receipt are hash-matched on A: and in the local OneDrive root, and that
exact run restored successfully in isolation. Item 13 is not yet accepted:
OneDrive still reports synchronization pending, so provider readback is not
proved, and the key has not yet been confirmed on the phone. Item 14 now has a
machine-readable company graph, accountable domain outcome roles separated
from repository executors, controller-owned bounded squads, typed authority
and review receipts, and a locked 19-case qualification suite. The live local
backend exposes the graph, bounded queries, role architecture, and 31 explicit
configuration decisions. All configurations remain provisional, pending
qualification, or Mac-owned external holds; none is `qualified_bounded` and
none has production authority. The SILV-32 final review rejection and its
consolidated corrections are documented in
`ITEM-14-SILV32-CORRECTION.md`. The terminal independent verdict is
`APPROVE ITEM 14 LOCAL FOUNDATION`, recorded in
`ITEM-14-FINAL-REVIEW-RESULT.md`. No live-roster change or production authority
is implied.
