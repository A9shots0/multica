## Workflow controller bootstrap and recovery

Configured squad workflows use `issue_workflow_plan` as their durable
controller record. Every squad leader enqueue path passes through the same
pre-dispatch guard. The guard installs the plan with `ON CONFLICT DO NOTHING`
before the task row is visible, so a retry or daemon restart cannot reset the
plan or create a second controller.

The server also runs `WorkflowPlanReconciler` immediately at startup and every
30 seconds. It scans every non-terminal squad issue when any controller health
condition needs attention:

- the issue has no durable plan;
- its squad template is missing, including when an older plan still exists;
- a controller-recovery Inbox item remains active; or
- the plan is in `working`, `queued`, or `correction` with no active carrier
  task.

A missing plan is installed only from a valid current template. An actionable
plan with no carrier resumes through the squad leader only when no matching
pending leader task exists. Existing plans, revisions, correction counts, and
workflow state are preserved.

When installation or resume fails, the issue is projected to `blocked` and a
`workflow_controller_unavailable` Inbox item is written for a workspace
manager. The item includes the recovery action (`retry_controller_recovery`),
the prior board status, and the error evidence. Its issue-scoped lookup makes
repeated passes idempotent. Once a plan is present, the notice is archived and
the prior status is restored only when the issue is still blocked; an issue that
was already blocked before recovery remains blocked.

If an active workflow task exits without an accepted structured transition,
the controller gives the squad leader one deduplicated recovery run. If that
recovery run also exits without advancing the plan, or the bounded provider-run
ceiling is exhausted, Multica stops the workflow durably: plan and issue become
`blocked`, Current Picture reports `technical_stop`, and Inbox receives one
owner-visible `workflow_incident`. It never creates an unbounded recovery loop.

Direct, non-workflow tasks use the same truthful board rule. A retryable failure
keeps the issue active only while its bounded retry child exists. Once a
non-retryable or exhausted failure leaves no queued, dispatched, running, or
local-directory-waiting task, task terminalization and the issue transition
commit atomically: the issue enters `blocked` with a
`terminal_task_failure_wait` marker naming the failed task. It never returns to
unowned `todo`. Task admission serializes on the same issue-row lock and clears
that marker when a new carrier commits. The runtime sweeper also scans terminal
failures to repair any pre-upgrade or interrupted reconciliation. Review, other
blocked, terminal, and custom statuses outside the in-progress category are
left unchanged.

The healthy automatic state is the combination of a valid squad template, a
durable `issue_workflow_plan`, and a queued, dispatched, running, or
local-directory-waiting task for an actionable plan state. A blocked issue plus
an active controller-recovery or workflow-incident Inbox record is the explicit
unhealthy state. Board uses durable blocked status as a `technical_stop`
fallback even if Current Picture is temporarily absent; Inbox carries the
evidence and required action; linked Chat sessions navigate to the same issue.

The SILV-48 staged-work shape is covered by the plan-install, plan-preservation,
stranded-carrier requeue, structured-event replay, and bounded recovery tests in
`server/internal/handler/workflow_control_test.go` and the provider-start ceiling
tests in `server/internal/handler/workflow_provider_boundary_regression_test.go`.
The SILV-53 visibility shape is covered by the owner Inbox tests in
`server/internal/handler/inbox_test.go` and `packages/views/inbox`, plus the
durable-status Board fallback tests in
`packages/views/issues/components/operational-workflow-lane.test.ts`.

Owners do not need Docker or daemon commands: server startup runs the
reconciler, subsequent passes retry recovery, and Inbox identifies the platform
state when a host, database, runtime, or workflow-output problem prevents
automatic continuation.

No additional host or runtime configuration is required. The reconciler uses
the server's existing database, event bus, and task service.
