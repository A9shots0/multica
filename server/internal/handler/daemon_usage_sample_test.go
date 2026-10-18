package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/usagesample"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestReportTaskUsageSampleIsBoundIdempotentAndKeepsUnknownNull(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Usage sample runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Usage sample agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 2, "continuity_observation_enabled": true,
	})
	dbfx.Cleanup(t, "DELETE FROM agent_task_usage_sample WHERE task_id=$1", taskID)

	makeBody := func(inputTokens int64) map[string]any {
		t.Helper()
		sample, err := usagesample.New(taskID+":2", 1, agent.UsageSample{
			Sequence: 1, BackendAttempt: 1, Provider: "claude", Model: "claude-test", ProviderCalls: 1, SourceEvent: "assistant",
			Usage:                        agent.TokenUsage{InputTokens: inputTokens, OutputTokens: 3},
			ModelContextWindowKind:       agent.ContextMeasurementUnknown,
			ContextTokensMeasurementKind: agent.ContextMeasurementUnknown,
		})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := sample.CanonicalJSON()
		digest, _ := sample.Digest()
		return map[string]any{"sample": json.RawMessage(raw), "sample_sha256": digest}
	}
	call := func(workspace string, body map[string]any) *testutil.Response {
		t.Helper()
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/usage-samples", body, workspace, "usage-sample-daemon")
		return testutil.Call(t, testHandler.ReportTaskUsageSample, withURLParam(req, "taskId", taskID))
	}

	var acknowledgement struct {
		SampleSHA256        string `json:"sample_sha256"`
		ShadowDecision      string `json:"shadow_decision"`
		CheckpointRequested bool   `json:"checkpoint_requested"`
	}
	call(testWorkspaceID, makeBody(10)).Want(http.StatusOK).JSON(&acknowledgement)
	if acknowledgement.SampleSHA256 == "" || acknowledgement.ShadowDecision != "unknown" || acknowledgement.CheckpointRequested {
		t.Fatalf("unexpected shadow acknowledgement: %#v", acknowledgement)
	}
	call(testWorkspaceID, makeBody(10)).Want(http.StatusOK)
	call(testWorkspaceID, makeBody(11)).Want(http.StatusConflict)
	call("00000000-0000-0000-0000-000000000000", makeBody(10)).Want(http.StatusNotFound)

	rows, err := testHandler.Queries.ListAgentTaskUsageSamples(ctx, db.ListAgentTaskUsageSamplesParams{
		WorkspaceID: parseUUID(testWorkspaceID), TaskID: parseUUID(taskID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("stored samples = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.TaskAttempt != 2 || row.ProviderAttempt != 1 || row.BackendAttempt != 1 || row.InputTokens != 10 || row.ShadowDecision != "unknown" {
		t.Fatalf("stored sample = %#v", row)
	}
	if row.ContextTokens.Valid || row.ModelContextWindow.Valid || row.ContextRatioPpm.Valid {
		t.Fatalf("unknown context must remain NULL: %#v", row)
	}
}

func TestReportTaskUsageSampleRequestsCheckpointOnlyForEnrolledExactThreshold(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Usage checkpoint runtime")
	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET provider='codex' WHERE id=$1`, runtimeID); err != nil {
		t.Fatal(err)
	}
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Usage checkpoint agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 1,
		"continuity_observation_enabled": true, "continuation_root_task_id": uuid.NewString(),
		"continuation_step": 1, "continuation_max_packages": 3,
	})
	dbfx.Cleanup(t, "DELETE FROM agent_task_usage_sample WHERE task_id=$1", taskID)
	sample, err := usagesample.New(taskID+":1", 1, agent.UsageSample{
		Sequence: 1, BackendAttempt: 1, Provider: "codex", Model: "gpt-test", ProviderCalls: 1,
		SourceEvent: "thread/tokenUsage/updated", ModelContextWindow: 200_000,
		ModelContextWindowKind: agent.ContextMeasurementExact, ContextTokens: 140_000,
		ContextTokensMeasurementKind: agent.ContextMeasurementExact,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := sample.CanonicalJSON()
	digest, _ := sample.Digest()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/usage-samples", map[string]any{
		"sample": json.RawMessage(raw), "sample_sha256": digest,
	}, testWorkspaceID, "usage-checkpoint-daemon")
	var acknowledgement struct {
		SampleSHA256        string `json:"sample_sha256"`
		ShadowDecision      string `json:"shadow_decision"`
		CheckpointRequested bool   `json:"checkpoint_requested"`
	}
	testutil.Call(t, testHandler.ReportTaskUsageSample, withURLParam(req, "taskId", taskID)).Want(http.StatusOK).JSON(&acknowledgement)
	if acknowledgement.SampleSHA256 != digest || acknowledgement.ShadowDecision != "would_request_checkpoint" || !acknowledgement.CheckpointRequested {
		t.Fatalf("checkpoint acknowledgement = %#v", acknowledgement)
	}

	claudeSample := sample
	claudeSample.Sequence = 2
	claudeSample.Provider = "claude"
	claudeRaw, _ := claudeSample.CanonicalJSON()
	claudeDigest, _ := claudeSample.Digest()
	claudeReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/usage-samples", map[string]any{
		"sample": json.RawMessage(claudeRaw), "sample_sha256": claudeDigest,
	}, testWorkspaceID, "usage-checkpoint-daemon")
	acknowledgement = struct {
		SampleSHA256        string `json:"sample_sha256"`
		ShadowDecision      string `json:"shadow_decision"`
		CheckpointRequested bool   `json:"checkpoint_requested"`
	}{}
	testutil.Call(t, testHandler.ReportTaskUsageSample, withURLParam(claudeReq, "taskId", taskID)).Want(http.StatusOK).JSON(&acknowledgement)
	if acknowledgement.ShadowDecision != "would_request_checkpoint" || acknowledgement.CheckpointRequested {
		t.Fatalf("Claude exact sample escaped observation-only mode: %#v", acknowledgement)
	}

	noPlanAgentID, noPlanIssueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Usage checkpoint shadow-only agent")
	noPlanTaskID := dbfx.Task(t, noPlanAgentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": noPlanIssueID, "status": "running", "attempt": 1,
		"continuity_observation_enabled": true,
	})
	dbfx.Cleanup(t, "DELETE FROM agent_task_usage_sample WHERE task_id=$1", noPlanTaskID)
	noPlanSample, err := usagesample.New(noPlanTaskID+":1", 1, agent.UsageSample{
		Sequence: 1, BackendAttempt: 1, Provider: "codex", Model: "gpt-test", ProviderCalls: 1,
		SourceEvent: "thread/tokenUsage/updated", ModelContextWindow: 200_000,
		ModelContextWindowKind: agent.ContextMeasurementExact, ContextTokens: 140_000,
		ContextTokensMeasurementKind: agent.ContextMeasurementExact,
	})
	if err != nil {
		t.Fatal(err)
	}
	noPlanRaw, _ := noPlanSample.CanonicalJSON()
	noPlanDigest, _ := noPlanSample.Digest()
	noPlanReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+noPlanTaskID+"/usage-samples", map[string]any{
		"sample": json.RawMessage(noPlanRaw), "sample_sha256": noPlanDigest,
	}, testWorkspaceID, "usage-checkpoint-daemon")
	acknowledgement = struct {
		SampleSHA256        string `json:"sample_sha256"`
		ShadowDecision      string `json:"shadow_decision"`
		CheckpointRequested bool   `json:"checkpoint_requested"`
	}{}
	testutil.Call(t, testHandler.ReportTaskUsageSample, withURLParam(noPlanReq, "taskId", noPlanTaskID)).Want(http.StatusOK).JSON(&acknowledgement)
	if acknowledgement.ShadowDecision != "would_request_checkpoint" || acknowledgement.CheckpointRequested {
		t.Fatalf("non-enrolled task escaped shadow-only mode: %#v", acknowledgement)
	}
}

func TestReportTaskUsageSampleRejectsTaskAttemptMismatch(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Usage mismatch runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Usage mismatch agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 1, "continuity_observation_enabled": true,
	})
	sample, err := usagesample.New(taskID+":9", 1, agent.UsageSample{
		Sequence: 1, BackendAttempt: 1, Provider: "codex", Model: "test", ProviderCalls: 1, SourceEvent: "usage",
		ModelContextWindowKind:       agent.ContextMeasurementUnknown,
		ContextTokensMeasurementKind: agent.ContextMeasurementUnknown,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := sample.CanonicalJSON()
	digest, _ := sample.Digest()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/usage-samples", map[string]any{
		"sample": json.RawMessage(raw), "sample_sha256": digest,
	}, testWorkspaceID, "usage-sample-daemon")
	testutil.Call(t, testHandler.ReportTaskUsageSample, withURLParam(req, "taskId", taskID)).Want(http.StatusConflict)
}
