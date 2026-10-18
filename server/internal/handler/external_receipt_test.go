package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/externalreceipt"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

type workforceBenchmarkCase struct {
	id, state, objective, milestone, evidence, blocker, ownerDecision, acceptance string
}

var workforceBenchmarkCases = []workforceBenchmarkCase{
	{"phonefarm-modernization", "working", "Modernize PhoneFarm through evidence-backed bounded stages.", "Obtain a valid supported phone-source measurement after replacement-device admission.", "replacement device admission passed", "Supported source-start endpoint contract refused the request before measurement.", "", "partial"},
	{"deployment-streamlining", "working", "Make concurrent deployment requests serialize, recover and reach production reliably.", "Prove one clean release from request through live system with rollback custody.", "unwanted Guide removed live", "End-to-end clean release proof remained incomplete at observation.", "", "partial"},
	{"brain-reliability", "working", "Current work had evolved into Brain restore and reliability validation.", "Close PhoneFarm coverage gaps after a verified restore.", "8 GB Mac restore passed", "PhoneFarm coverage remained incomplete.", "", "partial"},
	{"tristan-corrected-ui", "working", "After drift, produce a localhost-only A+B preview for owner review.", "Finish and independently verify the preview without production mutation.", "unauthorized Guide deployment identified", "Owner UI decision should wait for a verified preview.", "Select or revise the preview after it is ready.", "local"},
	{"instagram-music", "waiting_external", "Deliver the approved Instagram music behavior.", "Refresh Provider deployment dependency before asking for a cutover decision.", "source merged", "Observed emergency-cutover question may be stale after other deployment activity.", "", "merged"},
	{"backup-codes", "waiting_external", "Deliver approved backup-code management behavior.", "Recompute Provider source/base dependency before requesting owner action.", "UI approved", "Dependency may have changed since the rejection.", "", "merged"},
	{"discord-close", "complete", "Close the frozen Discord application completely.", "Done.", "process and Flatpak absence verified", "", "", "production_verified"},
	{"new-post-sizing", "complete", "Make My Posts fill wide workspaces and deploy only the UI fix.", "Done.", "production width and overflow verified", "", "", "production_verified"},
	{"phonefarm-outage-restoration", "blocked", "Restore employee-facing availability, then install the permanent Sidecar fix.", "Permanent activation remained blocked after temporary API restoration.", "targeted Sidecar restart restored authenticated APIs", "Permanent cutover blocked by release-guard compatibility at observation.", "", "merged"},
	{"storage-recovery", "blocked", "Restore services, exceed the storage reserve and make automatic cleanup keep up.", "Recover the reserve after concurrent builds consumed the prior cushion.", "services restored", "Free space later fell below the reserve while builds held the cleaner lock.", "", "partial"},
	{"emergency-deploy-reliability", "blocked", "Make deployment reliable and fast enough for owner-directed releases.", "Diagnose final service-handoff timeout after automatic rollback.", "controller rolled back to previous release", "New UI and Sidecar changes were not live; employee workflow remained unverified.", "", "partial"},
	{"juju-media", "waiting_owner", "Restore Juju's policy-authorized PhoneFarm gallery media in production.", "Free verified recoverable build storage, then submit one Sidecar-only release.", "38-item signed-in route replay passed", "Builder was 5.525 GiB short of required storage.", "Authorize at least 5.6 GiB of recoverable non-media build-artifact cleanup through the reviewed workflow, if still current.", "merged"},
}

func externalReceiptForCase(item workforceBenchmarkCase) publicapiv1.OutcomeReceiptV02 {
	stage := item.acceptance
	status := "partial"
	criteria := []string{item.milestone}
	if item.state == "complete" {
		status = "accepted"
		criteria = []string{}
	}
	if stage == "partial" {
		stage = "local"
	}
	dependencies := []string{}
	if item.blocker != "" {
		dependencies = append(dependencies, item.blocker)
	}
	decision := publicapiv1.OutcomeReceiptOwnerDecision{Options: []string{}, InvalidatedBy: []string{}}
	if item.ownerDecision != "" {
		category, why := "business", "The reported work cannot safely choose the business tradeoff."
		decision = publicapiv1.OutcomeReceiptOwnerDecision{
			Required: true, Category: &category, Question: &item.ownerDecision,
			Options: []string{"approve the bounded action", "keep it paused"}, WhyNow: &why,
			InvalidatedBy: []string{"dependency or scope changes"},
		}
	}
	return publicapiv1.OutcomeReceiptV02{
		WorkID: item.id, RunID: item.id + "-run-1", ReceiptRevision: 1, Provider: "codex",
		Objective: item.objective, Scope: "The named benchmark issue only.", RequestedMode: "monitor",
		Authority: publicapiv1.OutcomeReceiptAuthority{Reference: "observation-only"}, State: item.state,
		CurrentMilestone: item.milestone, ChangeSinceLastReceipt: item.evidence,
		Evidence:     []publicapiv1.OutcomeReceiptEvidence{{Type: "artifact", Reference: item.evidence}},
		Dependencies: dependencies, NextAction: item.milestone, OwnerDecision: decision, Risk: "medium",
		ResourceUsage: publicapiv1.OutcomeReceiptResourceUsage{Source: "unavailable"},
		Effect:        publicapiv1.OutcomeReceiptEffect{UserOutcome: "unchanged"},
		Freshness:     publicapiv1.OutcomeReceiptFreshness{ObservedAt: "2026-10-14T21:55:00Z"},
		Acceptance:    publicapiv1.OutcomeReceiptAcceptance{Stage: stage, Status: status, CriteriaRemaining: criteria},
	}
}

func seedExternalReceiptIssue(t *testing.T, title string) (string, int64) {
	t.Helper()
	issueID := dbfx.Issue(t, title)
	dbfx.Cleanup(t, `DELETE FROM issue_current_picture WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_external_work_receipt WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_evidence WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_execution WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_scope WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM inbox_item WHERE issue_id=$1`, issueID)
	dbfx.Insert(t, "issue_workflow_scope", map[string]any{
		"id":           uuid.NewString(),
		"workspace_id": testWorkspaceID, "issue_id": issueID, "revision": 1,
		"acceptance_version": 1, "state": "accepted", "owner_evidence_id": testUserID,
		"created_by": testUserID, "statement": []byte(`{"boundary":"benchmark issue only"}`),
	})
	var revision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision)
	return issueID, revision
}

func seedExternalReceiptIssueWithoutScope(t *testing.T, title string) (string, int64) {
	t.Helper()
	issueID := dbfx.Issue(t, title)
	dbfx.Cleanup(t, `DELETE FROM issue_current_picture WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_external_work_receipt WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_evidence WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_execution WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_scope WHERE issue_id=$1`, issueID)
	dbfx.Cleanup(t, `DELETE FROM inbox_item WHERE issue_id=$1`, issueID)
	var revision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision)
	return issueID, revision
}

func TestExternalReceiptBootstrapsObservationOnlyScopeFromOwnerCreatedIssue(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID, issueRevision := seedExternalReceiptIssueWithoutScope(t, "Outside work owner-created scope bootstrap")
	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	result, err := testHandler.ExternalReceiptService.Admit(context.Background(), externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
		IdempotencyKey: "outside-owner-scope-bootstrap",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        receipt,
	})
	if err != nil {
		t.Fatalf("admit owner-created issue receipt: %v", err)
	}
	if result.Replay {
		t.Fatal("first admission reported replay")
	}
	var scopeState, evidenceType, producerType string
	var impact []byte
	dbfx.QueryRow(t, `SELECT state, impact FROM issue_workflow_scope WHERE issue_id=$1`, issueID).Scan(&scopeState, &impact)
	dbfx.QueryRow(t, `SELECT evidence_type, producer_type FROM issue_workflow_evidence WHERE issue_id=$1 AND concern_type='scope'`, issueID).Scan(&evidenceType, &producerType)
	if scopeState != "accepted" || evidenceType != "owner_request" || producerType != "owner" || !bytes.Contains(impact, []byte("complete_company_outcome")) {
		t.Fatalf("scope/evidence = %s/%s/%s impact=%s", scopeState, evidenceType, producerType, impact)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_authority WHERE issue_id=$1`, issueID); got != 0 {
		t.Fatalf("bootstrap created %d authority rows, want 0", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_outcome WHERE issue_id=$1`, issueID); got != 0 {
		t.Fatalf("bootstrap created %d outcome rows, want 0", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_current_picture WHERE issue_id=$1`, issueID); got != 1 {
		t.Fatalf("bootstrap projected %d current pictures, want 1", got)
	}
	var latest []byte
	dbfx.QueryRow(t, `SELECT latest_meaningful_change FROM issue_current_picture WHERE issue_id=$1`, issueID).Scan(&latest)
	var change struct {
		RecordType   string `json:"record_type"`
		ExternalWork struct {
			Provider  string `json:"provider"`
			Milestone string `json:"current_milestone"`
		} `json:"external_work"`
	}
	if err := json.Unmarshal(latest, &change); err != nil {
		t.Fatalf("decode latest meaningful change: %v", err)
	}
	if change.RecordType != "execution" || change.ExternalWork.Provider != receipt.Provider || change.ExternalWork.Milestone != receipt.CurrentMilestone {
		t.Fatalf("latest meaningful change = %+v, want the first external execution", change)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1`, issueID); got != 0 {
		t.Fatalf("routine first receipt created %d Inbox rows, want 0", got)
	}
}

func TestExternalReceiptDoesNotBootstrapScopeFromAgentCreatedIssue(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID, issueRevision := seedExternalReceiptIssueWithoutScope(t, "Outside work agent-created scope refusal")
	dbfx.Exec(t, `UPDATE issue SET creator_type='agent' WHERE id=$1`, issueID)
	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	_, err := testHandler.ExternalReceiptService.Admit(context.Background(), externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
		IdempotencyKey: "outside-agent-scope-refusal",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        receipt,
	})
	if !errors.Is(err, externalreceipt.ErrWorkflowScopeUnavailable) {
		t.Fatalf("agent-created issue error = %v, want workflow scope unavailable", err)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_scope WHERE issue_id=$1`, issueID); got != 0 {
		t.Fatalf("refused bootstrap created %d scope rows", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_external_work_receipt WHERE issue_id=$1`, issueID); got != 0 {
		t.Fatalf("refused bootstrap created %d receipt rows", got)
	}
}

func TestExternalReceiptProjectsOnlyImportantUpdatesIntoOneInboxRow(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID, issueRevision := seedExternalReceiptIssue(t, "Outside work Inbox projection")
	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	request := externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
		IdempotencyKey: "outside-inbox-working",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        receipt,
	}
	if _, err := testHandler.ExternalReceiptService.Admit(context.Background(), request); err != nil {
		t.Fatalf("working receipt: %v", err)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='external_work_update'`, issueID); got != 0 {
		t.Fatalf("routine working receipt created %d Inbox rows, want 0", got)
	}

	prior := int64(1)
	receipt.ReceiptRevision = 2
	receipt.SupersedesReceiptRevision = &prior
	receipt.State = "waiting_external"
	receipt.Dependencies = []string{}
	receipt.ChangeSinceLastReceipt = "The provider reports its work is finished; independent acceptance is still pending."
	request.IdempotencyKey = "outside-inbox-awaiting-verification"
	request.Receipt = receipt
	if _, err := testHandler.ExternalReceiptService.Admit(context.Background(), request); err != nil {
		t.Fatalf("waiting-external receipt: %v", err)
	}
	var rowID, state, severity, category string
	dbfx.QueryRow(t, `SELECT id::text, details->>'reported_state', severity, details->>'owner_category' FROM inbox_item WHERE issue_id=$1 AND type='external_work_update'`, issueID).Scan(&rowID, &state, &severity, &category)
	if state != "waiting_external" || severity != "info" || category != "update" {
		t.Fatalf("waiting-external Inbox state/severity/category = %s/%s/%s, want waiting_external/info/update", state, severity, category)
	}

	prior = 2
	receipt.ReceiptRevision = 3
	receipt.SupersedesReceiptRevision = &prior
	receipt.State = "blocked"
	receipt.Dependencies = []string{"The deployment account is unavailable."}
	receipt.ChangeSinceLastReceipt = "The release could not continue."
	request.IdempotencyKey = "outside-inbox-blocked"
	request.Receipt = receipt
	if _, err := testHandler.ExternalReceiptService.Admit(context.Background(), request); err != nil {
		t.Fatalf("blocked receipt: %v", err)
	}
	var blockedID string
	dbfx.QueryRow(t, `SELECT id::text, details->>'reported_state', severity, details->>'owner_category' FROM inbox_item WHERE issue_id=$1 AND type='external_work_update'`, issueID).Scan(&blockedID, &state, &severity, &category)
	if blockedID != rowID || state != "blocked" || severity != "attention" || category != "incident" {
		t.Fatalf("blocked Inbox id/state/severity/category = %s/%s/%s/%s, want same-row/blocked/attention/incident", blockedID, state, severity, category)
	}
	dbfx.Exec(t, `UPDATE inbox_item SET read=true, archived=true WHERE id=$1`, rowID)

	prior = 3
	receipt.ReceiptRevision = 4
	receipt.SupersedesReceiptRevision = &prior
	receipt.State = "complete"
	receipt.Dependencies = []string{}
	receipt.ChangeSinceLastReceipt = "The importer is merged and its checks passed."
	receipt.OwnerDecision = publicapiv1.OutcomeReceiptOwnerDecision{Options: []string{}, InvalidatedBy: []string{}}
	receipt.Acceptance.Status = "accepted"
	receipt.Acceptance.CriteriaRemaining = []string{}
	request.IdempotencyKey = "outside-inbox-complete"
	request.Receipt = receipt
	if _, err := testHandler.ExternalReceiptService.Admit(context.Background(), request); err != nil {
		t.Fatalf("complete receipt: %v", err)
	}
	var updatedID, updatedState string
	var read, archived bool
	dbfx.QueryRow(t, `SELECT id::text, details->>'reported_state', read, archived FROM inbox_item WHERE issue_id=$1 AND type='external_work_update'`, issueID).Scan(&updatedID, &updatedState, &read, &archived)
	if got := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='external_work_update'`, issueID); got != 1 || updatedID != rowID || updatedState != "complete" || read || archived {
		t.Fatalf("updated Inbox row count/id/state/read/archive = %d/%s/%s/%t/%t", got, updatedID, updatedState, read, archived)
	}

	dbfx.Exec(t, `UPDATE inbox_item SET read=true, archived=true WHERE id=$1`, rowID)
	replayed, err := testHandler.ExternalReceiptService.Admit(context.Background(), request)
	if err != nil || !replayed.Replay {
		t.Fatalf("exact replay = %+v, err=%v", replayed, err)
	}
	dbfx.QueryRow(t, `SELECT read, archived FROM inbox_item WHERE id=$1`, rowID).Scan(&read, &archived)
	if !read || !archived {
		t.Fatalf("exact replay changed owner state: read=%t archived=%t", read, archived)
	}
}

func TestExternalReceiptBenchmarkReconstructsAllTwelveCasesWithoutAuthorityLeaks(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	total, matched, hardFailures := 0, 0, 0
	for _, item := range workforceBenchmarkCases {
		t.Run(item.id, func(t *testing.T) {
			issueID, issueRevision := seedExternalReceiptIssue(t, "External receipt benchmark: "+item.id)
			receipt := externalReceiptForCase(item)
			result, err := testHandler.ExternalReceiptService.Admit(context.Background(), externalreceipt.Request{
				WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
				IdempotencyKey: "benchmark-" + item.id,
				Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
				Receipt:        receipt,
			})
			if err != nil {
				hardFailures++
				t.Fatalf("admit benchmark receipt: %v", err)
			}
			if result.Replay {
				t.Fatal("first admission reported replay")
			}

			var documentRaw []byte
			var pictureRevision int64
			dbfx.QueryRow(t, `SELECT document, revision FROM issue_current_picture WHERE issue_id=$1`, issueID).Scan(&documentRaw, &pictureRevision)
			var document struct {
				Execution struct {
					Checkpoint map[string]any `json:"checkpoint"`
				} `json:"execution"`
				LatestMeaningfulChange struct {
					ExternalWork map[string]any `json:"external_work"`
				} `json:"latest_meaningful_change"`
			}
			if err := json.Unmarshal(documentRaw, &document); err != nil {
				hardFailures++
				t.Fatalf("decode Current Picture: %v", err)
			}
			checkpoint := document.Execution.Checkpoint
			checks := []bool{
				checkpoint["objective"] == item.objective,
				checkpoint["current_milestone"] == item.milestone,
				checkpoint["change_since_last_receipt"] == item.evidence,
				checkpoint["next_action"] == item.milestone,
				len(checkpoint["evidence"].([]any)) == 1,
				len(checkpoint["dependencies"].([]any)) == len(receipt.Dependencies),
				checkpoint["acceptance_report"].(map[string]any)["stage"] == receipt.Acceptance.Stage,
				checkpoint["owner_decision_proposal"].(map[string]any)["required"] == receipt.OwnerDecision.Required,
				document.LatestMeaningfulChange.ExternalWork["provider"] == receipt.Provider,
				document.LatestMeaningfulChange.ExternalWork["change_since_last_receipt"] == receipt.ChangeSinceLastReceipt,
			}
			for _, ok := range checks {
				total++
				if ok {
					matched++
				}
			}

			if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_outcome WHERE issue_id=$1`, issueID); got != 0 {
				hardFailures++
				t.Fatalf("provider receipt created %d native outcome rows", got)
			}
			if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_release WHERE issue_id=$1`, issueID); got != 0 {
				hardFailures++
				t.Fatalf("provider receipt created %d native release rows", got)
			}
			if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_decision WHERE issue_id=$1`, issueID); got != 0 {
				hardFailures++
				t.Fatalf("provider receipt created %d actionable decision rows", got)
			}
			var generatedAt time.Time
			dbfx.QueryRow(t, `SELECT generated_at FROM issue_current_picture WHERE issue_id=$1`, issueID).Scan(&generatedAt)
			summary := workflowprojection.OwnerIssueSummaryFromDocument(documentRaw, pictureRevision, generatedAt)
			if summary.Stage == workflowprojection.OwnerStageComplete {
				hardFailures++
				t.Fatal("provider completion became company completion")
			}
		})
	}
	if hardFailures != 0 {
		t.Fatalf("benchmark hard failures = %d, want 0", hardFailures)
	}
	if total == 0 || float64(matched)/float64(total) < .90 {
		t.Fatalf("benchmark reconstruction = %d/%d, want at least 90%%", matched, total)
	}
}

func TestPluginExternalReceiptRequiresGuardedWriteAndReplaysExactly(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	installationID := installPluginForAction(t, []string{"issues:read", "issues:write"})
	issueID, issueRevision := seedExternalReceiptIssue(t, "Guarded external receipt admission")
	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	attempts, elapsed, active, input, output, cost := int64(2), int64(180), int64(120), int64(1000), int64(200), 0.42
	receipt.ResourceUsage = publicapiv1.OutcomeReceiptResourceUsage{
		Source: "measured", Attempts: &attempts, WallClockSeconds: &elapsed,
		ActiveComputeSeconds: &active, InputTokens: &input, OutputTokens: &output, CostUSD: &cost,
	}

	call := func(key string, body publicapiv1.OutcomeReceiptV02, revision *int64) *httptest.ResponseRecorder {
		req := pluginActionRequest(http.MethodPost, "/v1/issues/"+issueID+"/external-receipts", installationID, body, map[string]string{"issue_ref": issueID})
		if revision != nil {
			req.Header.Set("If-Match", fmt.Sprintf(`W/"%d"`, *revision))
		}
		if key != "" {
			req.Header.Set(publicapiv1.HeaderIdempotencyKey, key)
		}
		w := httptest.NewRecorder()
		testHandler.CreatePluginExternalReceipt(w, req)
		return w
	}

	if got := call("missing-precondition", receipt, nil); got.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing If-Match status=%d body=%s", got.Code, got.Body.String())
	}
	first := call("guarded-key", receipt, &issueRevision)
	if first.Code != http.StatusCreated {
		t.Fatalf("first admission status=%d body=%s", first.Code, first.Body.String())
	}
	second := call("guarded-key", receipt, &issueRevision)
	if second.Code != http.StatusOK {
		t.Fatalf("exact replay status=%d body=%s", second.Code, second.Body.String())
	}
	var replay publicapiv1.ExternalReceiptAdmissionResponse
	if err := json.Unmarshal(second.Body.Bytes(), &replay); err != nil || !replay.Replay {
		t.Fatalf("replay response = %+v, err=%v", replay, err)
	}
	var updates externalWorkUpdatesResponse
	testutil.Call(t, testHandler.GetIssueExternalWorkUpdates,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/external-work-updates", nil), "id", issueID)).
		Want(http.StatusOK).JSON(&updates)
	if len(updates.Items) != 1 || updates.Items[0].Provider != receipt.Provider || updates.Items[0].Summary != receipt.ChangeSinceLastReceipt ||
		!updates.Items[0].ProviderReportOnly || updates.Items[0].EvidenceTotal != len(receipt.Evidence) ||
		updates.Items[0].ResourceUsage.Source != "measured" || updates.Items[0].ResourceUsage.CostUSD == nil || *updates.Items[0].ResourceUsage.CostUSD != cost {
		t.Fatalf("external work updates = %+v", updates)
	}
	mutated := receipt
	mutated.NextAction = "A different action under the same key"
	conflict := call("guarded-key", mutated, &issueRevision)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("mutated replay status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_external_work_receipt WHERE issue_id=$1`, issueID); got != 1 {
		t.Fatalf("durable audit rows = %d, want 1", got)
	}
	var contractVersion string
	dbfx.QueryRow(t, `SELECT contract_version FROM issue_external_work_receipt WHERE issue_id=$1`, issueID).Scan(&contractVersion)
	if contractVersion != publicapiv1.OutcomeReceiptContractVersion {
		t.Fatalf("stored contract version=%q", contractVersion)
	}

	disabled := *testHandler
	disabled.ExternalReceiptService = nil
	req := pluginActionRequest(http.MethodPost, "/v1/issues/"+issueID+"/external-receipts", installationID, receipt, map[string]string{"issue_ref": issueID})
	req.Header.Set("If-Match", fmt.Sprintf(`W/"%d"`, issueRevision))
	req.Header.Set(publicapiv1.HeaderIdempotencyKey, "disabled-adapter")
	w := httptest.NewRecorder()
	disabled.CreatePluginExternalReceipt(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled adapter status=%d body=%s", w.Code, w.Body.String())
	}

	unknown, _ := json.Marshal(map[string]any{
		"work_id": receipt.WorkID, "run_id": receipt.RunID, "receipt_revision": 1, "provider": receipt.Provider,
		"objective": receipt.Objective, "scope": receipt.Scope, "requested_mode": receipt.RequestedMode,
		"authority": receipt.Authority, "state": receipt.State, "current_milestone": receipt.CurrentMilestone,
		"change_since_last_receipt": receipt.ChangeSinceLastReceipt, "evidence": receipt.Evidence,
		"dependencies": receipt.Dependencies, "next_action": receipt.NextAction, "owner_decision": receipt.OwnerDecision,
		"risk": receipt.Risk, "resource_usage": receipt.ResourceUsage, "effect": receipt.Effect,
		"freshness": receipt.Freshness, "acceptance": receipt.Acceptance,
		"conversation_history": []string{"must not be accepted as hidden ambient input"},
	})
	unknownRequest := pluginActionRequest(http.MethodPost, "/v1/issues/"+issueID+"/external-receipts", installationID, nil, map[string]string{"issue_ref": issueID})
	unknownRequest.Body = io.NopCloser(bytes.NewReader(unknown))
	unknownRequest.Header.Set("If-Match", fmt.Sprintf(`W/"%d"`, issueRevision))
	unknownRequest.Header.Set(publicapiv1.HeaderIdempotencyKey, "unknown-ambient-field")
	unknownResponse := httptest.NewRecorder()
	testHandler.CreatePluginExternalReceipt(unknownResponse, unknownRequest)
	if unknownResponse.Code != http.StatusBadRequest {
		t.Fatalf("unknown ambient field status=%d body=%s", unknownResponse.Code, unknownResponse.Body.String())
	}
}

func TestExternalReceiptSupersessionRevokesStaleAuthorityAndSurvivesRestart(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID, issueRevision := seedExternalReceiptIssue(t, "Receipt authority and recovery")
	var scopeID string
	dbfx.QueryRow(t, `SELECT id FROM issue_workflow_scope WHERE issue_id=$1`, issueID).Scan(&scopeID)
	authorityID := dbfx.Insert(t, "issue_workflow_authority", map[string]any{
		"id": uuid.NewString(), "workspace_id": testWorkspaceID, "issue_id": issueID,
		"revision": 1, "scope_id": scopeID, "scope_revision": 1, "owner_evidence_id": testUserID,
		"state": "active", "envelope": []byte(`{"reference":"native-change-1","allowed_effects":["source write"]}`),
		"created_by": testUserID,
	})
	dbfx.Cleanup(t, `DELETE FROM issue_workflow_authority WHERE id=$1`, authorityID)

	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	receipt.RequestedMode = "change"
	receipt.Authority = publicapiv1.OutcomeReceiptAuthority{Reference: "native-change-1", AllowedEffects: []string{"source write"}}
	request := externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
		IdempotencyKey: "authority-revision-1",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        receipt,
	}
	first, err := testHandler.ExternalReceiptService.Admit(context.Background(), request)
	if err != nil || first.AuthorityClaimState != "matched_native_authority" {
		t.Fatalf("matched authority admission = %+v, err=%v", first, err)
	}

	prior := int64(1)
	receipt.ReceiptRevision = 2
	receipt.SupersedesReceiptRevision = &prior
	receipt.Authority.Reference = "provider-invented-authority"
	receipt.ChangeSinceLastReceipt = "The provider attempted to expand authority."
	symbolicExpiry := "on source revision change"
	receipt.Freshness.ExpiresAt = &symbolicExpiry
	request.IdempotencyKey = "authority-revision-2"
	request.Receipt = receipt
	second, err := testHandler.ExternalReceiptService.Admit(context.Background(), request)
	if err != nil || second.AuthorityClaimState != "unverified" || second.ExecutionRevision != first.ExecutionRevision+1 {
		t.Fatalf("unverified supersession = %+v, err=%v", second, err)
	}
	var executionAuthority *string
	dbfx.QueryRow(t, `SELECT authority_id::text FROM issue_workflow_execution WHERE issue_id=$1`, issueID).Scan(&executionAuthority)
	if executionAuthority != nil {
		t.Fatalf("stale native authority remained attached after unverified supersession: %s", *executionAuthority)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_external_work_receipt WHERE issue_id=$1`, issueID); got != 2 {
		t.Fatalf("receipt revisions = %d, want 2", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_evidence WHERE issue_id=$1 AND valid_until IS NULL`, issueID); got != 2 {
		t.Fatalf("symbolic observation expiry was converted into an unsafe wall-clock grant: null valid_until rows=%d", got)
	}

	restarted := *testHandler.ExternalReceiptService
	restarted.OnProjection = nil
	replayed, err := restarted.Admit(context.Background(), request)
	if err != nil || !replayed.Replay || replayed.ReceiptID != second.ReceiptID || replayed.ExecutionRevision != second.ExecutionRevision {
		t.Fatalf("durable replay after service reconstruction = %+v, err=%v", replayed, err)
	}

	staleRequest := request
	staleRequest.IdempotencyKey = "wrong-issue-revision"
	staleRequest.IssueRevision++
	staleRequest.Receipt.WorkID = "new-work"
	staleRequest.Receipt.RunID = "new-run"
	staleRequest.Receipt.ReceiptRevision = 1
	staleRequest.Receipt.SupersedesReceiptRevision = nil
	if _, err := restarted.Admit(context.Background(), staleRequest); !errors.Is(err, externalreceipt.ErrIssueRevisionConflict) {
		t.Fatalf("stale issue revision error=%v", err)
	}
}

func TestExternalReceiptConcurrentRetryHasOneLogicalEffect(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID, issueRevision := seedExternalReceiptIssue(t, "Concurrent receipt idempotency")
	service := *testHandler.ExternalReceiptService
	service.OnProjection = nil
	request := externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), IssueRevision: issueRevision,
		IdempotencyKey: "concurrent-retry-key",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        externalReceiptForCase(workforceBenchmarkCases[1]),
	}

	const callers = 8
	results := make(chan externalreceipt.Result, callers)
	errorsFound := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Admit(context.Background(), request)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent retry failed: %v", err)
	}
	first, replays := 0, 0
	for result := range results {
		if result.Replay {
			replays++
		} else {
			first++
		}
	}
	if first != 1 || replays != callers-1 {
		t.Fatalf("first admissions=%d replays=%d, want 1/%d", first, replays, callers-1)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_external_work_receipt WHERE issue_id=$1`, issueID); got != 1 {
		t.Fatalf("receipt rows=%d, want 1", got)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM issue_workflow_evidence WHERE issue_id=$1`, issueID); got != 1 {
		t.Fatalf("evidence rows=%d, want 1", got)
	}
	var revision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue_workflow_execution WHERE issue_id=$1`, issueID).Scan(&revision)
	if revision != 1 {
		t.Fatalf("execution revision=%d, want 1", revision)
	}
}

func TestExternalReceiptRejectsFutureAndExpiredObservations(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	service := *testHandler.ExternalReceiptService
	service.Now = func() time.Time { return time.Date(2026, 10, 14, 22, 0, 0, 0, time.UTC) }
	receipt := externalReceiptForCase(workforceBenchmarkCases[0])
	request := externalreceipt.Request{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(uuid.NewString()), IssueRevision: 1,
		IdempotencyKey: "temporal-validation",
		Actor:          externalreceipt.Actor{Type: "member", ID: parseUUID(testUserID), Credential: publicapiv1.CredentialUserOAuth},
		Receipt:        receipt,
	}

	request.Receipt.Freshness.ObservedAt = "2026-10-14T22:05:01Z"
	if _, err := service.Admit(context.Background(), request); !errors.Is(err, externalreceipt.ErrInvalidInput) {
		t.Fatalf("future observation error=%v", err)
	}

	request.Receipt.Freshness.ObservedAt = "2026-10-14T21:55:00Z"
	expired := "2026-10-14T21:59:59Z"
	request.Receipt.Freshness.ExpiresAt = &expired
	if _, err := service.Admit(context.Background(), request); !errors.Is(err, externalreceipt.ErrInvalidInput) {
		t.Fatalf("expired observation error=%v", err)
	}
}
