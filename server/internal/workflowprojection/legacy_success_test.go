package workflowprojection

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestLegacyNonLiveSuccessProducesCompleteOwnerPicture(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 30, 0, 0, time.UTC)
	params, err := LegacyNonLiveSuccessParams(
		testProjectionUUID(1), testProjectionUUID(2), testProjectionUUID(3),
		testProjectionUUID(4), testProjectionUUID(5), "independent review passed", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if params.ExpectedRevision != 0 || params.Revision != 1 || !params.ScopeRevision.Valid || params.ScopeRevision.Int64 != 1 ||
		params.OutcomeState != "satisfied" || params.OutcomeOpen ||
		params.ExecutionState != "completed" || params.ReleaseState != "not_required" || params.CurrentMilestone != "complete" {
		t.Fatalf("legacy success params = %#v", params)
	}
	summary := OwnerIssueSummaryFromDocument(params.Document, params.Revision, params.GeneratedAt.Time)
	if summary.Stage != OwnerStageComplete || summary.NextAction != nil || summary.LatestMeaningfulChange == nil ||
		summary.LatestMeaningfulChange.Code != "review_recorded" || summary.LatestMeaningfulChange.ResultCode != "succeeded" {
		t.Fatalf("owner summary = %#v", summary)
	}
	codes := map[string]bool{}
	for _, signal := range summary.Signals {
		codes[signal.Code] = true
	}
	if !codes["outcome_complete"] || !codes["outcome_done"] || len(codes) != 2 {
		t.Fatalf("owner signals = %#v", summary.Signals)
	}
}

func TestLegacyNonLiveSuccessRejectsIncompleteAuthority(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 30, 0, 0, time.UTC)
	valid := []pgtype.UUID{testProjectionUUID(1), testProjectionUUID(2), testProjectionUUID(3), testProjectionUUID(4), testProjectionUUID(5)}
	for i := range valid {
		ids := append([]pgtype.UUID(nil), valid...)
		ids[i] = pgtype.UUID{}
		if _, err := LegacyNonLiveSuccessParams(ids[0], ids[1], ids[2], ids[3], ids[4], "review passed", now); err == nil {
			t.Fatalf("missing identity %d was accepted", i)
		}
	}
	for i := range valid {
		ids := append([]pgtype.UUID(nil), valid...)
		ids[i] = pgtype.UUID{Valid: true}
		if _, err := LegacyNonLiveSuccessParams(ids[0], ids[1], ids[2], ids[3], ids[4], "review passed", now); err == nil {
			t.Fatalf("zero identity %d was accepted", i)
		}
	}
	if _, err := LegacyNonLiveSuccessParams(valid[0], valid[1], valid[2], valid[3], valid[4], " ", now); err == nil {
		t.Fatal("missing review evidence was accepted")
	}
	if _, err := LegacyNonLiveSuccessParams(valid[0], valid[1], valid[2], valid[3], valid[4], "review passed", time.Time{}); err == nil {
		t.Fatal("missing generated time was accepted")
	}
}

func testProjectionUUID(last byte) pgtype.UUID {
	var value [16]byte
	value[6] = 0x40
	value[8] = 0x80
	value[15] = last
	return pgtype.UUID{Bytes: value, Valid: true}
}
