package handler

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTaskToResponseExposesExactWorkflowPacket(t *testing.T) {
	packet := []byte(`{"task_kind":"independent_review","candidate_id":"candidate-1","acceptance_version":3,"implementation_evidence_refs":["evidence-1"],"typed_result_contract":{"type":"review"}}`)
	task := db.AgentTaskQueue{WorkflowActionID: pgtype.UUID{Valid: true}, Context: packet}
	response := taskToResponse(task, "workspace-1")
	var got, want any
	if err := json.Unmarshal(response.WorkflowPacket, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(packet, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workflow packet = %s, want %s", response.WorkflowPacket, packet)
	}

	ordinary := taskToResponse(db.AgentTaskQueue{Context: packet}, "workspace-1")
	if len(ordinary.WorkflowPacket) != 0 {
		t.Fatalf("ordinary task leaked workflow packet: %s", ordinary.WorkflowPacket)
	}
}
