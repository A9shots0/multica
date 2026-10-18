package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	// workflowDriverInboxType is the owner-visible surface for work the driver
	// could not resume on its own. It is distinct from
	// workflowControllerInboxType because that one means "the controller could
	// not be installed"; this one means "the controller is installed, the stage
	// is known, and resuming it kept failing".
	workflowDriverInboxType = "workflow_stage_needs_owner"

	// workflowNeedsPlanInboxType surfaces an issue the driver could not install
	// ANY controller for: no assignee, and no squad in the workspace declares a
	// workflow template either. Without this the issue lands in `needs_plan`
	// every sweep forever (driveIssue's EnsureIssueWorkflowPlan branch) with
	// zero trace anywhere the owner would see it — confirmed empirically across
	// three consecutive real driver sweeps before this fix, all producing an
	// identical needs_plan outcome and zero inbox_item rows.
	workflowNeedsPlanInboxType = "workflow_needs_plan"

	// workflowAutoAssignedInboxType marks the one-time notice raised when the
	// driver hands a previously-unassigned issue to a squad template's default
	// implementer. ProposeIssueWorkflowPlan (workflow_plan_ensure.go:103-107)
	// falls back to that agent whenever the issue itself named nobody, and
	// until this notice existed that hand-off was completely silent — the
	// owner had no way to know an issue got auto-assigned, to whom, or why.
	// One row per occurrence: once the issue has a real assignee, the
	// had-no-assignee condition that triggers this cannot recur for it.
	workflowAutoAssignedInboxType = "workflow_auto_assigned"

	defaultWorkflowDriverInterval = 2 * time.Minute
	// defaultWorkflowDriverMaxAttempts bounds automatic resumption of ONE
	// stage. Two resumes cover the common causes (runtime restart, context
	// exhaustion, a transient provider error); a third repeat is evidence the
	// stage itself cannot complete, which is an owner question, not a retry.
	defaultWorkflowDriverMaxAttempts = 2
	// defaultWorkflowDriverQuietPeriod leaves room for an agent to report its
	// own hand-off after a task ends. Driving sooner would race the very
	// mechanism the driver is meant to back up.
	defaultWorkflowDriverQuietPeriod = 3 * time.Minute
	// defaultWorkflowDriverCapacityBackoff is deliberately long. A provider
	// usage limit resets on the provider's schedule, not ours, and a tight
	// retry loop against an exhausted account only manufactures failure rows.
	defaultWorkflowDriverCapacityBackoff = 30 * time.Minute
	// defaultWorkflowDriverIssueTokenBudget counts only BILLABLE tokens —
	// fresh input, output and cache writes — because cache reads are an order
	// of magnitude cheaper and 92% of observed volume, so counting them would
	// measure the wrong thing entirely.
	//
	// 5M is roughly forty ordinary runs at the observed ~130k median. It is
	// generous for real work and still catches a runaway: one SILV-41 run on
	// 2026-09-19 burned 29.7M billable tokens in 25 minutes, 290x that issue's
	// own median and 39% of all billable spend recorded. Nothing stopped it,
	// because ordinary runs have no token cap and no timeout.
	defaultWorkflowDriverIssueTokenBudget int64 = 5_000_000
)

// WorkflowDriver makes stage hand-off the server's responsibility instead of
// the agent's.
//
// The deterministic workflow only ever advanced when a running agent
// remembered to POST a workflow event naming its successor. Across 469 task
// runs in the Silver Mediums pilot that happened 5 times, so an issue stopped
// permanently the moment an agent crashed, ran out of context, or simply
// exited without reporting — and nothing re-examined it afterwards.
// recoverUndispatchedWorkflowEvents only repairs the case where the event WAS
// recorded and its enqueue failed; it cannot see a stage that reported nothing.
//
// The driver closes that gap. For every issue whose plan has no live task and
// no recorded next step, it re-dispatches the agent the plan already names as
// owner of the CURRENT stage.
//
// It never invents progress. An unreported stage is resumed, never advanced:
// a crashed implementer is asked to finish implementing, not promoted into
// review. The only durable state the driver writes is a task (through the
// same enqueue path a hand-off uses) and, after MaxAttempts, one owner Inbox
// item. Deciding what a stage MEANS stays with workflowstate.Decide and the
// agents; the driver only decides that somebody must be working.
type WorkflowDriver struct {
	Pool        *pgxpool.Pool
	Queries     *db.Queries
	TaskService *TaskService
	Bus         *events.Bus
	Interval    time.Duration
	MaxAttempts int
	Now         func() time.Time
	// OnlyIssues, when non-empty, restricts the driver to these issue numbers.
	// Staging matters here: switching the driver on for a whole board wakes
	// every stalled issue at once, and in a workspace where 38% of runs fail
	// that turns one bad routing rule into dozens of wasted runs. Proving the
	// loop on a single real issue first is cheap; discovering it on seventeen
	// is not.
	OnlyIssues []int32
	// QuietPeriod is how long an issue must have had no durable change before
	// the driver touches it, so a task that has just finished still gets to
	// write its own hand-off before the server steps in.
	QuietPeriod time.Duration
	// CapacityBackoff is how long to leave a stage alone after its last run
	// died on provider quota or rate limiting.
	CapacityBackoff time.Duration
	// IssueTokenBudget caps the billable tokens one issue may consume before
	// the driver stops feeding it and asks the owner. Zero uses the default;
	// negative disables the cap.
	IssueTokenBudget int64
	// DryRun reports what the driver would do without installing a plan,
	// enqueuing a task, or writing an Inbox item. It exists so a change to the
	// routing rules can be reviewed against real board state before any agent
	// is woken.
	DryRun bool
}

// DriveAction records one decision, for the run log and for tests.
type DriveAction struct {
	IssueNumber int32  `json:"issue_number"`
	IssueID     string `json:"issue_id"`
	Stage       string `json:"stage"`
	Action      string `json:"action"`
	AgentID     string `json:"agent_id,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	Attempts    int    `json:"attempts,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

// DriveReport is one full pass over the stalled set.
type DriveReport struct {
	Scanned   int `json:"scanned"`
	Resumed   int `json:"resumed"`
	Advanced  int `json:"advanced"`
	Planned   int `json:"planned"`
	Escalated int `json:"escalated"`
	// Triaged counts stoppages handed to a squad leader to decide, rather than
	// interrupting the owner with an engineering judgement.
	Triaged int `json:"triaged"`
	// OwnerRaised counts issues newly surfaced to the owner because they are
	// parked on a decision only a person can make.
	OwnerRaised int           `json:"owner_raised"`
	Skipped     int           `json:"skipped"`
	Actions     []DriveAction `json:"actions"`
}

func (d *WorkflowDriver) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

// covers reports whether the driver is allowed to act on an issue.
func (d *WorkflowDriver) covers(issueNumber int32) bool {
	if len(d.OnlyIssues) == 0 {
		return true
	}
	for _, allowed := range d.OnlyIssues {
		if allowed == issueNumber {
			return true
		}
	}
	return false
}

func (d *WorkflowDriver) maxAttempts() int {
	if d.MaxAttempts > 0 {
		return d.MaxAttempts
	}
	return defaultWorkflowDriverMaxAttempts
}

// stageOwner maps a plan state to the agent that must be running for the
// workflow to be making progress.
//
// Only states where an AGENT owns the next move return an id. needs_owner and
// blocked are waiting on a person; done/live/rolled_back/failed are terminal.
// Resuming those would either bypass the owner or restart finished work.
func stageOwner(plan db.IssueWorkflowPlan) (pgtype.UUID, bool) {
	switch workflowstate.State(plan.CurrentState) {
	case workflowstate.Received, workflowstate.Queued, workflowstate.Working, workflowstate.Correction:
		return plan.ImplementerID, true
	case workflowstate.TechnicalReview:
		return plan.ReviewerID, true
	case workflowstate.Deploying, workflowstate.RollingBack:
		if plan.ReleaseAgentID.Valid {
			return plan.ReleaseAgentID, true
		}
		return plan.ImplementerID, true
	case workflowstate.ProductionVerifying:
		if plan.VerifierID.Valid {
			return plan.VerifierID, true
		}
		return plan.ReviewerID, true
	default:
		return pgtype.UUID{}, false
	}
}

// attemptsForStage counts tasks created on this issue since the plan last
// changed state.
//
// This is deliberately derived rather than stored: a stage that genuinely
// advances bumps plan.updated_at, which resets the count for free. A stored
// counter would need its own reset rule on every transition and would drift
// the first time one was missed — which is the exact class of bug the driver
// exists to fix.
func (d *WorkflowDriver) attemptsForStage(ctx context.Context, issueID pgtype.UUID, since pgtype.Timestamptz) (int, error) {
	var count int
	err := d.Pool.QueryRow(ctx,
		`SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND created_at >= $2`,
		issueID, since).Scan(&count)
	return count, err
}

// drivableIssuesSQL finds work that claims to be in flight while nothing is
// actually running on it.
//
// The driver deliberately does NOT reuse ListStalledWork for this. That query
// treats a plan in an active state as proof of actionability — it says so in
// its own cannot_see list — so an issue whose plan says `working` while no
// task exists is filtered OUT as healthy. That is exactly the false-healthy
// condition that let a whole board freeze while the detector reported nothing
// wrong, and it also means a plan installed by this driver would disappear
// before it could ever be dispatched.
//
// The predicate here is the inverse: an open issue, no live task, and either
// no plan at all or a plan whose current stage is owned by an agent. A short
// quiet period keeps the driver from racing a task that has just finished and
// is still writing its result.
//
// Blocked is excluded, and so is anything whose recovery.wait_kind names the
// owner. Those are not stalls — they are deliberate stops, and restarting one
// would walk straight through a gate somebody put there on purpose. SILV-55
// was blocked on "Approve or revise the attached CRM source-account
// responsive-layout preview before product code changes continue"; driving it
// would have resumed product changes the owner had not approved. Work waiting
// on a person belongs in that person's Inbox, not back in the queue.
const drivableIssuesSQL = `
SELECT issue.id, issue.number
FROM issue
LEFT JOIN issue_status status_entry
  ON status_entry.workspace_id = issue.workspace_id
 AND status_entry.key = issue.status
 AND status_entry.archived_at IS NULL
LEFT JOIN issue_workflow_plan plan
  ON plan.issue_id = issue.id AND plan.workspace_id = issue.workspace_id
WHERE COALESCE(status_entry.category, issue.status) NOT IN ('done', 'cancelled', 'blocked')
  AND COALESCE(issue.metadata->>'recovery.wait_kind', '') NOT LIKE 'owner%'
  -- A child cannot start ahead of the parent it depends on. SILV-44/45/46 are
  -- stages 2 and 3 of a chain whose parent SILV-42 is blocked awaiting the
  -- owner's approval of a cross-app schema contract; dispatching them builds
  -- on a decision that has not been made, and the driver did exactly that the
  -- moment a scope restriction was lifted. A blocked or owner-gated parent
  -- holds its children.
  AND NOT EXISTS (
    SELECT 1 FROM issue parent
    LEFT JOIN issue_status parent_status
      ON parent_status.workspace_id = parent.workspace_id
     AND parent_status.key = parent.status
     AND parent_status.archived_at IS NULL
    WHERE parent.id = issue.parent_issue_id
      AND (
        COALESCE(parent_status.category, parent.status) = 'blocked'
        OR COALESCE(parent.metadata->>'recovery.wait_kind', '') LIKE 'owner%'
      )
  )
  AND NOT EXISTS (
    SELECT 1 FROM agent_task_queue live
    WHERE live.issue_id = issue.id
      AND live.status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
  )
  AND (
    plan.issue_id IS NULL
    OR plan.current_state IN (
      'received', 'queued', 'working', 'technical_review', 'correction',
      'deploying', 'production_verifying', 'rolling_back'
    )
  )
  -- A stage whose last run died because the PROVIDER had no capacity is not
  -- going to succeed on the next tick. Retrying it every interval produces
  -- nothing but failed rows and status flapping: the Silver Mediums Codex
  -- account exhausted on 2026-09-21 with a reset five days out, and all three
  -- live issues re-failed within seconds of each other. Back these off hard
  -- and let the ordinary quiet period govern everything else.
  AND NOT EXISTS (
    SELECT 1 FROM agent_task_queue recent
    WHERE recent.issue_id = issue.id
      AND recent.status = 'failed'
      AND recent.error IS NOT NULL
      AND (
        recent.error ILIKE '%usage limit%' OR recent.error ILIKE '%available credits%'
        OR recent.error ILIKE '%rate limit%' OR recent.error ILIKE '%at capacity%'
      )
      AND COALESCE(recent.completed_at, recent.created_at) > $2
      AND NOT EXISTS (
        SELECT 1 FROM agent_task_queue newer
        WHERE newer.issue_id = issue.id
          AND (newer.created_at, newer.id) > (recent.created_at, recent.id)
          AND newer.status <> 'failed'
      )
  )
  AND GREATEST(
    issue.updated_at,
    COALESCE(plan.updated_at, '-infinity'::timestamptz),
    COALESCE((
      SELECT max(COALESCE(task.completed_at, task.started_at, task.created_at))
      FROM agent_task_queue task WHERE task.issue_id = issue.id
    ), '-infinity'::timestamptz)
  ) < $1
ORDER BY issue.number
LIMIT 200`

// listDrivable returns the issues the driver may act on this pass.
func (d *WorkflowDriver) listDrivable(ctx context.Context, quiet time.Duration) ([]StalledWorkIssue, error) {
	backoff := d.CapacityBackoff
	if backoff <= 0 {
		backoff = defaultWorkflowDriverCapacityBackoff
	}
	rows, err := d.Pool.Query(ctx, drivableIssuesSQL, d.now().Add(-quiet), d.now().Add(-backoff))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := make([]StalledWorkIssue, 0)
	for rows.Next() {
		var id pgtype.UUID
		var number int32
		if err := rows.Scan(&id, &number); err != nil {
			return nil, err
		}
		issues = append(issues, StalledWorkIssue{IssueID: util.UUIDToString(id), IssueNumber: number})
	}
	return issues, rows.Err()
}

// ownerWaitSQL finds open work that is parked on a decision only the owner can
// make, and that the owner has not been told about.
//
// These are the issues the driver deliberately refuses to restart. Refusing is
// only half an answer: SILV-55 sat on "Approve or revise the attached CRM
// source-account responsive-layout preview" and SILV-61 on "Decide Kaya
// least-privilege Review access" for days with nothing in the owner's Inbox,
// because the original decisions had been archived unread and nothing ever
// re-raised them. An issue that cannot move without a person must appear in
// that person's Inbox for as long as it stays parked.
const ownerWaitSQL = `
SELECT issue.id, issue.number, issue.workspace_id, issue.title,
       issue.metadata->>'recovery.wait_reason' AS wait_reason
FROM issue
LEFT JOIN issue_status status_entry
  ON status_entry.workspace_id = issue.workspace_id
 AND status_entry.key = issue.status
 AND status_entry.archived_at IS NULL
WHERE COALESCE(status_entry.category, issue.status) NOT IN ('done', 'cancelled')
  AND COALESCE(issue.metadata->>'recovery.wait_kind', '') LIKE 'owner%'
  AND btrim(COALESCE(issue.metadata->>'recovery.wait_reason', '')) <> ''
  AND NOT EXISTS (
    SELECT 1 FROM inbox_item item
    WHERE item.issue_id = issue.id
      AND item.recipient_type = 'member'
      AND item.type = 'workflow_owner_decision'
      AND item.archived = false
      -- An ANSWERED decision is not an open question. SILV-55 kept two
      -- answered rows (intake_state 'continued') sitting unarchived, which
      -- made the parked approval look already-raised and left the issue
      -- waiting on the owner with nothing to click.
      AND COALESCE(item.details->>'intake_state', '') NOT IN ('continued', 'answered')
  )
LIMIT 100`

// stoppedWorkflowSQL finds work the controller has given up on.
//
// An issue that exhausts its correction budget, or is otherwise stopped with a
// controller installed, lands in Blocked and the driver deliberately will not
// restart it. That is correct, but on its own it recreates the original
// complaint: work stops and nobody is told. The board's Blocked column is only
// useful to an owner who happens to be looking at it.
//
// This carries the reviewer's own words across, because "SILV-41 stopped" is
// not actionable and "the UI failed to compile on the Mac Studio" is.
const stoppedWorkflowSQL = `
SELECT issue.id, issue.number, issue.workspace_id, issue.title,
       plan.correction_count, plan.max_corrections,
       COALESCE((
         SELECT event.reason FROM issue_workflow_event event
         WHERE event.issue_id = issue.id
         ORDER BY event.created_at DESC LIMIT 1
       ), '') AS last_reason
FROM issue
JOIN issue_workflow_plan plan
  ON plan.issue_id = issue.id AND plan.workspace_id = issue.workspace_id
LEFT JOIN issue_status status_entry
  ON status_entry.workspace_id = issue.workspace_id
 AND status_entry.key = issue.status
 AND status_entry.archived_at IS NULL
WHERE COALESCE(status_entry.category, issue.status) = 'blocked'
  -- Owner-gated work is already surfaced by surfaceOwnerWaits with its own
  -- question; raising it twice would put the same issue in the Inbox as both
  -- a decision and a stoppage.
  AND COALESCE(issue.metadata->>'recovery.wait_kind', '') NOT LIKE 'owner%'
  AND NOT EXISTS (
    SELECT 1 FROM inbox_item item
    WHERE item.issue_id = issue.id
      AND item.recipient_type = 'member'
      AND item.archived = false
      AND item.type IN ('workflow_owner_decision', 'workflow_stage_needs_owner')
      AND COALESCE(item.details->>'intake_state', '') NOT IN ('continued', 'answered')
  )
LIMIT 100`

// surfaceStoppedWorkflows tells the owner about work the controller stopped.
func (d *WorkflowDriver) surfaceStoppedWorkflows(ctx context.Context) (int, error) {
	rows, err := d.Pool.Query(ctx, stoppedWorkflowSQL)
	if err != nil {
		return 0, err
	}
	type stopped struct {
		id, workspaceID pgtype.UUID
		number          int32
		title, reason   string
		used, allowed   int32
	}
	items := make([]stopped, 0)
	for rows.Next() {
		var it stopped
		if err := rows.Scan(&it.id, &it.number, &it.workspaceID, &it.title, &it.used, &it.allowed, &it.reason); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	raised := 0
	for _, it := range items {
		recipients, err := d.Queries.ListWorkspaceManagerUserIDs(ctx, it.workspaceID)
		if err != nil || len(recipients) == 0 {
			continue
		}
		question := "This work stopped and needs your decision."
		if it.used >= it.allowed && it.allowed > 0 {
			question = fmt.Sprintf("This work was sent back for changes %d times and still did not pass review, so it stopped. Decide whether to keep going, change what is being asked for, or drop it.", it.used)
		}
		body := question
		if strings.TrimSpace(it.reason) != "" {
			body = question + " The last reviewer finding was: " + strings.TrimSpace(it.reason)
		}
		details, _ := json.Marshal(map[string]string{
			"owner_relevant":  "true",
			"owner_category":  "needs_you",
			"required_action": "decide_stopped_work",
			"question":        question,
			"consequence":     "Nothing further happens on this issue until you decide.",
		})
		item, err := d.Queries.CreateInboxItem(ctx, db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: it.workspaceID,
			RecipientType: "member", RecipientID: recipients[0],
			Type: "workflow_owner_decision", Severity: "action_required", IssueID: it.id,
			Title:     it.title,
			Body:      pgtypeText(body),
			ActorType: pgtypeText("system"), ActorID: pgtype.UUID{}, Details: details,
		})
		if err != nil {
			slog.Warn("raise stopped workflow failed", "issue_number", it.number, "error", err)
			continue
		}
		raised++
		if d.Bus != nil {
			d.Bus.Publish(events.Event{
				Type:        protocol.EventInboxNew,
				WorkspaceID: util.UUIDToString(it.workspaceID),
				Payload:     map[string]any{"item": item},
			})
		}
	}
	return raised, nil
}

// surfaceOwnerWaits admits one Inbox item per parked issue and returns how many
// it raised.
func (d *WorkflowDriver) surfaceOwnerWaits(ctx context.Context) (int, error) {
	rows, err := d.Pool.Query(ctx, ownerWaitSQL)
	if err != nil {
		return 0, err
	}
	type wait struct {
		id, workspaceID pgtype.UUID
		number          int32
		title, reason   string
	}
	waits := make([]wait, 0)
	for rows.Next() {
		var w wait
		if err := rows.Scan(&w.id, &w.number, &w.workspaceID, &w.title, &w.reason); err != nil {
			rows.Close()
			return 0, err
		}
		waits = append(waits, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	raised := 0
	for _, w := range waits {
		recipients, err := d.Queries.ListWorkspaceManagerUserIDs(ctx, w.workspaceID)
		if err != nil || len(recipients) == 0 {
			continue
		}
		// The Inbox read filter keys on type and these details fields, not on
		// severity alone, so a row missing them renders but never groups into
		// the owner's "needs you" surface.
		details, _ := json.Marshal(map[string]string{
			"owner_relevant":  "true",
			"owner_category":  "needs_you",
			"required_action": "answer_owner_wait",
			"question":        w.reason,
			"consequence":     "This issue stays paused until you answer.",
		})
		item, err := d.Queries.CreateInboxItem(ctx, db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: w.workspaceID,
			RecipientType: "member", RecipientID: recipients[0],
			Type: "workflow_owner_decision", Severity: "action_required", IssueID: w.id,
			Title:     w.title,
			Body:      pgtypeText(w.reason),
			ActorType: pgtypeText("system"), ActorID: pgtype.UUID{}, Details: details,
		})
		if err != nil {
			slog.Warn("raise owner wait failed", "issue_number", w.number, "error", err)
			continue
		}
		raised++
		if d.Bus != nil {
			d.Bus.Publish(events.Event{
				Type:        protocol.EventInboxNew,
				WorkspaceID: util.UUIDToString(w.workspaceID),
				Payload:     map[string]any{"item": item},
			})
		}
	}
	return raised, nil
}

// DriveOnce runs one pass: read the stalled set, and for each issue either
// resume its current stage or escalate it to the owner.
func (d *WorkflowDriver) DriveOnce(ctx context.Context) (DriveReport, error) {
	report := DriveReport{Actions: make([]DriveAction, 0)}
	if d == nil || d.Queries == nil || d.Pool == nil {
		return report, errors.New("workflow driver is not configured")
	}
	// A dry run never reaches a dispatch call, so it does not need a
	// TaskService; requiring one only where writes happen keeps the preview
	// harness unable to enqueue even by mistake.
	if !d.DryRun && d.TaskService == nil {
		return report, errors.New("workflow driver has no task service")
	}
	quiet := d.QuietPeriod
	if quiet <= 0 {
		quiet = defaultWorkflowDriverQuietPeriod
	}
	if !d.DryRun {
		raised, waitErr := d.surfaceOwnerWaits(ctx)
		if waitErr != nil {
			slog.Warn("surface owner waits failed", "error", waitErr)
		} else if raised > 0 {
			report.OwnerRaised = raised
			slog.Info("raised parked work to the owner", "count", raised)
		}
		if stopped, stopErr := d.surfaceStoppedWorkflows(ctx); stopErr != nil {
			slog.Warn("surface stopped workflows failed", "error", stopErr)
		} else if stopped > 0 {
			report.OwnerRaised += stopped
			slog.Info("raised stopped work to the owner", "count", stopped)
		}
	}
	candidates, err := d.listDrivable(ctx, quiet)
	if err != nil {
		return report, fmt.Errorf("list drivable issues: %w", err)
	}
	report.Scanned = len(candidates)
	for _, stalled := range candidates {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		if !d.covers(stalled.IssueNumber) {
			report.Skipped++
			continue
		}
		action, err := d.driveIssue(ctx, stalled)
		if err != nil {
			slog.Warn("workflow driver could not advance issue",
				"issue_number", stalled.IssueNumber, "issue_id", stalled.IssueID, "error", err)
			action.Action = "error"
			action.Detail = err.Error()
		}
		switch action.Action {
		case "resumed":
			report.Resumed++
		case "planned":
			report.Planned++
		case "escalated":
			report.Escalated++
		case "advanced":
			report.Advanced++
		case "triage":
			report.Triaged++
		default:
			report.Skipped++
		}
		report.Actions = append(report.Actions, action)
	}
	return report, nil
}

func (d *WorkflowDriver) driveIssue(ctx context.Context, stalled StalledWorkIssue) (DriveAction, error) {
	act := DriveAction{IssueNumber: stalled.IssueNumber, IssueID: stalled.IssueID, Stage: stalled.WorkflowState, Action: "skipped"}
	stageAdvanced := false

	issueID, err := util.ParseUUID(stalled.IssueID)
	if err != nil {
		return act, fmt.Errorf("parse issue id: %w", err)
	}
	issue, err := d.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return act, fmt.Errorf("load issue: %w", err)
	}

	plan, err := d.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		ordinaryChat, chatErr := d.Queries.IsOrdinaryChatIssue(ctx, db.IsOrdinaryChatIssueParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if chatErr != nil {
			return act, fmt.Errorf("read ordinary Chat execution: %w", chatErr)
		}
		if ordinaryChat.Bool {
			// Ordinary delegation has its own correction/completion path. Do
			// not install a legacy controller or ask its owner to create one.
			act.Detail = "ordinary Chat execution is not a legacy squad workflow"
			return act, nil
		}
		// An ordinary agent-routed issue never got a controller. Install one
		// now and let the NEXT pass dispatch it: a freshly installed plan is
		// in `received`, and keeping install and dispatch in separate passes
		// means a bad install is visible on the board before anything runs.
		if d.DryRun {
			act.Action = "planned"
			proposal, ok, propErr := ProposeIssueWorkflowPlan(ctx, d.Queries, issue)
			if propErr != nil {
				return act, propErr
			}
			if !ok {
				act.Action = "needs_plan"
				act.Detail = "no squad declares a controller for this issue"
				return act, nil
			}
			impl, _ := d.Queries.GetAgent(ctx, proposal.ImplementerID)
			rev, _ := d.Queries.GetAgent(ctx, proposal.ReviewerID)
			act.AgentID = util.UUIDToString(proposal.ImplementerID)
			act.Detail = fmt.Sprintf("squad %q: %s implements, %s reviews", proposal.Squad.Name, impl.Name, rev.Name)
			return act, nil
		}
		installed, created, ensureErr := EnsureIssueWorkflowPlan(ctx, d.Queries, issue)
		if ensureErr != nil {
			return act, fmt.Errorf("install workflow plan: %w", ensureErr)
		}
		if !created {
			act.Action = "needs_plan"
			act.Detail = "no squad in this workspace declares a controller for this issue"
			// Gap A: with no assignee and no squad template, this issue would
			// otherwise land here on every single sweep forever with nothing
			// else happening — no plan, no task, no owner-visible trace. Raise
			// it once so the owner can give it an assignee or a squad.
			if raiseErr := d.raiseOwnerInboxItem(ctx, issue, workflowNeedsPlanInboxType, "assign_unworkable_issue",
				issue.Title,
				"This issue has nobody who can work it. It has no assignee, and no squad in this workspace declares a workflow controller for it, so it will sit here forever unless you assign it to an agent or a squad.",
				map[string]string{"workflow_state": "needs_plan"},
			); raiseErr != nil {
				slog.Warn("raise needs-plan owner notice failed", "issue_number", issue.Number, "error", raiseErr)
			}
			return act, nil
		}
		// UpsertIssueWorkflowPlan hardcodes max_corrections = 1, so ONE failed
		// review ends the issue: SILV-41 and SILV-65 both reached
		// correction_limit_reached on their first review cycle, on work a
		// reviewer had correctly sent back for a fix. Real work usually needs
		// more than one round, and the schema permits up to 3. The bound still
		// exists — it just stops being a single strike.
		if _, limErr := d.Pool.Exec(ctx,
			`UPDATE issue_workflow_plan SET max_corrections = 3 WHERE issue_id = $1 AND max_corrections < 3`,
			issue.ID); limErr != nil {
			return act, fmt.Errorf("widen correction budget: %w", limErr)
		}
		act.Action = "planned"
		act.Stage = installed.CurrentState
		act.AgentID = util.UUIDToString(installed.ImplementerID)
		return act, nil
	}
	if err != nil {
		return act, fmt.Errorf("load workflow plan: %w", err)
	}
	act.Stage = plan.CurrentState

	// An owner-runtime route carries its own successor rules and its own
	// dispatcher. Re-dispatching it from here would create a second,
	// independently routed hand-off beside the authoritative one.
	if len(plan.OwnerRuntimeRoute) > 0 {
		act.Detail = "owner runtime route owns its own dispatch"
		return act, nil
	}

	// Settle WHO the implementer is before asking whether implementation is
	// finished. An agent that re-routes an issue to a better specialist does
	// the work under its own identity, so checking the stage against a stale
	// implementer would either miss the completed work or, worse, advance on
	// somebody else's finished task and later send the correction back to an
	// agent that never touched it.
	if !d.DryRun {
		if owner, drivableNow := stageOwner(plan); drivableNow {
			if _, recErr := d.reconcileStageOwner(ctx, issue, plan, owner); recErr != nil {
				return act, fmt.Errorf("reconcile stage owner: %w", recErr)
			}
			refreshed, refErr := d.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
			if refErr != nil {
				return act, fmt.Errorf("reload plan after reconcile: %w", refErr)
			}
			plan = refreshed
			act.Stage = plan.CurrentState
		}
	}

	// Then see whether the current stage has actually been completed. Agents on
	// the ordinary dispatch path have tools and do real work, but carry no
	// workflow assignment, so an implementer can finish the job and leave the
	// plan reading `working` forever. The server watches for the finished task
	// instead of waiting to be told about it.
	if !d.DryRun {
		advanced, advErr := d.advanceCompletedStage(ctx, issue, &plan)
		if advErr != nil {
			return act, fmt.Errorf("advance completed stage: %w", advErr)
		}
		if advanced {
			stageAdvanced = true
			act.Stage = plan.CurrentState
		}
	}

	target, drivable := stageOwner(plan)
	if !drivable {
		// needs_owner / blocked / terminal. These are already correct on the
		// board; the driver must not overrule a person or restart finished work.
		act.Detail = "stage is not agent-owned"
		return act, nil
	}
	if !target.Valid {
		return act, d.escalate(ctx, issue, plan, &act, "the plan does not name an agent for this stage")
	}

	budget := d.IssueTokenBudget
	if budget == 0 {
		budget = defaultWorkflowDriverIssueTokenBudget
	}
	if budget > 0 && !d.DryRun {
		var spent int64
		if err := d.Pool.QueryRow(ctx,
			// Counted from plan creation, not from the issue's whole history.
			// The budget exists to stop the driver feeding a runaway, so it
			// measures what the driver is accountable for. SILV-41 carries
			// 30.7M billable tokens from a single pre-driver run on
			// 2026-09-19; charging that against work now running correctly
			// would pause a healthy issue for a debt it cannot repay.
			`SELECT COALESCE(sum(u.input_tokens + u.output_tokens + u.cache_write_tokens), 0)
			 FROM task_usage u JOIN agent_task_queue t ON t.id = u.task_id
			 WHERE t.issue_id = $1 AND t.created_at >= $2`, issue.ID, plan.CreatedAt).Scan(&spent); err != nil {
			return act, fmt.Errorf("read issue token spend: %w", err)
		}
		if spent >= budget {
			return act, d.escalate(ctx, issue, plan, &act, fmt.Sprintf(
				"this issue has used %.1fM of its %.1fM token budget since work started and still has not finished, so it was paused rather than given more. Decide whether it is worth continuing, needs a smaller scope, or should be dropped.",
				float64(spent)/1e6, float64(budget)/1e6))
		}
	}

	attempts, err := d.attemptsForStage(ctx, issue.ID, plan.UpdatedAt)
	if err != nil {
		return act, fmt.Errorf("count stage attempts: %w", err)
	}
	act.Attempts = attempts
	if attempts >= d.maxAttempts() {
		return act, d.escalate(ctx, issue, plan, &act,
			fmt.Sprintf("this step was restarted %d times without reporting a result", attempts))
	}

	if d.DryRun {
		act.Action = "resumed"
		act.AgentID = util.UUIDToString(target)
		act.Detail = "would dispatch the stage owner"
		return act, nil
	}

	// Dispatch through the ordinary issue path rather than
	// EnqueueTaskForWorkflowTransition. That path builds its prompt from a
	// persisted workflow EVENT, and the driver exists precisely because no
	// event was written — synthesising one would record a hand-off that no
	// agent ever made, which is worse than the stall it repairs.
	//
	// The ordinary path routes to the issue's assignee, so the assignee is
	// first aligned with whoever owns the current stage. That also makes the
	// board honest: the person shown as owning the issue is the one actually
	// being asked to work.
	// Re-run after any advance: the stage may have changed seat since the
	// reconcile above, and an assignee that would leave one agent both
	// implementing and reviewing must still be caught.
	target, err = d.reconcileStageOwner(ctx, issue, plan, target)
	if err != nil {
		return act, fmt.Errorf("reconcile stage owner: %w", err)
	}
	if !target.Valid {
		return act, d.escalate(ctx, issue, plan, &act,
			"no agent is named for this issue's current step")
	}
	act.AgentID = util.UUIDToString(target)
	// Gap B: capture this BEFORE syncStageAssignee writes the assignee. If the
	// issue had none, target can only have come from the squad template's own
	// default implementer (ProposeIssueWorkflowPlan, workflow_plan_ensure.go:
	// 103-107 — the override to the issue's own assignee requires one to
	// already exist), so a successful sync here is exactly the silent
	// auto-assign this closes.
	hadNoAssignee := !issue.AssigneeID.Valid
	if moved, syncErr := d.syncStageAssignee(ctx, issue, plan, target); syncErr != nil {
		return act, fmt.Errorf("align assignee with stage: %w", syncErr)
	} else if !moved {
		return act, d.escalate(ctx, issue, plan, &act,
			"this issue could not be pointed at the agent that owns its current step")
	} else if hadNoAssignee {
		d.surfaceAutoAssign(ctx, issue, plan, target)
	}
	issue, err = d.Queries.GetIssue(ctx, issue.ID)
	if err != nil {
		return act, fmt.Errorf("reload issue after assignment: %w", err)
	}
	// Mark the stage started BEFORE dispatching.
	//
	// workflowstate.Decide only accepts implementation_ready from `working` or
	// `correction`, so an agent dispatched while the plan still reads
	// `received` cannot legally report its own hand-off no matter how clearly
	// it is asked to. SILV-43's implementer said exactly that: "Workflow plan
	// revision 2 is still in `received` ... `implementation_ready` remains
	// invalid." In the original design task_started carried this transition,
	// and nothing fires it when the server is the one dispatching.
	//
	// Ordering matters: the task must be created AFTER plan.updated_at moves,
	// or attemptsForStage counts zero attempts forever and the driver
	// re-dispatches the same stage on every pass.
	if state := workflowstate.State(plan.CurrentState); state == workflowstate.Received || state == workflowstate.Queued {
		if _, startErr := d.Pool.Exec(ctx,
			`UPDATE issue_workflow_plan SET current_state = $2, revision = revision + 1, updated_at = now()
			 WHERE issue_id = $1 AND current_state = $3`,
			issue.ID, string(workflowstate.Working), plan.CurrentState); startErr != nil {
			return act, fmt.Errorf("mark stage started: %w", startErr)
		}
		plan.CurrentState = string(workflowstate.Working)
		act.Stage = plan.CurrentState
	}
	task, err := d.TaskService.EnqueueTaskForIssueWithHandoff(ctx, issue, stageHandoffNote(issue, plan), pgtype.UUID{})
	if errors.Is(err, ErrDuplicatePendingTask) {
		act.Detail = "the stage owner already has pending work"
		return act, nil
	}
	if err != nil {
		// A dispatch that cannot happen is exactly the silent stall this
		// component exists to remove, so it becomes an owner-visible item
		// rather than another log line.
		return act, d.escalate(ctx, issue, plan, &act, "this step could not be started: "+err.Error())
	}
	act.Action = "resumed"
	act.AgentID = util.UUIDToString(task.AgentID)
	act.TaskID = util.UUIDToString(task.ID)
	switch {
	case stageAdvanced:
		act.Action = "advanced"
		act.Detail = "implementation finished; handed to review"
	case attempts == 0:
		act.Detail = "first dispatch"
	}
	return act, nil
}

// advanceCompletedStage moves an implementation stage into review once the
// implementer's task has actually finished.
//
// This is the piece that replaces the agent's missing hand-off. It does NOT
// judge whether the work is good — that is exactly the reviewer's job, and
// routing a half-finished candidate into review is recoverable (the reviewer
// sends it back as a correction, bounded by max_corrections) whereas leaving
// it stalled is not.
//
// Only a task by the CURRENT implementer that completed successfully after the
// plan last moved counts. That excludes older runs, failed runs, and runs by
// an agent the plan no longer names, so a plan cannot advance on the strength
// of work that was never done for this stage.
func (d *WorkflowDriver) advanceCompletedStage(ctx context.Context, issue db.Issue, plan *db.IssueWorkflowPlan) (bool, error) {
	switch workflowstate.State(plan.CurrentState) {
	case workflowstate.Received, workflowstate.Queued, workflowstate.Working, workflowstate.Correction:
	default:
		return false, nil
	}
	if !plan.ReviewerID.Valid || plan.ReviewerID == plan.ImplementerID {
		return false, nil
	}
	var landed bool
	if err := d.Pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM agent_task_queue
		   WHERE issue_id = $1 AND agent_id = $2 AND status = 'completed'
		     AND COALESCE(completed_at, created_at) >= $3)`,
		issue.ID, plan.ImplementerID, plan.UpdatedAt).Scan(&landed); err != nil {
		return false, err
	}
	if !landed {
		return false, nil
	}
	if _, err := d.Pool.Exec(ctx,
		`UPDATE issue_workflow_plan
		 SET current_state = $2, revision = revision + 1, updated_at = now()
		 WHERE issue_id = $1 AND current_state = $3`,
		issue.ID, string(workflowstate.TechnicalReview), plan.CurrentState); err != nil {
		return false, err
	}
	// Keep the durable status in step so the board shows In review rather than
	// leaving the owner to infer the stage from who happens to be assigned.
	if _, err := d.Queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
		ID: issue.ID, WorkspaceID: issue.WorkspaceID, Status: "in_review",
	}); err != nil {
		return false, err
	}
	refreshed, err := d.Queries.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return false, err
	}
	*plan = refreshed
	return true, nil
}

// reconcileStageOwner settles a disagreement between who the issue is assigned
// to and who the plan says owns the current stage.
//
// Implementation follows ASSIGNMENT. An agent that picks up a piece of work and
// hands it to a better-suited specialist is doing its job — the first run on
// SILV-65 read a Phone Farm gallery fault and moved it to the Phone Farm
// specialist — so the plan adopts that choice rather than dragging the issue
// back to whoever was named first. Treating a deliberate re-route as a conflict
// would bury real routing decisions in the owner's Inbox.
//
// Review does NOT follow assignment. The reviewer seat exists to be
// independent of the implementer, so a review stage keeps the plan's reviewer
// and the assignee is moved to match. If adopting an assignee would leave the
// same agent implementing and reviewing, no valid owner exists and the caller
// escalates instead.
func (d *WorkflowDriver) reconcileStageOwner(ctx context.Context, issue db.Issue, plan db.IssueWorkflowPlan, target pgtype.UUID) (pgtype.UUID, error) {
	implementerStage := false
	switch workflowstate.State(plan.CurrentState) {
	case workflowstate.Received, workflowstate.Queued, workflowstate.Working, workflowstate.Correction:
		implementerStage = true
	}
	if !implementerStage {
		return target, nil
	}
	if !issue.AssigneeID.Valid || issue.AssigneeType.String != "agent" || issue.AssigneeID == target {
		return target, nil
	}
	if issue.AssigneeID == plan.ReviewerID {
		// The reviewer holds the assignee seat because the REVIEW stage put it
		// there, and a failed review has just handed the issue back. That is
		// workflow residue, not a routing decision, so the plan's implementer
		// keeps correction and syncStageAssignee moves the seat back.
		return target, nil
	}
	// Adopting the assignee resets the stage's attempt budget through
	// plan.updated_at, which is what we want: a different implementer is a
	// fresh attempt, not a continuation of the previous one's failures.
	if _, err := d.Pool.Exec(ctx,
		`UPDATE issue_workflow_plan SET implementer_id = $2, revision = revision + 1, updated_at = now()
		 WHERE issue_id = $1 AND reviewer_id <> $2`, issue.ID, issue.AssigneeID); err != nil {
		return pgtype.UUID{}, err
	}
	return issue.AssigneeID, nil
}

// stageHandoffNote tells the dispatched agent which seat it holds and how to
// report its result.
//
// The ordinary dispatch path gives an agent real tools but carries no workflow
// assignment, so without this the agent has no idea it is one stage of a
// reviewed workflow and no idea that a hand-off is expected. The note closes
// that gap from the prompt side; the server advancing on task completion
// remains the backstop for when an agent still says nothing. An ordinary task
// is allowed to record a workflow event — the endpoint only requires a running
// task whose agent matches the actor — so the instruction is actionable.
//
// It deliberately tells the agent NOT to report when it could not finish:
// a false implementation_ready sends unfinished work to review, and a review
// that never happened is worse than a stage that visibly stalls.
func stageHandoffNote(issue db.Issue, plan db.IssueWorkflowPlan) string {
	id := util.UUIDToString(issue.ID)
	switch workflowstate.State(plan.CurrentState) {
	case workflowstate.TechnicalReview:
		return "You are the independent reviewer for this issue. Do not change the implementation. Review has two separate, both-required jobs — neither substitutes for the other. " +
			"(1) TECHNICAL CORRECTNESS: check the change against the issue's acceptance criteria, then actually run `go build ./...` and `go test ./...` against the candidate yourself (use `go build -buildvcs=false ./...` / `go test -buildvcs=false ./...` if the worktree's git backing store is unavailable) and capture their real exit codes and output; do not approve on inspection alone. " +
			"Write that proof to a JSON file shaped {schema_version:1,contract:\"build_test_run.v1\",runs:[{command,exit_code,observed_at,output_sha256,output_tail}]} — one run for the build command and one for the test command, each with its real exit code, an RFC3339 observed_at, a SHA-256 of its actual output, and a short tail of that output. " +
			"(2) DESIGN AND BLAST RADIUS — this is the part 'it builds and the tests pass' does NOT cover, and it is the actual point of having a reviewer: a change can be technically correct and still be a bad decision a senior engineer would never sign off on (for example: representing one client's two subscription-tier metrics by duplicating the client record itself, because the ticket only asked for '2 of every metric' and never said 'don't multiply the client'). Acceptance criteria tell you the ticket's literal ask was met; they do not tell you the design is sane. Before approving, go actually look at how the rest of the codebase already represents or handles the same kind of thing — grep for the existing pattern, open the neighboring code, compare — do not assume, and do not review the diff in isolation. Ask, as a senior engineer reviewing a junior's PR would: does this match how similar data or behavior is already modeled elsewhere, or does it invent a second, inconsistent way to do the same thing? Could this duplicate, multiply, break, or conflict with anything outside this one ticket's narrow scope — another route, table, contract, or app that reads this data? Would you actually sign off on this design, not just 'does it technically satisfy the ask'? " +
			"Record that reasoning in a JSON file shaped {schema_version:1,contract:\"blast_radius_review.v1\",related_areas_checked:[\"<file or area you actually opened and compared this change against>\", ...],reasoning:\"<what you checked, what this could affect outside the ticket's narrow scope, and why this is NOT a case of shipping something that looks technically fine but is actually a bad call>\"}. related_areas_checked must name real places you looked (at least one), and reasoning must be substantive — a restatement of the ticket, or a bare 'looks fine' / 'LGTM', will be refused. " +
			"Then record " +
			"`multica issue workflow-event " + id + " review_passed --reason \"<one line>\" --test-evidence-file <path> --blast-radius-file <path>` " +
			"or `multica issue workflow-event " + id + " review_failed --reason \"<what is wrong>\"`. " +
			"review_passed is refused unless the test evidence shows a genuinely passing build and test run AND the blast-radius reasoning is present and substantive — a clean build is necessary but not sufficient, and do not report exit code 0 for a command you did not run. " +
			"A failed review returns the issue to the implementer for correction, so be specific about what must change — a design objection (not just a failing test) is a completely valid reason to fail review."
	case workflowstate.Correction:
		return "An independent review of this issue failed and it has come back to you for correction. Read the reviewer's comment first, fix what it identified, then record " +
			"`multica issue workflow-event " + id + " implementation_ready --reason \"<one line>\"` to send it back for review. " +
			"If you cannot fix it, do not record that event — explain what is blocking in an issue comment instead."
	case workflowstate.Received, workflowstate.Queued, workflowstate.Working:
		return "You own the implementation of this issue. Do the work and verify it yourself. When the change is complete, record " +
			"`multica issue workflow-event " + id + " implementation_ready --reason \"<one line>\"` so it is routed to an independent reviewer. " +
			"If you cannot complete it, do not record that event — explain what is blocking in an issue comment instead."
	default:
		return ""
	}
}

// syncStageAssignee points the issue at whoever owns its current stage.
//
// It always moves. Protecting a hand-picked assignee belongs in
// reconcileStageOwner, which already lets the assignee WIN on an
// implementation stage; by the time we get here, target is either that
// assignee or a seat the plan owns outright. Re-checking the assignee here
// blocked the real hand-off: SILV-65 had been re-routed to the Phone Farm
// specialist mid-implementation, so when the stage advanced, the move to the
// Quality Checker was refused and a correct progression was reported to the
// owner as a problem.
func (d *WorkflowDriver) syncStageAssignee(ctx context.Context, issue db.Issue, plan db.IssueWorkflowPlan, target pgtype.UUID) (bool, error) {
	if !target.Valid {
		return false, nil
	}
	if issue.AssigneeID.Valid && issue.AssigneeID == target {
		return true, nil
	}
	_, err := d.Pool.Exec(ctx,
		`UPDATE issue SET assignee_type = 'agent', assignee_id = $2,
		        revision = revision + 1, updated_at = now()
		 WHERE id = $1`, issue.ID, target)
	if err != nil {
		return false, err
	}
	return true, nil
}

// spendEvidence summarises where an issue's tokens actually went, so triage
// starts from numbers instead of re-deriving them at further cost.
func (d *WorkflowDriver) spendEvidence(ctx context.Context, issue db.Issue, since pgtype.Timestamptz) string {
	var runs int
	var total, largest, cacheReads int64
	err := d.Pool.QueryRow(ctx,
		`SELECT count(DISTINCT t.id),
		        COALESCE(sum(u.input_tokens + u.output_tokens + u.cache_write_tokens), 0),
		        COALESCE(max(u.input_tokens + u.output_tokens + u.cache_write_tokens), 0),
		        COALESCE(sum(u.cache_read_tokens), 0)
		 FROM agent_task_queue t LEFT JOIN task_usage u ON u.task_id = t.id
		 WHERE t.issue_id = $1 AND t.created_at >= $2`, issue.ID, since).Scan(&runs, &total, &largest, &cacheReads)
	if err != nil || runs == 0 {
		return ""
	}
	avg := total / int64(runs)
	return fmt.Sprintf(
		" Spend so far: %d runs, %.2fM billable tokens, average %.0fk per run, largest single run %.2fM, plus %.1fM cache reads.",
		runs, float64(total)/1e6, float64(avg)/1e3, float64(largest)/1e6, float64(cacheReads)/1e6)
}

// triageNote hands a squad leader a stopped stage with its spend evidence.
//
// An earlier version asked the leader to judge whether the spend was
// "justified". That was wrong: of the patterns that burn tokens, only one —
// work that is genuinely large — is ever legitimate. Re-reading the same files
// every run, retrying an approach that keeps failing, re-deriving context that
// is already on the issue, searching broadly where a targeted read would do,
// and an implementer and reviewer disagreeing in circles are all DEFECTS.
// There is nothing to deliberate about them; finding one means fixing it.
// Framing them as a judgement call invites an agent to rationalise waste.
func triageNote(reason, evidence string) string {
	return "This issue stopped and YOU decide what happens next, not Drew." +
		" What happened: " + reason + evidence +
		" Work out where the cost went before doing anything else: read the run history and what each run produced." +
		" These patterns are DEFECTS, not judgement calls — if you find one, fix that cause and let the work continue," +
		" and record on the issue which one it was:" +
		" the same files read again on every run; a loop retrying an approach that keeps failing;" +
		" context re-derived each run instead of read from the issue; a broad search where a targeted read would do;" +
		" implementer and reviewer disagreeing in circles." +
		" The ONLY legitimate reason for high spend is that the work is genuinely large. If that is the case," +
		" say so plainly with the evidence and let it continue." +
		" If the work is not worth what it costs, close it and say why. If it belongs to a better-suited agent," +
		" hand it over." +
		" Only record an owner decision if the call is genuinely Drew's — whether the work is still worth doing," +
		" something customer or employee facing, money, or access — and then state your recommendation rather than" +
		" asking an open question. Drew is not a developer; an engineering judgement is yours to make, not his to answer."
}

// escalateToController hands a stopped stage to the squad leader to triage.
//
// The owner is not the right recipient for "this issue used its budget without
// finishing — continue, narrow, or drop?". That is an engineering judgement,
// and routing it to a person who does not write code turns every technical
// stoppage into an interruption they cannot act on. The squad already names a
// leader for exactly this.
//
// One triage run per stage: if the leader has already looked since the plan
// last moved and the stage is still stopped, triage did not resolve it and the
// caller falls through to the owner.
func (d *WorkflowDriver) escalateToController(ctx context.Context, issue db.Issue, plan db.IssueWorkflowPlan, reason string) (string, bool) {
	if d.TaskService == nil || !plan.SquadID.Valid {
		return "", false
	}
	squad, err := d.Queries.GetSquad(ctx, plan.SquadID)
	if err != nil || !squad.LeaderID.Valid || squad.LeaderID == plan.ImplementerID {
		return "", false
	}
	leader, err := d.Queries.GetAgent(ctx, squad.LeaderID)
	if err != nil || leader.ArchivedAt.Valid || !leader.RuntimeID.Valid {
		return "", false
	}
	var alreadyTriaged bool
	if err := d.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent_task_queue
		   WHERE issue_id = $1 AND agent_id = $2 AND created_at >= $3)`,
		issue.ID, squad.LeaderID, plan.UpdatedAt).Scan(&alreadyTriaged); err != nil || alreadyTriaged {
		return "", false
	}
	if _, err := d.TaskService.EnqueueTaskForSquadLeaderWithHandoff(
		ctx, issue, squad.LeaderID, plan.SquadID,
		triageNote(reason, d.spendEvidence(ctx, issue, plan.CreatedAt)), pgtype.UUID{},
	); err != nil {
		if !errors.Is(err, ErrDuplicatePendingTask) {
			slog.Warn("triage dispatch failed", "issue_number", issue.Number, "error", err)
		}
		return "", false
	}
	return leader.Name, true
}

// surfaceAutoAssign tells the owner that an issue which had no assignee was
// just handed to a squad template's default implementer (Gap B). It fails
// soft — a lookup or Inbox-write error is logged, not returned — because the
// assignment and dispatch this backs up have already committed; refusing the
// drive pass over a notification that could not be written would be worse
// than the silent gap it closes.
func (d *WorkflowDriver) surfaceAutoAssign(ctx context.Context, issue db.Issue, plan db.IssueWorkflowPlan, assignee pgtype.UUID) {
	agent, err := d.Queries.GetAgent(ctx, assignee)
	if err != nil {
		slog.Warn("surface auto-assign: load agent failed", "issue_number", issue.Number, "error", err)
		return
	}
	squadName := ""
	if plan.SquadID.Valid {
		if squad, sqErr := d.Queries.GetSquad(ctx, plan.SquadID); sqErr == nil {
			squadName = squad.Name
		}
	}
	body := fmt.Sprintf("This issue had no assignee, so it was auto-assigned to %s", agent.Name)
	if squadName != "" {
		body += fmt.Sprintf(" via %s's default implementer", squadName)
	}
	body += ". Reassign it if that is wrong."
	if raiseErr := d.raiseOwnerInboxItem(ctx, issue, workflowAutoAssignedInboxType, "review_auto_assignment",
		issue.Title, body,
		map[string]string{"assigned_agent_id": util.UUIDToString(assignee), "workflow_state": plan.CurrentState},
	); raiseErr != nil {
		slog.Warn("raise auto-assign owner notice failed", "issue_number", issue.Number, "error", raiseErr)
	}
}

// raiseOwnerInboxItem creates one owner-visible Inbox row for issue, unless a
// non-archived row of the same type already exists for it.
//
// This is the exact dedup shape escalate has always used ("one question, not
// one per pass"): a SELECT EXISTS on (issue_id, type, archived=false) before
// any write. Reusing it here means a stage stuck on needs_plan, or an issue
// that was auto-assigned, gets ONE Inbox row that survives every later sweep
// rather than a fresh row every 60 seconds.
func (d *WorkflowDriver) raiseOwnerInboxItem(ctx context.Context, issue db.Issue, itemType, requiredAction, title, body string, extraDetails map[string]string) error {
	var exists bool
	if err := d.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM inbox_item WHERE issue_id=$1 AND type=$2 AND archived=false)`,
		issue.ID, itemType).Scan(&exists); err != nil {
		return fmt.Errorf("check existing %s notice: %w", itemType, err)
	}
	if exists {
		return nil
	}

	recipients, err := d.Queries.ListWorkspaceManagerUserIDs(ctx, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list owner recipients: %w", err)
	}
	if len(recipients) == 0 {
		return errors.New("workspace has no manager to notify")
	}

	// owner_relevant/owner_category/required_action are what the Inbox read
	// filter and the owner surfaces key on; severity alone renders but does
	// not group correctly (same requirement escalate and surfaceOwnerWaits
	// both already depend on).
	details := map[string]string{
		"owner_relevant":  "true",
		"owner_category":  "needs_you",
		"required_action": requiredAction,
	}
	for k, v := range extraDetails {
		details[k] = v
	}
	detailsJSON, _ := json.Marshal(details)

	item, err := d.Queries.CreateInboxItem(ctx, db.CreateInboxItemParams{
		ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID,
		RecipientType: "member", RecipientID: recipients[0],
		Type: itemType, Severity: "action_required", IssueID: issue.ID,
		Title:     title,
		Body:      pgtypeText(body),
		ActorType: pgtypeText("system"), ActorID: pgtype.UUID{}, Details: detailsJSON,
	})
	if err != nil {
		return fmt.Errorf("create %s notice: %w", itemType, err)
	}
	if d.Bus != nil {
		d.Bus.Publish(events.Event{
			Type:        protocol.EventInboxNew,
			WorkspaceID: util.UUIDToString(issue.WorkspaceID),
			Payload:     map[string]any{"item": item},
		})
	}
	return nil
}

// escalate hands a stopped stage to its controller, and only asks the owner
// when no controller can take it.
//
// Deduplicated on (issue, type, not archived): a stage that keeps failing must
// produce ONE question, not one per pass. The owner's Inbox is only useful if
// everything in it is genuinely theirs to answer.
func (d *WorkflowDriver) escalate(ctx context.Context, issue db.Issue, plan db.IssueWorkflowPlan, act *DriveAction, reason string) error {
	act.Action = "escalated"
	act.Detail = reason
	if d.DryRun {
		act.Detail = "would ask the owner: " + reason
		return nil
	}
	if leaderName, ok := d.escalateToController(ctx, issue, plan, reason); ok {
		act.Action = "triage"
		act.AgentID = ""
		act.Detail = leaderName + " is deciding what to do: " + reason
		return nil
	}

	var exists bool
	if err := d.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM inbox_item WHERE issue_id=$1 AND type=$2 AND archived=false)`,
		issue.ID, workflowDriverInboxType).Scan(&exists); err != nil {
		return fmt.Errorf("check existing owner notice: %w", err)
	}
	if exists {
		act.Detail = reason + " (owner already notified)"
		return nil
	}

	recipients, err := d.Queries.ListWorkspaceManagerUserIDs(ctx, issue.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list owner recipients: %w", err)
	}
	if len(recipients) == 0 {
		return errors.New("workspace has no manager to notify")
	}

	// owner_relevant/owner_category/required_action are what the Inbox read
	// filter and the owner surfaces key on; severity alone renders but does
	// not group correctly.
	details, _ := json.Marshal(map[string]string{
		"owner_relevant":  "true",
		"owner_category":  "needs_you",
		"required_action": "decide_stalled_stage",
		"workflow_state":  plan.CurrentState,
		"evidence":        reason,
	})
	item, err := d.Queries.CreateInboxItem(ctx, db.CreateInboxItemParams{
		ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID,
		RecipientType: "member", RecipientID: recipients[0],
		Type: workflowDriverInboxType, Severity: "action_required", IssueID: issue.ID,
		Title:     issue.Title,
		Body:      pgtypeText("This work stopped and could not be restarted automatically. " + reason),
		ActorType: pgtypeText("system"), ActorID: pgtype.UUID{}, Details: details,
	})
	if err != nil {
		return fmt.Errorf("create owner notice: %w", err)
	}
	if d.Bus != nil {
		d.Bus.Publish(events.Event{
			Type:        protocol.EventInboxNew,
			WorkspaceID: util.UUIDToString(issue.WorkspaceID),
			Payload:     map[string]any{"item": item},
		})
	}
	return nil
}

// Run drives the workflow on an interval for the life of ctx.
func (d *WorkflowDriver) Run(ctx context.Context) {
	interval := d.Interval
	if interval <= 0 {
		interval = defaultWorkflowDriverInterval
	}
	for {
		report, err := d.DriveOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("workflow drive pass failed", "error", err)
		} else if report.Resumed > 0 || report.Escalated > 0 || report.Advanced > 0 || report.Triaged > 0 {
			slog.Info("workflow drive pass complete",
				"scanned", report.Scanned, "resumed", report.Resumed, "advanced", report.Advanced, "triaged", report.Triaged,
				"escalated", report.Escalated, "needs_plan", report.Planned,
				"skipped", report.Skipped, "actions", report.Actions)
		} else {
			slog.Debug("workflow drive pass complete",
				"scanned", report.Scanned, "skipped", report.Skipped)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
