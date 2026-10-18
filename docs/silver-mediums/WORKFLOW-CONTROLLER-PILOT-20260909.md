# Deterministic Workflow Controller Pilot - 2026-09-09

## Purpose

Prove on the local self-hosted Multica pilot that a squad-owned issue can move
from implementation to independent review without Mika reading a worker comment
and manually relaying the next assignment.

This is local evidence only. It does not prove hosted Multica, SMS, owner
decision delivery, release/rollback automation, or the final board design.

## Installed local components

- Backend image: `multica-backend:dev`, rebuilt from the pilot checkout.
- Database migrations: `457_issue_workflow_control` through
  `464_issue_workflow_correction_control`.
- Windows pilot CLI/daemon:
  `C:\Users\a9sho\.multica\bin\multica.exe`.
- Installed CLI SHA-256:
  `36A11B5B3AD1E88C6C4AEF8E885F99B53F06BAD090D02711D55F349F95E8FD69`.
- The hosted desktop daemon was not changed.

## Test issue and declared route

- Issue: `SILV-6` / `01a08889-b52a-73aa-9528-fb4c79003b47`
- Squad: `Pilot Operations`
- Intake/leader: `Mika`
- Implementer: `Pilot Worker`
- Independent reviewer: `Pilot Reviewer`
- Owner gate: no
- Deployment gate: no

## First run - useful failure

The backend had been rebuilt, but the Windows pilot daemon still used the old
CLI. `workflow-event --reason` was unavailable, so no structured event was
recorded. The legacy comment fallback therefore woke Mika and reproduced the
old relay loop. The run was not counted as a pass.

The old pilot CLI was backed up as
`C:\Users\a9sho\.multica\bin\multica.exe.pre-workflow-controller`, replaced
with the matching local build, and exactly one hidden pilot daemon was started.

## Verified rerun

Snapshot boundary: 2026-09-09 23:47:30-23:49:04 UTC, local PostgreSQL and
local backend runtime.

| Order | Task | Trigger | Result |
|---|---|---|---|
| 1 | Mika `01a08891-c687-7941-889b-586fb47fbca3` | Human test comment | Delegated once to Pilot Worker. |
| 2 | Pilot Worker `01a08892-2820-706c-951e-b0681b13263f` | Mika comment | Recorded `implementation_ready`. |
| 3 | Pilot Reviewer `01a08892-70ea-7dd8-9f13-083b63c582a0` | Workflow event `01a08892-70e8-7447-806b-8c9d7a3937ac` | Recorded `review_passed`. |

Measured results:

- Tasks in the rerun: 3.
- Mika tasks in the rerun: 1.
- Mika relay tasks after worker/reviewer results: 0.
- Reviewer was queued directly from the worker's structured event: yes.
- Worker and reviewer both retained the exact squad ID: yes.
- Workflow events recorded: 2 (`implementation_ready`, `review_passed`).
- Board projection: `in_progress` -> `in_review` -> `done`.
- Active tasks after completion: 0.

## Automated checks

- `go test ./internal/workflowstate -count=1` - passed.
- `go test ./internal/handler -count=1` - passed.
- Focused CLI/server compilation tests - passed before the local rebuild.
- A fresh temporary database applied the complete migration history through
  migration 463, produced both workflow tables and all four required unique
  indexes, and was removed after the check.
- Persistent local startup applied migrations through 463 and `/health`
  returned commit `aadc64129`.
- A broad `go test ./cmd/multica ./cmd/server -count=1` was stopped after the
  CLI package ran for several minutes; this is not recorded as a pass or fail.

## Proven and not yet proven

Proven in the local pilot:

- exact worker-to-reviewer routing without a Mika relay;
- independent reviewer identity enforcement;
- role-specific, short agent instructions;
- idempotent workflow-event persistence/task creation in handler tests;
- truthful `in_review` and `done` projection for the tested path.

Not yet proven:

- review failure -> correction -> re-review in the live runtime;
- owner decision notification and exact-step resume;
- dependency blocker escalation and automatic resume;
- deployment -> independent production verification -> live;
- failed production verification -> urgent alert -> rollback -> restored health;
- interruption recovery in the live daemon;
- SMS delivery;
- agent/model redesign across the full 18-agent production workspace;
- a final owner-facing board and inbox design.

The next implementation slice should close one of these unproven paths with a
bounded scenario rather than expanding to all production issues.

## Failed-review correction slice

Installed backend commit: `ecd79eb56` (includes correction control commit
`5d9ca0249`). Persistent local migrations now run through
`464_issue_workflow_correction_control`.

The controller now requires a failed reviewer to return a compact correction
packet containing failed criteria, evidence, and the required correction. It
permits one automatic correction attempt. A second review failure stops at
Blocked and wakes the squad leader instead of starting another worker/reviewer
loop.

### Failures caught before acceptance

- `SILV-7` showed that creating the workflow plan still depends on intake
  configuration. Mika delegated before a plan existed, so the structured event
  was rejected. This run is not a pass.
- `SILV-8` showed an ordering race: a worker's readable comment could wake Mika
  just before the structured event queued the reviewer. That produced duplicate
  reviewers and Mika relays. The run was contained and is not a pass.
- Commit `ecd79eb56` makes workflow-role comments display-only. At task
  completion, a valid dispatched event suppresses recovery; a missing or
  undispatched event wakes the squad leader exactly once. Focused database tests
  cover both sides of this boundary.

### Verified correction-and-pass run

Snapshot boundary: 2026-09-10 01:26:26-01:29:06 UTC, local PostgreSQL and local
backend runtime only.

Issue `SILV-9` completed this exact chain:

1. Mika intake once.
2. Pilot Worker produced `DRAFT`.
3. Pilot Reviewer recorded structured `review_failed`.
4. The controller sent one bounded correction directly to Pilot Worker.
5. Pilot Worker produced `APPROVED`; Pilot Reviewer passed it; the controller
   moved the issue to Done.

Measured result: five total runs, one Mika run, two worker runs, two reviewer
runs, four workflow events, zero duplicate relays, and zero active tasks after
completion. Reported usage was 130,307 fresh input tokens, 6,821 output tokens,
and 1,017,344 cached input tokens. Routing correctness passed; token efficiency
did not.

### Verified retry-ceiling run

Snapshot boundary: 2026-09-10 01:30:46-01:34:06 UTC, local PostgreSQL and local
backend runtime only.

Issue `SILV-10` intentionally failed both reviews. The first failure created one
correction task. The second failure moved the workflow to Blocked with reason
`correction_limit_reached` and woke Mika once as squad leader. Mika reported the
stopped state without creating more work. The issue had six total runs and zero
active tasks afterward. Reported usage was 184,935 fresh input tokens, 9,109
output tokens, and 1,294,080 cached input tokens.

## Current boundary

Now proven locally:

- successful worker-to-reviewer handoff without Mika relay;
- one bounded failed-review correction and independent re-review;
- deterministic stop after the correction ceiling;
- comment/event ordering cannot create duplicate relays in the tested path;
- completion-time recovery exists when a workflow task fails to dispatch a
  structured transition.

Still unproven or unfinished:

- Needs You and exact-step owner resume;
- deployment, production verification, rollback, interruption recovery, and
  SMS;
- acceptable token cost;
- final board and inbox design;
- any hosted or production behavior.

The next controller slice is intake enforcement: every squad-owned executable
issue must receive its plan before the first delegation can run. Only after that
should the pilot move to Needs You/resume.

## Automatic squad workflow-plan slice

Snapshot boundary: 2026-09-10 02:05:37-02:08:05 UTC, loopback local Multica
only.

- Added an explicit reusable workflow template owned by a squad. It names the
  implementer, independent reviewer, optional release/verifier roles, owner and
  deployment gates, and a bounded correction count.
- A configured template is copied to a newly assigned issue before the first
  squad-leader task is created. Existing issue progress is never reset.
- Missing, archived, or removed configured role members fail closed before a
  leader run starts. Squads without a template retain stock Multica behavior.
- Added member-managed API and CLI configuration plus focused migration,
  handler, routing, and CLI tests.

`SILV-11` was created already assigned to Pilot Operations. No issue-level
workflow-plan command was run. Its plan was created at
`02:05:59.849250 UTC`, before Mika's first task at
`02:05:59.856813 UTC`, and the issue then completed:

`Mika once -> Pilot Worker -> Pilot Reviewer -> Done`

There were exactly three completed runs, two structured events, and zero
unfinished tasks. Usage was 77,898 fresh input, 4,177 output, and 568,576
cached input tokens. Automatic routing passed; cost remains unacceptable.

The next controller slice is Needs You and exact-step resume. Deployment and
regression-suite enforcement remain later, separate proof paths.
