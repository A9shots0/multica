package workflowaction

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestReviewObservedStatusRecognizesEveryDurableLifecycleState(t *testing.T) {
	for _, status := range []string{"queued", "dispatched", "waiting_local_directory", "running", "deferred", "completed", "failed", "cancelled"} {
		if !reviewObservedStatus(status) {
			t.Errorf("status %q was not observable", status)
		}
	}
	for _, status := range []string{"", "invented"} {
		if reviewObservedStatus(status) {
			t.Errorf("status %q was accepted", status)
		}
	}
}

func TestExpectedReviewTaskContextPreservesImmutablePacket(t *testing.T) {
	action := db.IssueWorkflowActionReceipt{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, EffectKey: pgtype.Text{String: "effect/review", Valid: true}, Payload: []byte(`{"candidate_id":"candidate-1","candidate_source_revision":"git:abc","candidate_artifact":{"commit":"abc"},"candidate_build_contract":{"test":"go test"},"scope_goal":{"text":"repair"},"acceptance_version":3,"implementation_evidence_refs":["evidence-1"],"typed_result_contract":{"type":"review"}}`)}
	packet := expectedReviewTaskContext(action, db.GetWorkflowReviewTaskSpecRow{})
	var got map[string]any
	if err := json.Unmarshal(packet, &got); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"workflow_action_id", "effect_key", "task_kind", "candidate_id", "candidate_source_revision", "candidate_artifact", "candidate_build_contract", "scope_goal", "acceptance_version", "implementation_evidence_refs", "typed_result_contract"} {
		if _, ok := got[field]; !ok {
			t.Errorf("workflow packet omitted %q: %s", field, packet)
		}
	}
}
