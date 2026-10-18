package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/designgate"
	"github.com/multica-ai/multica/server/internal/developmentmemory"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

const (
	designGateRepository = "a9shots0/smcrm"
	designGateSourceSHA  = "1111111111111111111111111111111111111111"
	designGateTaskSHA    = "2222222222222222222222222222222222222222"
	designGateCaptureSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

var designGateCaptureHashes = map[string]string{
	"phone": designGateCaptureSHA, "medium": strings.Repeat("b", 64),
	"desktop_1440": strings.Repeat("c", 64), "desktop_1920": strings.Repeat("d", 64),
}

func TestOrdinaryDesignerClaimAndIntermediateCheckpoint(t *testing.T) {
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "ordinary designer checkpoint runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Application specialist checkpoint")
	dbfx.Exec(t, `UPDATE agent SET instructions='Preserve my product shell.' WHERE id=$1`, agentID)
	dbfx.Exec(t, `UPDATE issue SET status='in_progress' WHERE id=$1`, issueID)
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)
	claimReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "design-checkpoint")
	claimReq = withURLParam(claimReq, "runtimeId", runtimeID)
	var claim struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, claimReq).Want(http.StatusOK).JSON(&claim)
	if claim.Task == nil || claim.Task.Agent == nil || !strings.Contains(claim.Task.Agent.Instructions, "Preserve my product shell.") || !strings.Contains(claim.Task.Agent.Instructions, service.DesignPracticeInstructions) {
		t.Fatal("ordinary specialist did not receive shared design practice alongside its own instructions")
	}
	packet, _ := designGateContextPacket(t)
	binding, err := designgate.BindingFromContext(packet, []string{designGateRepository})
	if err != nil {
		t.Fatal(err)
	}
	if err := developmentmemory.NewStore(testPool).BindDesignDelivery(ctx, taskID, *binding); err != nil {
		t.Fatal(err)
	}
	startReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/start", nil, testWorkspaceID, "design-checkpoint")
	testutil.Call(t, testHandler.StartTask, withURLParam(startReq, "taskId", taskID)).Want(http.StatusOK)
	notReady := false
	checkpoint := designgate.Checkpoint{
		SchemaVersion: 1, ProductID: binding.ProductID, PackSHA256: binding.PackSHA256,
		Stage: "research", Status: "awaiting_client_feedback", ImplementationReady: &notReady,
		Summary: "Page proposal ready.", Question: "Does this describe the right job?", Recommendation: "Prioritize recognizing media and filling the schedule.",
		Artifacts: []designgate.CheckpointArtifact{{Label: "Page proposal", Location: "https://example.test/page-proposal", SHA256: designGateCaptureSHA}},
	}
	complete := func(value designgate.Checkpoint, status int) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		request := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
			"output": "Page proposal ready. Does this describe the right job?\n" + designgate.CheckpointOpenTag + string(raw) + designgate.CheckpointCloseTag,
		}, testWorkspaceID, "design-checkpoint")
		testutil.Call(t, testHandler.CompleteTask, withURLParam(request, "taskId", taskID)).Want(status)
	}
	invalid := checkpoint
	invalid.Status = "approved"
	complete(invalid, http.StatusUnprocessableEntity)
	var taskStatus, issueStatus string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&taskStatus)
	if taskStatus != "running" {
		t.Fatalf("invalid checkpoint changed run to %s", taskStatus)
	}
	complete(checkpoint, http.StatusOK)
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&taskStatus)
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, issueID).Scan(&issueStatus)
	var successors int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id<>$2`, issueID, taskID).Scan(&successors)
	if taskStatus != "completed" || issueStatus != "in_progress" || successors != 0 {
		t.Fatalf("checkpoint advanced or accepted unfinished work: task=%s issue=%s successors=%d", taskStatus, issueStatus, successors)
	}
}

func TestProductUIDesignTaskRequiresBoundPackAndValidatedReceipt(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	setHandlerTestWorkspaceRepos(t, []map[string]string{{"url": "https://github.com/A9shots0/SMCrm.git"}})
	runtimeID := createClaimReclaimRuntime(t, ctx, "Product UI design gate runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Product UI design gate")
	dbfx.Exec(t, `UPDATE agent SET name='Product & UI' WHERE id=$1`, agentID)
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	packet, packSHA := designGateContextPacket(t)
	priorMemory := testHandler.DevelopmentMemory
	priorStore := testHandler.DevelopmentMemoryStore
	testHandler.DevelopmentMemory = &recordingDevelopmentMemory{packet: packet}
	testHandler.DevelopmentMemoryStore = developmentmemory.NewStore(testPool)
	t.Cleanup(func() {
		testHandler.DevelopmentMemory = priorMemory
		testHandler.DevelopmentMemoryStore = priorStore
	})

	claimReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "design-gate")
	claimReq = withURLParam(claimReq, "runtimeId", runtimeID)
	claim := testutil.Call(t, testHandler.ClaimTaskByRuntime, claimReq).Want(http.StatusOK)
	var response struct {
		Task *struct {
			ID             string              `json:"id"`
			DesignDelivery *designgate.Binding `json:"design_delivery"`
		} `json:"task"`
	}
	claim.JSON(&response)
	if response.Task == nil || response.Task.ID != taskID || response.Task.DesignDelivery == nil {
		t.Fatalf("claim did not carry the design binding: %#v", response.Task)
	}
	if response.Task.DesignDelivery.ProductID != "crm" || response.Task.DesignDelivery.PackSHA256 != packSHA {
		t.Fatalf("claim bound the wrong pack: %#v", response.Task.DesignDelivery)
	}

	var stored json.RawMessage
	dbfx.QueryRow(t, `SELECT context -> 'design_delivery_v1' FROM agent_task_queue WHERE id=$1`, taskID).Scan(&stored)
	if !strings.Contains(string(stored), packSHA) || !strings.Contains(string(stored), designGateRepository) {
		t.Fatalf("task did not persist exact pack binding: %s", stored)
	}

	startReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/start", nil, testWorkspaceID, "design-gate")
	startReq = withURLParam(startReq, "taskId", taskID)
	testutil.Call(t, testHandler.StartTask, startReq).Want(http.StatusOK)
	reviews := make([]designReviewRef, 0, 3)
	for _, role := range []string{"product_designer", "data_visualization_designer", "creative_director"} {
		reviewerAgentID := dbfx.Agent(t, "Quality Checker "+role, runtimeID)
		reviewOutput := designGateReviewOutput(t, role)
		var reviewTaskID string
		dbfx.QueryRow(t, `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, parent_task_id,
			started_at, completed_at, result
		)
		VALUES ($1, $2, $3, 'completed', 0, $4, now(), now(), $5::jsonb)
		RETURNING id
	`, reviewerAgentID, runtimeID, issueID, taskID, mustJSON(map[string]any{"output": reviewOutput})).Scan(&reviewTaskID)
		reviews = append(reviews, designReviewRef{Role: role, TaskID: reviewTaskID, AgentID: reviewerAgentID, Output: reviewOutput})
		t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id=$1`, reviewTaskID) })
	}

	rejectedReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": "Mockup ready: http://127.0.0.1:8765/clients",
	}, testWorkspaceID, "design-gate")
	rejectedReq = withURLParam(rejectedReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, rejectedReq).Want(http.StatusUnprocessableEntity)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&status)
	if status != "running" {
		t.Fatalf("rejected design delivery changed task status to %q", status)
	}

	selfReviewReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": designGateValidOutput(t, *response.Task.DesignDelivery, []designReviewRef{
			{Role: "product_designer", TaskID: taskID, AgentID: agentID, Output: designGateReviewOutput(t, "product_designer")},
			{Role: "data_visualization_designer", TaskID: reviews[1].TaskID, AgentID: reviews[1].AgentID, Output: reviews[1].Output},
			{Role: "creative_director", TaskID: reviews[2].TaskID, AgentID: reviews[2].AgentID, Output: reviews[2].Output},
		}),
	}, testWorkspaceID, "design-gate")
	selfReviewReq = withURLParam(selfReviewReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, selfReviewReq).Want(http.StatusUnprocessableEntity)

	missingReviewReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": designGateValidOutput(t, *response.Task.DesignDelivery, []designReviewRef{
			{Role: "product_designer", TaskID: "00000000-0000-7000-8000-000000000991", AgentID: "00000000-0000-7000-8000-000000000992", Output: designGateReviewOutput(t, "product_designer")},
			reviews[1], reviews[2],
		}),
	}, testWorkspaceID, "design-gate")
	missingReviewReq = withURLParam(missingReviewReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, missingReviewReq).Want(http.StatusUnprocessableEntity)

	acceptedReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"output": designGateValidOutput(t, *response.Task.DesignDelivery, reviews),
	}, testWorkspaceID, "design-gate")
	acceptedReq = withURLParam(acceptedReq, "taskId", taskID)
	testutil.Call(t, testHandler.CompleteTask, acceptedReq).Want(http.StatusOK)
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&status)
	if status != "completed" {
		t.Fatalf("validated design delivery status = %q, want completed", status)
	}
}

func TestOrdinaryAgentDoesNotReceiveDesignGate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	setHandlerTestWorkspaceRepos(t, []map[string]string{{"url": "https://github.com/A9shots0/SMCrm.git"}})
	runtimeID := createClaimReclaimRuntime(t, ctx, "Ordinary no design gate runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Backend Engineer")
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	packet, _ := designGateContextPacket(t)
	priorMemory := testHandler.DevelopmentMemory
	priorStore := testHandler.DevelopmentMemoryStore
	testHandler.DevelopmentMemory = &recordingDevelopmentMemory{packet: packet}
	testHandler.DevelopmentMemoryStore = developmentmemory.NewStore(testPool)
	t.Cleanup(func() {
		testHandler.DevelopmentMemory = priorMemory
		testHandler.DevelopmentMemoryStore = priorStore
	})

	claimReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "ordinary-no-design-gate")
	claimReq = withURLParam(claimReq, "runtimeId", runtimeID)
	claim := testutil.Call(t, testHandler.ClaimTaskByRuntime, claimReq).Want(http.StatusOK)
	var response struct {
		Task *struct {
			ID             string              `json:"id"`
			DesignDelivery *designgate.Binding `json:"design_delivery"`
		} `json:"task"`
	}
	claim.JSON(&response)
	if response.Task == nil || response.Task.ID != taskID || response.Task.DesignDelivery != nil {
		t.Fatalf("ordinary claim unexpectedly received design gate: %#v", response.Task)
	}
	var hasBinding bool
	dbfx.QueryRow(t, `SELECT context ? 'design_delivery_v1' FROM agent_task_queue WHERE id=$1`, taskID).Scan(&hasBinding)
	if hasBinding {
		t.Fatal("ordinary task persisted a design-delivery binding")
	}
}

func TestProductUIDesignTaskFailsClosedWhenPackServiceIsUnconfigured(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	setHandlerTestWorkspaceRepos(t, []map[string]string{{"url": "https://github.com/A9shots0/SMCrm.git"}})
	runtimeID := createClaimReclaimRuntime(t, ctx, "Unconfigured design gate runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Unconfigured Product UI")
	dbfx.Exec(t, `UPDATE agent SET name='Product & UI' WHERE id=$1`, agentID)
	taskID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)

	priorMemory := testHandler.DevelopmentMemory
	testHandler.DevelopmentMemory = nil
	t.Cleanup(func() { testHandler.DevelopmentMemory = priorMemory })

	claimReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "unconfigured-design-gate")
	claimReq = withURLParam(claimReq, "runtimeId", runtimeID)
	testutil.Call(t, testHandler.ClaimTaskByRuntime, claimReq).Want(http.StatusServiceUnavailable)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&status)
	if status == "dispatched" || status == "running" || status == "completed" {
		t.Fatalf("ungated Product & UI task remained runnable with status %q", status)
	}
}

func TestProductUIDeliveryRequirementUsesAgentIdentityNotPromptGuessing(t *testing.T) {
	if !productUIDeliveryRequired(&TaskAgentData{Name: "Product & UI"}) {
		t.Fatal("Product & UI agent identity did not enable the design gate")
	}
	if productUIDeliveryRequired(&TaskAgentData{
		Name:         "Backend Engineer",
		Instructions: "Please make a UI mockup and redesign this dashboard.",
	}) {
		t.Fatal("ordinary agent was gated from prompt keywords instead of configured identity")
	}
}

func designGateContextPacket(t *testing.T) (json.RawMessage, string) {
	t.Helper()
	pack := map[string]any{
		"schema_version": 1,
		"product": map[string]any{
			"id": "crm", "repository_identity": designGateRepository, "source_revision": designGateSourceSHA,
		},
		"sources":           []any{map[string]any{"id": "authenticated.shell"}},
		"component_library": []any{map[string]any{"id": "data.table"}},
		"page_patterns":     []any{map[string]any{"id": "operational.index"}},
		"owner_review":      map[string]any{"state": "approved"},
	}
	packRaw, _ := json.Marshal(pack)
	digest := sha256.Sum256(packRaw)
	packSHA := hex.EncodeToString(digest[:])
	packet, _ := json.Marshal(map[string]any{
		"schema_version": 1,
		"design_packs": []any{map[string]any{
			"repository_identity": designGateRepository, "product_id": "crm", "schema_version": 1,
			"source_revision": designGateSourceSHA, "sha256": packSHA, "approval_state": "approved", "pack": pack,
		}},
	})
	return packet, packSHA
}

type designReviewRef struct {
	Role, TaskID, AgentID, Output string
}

func designGateValidOutput(t *testing.T, binding designgate.Binding, reviewRefs []designReviewRef) string {
	t.Helper()
	capture := func(viewport, state string) map[string]any {
		return map[string]any{"viewport": viewport, "data_state": state, "location": "/tmp/" + viewport + ".png", "sha256": designGateCaptureHashes[viewport]}
	}
	reviews := make([]any, 0, len(reviewRefs))
	for _, review := range reviewRefs {
		digest := sha256.Sum256([]byte(review.Output))
		reviews = append(reviews, map[string]any{
			"role": review.Role, "verdict": "ship", "summary": "Independent review passed.", "evidence_ref": review.Role + ".md",
			"review_task_id": review.TaskID, "reviewer_agent_id": review.AgentID, "verdict_sha256": hex.EncodeToString(digest[:]),
		})
	}
	delivery := map[string]any{
		"schema_version": 2, "created_at": "2026-10-03T14:00:00Z",
		"task": map[string]any{
			"id": "crm.clients.redesign", "product_id": "crm", "request_summary": "Improve the clients page",
			"target_surface": "Clients", "represented_role": "operator", "repository_identity": designGateRepository,
			"repository_revision": designGateTaskSHA,
		},
		"design_pack": map[string]any{
			"product_id": "crm", "schema_version": 1, "source_revision": binding.SourceRevision,
			"sha256": binding.PackSHA256, "approval_state": "approved", "validated_without_drift": true,
		},
		"source_review": map[string]any{
			"files_inspected": []any{"src/pages/Clients.tsx"}, "workflow_summary": "Operators compare client health.",
			"decisions_supported": []any{"Which client needs attention?"}, "available_data": []any{"client status"},
			"unavailable_data": []any{}, "permissions_checked": true,
		},
		"reuse": map[string]any{
			"shell_reused": true, "shell_source_ids": []any{"authenticated.shell"}, "page_pattern_ref": "operational.index",
			"component_refs": []any{"data.table"}, "new_components": []any{},
		},
		"evidence": map[string]any{
			"preview_url": "http://127.0.0.1:8765/clients",
			"captures": []any{
				capture("phone", "minimum"), capture("medium", "typical"),
				capture("desktop_1440", "maximum"), capture("desktop_1920", "typical"),
			},
			"quality_checks": map[string]any{
				"alignment": true, "overflow": true, "focus_and_keyboard": true,
				"interaction_states": true, "copy": true, "data_truth": true,
			},
		},
		"reviews":        reviews,
		"owner_delivery": map[string]any{"state": "ready", "summary": "Ready for owner review.", "open_decisions": []any{}},
	}
	raw, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	return "Mockup is ready.\n" + designgate.OpenTag + string(raw) + designgate.CloseTag
}

func designGateReviewOutput(t *testing.T, role string) string {
	t.Helper()
	required := map[string][]string{
		"product_designer":            {"workflow_fit", "information_architecture", "interaction_design", "responsive_composition", "accessibility_usability", "content_clarity", "cognitive_load_progressive_disclosure", "edge_state_coherence"},
		"data_visualization_designer": {"decision_relevance", "data_truth_provenance", "metric_semantics", "comparison_integrity", "visual_encoding", "density_scanability", "missing_zero_unknown", "state_coverage"},
		"creative_director":           {"visual_hierarchy", "brand_product_specificity", "shell_component_fidelity", "composition_balance", "typography_spacing", "color_emphasis_restraint", "polish_consistency", "owner_readiness"},
	}
	criteria := make([]any, 0, len(required[role]))
	for _, id := range required[role] {
		criteria = append(criteria, map[string]any{
			"id": id, "status": "pass", "severity": "none",
			"rationale":     "The inspected screenshot and source provide task-specific evidence.",
			"evidence_refs": []any{designGateCaptureHashes["phone"]},
		})
	}
	raw, err := json.Marshal(map[string]any{
		"schema_version": 1, "role": role, "verdict": "ship", "summary": "Meets the professional release bar.",
		"standards_applied": []any{"WCAG 2.2", "Nielsen usability heuristics"},
		"reviewed_capture_sha256": []any{
			designGateCaptureHashes["phone"], designGateCaptureHashes["medium"],
			designGateCaptureHashes["desktop_1440"], designGateCaptureHashes["desktop_1920"],
		}, "criteria": criteria,
		"absolute_bar": map[string]any{
			"professional_standard_met": true, "not_only_better_than_current": true,
			"task_specific_rationale_present": true, "no_unresolved_high_severity_findings": true,
			"summary": "The design meets product-specific professional criteria on its own merits.",
		},
		"blocking_findings": []any{}, "required_changes": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return designgate.ReviewOpenTag + string(raw) + designgate.ReviewCloseTag
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
