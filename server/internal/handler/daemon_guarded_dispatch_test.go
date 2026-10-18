package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestBuildClaimedTaskResponseRejectsGuardedTaskBeforeAgentPayload(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Guarded claim runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Guarded claim agent")
	taskID := seedQueuedIssueTask(t, ctx, agentID, runtimeID, issueID)
	envelope := guardedClaimEnvelope(t, testWorkspaceID, issueID, agentID, runtimeID)
	contextJSON, err := json.Marshal(map[string]any{"guarded_dispatch": envelope})
	if err != nil {
		t.Fatalf("marshal guarded context: %v", err)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET context = $1::jsonb WHERE id = $2`, contextJSON, taskID)

	task, err := testHandler.TaskService.ClaimTaskForRuntime(ctx, parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if task == nil || uuidToString(task.ID) != taskID {
		t.Fatalf("claimed task = %+v, want %s", task, taskID)
	}
	runtime, err := testHandler.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
		ID: parseUUID(runtimeID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil,
		testWorkspaceID, "guarded-claim")
	resp, delivered, agentSkills, builtinSkills, failure := testHandler.buildClaimedTaskResponse(
		req, task, runtime, runtimeID, testWorkspaceID,
	)
	if failure == nil || failure.status != http.StatusConflict || failure.outcome != "error_guarded_dispatch_admission" {
		t.Fatalf("failure = %+v, want guarded-dispatch conflict", failure)
	}
	if resp.Agent != nil || len(resp.PluginHookTools) != 0 || len(resp.RemoteMCPConnections) != 0 ||
		len(delivered) != 0 || agentSkills != 0 || builtinSkills != 0 {
		t.Fatalf("guarded rejection exposed claim payload: resp=%+v delivered=%v skills=%d/%d", resp, delivered, agentSkills, builtinSkills)
	}

	var status, errorMessage, failureReason string
	dbfx.QueryRow(t, `SELECT status, error, failure_reason FROM agent_task_queue WHERE id = $1`, taskID).
		Scan(&status, &errorMessage, &failureReason)
	if status != "failed" || !strings.Contains(errorMessage, "not currently authorized") {
		t.Fatalf("settled task = status %q error %q", status, errorMessage)
	}
	if failureReason != taskfailure.ReasonInvalidTaskIdentity.String() {
		t.Fatalf("failure_reason = %q, want %q", failureReason, taskfailure.ReasonInvalidTaskIdentity)
	}
}

func TestBuildClaimedTaskResponseRejectsGuardedIdentitySubstitutionBeforeAgentPayload(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	mutations := map[string]func(*agentqualification.GuardedDispatchEnvelopeV1){
		"workspace": func(envelope *agentqualification.GuardedDispatchEnvelopeV1) { envelope.WorkspaceID = "other-workspace" },
		"issue":     func(envelope *agentqualification.GuardedDispatchEnvelopeV1) { envelope.IssueID = "other-issue" },
		"agent":     func(envelope *agentqualification.GuardedDispatchEnvelopeV1) { envelope.ExecutorAgentID = "other-agent" },
		"runtime":   func(envelope *agentqualification.GuardedDispatchEnvelopeV1) { envelope.RuntimeID = "other-runtime" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			runtimeID := createClaimReclaimRuntime(t, ctx, "Guarded substitution "+name+" runtime")
			agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Guarded substitution "+name+" agent")
			taskID := seedQueuedIssueTask(t, ctx, agentID, runtimeID, issueID)
			envelope := guardedClaimEnvelope(t, testWorkspaceID, issueID, agentID, runtimeID)
			mutate(&envelope)
			contextJSON, err := json.Marshal(map[string]any{"guarded_dispatch": envelope})
			if err != nil {
				t.Fatalf("marshal guarded context: %v", err)
			}
			dbfx.Exec(t, `UPDATE agent_task_queue SET context = $1::jsonb WHERE id = $2`, contextJSON, taskID)

			task, err := testHandler.TaskService.ClaimTaskForRuntime(ctx, parseUUID(runtimeID))
			if err != nil || task == nil {
				t.Fatalf("claim task = %+v, error = %v", task, err)
			}
			runtime, err := testHandler.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
				ID: parseUUID(runtimeID), WorkspaceID: parseUUID(testWorkspaceID),
			})
			if err != nil {
				t.Fatalf("load runtime: %v", err)
			}
			req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil,
				testWorkspaceID, "guarded-substitution-"+name)
			resp, delivered, agentSkills, builtinSkills, failure := testHandler.buildClaimedTaskResponse(
				req, task, runtime, runtimeID, testWorkspaceID,
			)
			if failure == nil || failure.outcome != "error_guarded_dispatch_admission" {
				t.Fatalf("failure = %+v, want guarded-dispatch denial", failure)
			}
			if resp.Agent != nil || len(resp.PluginHookTools) != 0 || len(resp.RemoteMCPConnections) != 0 ||
				len(delivered) != 0 || agentSkills != 0 || builtinSkills != 0 {
				t.Fatalf("identity substitution exposed payload: resp=%+v delivered=%v skills=%d/%d", resp, delivered, agentSkills, builtinSkills)
			}
		})
	}
}

func TestBuildClaimedTaskResponsePreservesLegacyContext(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Legacy claim runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Legacy claim agent")
	taskID := seedQueuedIssueTask(t, ctx, agentID, runtimeID, issueID)
	dbfx.Exec(t, `UPDATE agent_task_queue SET context = $1::jsonb WHERE id = $2`, `{"head_sha":"abc123"}`, taskID)

	task, err := testHandler.TaskService.ClaimTaskForRuntime(ctx, parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if task == nil || uuidToString(task.ID) != taskID {
		t.Fatalf("claimed task = %+v, want %s", task, taskID)
	}
	runtime, err := testHandler.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
		ID: parseUUID(runtimeID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil,
		testWorkspaceID, "legacy-claim")
	resp, _, _, _, failure := testHandler.buildClaimedTaskResponse(req, task, runtime, runtimeID, testWorkspaceID)
	if failure != nil {
		t.Fatalf("legacy context rejected: %+v", failure)
	}
	if resp.Agent == nil || resp.Agent.ID != agentID {
		t.Fatalf("legacy claim agent = %+v, want %s", resp.Agent, agentID)
	}
}

func guardedClaimEnvelope(t *testing.T, workspaceID, issueID, agentID, runtimeID string) agentqualification.GuardedDispatchEnvelopeV1 {
	t.Helper()
	artifact, err := agentqualification.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatalf("load role artifact: %v", err)
	}
	return agentqualification.GuardedDispatchEnvelopeV1{
		Version: agentqualification.GuardedDispatchEnvelopeVersionV2, ConfigurationID: "future.data-sync.runtime.v1",
		WorkspaceID: workspaceID, IssueID: issueID,
		RoleID: "data_sync_integrity", ExecutorAgentID: agentID, RuntimeID: runtimeID,
		RuntimeProvider: "future-provider", Model: "future-model", ReasoningEffort: "future-effort",
		ArtifactBindingID: artifact.BindingID, ArtifactBinding: artifact, RuntimeBindingReceiptID: "runtime-binding-1",
	}
}
