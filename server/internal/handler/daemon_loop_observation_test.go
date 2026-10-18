package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/loopguard"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestReportTaskLoopObservationIsBoundIdempotentAndHashOnly(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Loop observation runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Loop observation agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 2, "continuity_observation_enabled": true,
	})
	dbfx.Cleanup(t, "DELETE FROM agent_task_loop_observation WHERE task_id=$1", taskID)

	observation := loopObservationFixture(t)
	raw, _ := observation.CanonicalJSON()
	digest, _ := observation.Digest()
	body := map[string]any{
		"task_attempt": 2, "provider_attempt": 1,
		"observation": json.RawMessage(raw), "observation_sha256": digest,
	}
	call := func(workspace string, requestBody map[string]any) *testutil.Response {
		t.Helper()
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/loop-observations", requestBody, workspace, "loop-shadow-daemon")
		return testutil.Call(t, testHandler.ReportTaskLoopObservation, withURLParam(req, "taskId", taskID))
	}

	call(testWorkspaceID, body).Want(http.StatusOK)
	call(testWorkspaceID, body).Want(http.StatusOK)
	call("00000000-0000-0000-0000-000000000000", body).Want(http.StatusNotFound)

	rows, err := testHandler.Queries.ListAgentTaskLoopObservations(ctx, db.ListAgentTaskLoopObservationsParams{
		WorkspaceID: parseUUID(testWorkspaceID), TaskID: parseUUID(taskID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("stored observations = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.TaskAttempt != 2 || row.ProviderAttempt != 1 || row.Provider != "codex" || row.RepeatedCycles != 3 || row.ShadowDecision != loopguard.ShadowWouldCheckpoint {
		t.Fatalf("stored observation = %#v", row)
	}
	if row.InputSha256 == "" || row.OutputSha256 == "" || row.FingerprintSha256 == "" {
		t.Fatalf("hash evidence missing: %#v", row)
	}
}

func TestReportTaskLoopObservationRejectsAttemptMismatch(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Loop mismatch runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Loop mismatch agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 1, "continuity_observation_enabled": true,
	})
	observation := loopObservationFixture(t)
	raw, _ := observation.CanonicalJSON()
	digest, _ := observation.Digest()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/loop-observations", map[string]any{
		"task_attempt": 9, "provider_attempt": 1,
		"observation": json.RawMessage(raw), "observation_sha256": digest,
	}, testWorkspaceID, "loop-shadow-daemon")
	testutil.Call(t, testHandler.ReportTaskLoopObservation, withURLParam(req, "taskId", taskID)).Want(http.StatusConflict)
}

func loopObservationFixture(t *testing.T) loopguard.Observation {
	t.Helper()
	detector := loopguard.NewDetector("codex")
	var observation *loopguard.Observation
	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		detector.ObserveToolUse(id, "read_file", map[string]any{"path": "README.md"})
		observation = detector.ObserveToolResult(id, "", "same output", "completed")
	}
	if observation == nil {
		t.Fatal("fixture did not produce observation")
	}
	return *observation
}
