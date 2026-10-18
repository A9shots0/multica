package daemon

import (
	"encoding/json"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTaskGuardedExecutionWireIsSeparateAndOptional(t *testing.T) {
	legacy, err := json.Marshal(Task{ID: "task-1", WorkflowPacket: json.RawMessage(`{"workflow":"packet"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if string(legacy) == "" || jsonContainsKey(t, legacy, "guarded_execution") {
		t.Fatalf("legacy task unexpectedly emitted guarded execution: %s", legacy)
	}

	raw, err := json.Marshal(Task{ID: "task-1", GuardedExecution: &agentqualification.GuardedExecutionPackageV1{Version: agentqualification.GuardedExecutionPackageVersionV1}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded Task
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GuardedExecution == nil || decoded.GuardedExecution.Version != agentqualification.GuardedExecutionPackageVersionV1 {
		t.Fatalf("guarded execution package did not survive claim wire: %s", raw)
	}
}

func TestTaskGuardedExecutionWireRejectsUnknownNullAndDuplicatePackages(t *testing.T) {
	for name, raw := range map[string][]byte{
		"unknown":   []byte(`{"id":"task-1","guarded_execution":{"version":"silver-mediums.guarded-execution-package.v1","unknown":true}}`),
		"null":      []byte(`{"id":"task-1","guarded_execution":null}`),
		"duplicate": []byte(`{"id":"task-1","guarded_execution":{},"guarded_execution":{}}`),
	} {
		t.Run(name, func(t *testing.T) {
			var task Task
			if err := json.Unmarshal(raw, &task); err == nil {
				t.Fatalf("accepted invalid guarded claim wire: %s", raw)
			}
		})
	}
}

func jsonContainsKey(t *testing.T, raw []byte, key string) bool {
	t.Helper()
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	_, ok := values[key]
	return ok
}
