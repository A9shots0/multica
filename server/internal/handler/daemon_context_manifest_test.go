package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/contextmanifest"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestReportTaskContextManifestIsBoundAndIdempotent(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Context manifest runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Context manifest agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 2, "continuity_observation_enabled": true,
	})
	dbfx.Cleanup(t, "DELETE FROM agent_task_context_manifest WHERE task_id=$1", taskID)

	makeBody := func(promptBytes int64) map[string]any {
		t.Helper()
		manifest, err := contextmanifest.New(taskID+":2", "1", 1024, []contextmanifest.Block{{
			Kind: "user_prompt", SourceIdentity: "provider_input:user_prompt", SourceRevision: "sha256:abc",
			ContentBytes: promptBytes, Reason: "Final rendered prompt", MeasurementState: contextmanifest.MeasurementExact,
		}})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := manifest.CanonicalJSON()
		if err != nil {
			t.Fatal(err)
		}
		digest, err := manifest.Digest()
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{"manifest": json.RawMessage(raw), "manifest_sha256": digest}
	}
	call := func(workspace string, body map[string]any) *testutil.Response {
		t.Helper()
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/context-manifest", body, workspace, "context-manifest-daemon")
		return testutil.Call(t, testHandler.ReportTaskContextManifest, withURLParam(req, "taskId", taskID))
	}

	call(testWorkspaceID, makeBody(40)).Want(http.StatusOK)
	call(testWorkspaceID, makeBody(40)).Want(http.StatusOK)
	call(testWorkspaceID, makeBody(41)).Want(http.StatusConflict)
	call("00000000-0000-0000-0000-000000000000", makeBody(40)).Want(http.StatusNotFound)

	rows, err := testHandler.Queries.ListAgentTaskContextManifests(ctx, db.ListAgentTaskContextManifestsParams{
		WorkspaceID: parseUUID(testWorkspaceID), TaskID: parseUUID(taskID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TaskAttempt != 2 || rows[0].ProviderAttempt != 1 || rows[0].AggregateBytes != 40 || rows[0].MeasurementState != "exact" {
		t.Fatalf("stored manifests = %#v", rows)
	}
}

func TestReportTaskContextManifestRejectsAttemptMismatch(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Context manifest mismatch runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Context manifest mismatch agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 1, "continuity_observation_enabled": true,
	})
	manifest, err := contextmanifest.New(taskID+":9", "1", 1024, []contextmanifest.Block{{
		Kind: "user_prompt", SourceIdentity: "provider_input:user_prompt", SourceRevision: "sha256:abc",
		ContentBytes: 1, Reason: "Final rendered prompt", MeasurementState: contextmanifest.MeasurementExact,
	}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := manifest.CanonicalJSON()
	digest, _ := manifest.Digest()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/context-manifest", map[string]any{
		"manifest": json.RawMessage(raw), "manifest_sha256": digest,
	}, testWorkspaceID, "context-manifest-daemon")
	testutil.Call(t, testHandler.ReportTaskContextManifest, withURLParam(req, "taskId", taskID)).Want(http.StatusConflict)
}

func TestContinuityObservationEndpointsRejectUnenrolledTaskWithoutWrites(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Observation disabled runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Observation disabled agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 1,
	})

	for _, endpoint := range []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"context-manifest", testHandler.ReportTaskContextManifest},
		{"usage-samples", testHandler.ReportTaskUsageSample},
		{"loop-observations", testHandler.ReportTaskLoopObservation},
	} {
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/"+endpoint.path, map[string]any{}, testWorkspaceID, "observation-disabled-daemon")
		testutil.Call(t, endpoint.handler, withURLParam(req, "taskId", taskID)).Want(http.StatusConflict)
	}

	var manifests, samples, observations int
	if err := testPool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM agent_task_context_manifest WHERE task_id=$1),
		(SELECT count(*) FROM agent_task_usage_sample WHERE task_id=$1),
		(SELECT count(*) FROM agent_task_loop_observation WHERE task_id=$1)`, taskID).Scan(&manifests, &samples, &observations); err != nil {
		t.Fatal(err)
	}
	if manifests != 0 || samples != 0 || observations != 0 {
		t.Fatalf("unenrolled task wrote observations: manifests=%d samples=%d loops=%d", manifests, samples, observations)
	}
}
