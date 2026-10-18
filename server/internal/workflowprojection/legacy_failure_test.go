package workflowprojection

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestLegacyTechnicalStopProjectionIsTruthfulAndActionable(t *testing.T) {
	now := time.Date(2026, 9, 14, 21, 0, 0, 0, time.UTC)
	taskID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	projection, err := legacyTechnicalStopProjection(taskID, "turn cap", now)
	if err != nil {
		t.Fatal(err)
	}
	summary := OwnerIssueSummaryFromDocument(projection.Document, projection.Revision, projection.GeneratedAt)
	if summary.Stage != OwnerStageTechnicalStop || summary.LatestMeaningfulChange == nil || summary.LatestMeaningfulChange.ResultCode != "failed" {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.NextAction == nil || summary.NextAction.Kind != "inspect_incident" || summary.NextAction.Automatic {
		t.Fatalf("next action = %#v", summary.NextAction)
	}
}
