package workflowprojection

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSystemGraphRecordsAreRebuildableAndOwnerMeaningful(t *testing.T) {
	now := time.Date(2026, 9, 14, 13, 0, 0, 0, time.UTC)
	records := []Record{
		{
			Type: "consequence", ID: "10000000-0000-0000-0000-000000000001", Revision: 1, CreatedAt: now,
			Payload: systemGraphRecordJSON(t, map[string]any{
				"id": "10000000-0000-0000-0000-000000000001", "created_at": now.Format(time.RFC3339Nano),
				"source_issue_id": "20000000-0000-0000-0000-000000000001", "affected_issue_id": "20000000-0000-0000-0000-000000000002",
				"receipt_id": "consequence-a", "idempotency_key": "consequence-key", "relationship_kind": "shared_contract_consequence",
				"payload_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "receipt": map[string]any{"version": "consequence_receipt.v1"},
			}),
		},
		{
			Type: "regression_change", ID: "10000000-0000-0000-0000-000000000002", Revision: 4, CreatedAt: now.Add(time.Minute),
			Payload: systemGraphRecordJSON(t, map[string]any{
				"id": "10000000-0000-0000-0000-000000000002", "created_at": now.Add(time.Minute).Format(time.RFC3339Nano),
				"registry_version": 4, "proposal_id": "proposal-retire", "obligation_id": "reg-account-assignment",
				"record_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "record": map[string]any{"registry_version": 4},
			}),
		},
		{
			Type: "regression_rejected", ID: "10000000-0000-0000-0000-000000000003", Revision: 1, CreatedAt: now.Add(2 * time.Minute),
			Payload: systemGraphRecordJSON(t, map[string]any{
				"id": "10000000-0000-0000-0000-000000000003", "created_at": now.Add(2 * time.Minute).Format(time.RFC3339Nano),
				"sequence": 1, "registry_version": 4, "proposal_id": "proposal-conflict", "obligation_id": "reg-account-assignment",
				"submission_hash": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				"attempt_hash":    "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "attempt": map[string]any{"sequence": 1},
			}),
		},
	}
	projection, err := Project(records, 1, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var sources []SourceRevision
	if err := json.Unmarshal(projection.SourceRevisions, &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources) != 3 {
		t.Fatalf("system graph records missing from rebuild inputs: %+v", sources)
	}
	for _, want := range []struct {
		record Record
		code   string
	}{
		{records[0], "cross_issue_consequence_recorded"},
		{records[1], "regression_obligation_changed"},
	} {
		change, err := MeaningfulChangeFromRecord(want.record)
		if err != nil || change.Code != want.code || change.ResultCode != "recorded" {
			t.Fatalf("meaningful change = %+v err=%v, want code %s", change, err, want.code)
		}
	}
	if _, err := MeaningfulChangeFromRecord(records[2]); err == nil {
		t.Fatal("rejected-attempt audit leaked into owner-meaningful activity")
	}
}

func TestSystemGraphRecordEnvelopeMismatchFailsClosed(t *testing.T) {
	now := time.Date(2026, 9, 14, 13, 0, 0, 0, time.UTC)
	record := Record{
		Type: "regression_change", ID: "10000000-0000-0000-0000-000000000002", Revision: 2, CreatedAt: now,
		Payload: systemGraphRecordJSON(t, map[string]any{
			"id": "10000000-0000-0000-0000-000000000002", "created_at": now.Format(time.RFC3339Nano),
			"registry_version": 1, "proposal_id": "proposal", "obligation_id": "obligation",
			"record_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "record": map[string]any{},
		}),
	}
	if _, err := Project([]Record{record}, 1, now); err == nil {
		t.Fatal("projection accepted a regression envelope/payload revision mismatch")
	}
}

func systemGraphRecordJSON(t *testing.T, value map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
