package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/developmentmemory"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type recordingDevelopmentMemory struct {
	task   developmentmemory.Task
	packet json.RawMessage
	err    error
}

func (r *recordingDevelopmentMemory) PrepareContext(_ context.Context, task developmentmemory.Task) (json.RawMessage, error) {
	r.task = task
	return r.packet, r.err
}

func TestClaimTaskByRuntime_AttachesServerFetchedDevelopmentMemory(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	setHandlerTestWorkspaceRepos(t, []map[string]string{{"url": "https://github.com/A9shots0/SMCrm.git"}})
	runtimeID := createClaimReclaimRuntime(t, ctx, "Development memory claim runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Development memory claim agent")
	dbfx.Exec(t, `UPDATE issue SET description = $2 WHERE id = $1`, issueID, "Keep accepted frontend and backend decisions aligned.")
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	recorder := &recordingDevelopmentMemory{packet: json.RawMessage(`{"schema_version":1,"workstreams":[{"id":"memory-1"}]}`)}
	prior := testHandler.DevelopmentMemory
	priorStore := testHandler.DevelopmentMemoryStore
	testHandler.DevelopmentMemory = recorder
	testHandler.DevelopmentMemoryStore = developmentmemory.NewStore(testPool)
	t.Cleanup(func() { testHandler.DevelopmentMemory = prior; testHandler.DevelopmentMemoryStore = priorStore })

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "development-memory-claim")
	req = withURLParam(req, "runtimeId", runtimeID)
	w := testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK)
	var response struct {
		Task *struct {
			ID                 string          `json:"id"`
			DevelopmentContext json.RawMessage `json:"development_context"`
		} `json:"task"`
	}
	w.JSON(&response)
	if response.Task == nil || response.Task.ID != taskID {
		t.Fatalf("claimed task = %#v, want %s", response.Task, taskID)
	}
	if !json.Valid(response.Task.DevelopmentContext) || !strings.Contains(string(response.Task.DevelopmentContext), "memory-1") {
		t.Fatalf("development context = %s", response.Task.DevelopmentContext)
	}
	if recorder.task.IssueID != issueID {
		t.Fatalf("prepared identity = %#v", recorder.task)
	}
	if recorder.task.Goal != "Keep accepted frontend and backend decisions aligned." {
		t.Fatalf("prepared goal = %q", recorder.task.Goal)
	}
	if len(recorder.task.Repositories) != 1 || recorder.task.Repositories[0].URL != "https://github.com/A9shots0/SMCrm.git" {
		t.Fatalf("prepared repositories = %#v", recorder.task.Repositories)
	}
	var binding json.RawMessage
	dbfx.QueryRow(t, `SELECT context -> 'development_memory_v1' FROM agent_task_queue WHERE id=$1`, taskID).Scan(&binding)
	if !strings.Contains(string(binding), "a9shots0/smcrm") {
		t.Fatalf("stored binding = %s", binding)
	}
}

func TestClaimTaskByRuntime_RequeuesWhenConfiguredDevelopmentMemoryFails(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	setHandlerTestWorkspaceRepos(t, []map[string]string{{"url": "https://github.com/A9shots0/SMCrm.git"}})
	runtimeID := createClaimReclaimRuntime(t, ctx, "Development memory failure runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Development memory failure agent")
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	prior := testHandler.DevelopmentMemory
	testHandler.DevelopmentMemory = &recordingDevelopmentMemory{err: errors.New("intelligence unavailable")}
	t.Cleanup(func() { testHandler.DevelopmentMemory = prior })

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "development-memory-failure")
	req = withURLParam(req, "runtimeId", runtimeID)
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusServiceUnavailable)

	var status string
	var dispatched bool
	dbfx.QueryRow(t, `SELECT status, dispatched_at IS NOT NULL FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status, &dispatched)
	if status != "queued" || dispatched {
		t.Fatalf("task after memory failure = status %q dispatched=%v, want queued false", status, dispatched)
	}
}

func TestCompleteTaskWithoutDurableMemoryDoesNotCreateProposal(t *testing.T) {
	taskID, _ := claimTaskWithBoundDevelopmentMemory(t, "Development memory completion")
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": "Implemented the verified contract.",
	}, testWorkspaceID, "development-memory-terminal")
	req = withURLParam(req, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, req).Want(http.StatusOK)

	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM development_memory_proposal WHERE task_id=$1`, taskID).Scan(&count)
	if count != 0 {
		t.Fatalf("unstructured task output created %d memory proposals", count)
	}
}

func TestFailTaskWithoutDurableMemoryDoesNotCreateProposal(t *testing.T) {
	taskID, _ := claimTaskWithBoundDevelopmentMemory(t, "Development memory failure")
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/fail", map[string]any{
		"error": "Frontend direction failed owner acceptance.",
	}, testWorkspaceID, "development-memory-terminal")
	req = withURLParam(req, "taskId", taskID)
	testutil.Call(t, testHandler.FailTask, req).Want(http.StatusOK)

	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM development_memory_proposal WHERE task_id=$1`, taskID).Scan(&count)
	if count != 0 {
		t.Fatalf("unstructured failure created %d memory proposals", count)
	}
}

func TestCompleteTaskCreatesPendingProposalForDurableMemory(t *testing.T) {
	taskID, _ := claimTaskWithBoundDevelopmentMemory(t, "Durable development memory")
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": durableMemoryOutput(taskID, "Chat and Inbox are the owner's primary Multica surfaces."),
	}, testWorkspaceID, "development-memory-terminal")
	req = withURLParam(req, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, req).Want(http.StatusOK)

	var reviewStatus, statement string
	dbfx.QueryRow(t, `SELECT review_status, candidate->'memory_items'->0->>'statement' FROM development_memory_proposal WHERE task_id=$1`, taskID).Scan(&reviewStatus, &statement)
	if reviewStatus != "pending" || statement != "Chat and Inbox are the owner's primary Multica surfaces." {
		t.Fatalf("proposal = review %q statement %q", reviewStatus, statement)
	}
}

func TestIssueDevelopmentMemoryRequiresValidHumanReviewBeforeDelivery(t *testing.T) {
	taskID, issueID := claimTaskWithBoundDevelopmentMemory(t, "Development memory review")
	completeReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": durableMemoryOutput(taskID, "CRM owns the user-facing surface."),
	}, testWorkspaceID, "development-memory-terminal")
	completeReq = withURLParam(completeReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, completeReq).Want(http.StatusOK)

	badPayload := map[string]any{
		"summary": "Untrusted repository claim", "next_action": "Do not deliver",
		"remaining_uncertainty": []string{}, "memory_items": []any{},
		"repository_evidence": []map[string]any{{"canonical_identity": "attacker/repo", "revision": nil, "branch": nil, "dirty": nil, "evidence_refs": []string{}}},
	}
	badReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/development-memory/"+taskID+"/approve", map[string]any{"payload": badPayload, "note": "invalid scope"})
	badReq = withURLParams(badReq, "id", issueID, "proposalId", taskID)
	testutil.Call(t, testHandler.ApproveIssueDevelopmentMemory, badReq).Want(http.StatusBadRequest)

	goodPayload := map[string]any{
		"summary": "Keep the accepted contract aligned.", "next_action": "Run owner acceptance.",
		"remaining_uncertainty": []string{"Owner acceptance remains open."},
		"memory_items":          []map[string]any{{"repository_identity": "a9shots0/smcrm", "kind": "decision", "statement": "CRM owns the user-facing surface.", "evidence_refs": []string{"task:" + taskID}}},
		"repository_evidence":   []map[string]any{{"canonical_identity": "a9shots0/smcrm", "revision": nil, "branch": nil, "dirty": nil, "evidence_refs": []string{}}},
	}
	approveReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/development-memory/"+taskID+"/approve", map[string]any{"payload": goodPayload, "note": "reviewed by owner"})
	approveReq = withURLParams(approveReq, "id", issueID, "proposalId", taskID)
	testutil.Call(t, testHandler.ApproveIssueDevelopmentMemory, approveReq).Want(http.StatusOK)

	var reviewStatus string
	var delivered bool
	dbfx.QueryRow(t, `SELECT review_status, delivered_at IS NOT NULL FROM development_memory_proposal WHERE task_id=$1`, taskID).Scan(&reviewStatus, &delivered)
	if reviewStatus != "approved" || delivered {
		t.Fatalf("review status=%q delivered=%v", reviewStatus, delivered)
	}

	dbfx.Exec(t, `UPDATE development_memory_proposal SET attempt_count=24, last_error='temporary outage', dead_lettered_at=now() WHERE task_id=$1`, taskID)
	retryReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/development-memory/"+taskID+"/retry", nil)
	retryReq = withURLParams(retryReq, "id", issueID, "proposalId", taskID)
	testutil.Call(t, testHandler.RetryIssueDevelopmentMemory, retryReq).Want(http.StatusOK)
	var deadLetterCleared, dueNow bool
	var attemptCount int
	dbfx.QueryRow(t, `SELECT dead_lettered_at IS NULL, next_attempt_at <= now(), attempt_count FROM development_memory_proposal WHERE task_id=$1`, taskID).Scan(&deadLetterCleared, &dueNow, &attemptCount)
	if !deadLetterCleared || !dueNow {
		t.Fatalf("retry state = dead letter cleared %v due now %v", deadLetterCleared, dueNow)
	}
	if attemptCount != 0 {
		t.Fatalf("attempt_count=%d, want reset retry budget", attemptCount)
	}
}

func TestIssueDevelopmentMemoryRejectionRequiresAReason(t *testing.T) {
	taskID, issueID := claimTaskWithBoundDevelopmentMemory(t, "Development memory rejection")
	completeReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": durableMemoryOutput(taskID, "Issues are the owner's primary work surface."),
	}, testWorkspaceID, "development-memory-terminal")
	completeReq = withURLParam(completeReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, completeReq).Want(http.StatusOK)

	emptyReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/development-memory/"+taskID+"/reject", map[string]any{"note": ""})
	emptyReq = withURLParams(emptyReq, "id", issueID, "proposalId", taskID)
	testutil.Call(t, testHandler.RejectIssueDevelopmentMemory, emptyReq).Want(http.StatusBadRequest)

	rejectReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/development-memory/"+taskID+"/reject", map[string]any{"note": "This direction failed owner acceptance."})
	rejectReq = withURLParams(rejectReq, "id", issueID, "proposalId", taskID)
	testutil.Call(t, testHandler.RejectIssueDevelopmentMemory, rejectReq).Want(http.StatusOK)

	var reviewStatus string
	var approvedPayloadIsNull bool
	dbfx.QueryRow(t, `SELECT review_status, approved_payload IS NULL FROM development_memory_proposal WHERE task_id=$1`, taskID).Scan(&reviewStatus, &approvedPayloadIsNull)
	if reviewStatus != "rejected" || !approvedPayloadIsNull {
		t.Fatalf("review status=%q approved payload null=%v", reviewStatus, approvedPayloadIsNull)
	}
}

func TestDevelopmentMemoryAttentionListsOnlyWorkspaceAction(t *testing.T) {
	taskID, issueID := claimTaskWithBoundDevelopmentMemory(t, "Development memory attention")
	completeReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": durableMemoryOutput(taskID, "Chat can coordinate work across multiple repositories."),
	}, testWorkspaceID, "development-memory-terminal")
	completeReq = withURLParam(completeReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, completeReq).Want(http.StatusOK)

	listReq := newRequest(http.MethodGet, "/api/development-memory/attention?workspace_id="+testWorkspaceID, nil)
	list := testutil.Call(t, testHandler.ListDevelopmentMemoryAttention, listReq).Want(http.StatusOK)
	var proposals []developmentmemory.Proposal
	list.JSON(&proposals)
	if len(proposals) != 1 || proposals[0].ID != taskID || proposals[0].IssueTitle == nil || strings.TrimSpace(*proposals[0].IssueTitle) == "" {
		t.Fatalf("attention proposals = %#v", proposals)
	}

	rejectReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/development-memory/"+taskID+"/reject", map[string]any{"note": "Not useful for future work."})
	rejectReq = withURLParams(rejectReq, "id", issueID, "proposalId", taskID)
	testutil.Call(t, testHandler.RejectIssueDevelopmentMemory, rejectReq).Want(http.StatusOK)
	list = testutil.Call(t, testHandler.ListDevelopmentMemoryAttention, listReq).Want(http.StatusOK)
	list.JSON(&proposals)
	if len(proposals) != 0 {
		t.Fatalf("reviewed proposal remained in attention: %#v", proposals)
	}
}

func durableMemoryOutput(taskID, statement string) string {
	payload := map[string]any{
		"summary":               "Owner context proposed for future work.",
		"next_action":           "Ask the owner whether to remember it.",
		"remaining_uncertainty": []string{},
		"memory_items": []map[string]any{{
			"repository_identity": "a9shots0/smcrm", "scope": "workstream", "kind": "decision",
			"statement": statement, "evidence_refs": []string{"task:" + taskID},
		}},
		"repository_evidence": []map[string]any{{
			"canonical_identity": "a9shots0/smcrm", "revision": nil, "branch": nil, "dirty": nil,
			"evidence_refs": []string{},
		}},
	}
	raw, _ := json.Marshal(payload)
	return "Done.\n<development-memory>" + string(raw) + "</development-memory>"
}

func claimTaskWithBoundDevelopmentMemory(t *testing.T, name string) (string, string) {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	setHandlerTestWorkspaceRepos(t, []map[string]string{{"url": "https://github.com/A9shots0/SMCrm.git"}})
	runtimeID := createClaimReclaimRuntime(t, ctx, name+" runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, name+" agent")
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	priorMemory := testHandler.DevelopmentMemory
	priorStore := testHandler.DevelopmentMemoryStore
	testHandler.DevelopmentMemory = &recordingDevelopmentMemory{packet: json.RawMessage(`{"schema_version":1,"workstreams":[{"id":"memory-terminal"}]}`)}
	testHandler.DevelopmentMemoryStore = developmentmemory.NewStore(testPool)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM development_memory_proposal WHERE task_id=$1`, taskID)
		testHandler.DevelopmentMemory = priorMemory
		testHandler.DevelopmentMemoryStore = priorStore
	})

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "development-memory-terminal")
	req = withURLParam(req, "runtimeId", runtimeID)
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK)
	startReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/start", nil, testWorkspaceID, "development-memory-terminal")
	startReq = withURLParam(startReq, "taskId", taskID)
	testutil.Call(t, testHandler.StartTask, startReq).Want(http.StatusOK)
	return taskID, issueID
}
