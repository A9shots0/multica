package workflowprojection

import (
	"testing"
	"time"
)

func TestMeaningfulChangeFromRecordStrict(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	record := Record{Type: "review", ID: "review-1", Revision: 1, CreatedAt: now, Payload: []byte(`{"id":"review-1","revision":1,"created_at":"2026-09-10T12:00:00Z","outcome":"passed","candidate_id":"candidate-1","candidate_revision":1,"scope_id":"scope-1","scope_revision":1,"reviewer_id":"reviewer-1","criteria":[],"coverage":{},"evidence_refs":["evidence-1"],"gaps":[],"correction_packet":{},"correction_count":0,"max_corrections":1}`)}
	got, err := MeaningfulChangeFromRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "review_recorded" || got.ResultCode != "succeeded" || got.Reference.ID != "review-1" || got.OccurredAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("change = %#v", got)
	}
	for _, mutate := range []func(*Record){
		func(r *Record) { r.Type = "action_receipt" },
		func(r *Record) { r.Revision = 0 },
		func(r *Record) { r.Payload = []byte(`{"id":"other","revision":1}`) },
		func(r *Record) { r.Payload = []byte(`{`) },
	} {
		bad := record
		mutate(&bad)
		if _, err := MeaningfulChangeFromRecord(bad); err == nil {
			t.Fatalf("accepted malformed record: %#v", bad)
		}
	}
}
