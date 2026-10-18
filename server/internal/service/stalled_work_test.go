package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stalledWorkQueryStub struct {
	params db.ListStalledWorkParams
	rows   []db.ListStalledWorkRow
}

func (s *stalledWorkQueryStub) ListStalledWork(_ context.Context, params db.ListStalledWorkParams) ([]db.ListStalledWorkRow, error) {
	s.params = params
	return s.rows, nil
}

func TestStalledWorkReportCarriesCountsQueryAndVisibilityLimits(t *testing.T) {
	now := time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)
	issueID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	workspaceID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	stub := &stalledWorkQueryStub{rows: []db.ListStalledWorkRow{
		{
			IssueID: issueID, WorkspaceID: workspaceID, IssueNumber: 43,
			IssueStatus: "in_review", HasWorkflowPlan: false,
			StalledSince: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}, TotalCount: 2,
		},
	}}
	detector := StalledWorkDetector{
		Queries: stub, Threshold: 15 * time.Minute, Limit: 1,
		Now: func() time.Time { return now },
	}

	report, err := detector.DetectOnce(context.Background())
	if err != nil {
		t.Fatalf("DetectOnce() error = %v", err)
	}
	if report.Query != stalledWorkQueryName || report.Threshold != "15m0s" || !report.Cutoff.Equal(now.Add(-15*time.Minute)) {
		t.Fatalf("report provenance = %+v", report)
	}
	if report.Counts != (StalledWorkCounts{Matched: 2, Returned: 1}) || !report.Truncated {
		t.Fatalf("report counts = %+v truncated=%v", report.Counts, report.Truncated)
	}
	if len(report.CannotSee) != 4 || len(report.Issues) != 1 || report.Issues[0].IssueNumber != 43 ||
		report.Issues[0].HealthState != "stalled" || report.Issues[0].Reason != "workflow_plan_missing" {
		t.Fatalf("report evidence = %+v", report)
	}
	if !stub.params.Cutoff.Valid || !stub.params.Cutoff.Time.Equal(report.Cutoff) || stub.params.RowLimit != 1 {
		t.Fatalf("query params = %+v, want cutoff %s and limit 1", stub.params, report.Cutoff)
	}
}
