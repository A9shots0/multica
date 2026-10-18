package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestCompletedCheckpointFinalizationRecoversAtEveryCrashBoundaryWithoutProviderCall(t *testing.T) {
	for _, crashBoundary := range []string{"after_controller_checkpoint", "after_receipt", "after_grade", "after_observation", "before_campaign_record"} {
		t.Run(crashBoundary, func(t *testing.T) {
			state, matrix, campaign, artifactRoot := completedRealisticCLIState(t)
			prepareRealisticFinalizationBoundary(t, crashBoundary, state)

			providerCallsAtCheckpoint := state.Controller.ProviderCalls
			recordCalls := 0
			record := func(_ context.Context, _ string, matrix agentqualification.RealisticRunMatrix, observation agentqualification.RealisticObservation) (agentqualification.RealisticCampaignState, error) {
				recordCalls++
				var err error
				campaign, err = agentqualification.RecordRealisticObservation(campaign, matrix, observation)
				return campaign, err
			}

			first, err := finalizeCompletedRealisticObservation(context.Background(), artifactRoot, state, campaign, matrix, record)
			if err != nil {
				t.Fatal(err)
			}
			second, err := finalizeCompletedRealisticObservation(context.Background(), artifactRoot, state, campaign, matrix, record)
			if err != nil {
				t.Fatal(err)
			}
			if state.Controller.ProviderCalls != providerCallsAtCheckpoint {
				t.Fatalf("post-checkpoint recovery changed provider calls: before=%d after=%d", providerCallsAtCheckpoint, state.Controller.ProviderCalls)
			}
			if recordCalls != 2 || len(campaign.Observations) != 1 || campaign.TotalProviderCalls != providerCallsAtCheckpoint {
				t.Fatalf("finalization was not exactly-once: calls=%d campaign=%+v", recordCalls, campaign)
			}
			if !reflect.DeepEqual(first["observation_path"], second["observation_path"]) {
				t.Fatalf("resume did not reuse sealed observation: first=%v second=%v", first, second)
			}
		})
	}
}

func TestIncompleteEvidenceFinalizationIsSealedFailedConsumedAndRestartable(t *testing.T) {
	state, matrix, campaign, artifactRoot := completedRealisticCLIState(t)
	providerCallsAtCheckpoint := state.Controller.ProviderCalls
	state.Controller.ToolEvents = state.Controller.ToolEvents[1:]
	state.Controller.AttemptHistory[0].ProviderCalls = providerCallsAtCheckpoint

	recordCalls := 0
	record := func(_ context.Context, _ string, matrix agentqualification.RealisticRunMatrix, observation agentqualification.RealisticObservation) (agentqualification.RealisticCampaignState, error) {
		recordCalls++
		var err error
		campaign, err = agentqualification.RecordRealisticObservation(campaign, matrix, observation)
		return campaign, err
	}

	first, err := finalizeCompletedRealisticObservation(context.Background(), artifactRoot, state, campaign, matrix, record)
	if err != nil {
		t.Fatal(err)
	}
	second, err := finalizeCompletedRealisticObservation(context.Background(), artifactRoot, state, campaign, matrix, record)
	if err != nil {
		t.Fatal(err)
	}
	if first["status"] != "candidate_fail" || second["status"] != "candidate_fail" {
		t.Fatalf("incomplete evidence did not remain a candidate failure: first=%v second=%v", first, second)
	}
	if state.Controller.ProviderCalls != providerCallsAtCheckpoint || recordCalls != 2 {
		t.Fatalf("restart changed paid calls or was not idempotently recorded: provider_calls=%d record_calls=%d", state.Controller.ProviderCalls, recordCalls)
	}
	if len(campaign.Observations) != 1 || campaign.Observations[0].Outcome != "candidate_fail" || campaign.Observations[0].FailureClass != "competence_failure" {
		t.Fatalf("incomplete evidence was not recorded as competence failure: %+v", campaign)
	}
	key := state.Candidate.ConfigurationID + ":" + state.Workspace.CaseID
	if campaign.ActiveClaims[key] != "" || campaign.Eliminated[state.Candidate.ConfigurationID] != "R01:competence_failure" {
		t.Fatalf("claim was stranded or candidate not eliminated: %+v", campaign)
	}
	var grade agentqualification.BlindGrade
	if err := readJSONFile(filepath.Join(state.EvidenceRoot, "grade.json"), &grade); err != nil {
		t.Fatal(err)
	}
	if grade.Verdict != "FAIL" || !containsString(grade.FailedChecks, "required_evidence_missing:failing_test_before") {
		t.Fatalf("sealed blind grade did not retain missing evidence: %+v", grade)
	}
	if first["execution_receipt"] != second["execution_receipt"] || first["observation_path"] != second["observation_path"] {
		t.Fatalf("restart did not reuse immutable artifacts: first=%v second=%v", first, second)
	}
}

func prepareRealisticFinalizationBoundary(t *testing.T, boundary string, state realisticRunState) {
	t.Helper()
	if boundary == "after_controller_checkpoint" {
		return
	}
	receipt, receiptPath, err := loadOrSealRealisticExecutionReceipt(state)
	if err != nil {
		t.Fatal(err)
	}
	if boundary == "after_receipt" {
		return
	}
	group, _ := agentqualification.CurrentRealisticWorkflowGroup()
	manifest, _ := agentqualification.CurrentRealisticFixtureManifest()
	grade, err := agentqualification.GradeVerifiedRealisticWorkspace(state.Workspace.CaseID, state.Workspace.Root, state.Workspace.InitialCopy, state.EvidenceRoot, agentqualification.RealisticIndependentEvidence{ToolReceipt: receipt}, group, manifest)
	if err != nil {
		t.Fatal(err)
	}
	gradePath := filepath.Join(state.EvidenceRoot, "grade.json")
	sealedGrade := agentqualification.BlindGrade{SchemaVersion: "silver-mediums.realistic-grade.v1", CaseID: state.Workspace.CaseID, GraderVersion: "realistic-hidden-verifier-v1", Verdict: grade.Verdict, FailedChecks: grade.FailedChecks}
	if err := writeOrVerifyJSONExclusive(gradePath, sealedGrade); err != nil {
		t.Fatal(err)
	}
	if boundary == "after_grade" {
		return
	}
	outcome, failureClass := "pass", ""
	if grade.Verdict != "PASS" {
		outcome, failureClass = "candidate_fail", "competence_failure"
	}
	observation := agentqualification.RealisticObservation{ClaimID: state.ClaimID, ConfigurationID: state.Candidate.ConfigurationID, CaseID: state.Workspace.CaseID, Outcome: outcome, FailureClass: failureClass, ProviderCalls: state.Controller.ProviderCalls, TotalTokens: state.Controller.InputTokens + state.Controller.OutputTokens + state.Controller.ReasoningTokens, CostMicros: state.Controller.CostMicros}
	if _, _, err := agentqualification.SealOrReuseRealisticObservation(state.EvidenceRoot, receiptPath, gradePath, observation, *receipt); err != nil {
		t.Fatal(err)
	}
}

func completedRealisticCLIState(t *testing.T) (realisticRunState, agentqualification.RealisticRunMatrix, agentqualification.RealisticCampaignState, string) {
	t.Helper()
	artifactRoot, _ := filepath.Abs(t.TempDir())
	matrix, err := agentqualification.CurrentRealisticRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	workspaceParent := filepath.Join(artifactRoot, "workspaces")
	if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
		t.Fatal(err)
	}
	workspace, err := agentqualification.PrepareRealisticCaseWorkspace(workspaceParent, "R01")
	if err != nil {
		t.Fatal(err)
	}
	failedOutput, err := workspace.RunCheck("python -m unittest discover -s tests -v")
	if err == nil {
		t.Fatal("defective R01 fixture unexpectedly passed")
	}
	if err := workspace.Edit("consumer_faulty/normalize.py", []byte("def normalize_role(role: str) -> str:\n    return role\n")); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("EVIDENCE.md", []byte("Current local truth: affected and control checks passed; authority unchanged.\n")); err != nil {
		t.Fatal(err)
	}
	passedOutput, err := workspace.RunCheck("python -m unittest discover -s tests -v")
	if err != nil {
		t.Fatal(err)
	}
	rawJSONL := []byte("{\"type\":\"step_start\",\"timestamp\":1000,\"sessionID\":\"ses\",\"part\":{\"sessionID\":\"ses\",\"type\":\"step-start\"}}\n{\"type\":\"text\",\"timestamp\":1100,\"sessionID\":\"ses\",\"part\":{\"sessionID\":\"ses\",\"messageID\":\"msg\",\"type\":\"text\",\"text\":\"{\\\"action\\\":{\\\"kind\\\":\\\"finish\\\",\\\"summary\\\":\\\"Current local truth verified.\\\"}}\"}}\n{\"type\":\"step_finish\",\"timestamp\":1200,\"sessionID\":\"ses\",\"part\":{\"sessionID\":\"ses\",\"messageID\":\"msg\",\"type\":\"step-finish\",\"cost\":0.001,\"tokens\":{\"input\":10,\"output\":4,\"reasoning\":2,\"cache\":{\"read\":3,\"write\":0}}}}")
	rawExport := []byte(`{"info":{"id":"ses"},"messages":[{"info":{"id":"msg","sessionID":"ses","role":"assistant","modelID":"qwen/qwen3-coder-flash","providerID":"openrouter","summary":false,"finish":"stop","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}},"error":null},"parts":[{"id":"text","sessionID":"ses","messageID":"msg","type":"text","text":"{\"action\":{\"kind\":\"finish\",\"summary\":\"Current local truth verified.\"}}","metadata":{"openrouter_metadata":{"endpoints":{"available":[{"provider":"Alibaba","selected":true}]}}}},{"id":"finish","sessionID":"ses","messageID":"msg","type":"step-finish","cost":0.001,"tokens":{"input":10,"output":4,"reasoning":2,"cache":{"read":3,"write":0}}}]}]}`)
	response, err := translateRealisticOpenCodeArtifact(matrix.Candidates[0], fixtureRealisticRouting(matrix.Candidates[0]), rawJSONL, rawExport)
	if err != nil {
		t.Fatal(err)
	}
	finalHash, _ := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	receiptHash := sha256.Sum256(response.RawReceipt)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	failedHash := sha256.Sum256(failedOutput)
	passedHash := sha256.Sum256(passedOutput)
	editHash := sha256.Sum256([]byte("bounded R01 edits"))
	result := agentqualification.RealisticControllerResult{
		CaseID: "R01", RunFamilyID: "resume-finalize-family", Attempt: 1, TerminalState: "completed", FinalSummary: "Current local truth verified.",
		ProviderCalls: 1, ProviderReceipts: [][]byte{response.RawReceipt}, ProviderReceiptHashes: []string{hex.EncodeToString(receiptHash[:])}, InputTokens: response.InputTokens, OutputTokens: response.OutputTokens, ReasoningTokens: response.ReasoningTokens, CostMicros: response.CostMicros,
		ProviderAccountingStatus: response.AccountingStatus, ProviderTokenAccountingStatus: response.TokenAccountingStatus, ProviderCostAccountingStatus: response.CostAccountingStatus,
		ToolEvents: []agentqualification.RealisticToolEvent{
			{Sequence: 1, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 1, StartedAt: now, CompletedAt: now, OutputSHA256: hex.EncodeToString(failedHash[:])},
			{Sequence: 2, Tool: "edit_within_workspace", ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hex.EncodeToString(editHash[:]), TouchedPaths: []string{"consumer_faulty/normalize.py", "EVIDENCE.md"}},
			{Sequence: 3, Tool: "run_allowlisted_local_checks", Arguments: []string{"python", "-m", "unittest", "discover", "-s", "tests", "-v"}, ExitCode: 0, StartedAt: now, CompletedAt: now, OutputSHA256: hex.EncodeToString(passedHash[:])},
		},
	}
	result.AttemptHistory = []agentqualification.RealisticAttemptCheckpoint{{Attempt: 1, TerminalState: "completed", ProviderCalls: 1, TotalTokens: result.InputTokens + result.OutputTokens + result.ReasoningTokens, CostMicros: result.CostMicros, WorkspaceTreeSHA256: finalHash}}
	state := realisticRunState{ClaimID: result.RunFamilyID, Candidate: matrix.Candidates[0], Workspace: workspace, Controller: result, EvidenceRoot: filepath.Join(artifactRoot, "evidence")}
	campaign := agentqualification.NewRealisticCampaignState()
	campaign.ActiveClaims[state.Candidate.ConfigurationID+":"+state.Workspace.CaseID] = state.ClaimID
	return state, matrix, campaign, artifactRoot
}

func TestWriteOrVerifyJSONExclusiveRejectsConflictingCrashArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grade.json")
	if err := writeOrVerifyJSONExclusive(path, map[string]string{"verdict": "PASS"}); err != nil {
		t.Fatal(err)
	}
	if err := writeOrVerifyJSONExclusive(path, map[string]string{"verdict": "FAIL"}); err == nil {
		t.Fatal("conflicting write-once artifact was accepted")
	}
	raw, _ := json.MarshalIndent(map[string]string{"verdict": "PASS"}, "", "  ")
	if existing, _ := os.ReadFile(path); !reflect.DeepEqual(existing, raw) {
		t.Fatal("conflicting recovery changed the original artifact")
	}
}

func TestFinalizeRealisticFailureSealsAllowedZeroCallFailures(t *testing.T) {
	for _, failureClass := range []string{"test_invalidity", "budget_breach", "credential_failure"} {
		t.Run(failureClass, func(t *testing.T) {
			artifactRoot, _ := filepath.Abs(t.TempDir())
			matrix, err := agentqualification.CurrentRealisticRunMatrixByID("realistic-foundation-screen-v13")
			if err != nil {
				t.Fatal(err)
			}
			candidate := matrix.Candidates[0]
			claimID := "zero-call-" + failureClass
			state := realisticRunState{
				ClaimID: claimID, Candidate: candidate,
				Workspace:    agentqualification.RealisticCaseWorkspace{CaseID: "R01"},
				EvidenceRoot: filepath.Join(artifactRoot, "evidence"),
				Controller: agentqualification.RealisticControllerResult{
					CaseID: "R01", RunFamilyID: claimID, TerminalState: "invalid", FailureClass: failureClass,
					ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact",
				},
			}
			campaign := agentqualification.NewRealisticCampaignState()
			campaign.ActiveClaims[candidate.ConfigurationID+":R01"] = claimID
			record := func(_ context.Context, _ string, matrix agentqualification.RealisticRunMatrix, observation agentqualification.RealisticObservation) (agentqualification.RealisticCampaignState, error) {
				var recordErr error
				campaign, recordErr = agentqualification.RecordRealisticObservation(campaign, matrix, observation)
				return campaign, recordErr
			}

			result, err := finalizeRealisticFailureWithRecorder(context.Background(), artifactRoot, state, campaign, matrix, nil, record)
			if err != nil {
				t.Fatal(err)
			}
			wantStatus, wantHalt := "global_halt", failureClass
			if failureClass == "test_invalidity" {
				wantStatus, wantHalt = "invalid", ""
			}
			if result["status"] != wantStatus || campaign.GlobalHaltReason != wantHalt || len(campaign.Observations) != 1 || len(campaign.ActiveClaims) != 0 {
				t.Fatalf("zero-call failure was not classified and consumed: result=%v campaign=%+v", result, campaign)
			}
			sealed := campaign.Observations[0]
			if sealed.ProviderCalls != 0 || sealed.TotalTokens != 0 || sealed.CostMicros != 0 || campaign.TotalProviderCalls != 0 || campaign.TotalTokens != 0 || campaign.TotalCostMicros != 0 {
				t.Fatalf("zero-call halt acquired paid accounting: observation=%+v campaign=%+v", sealed, campaign)
			}
			path, ok := result["failure_evidence"].(string)
			if !ok || path == "" {
				t.Fatalf("failure seal path is missing: %v", result)
			}
			if err := agentqualification.VerifyRealisticPaidFailureSeal(path, sealed); err != nil {
				t.Fatal(err)
			}
			reused, reusedPath, err := agentqualification.SealRealisticFailure(state.EvidenceRoot, agentqualification.RealisticObservation{ClaimID: claimID, ConfigurationID: candidate.ConfigurationID, CaseID: "R01", Outcome: wantStatus, FailureClass: failureClass}, state.Controller)
			if err != nil || reusedPath != path || reused != sealed {
				t.Fatalf("zero-call failure seal was not immutable and reusable: path=%s err=%v observation=%+v", reusedPath, err, reused)
			}
			var receipts []json.RawMessage
			if err := readJSONFile(filepath.Join(filepath.Dir(path), "provider-receipts.json"), &receipts); err != nil || len(receipts) != 0 {
				t.Fatalf("zero-call seal contains provider receipts: receipts=%v err=%v", receipts, err)
			}
		})
	}
}

func TestFinalizeRealisticFailureRejectsZeroCallCandidateFailure(t *testing.T) {
	artifactRoot, _ := filepath.Abs(t.TempDir())
	matrix, _ := agentqualification.CurrentRealisticRunMatrix()
	state := realisticRunState{
		ClaimID: "candidate-zero", Candidate: matrix.Candidates[0], Workspace: agentqualification.RealisticCaseWorkspace{CaseID: "R01"}, EvidenceRoot: filepath.Join(artifactRoot, "evidence"),
		Controller: agentqualification.RealisticControllerResult{CaseID: "R01", RunFamilyID: "candidate-zero", TerminalState: "failed", FailureClass: "response_or_tool_protocol_failure", ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact"},
	}
	called := false
	record := func(context.Context, string, agentqualification.RealisticRunMatrix, agentqualification.RealisticObservation) (agentqualification.RealisticCampaignState, error) {
		called = true
		return agentqualification.RealisticCampaignState{}, nil
	}
	if _, err := finalizeRealisticFailureWithRecorder(context.Background(), artifactRoot, state, agentqualification.NewRealisticCampaignState(), matrix, nil, record); err == nil || called {
		t.Fatalf("zero-call candidate failure reached sealing or campaign record: called=%v err=%v", called, err)
	}
}
