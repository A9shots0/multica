package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	stalledWorkQueryName        = "ListStalledWork"
	defaultStalledWorkLimit     = 100
	defaultStalledWorkInterval  = time.Minute
	defaultStalledWorkThreshold = 15 * time.Minute
)

var stalledWorkVisibilityLimits = []string{
	"work represented outside issue_workflow_plan, issue_current_picture, issue_workflow_action_receipt, or agent_task_queue",
	"in-memory or external work that has not written a durable task or workflow record",
	"the exact moment actionability was lost; stalled_since uses the latest durable issue, plan, picture, or task timestamp",
	"whether a non-empty next_automatic_action can execute; its durable presence is treated as actionable",
}

type StalledWorkCounts struct {
	Matched  int64 `json:"matched"`
	Returned int   `json:"returned"`
}

type StalledWorkIssue struct {
	IssueID         string    `json:"issue_id"`
	WorkspaceID     string    `json:"workspace_id"`
	IssueNumber     int32     `json:"issue_number"`
	IssueStatus     string    `json:"issue_status"`
	HealthState     string    `json:"health_state"`
	Reason          string    `json:"reason"`
	ParentIssueID   string    `json:"parent_issue_id,omitempty"`
	WorkflowState   string    `json:"workflow_state,omitempty"`
	HasWorkflowPlan bool      `json:"has_workflow_plan"`
	StalledSince    time.Time `json:"stalled_since"`
}

type StalledWorkReport struct {
	Query     string             `json:"query"`
	Cutoff    time.Time          `json:"cutoff"`
	Threshold string             `json:"threshold"`
	Counts    StalledWorkCounts  `json:"counts"`
	Truncated bool               `json:"truncated"`
	CannotSee []string           `json:"cannot_see"`
	Issues    []StalledWorkIssue `json:"issues"`
}

type stalledWorkQuerier interface {
	ListStalledWork(context.Context, db.ListStalledWorkParams) ([]db.ListStalledWorkRow, error)
}

// StalledWorkDetector reports durable issue/controller gaps without changing
// issue, workflow, task, Inbox, or activity state. Resolution remains owned by
// the deterministic controller or a separately authorized owner action.
type StalledWorkDetector struct {
	Queries   stalledWorkQuerier
	Threshold time.Duration
	Interval  time.Duration
	Limit     int32
	Now       func() time.Time
}

func (d *StalledWorkDetector) DetectOnce(ctx context.Context) (StalledWorkReport, error) {
	if d == nil || d.Queries == nil {
		return StalledWorkReport{}, errors.New("stalled work detector is not configured")
	}
	threshold := d.Threshold
	if threshold <= 0 {
		threshold = defaultStalledWorkThreshold
	}
	limit := d.Limit
	if limit <= 0 {
		limit = defaultStalledWorkLimit
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	cutoff := now.Add(-threshold)
	report := StalledWorkReport{
		Query: stalledWorkQueryName, Cutoff: cutoff, Threshold: threshold.String(),
		CannotSee: append([]string(nil), stalledWorkVisibilityLimits...),
		Issues:    make([]StalledWorkIssue, 0),
	}
	rows, err := d.Queries.ListStalledWork(ctx, db.ListStalledWorkParams{Cutoff: pgtype.Timestamptz{Time: cutoff, Valid: true}, RowLimit: limit})
	if err != nil {
		return report, err
	}
	if len(rows) > 0 {
		report.Counts.Matched = rows[0].TotalCount
	}
	report.Counts.Returned = len(rows)
	report.Truncated = report.Counts.Matched > int64(report.Counts.Returned)
	for _, row := range rows {
		issue := StalledWorkIssue{
			IssueID: util.UUIDToString(row.IssueID), WorkspaceID: util.UUIDToString(row.WorkspaceID),
			IssueNumber: row.IssueNumber, IssueStatus: row.IssueStatus, HealthState: "stalled",
			HasWorkflowPlan: row.HasWorkflowPlan, StalledSince: row.StalledSince.Time.UTC(),
		}
		if row.HasWorkflowPlan {
			issue.Reason = "workflow_plan_has_no_actionable_step"
		} else {
			issue.Reason = "workflow_plan_missing"
		}
		if row.ParentIssueID.Valid {
			issue.ParentIssueID = util.UUIDToString(row.ParentIssueID)
		}
		if row.WorkflowState.Valid {
			issue.WorkflowState = row.WorkflowState.String
		}
		report.Issues = append(report.Issues, issue)
	}
	return report, nil
}

func (d *StalledWorkDetector) Run(ctx context.Context) {
	interval := d.Interval
	if interval <= 0 {
		interval = defaultStalledWorkInterval
	}
	for {
		report, err := d.DetectOnce(ctx)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("stalled work detection failed", "query", stalledWorkQueryName, "error", err)
			}
		} else {
			level := slog.LevelInfo
			message := "stalled work scan complete"
			if report.Counts.Matched > 0 {
				level = slog.LevelWarn
				message = "stalled work detected"
			}
			slog.Log(ctx, level, message,
				"query", report.Query,
				"cutoff", report.Cutoff,
				"threshold", report.Threshold,
				"matched_count", report.Counts.Matched,
				"returned_count", report.Counts.Returned,
				"truncated", report.Truncated,
				"cannot_see", report.CannotSee,
				"issues", report.Issues,
			)
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
