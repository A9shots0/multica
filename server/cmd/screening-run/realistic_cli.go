package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed realistic_cli.go
var realisticCampaignExecutorSource []byte

type realisticRunState struct {
	ClaimID      string                                       `json:"claim_id"`
	Candidate    agentqualification.RealisticCandidate        `json:"candidate"`
	Workspace    agentqualification.RealisticCaseWorkspace    `json:"workspace"`
	Controller   agentqualification.RealisticControllerResult `json:"controller"`
	EvidenceRoot string                                       `json:"evidence_root"`
}

func realisticPlan(artifactRoot string) (map[string]any, error) {
	return realisticPlanForCandidates(artifactRoot, nil)
}

func realisticPlanForCandidates(artifactRoot string, cohort []string) (map[string]any, error) {
	return realisticPlanForMatrix(artifactRoot, "realistic-foundation-screen-v1", cohort)
}

func realisticPlanForMatrix(artifactRoot, matrixID string, cohort []string) (map[string]any, error) {
	matrix, err := agentqualification.CurrentRealisticRunMatrixByID(matrixID)
	if err != nil {
		return nil, err
	}
	matrix, err = boundedRealisticCohort(matrix, cohort)
	if err != nil {
		return nil, err
	}
	state, err := loadRealisticCampaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	next, err := agentqualification.NextRealisticAssignments(state, matrix)
	if err != nil {
		return nil, err
	}
	freezeErr := agentqualification.RequireCompleteRealisticFreeze()
	if freezeErr == nil {
		freezeErr = requireReviewedRealisticMatrix(matrix)
	}
	return map[string]any{"status": "plan_only", "provider_called": false, "freeze_ready": freezeErr == nil, "freeze_error": errorString(freezeErr), "next_assignments": next, "campaign": state}, nil
}

func requireReviewedRealisticMatrix(matrix agentqualification.RealisticRunMatrix) error {
	if matrix.Status != "frozen_reviewed" {
		return fmt.Errorf("realistic provider execution is locked until the selected matrix is independently reviewed and frozen")
	}
	return nil
}

func executeRealisticObservation(ctx context.Context, artifactRoot, candidateID, caseID string, resume bool) (map[string]any, error) {
	return executeRealisticObservationForCandidates(ctx, artifactRoot, candidateID, caseID, resume, nil)
}

func executeRealisticObservationForCandidates(ctx context.Context, artifactRoot, candidateID, caseID string, resume bool, cohort []string) (map[string]any, error) {
	return executeRealisticObservationForMatrix(ctx, artifactRoot, candidateID, caseID, resume, "realistic-foundation-screen-v1", cohort)
}

func executeRealisticObservationForMatrix(ctx context.Context, artifactRoot, candidateID, caseID string, resume bool, matrixID string, cohort []string) (map[string]any, error) {
	requiredExecutorHash, err := agentqualification.RequiredRealisticCampaignExecutorSHA256()
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(realisticCampaignExecutorSource)) != requiredExecutorHash {
		return nil, fmt.Errorf("realistic campaign executor is not the frozen reviewed source")
	}
	if err := agentqualification.RequireCompleteRealisticFreeze(); err != nil {
		return nil, err
	}
	matrix, err := agentqualification.CurrentRealisticRunMatrixByID(matrixID)
	if err != nil {
		return nil, err
	}
	if err := requireReviewedRealisticMatrix(matrix); err != nil {
		return nil, err
	}
	matrix, err = boundedRealisticCohort(matrix, cohort)
	if err != nil {
		return nil, err
	}
	campaign, err := ensureRealisticCampaignBaseline(ctx, artifactRoot)
	if err != nil {
		return nil, err
	}
	observationRoot := filepath.Join(artifactRoot, "observations", safeArtifactName(candidateID), strings.ToLower(caseID))
	statePath := filepath.Join(observationRoot, "run-state.json")
	var state realisticRunState
	if resume {
		if err := readJSONFile(statePath, &state); err != nil {
			return nil, err
		}
		if state.Candidate.ConfigurationID != candidateID || state.Workspace.CaseID != caseID || state.ClaimID == "" {
			return nil, fmt.Errorf("saved realistic run does not match requested assignment")
		}
		if campaign, err = ensureRealisticObservationClaim(artifactRoot, matrix, candidateID, caseID, state.ClaimID); err != nil {
			return nil, err
		}
	} else {
		next, nextErr := agentqualification.NextRealisticAssignments(campaign, matrix)
		if nextErr != nil {
			return nil, nextErr
		}
		var assignment *agentqualification.RealisticAssignment
		for index := range next {
			if next[index].Candidate.ConfigurationID == candidateID && next[index].CaseID == caseID {
				copy := next[index]
				assignment = &copy
			}
		}
		if assignment == nil {
			return nil, fmt.Errorf("requested candidate/case is not in the next frozen campaign wave")
		}
		if _, err := os.Stat(statePath); err == nil {
			return nil, fmt.Errorf("realistic observation already started; use resume")
		}
		runFamilyID, err := newRunFamilyID(candidateID, caseID)
		if err != nil {
			return nil, err
		}
		workspaceParent := filepath.Join(observationRoot, "workspaces")
		if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
			return nil, err
		}
		workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(workspaceParent, caseID, runFamilyID)
		if err != nil {
			return nil, err
		}
		state = realisticRunState{ClaimID: runFamilyID, Candidate: assignment.Candidate, Workspace: workspace, Controller: agentqualification.RealisticControllerResult{CaseID: caseID, RunFamilyID: runFamilyID}, EvidenceRoot: filepath.Join(observationRoot, "evidence")}
		if err := writeJSONExclusive(statePath, state); err != nil {
			return nil, fmt.Errorf("exclusive realistic observation state: %w", err)
		}
		if campaign, err = claimRealisticObservation(artifactRoot, matrix, candidateID, caseID, state.ClaimID); err != nil {
			return nil, err
		}
	}
	provider := openCodeRealisticProvider{candidate: state.Candidate, baselineSpendMicros: campaign.BaselineProviderSpendMicros, maxSpendMicros: matrix.MaxIncrementalSpendMicros}
	browser := agentqualification.FileRealisticBrowserCollector{RunFamilyID: state.Controller.RunFamilyID, ArtifactRoot: state.EvidenceRoot}
	checkpoint := func(controller agentqualification.RealisticControllerResult) error {
		state.Controller = controller
		return writeJSONAtomic(statePath, state)
	}
	if !resume {
		runFamilyID := state.Controller.RunFamilyID
		browser.RunFamilyID = runFamilyID
		state.Controller, err = agentqualification.RunRealisticCandidate(ctx, state.Workspace, runFamilyID, 1, provider, browser, checkpoint)
		if err != nil {
			if errors.Is(err, agentqualification.ErrRealisticCheckpointDurability) {
				return nil, err
			}
			if persistErr := writeJSONAtomic(statePath, state); persistErr != nil {
				return nil, fmt.Errorf("persist paid failure controller before sealing: %w", persistErr)
			}
			return finalizeRealisticFailure(ctx, artifactRoot, state, campaign, matrix, err)
		}
	} else if state.Controller.TerminalState != "completed" {
		browser.RunFamilyID = state.Controller.RunFamilyID
		state.Controller, err = agentqualification.ResumeRealisticCandidate(ctx, state.Workspace, state.Controller, provider, browser, checkpoint)
		if err != nil {
			if errors.Is(err, agentqualification.ErrRealisticCheckpointDurability) {
				return nil, err
			}
			if persistErr := writeJSONAtomic(statePath, state); persistErr != nil {
				return nil, fmt.Errorf("persist paid failure controller before sealing: %w", persistErr)
			}
			return finalizeRealisticFailure(ctx, artifactRoot, state, campaign, matrix, err)
		}
	}
	if err := writeJSONAtomic(statePath, state); err != nil {
		return nil, err
	}
	if state.Controller.TerminalState == "interrupted" || state.Controller.TerminalState == "awaiting_browser_evidence" {
		return map[string]any{"status": state.Controller.TerminalState, "provider_called": state.Controller.ProviderCalls > 0, "automatic_retry": false, "run_state": statePath, "browser_request": filepath.Join(state.EvidenceRoot, "browser-request.json")}, nil
	}
	if state.Controller.TerminalState != "completed" {
		return finalizeRealisticFailure(ctx, artifactRoot, state, campaign, matrix, nil)
	}
	return finalizeCompletedRealisticObservation(ctx, artifactRoot, state, campaign, matrix, recordRealisticObservationAtomic)
}

type realisticObservationRecorder func(context.Context, string, agentqualification.RealisticRunMatrix, agentqualification.RealisticObservation) (agentqualification.RealisticCampaignState, error)

// finalizeCompletedRealisticObservation is deliberately restartable. A durable
// completed controller checkpoint is the paid-work boundary: every later stage
// either creates its write-once artifact or verifies and reuses the exact one
// already present. No provider is consulted here.
func finalizeCompletedRealisticObservation(ctx context.Context, artifactRoot string, state realisticRunState, campaign agentqualification.RealisticCampaignState, matrix agentqualification.RealisticRunMatrix, record realisticObservationRecorder) (map[string]any, error) {
	receipt, receiptPath, err := loadOrSealRealisticExecutionReceipt(state)
	if err != nil {
		return nil, err
	}
	group, err := agentqualification.CurrentRealisticWorkflowGroup()
	if err != nil {
		return nil, err
	}
	manifest, err := agentqualification.CurrentRealisticFixtureManifest()
	if err != nil {
		return nil, err
	}
	grade, err := agentqualification.GradeVerifiedRealisticWorkspace(state.Workspace.CaseID, state.Workspace.Root, state.Workspace.InitialCopy, state.EvidenceRoot, agentqualification.RealisticIndependentEvidence{BrowserProofs: state.Controller.BrowserProofs, ToolReceipt: receipt}, group, manifest)
	if err != nil {
		return nil, err
	}
	gradePath := filepath.Join(state.EvidenceRoot, "grade.json")
	sealedGrade := agentqualification.BlindGrade{SchemaVersion: "silver-mediums.realistic-grade.v1", CaseID: state.Workspace.CaseID, GraderVersion: "realistic-hidden-verifier-v1", Verdict: grade.Verdict, FailedChecks: grade.FailedChecks}
	if err := writeOrVerifyJSONExclusive(gradePath, sealedGrade); err != nil {
		return nil, err
	}
	outcome, failureClass := "pass", ""
	if grade.Verdict != "PASS" {
		outcome, failureClass = "candidate_fail", "competence_failure"
	}
	observation := agentqualification.RealisticObservation{ClaimID: state.ClaimID, ConfigurationID: state.Candidate.ConfigurationID, CaseID: state.Workspace.CaseID, Outcome: outcome, FailureClass: failureClass, ProviderCalls: state.Controller.ProviderCalls, TotalTokens: state.Controller.InputTokens + state.Controller.OutputTokens + state.Controller.ReasoningTokens, CostMicros: state.Controller.CostMicros}
	observation, observationPath, err := agentqualification.SealOrReuseRealisticObservation(state.EvidenceRoot, receiptPath, gradePath, observation, *receipt)
	if err != nil {
		return nil, err
	}
	campaign, err = record(ctx, artifactRoot, matrix, observation)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": outcome, "provider_called": state.Controller.ProviderCalls > 0, "automatic_retry": false, "candidate_id": state.Candidate.ConfigurationID, "case_id": state.Workspace.CaseID, "grade": sealedGrade, "execution_receipt": receiptPath, "grade_path": gradePath, "observation_path": observationPath, "campaign": campaign}, nil
}

func loadOrSealRealisticExecutionReceipt(state realisticRunState) (*agentqualification.RealisticExecutionReceipt, string, error) {
	path := filepath.Join(state.EvidenceRoot, "execution-receipt.json")
	var receipt agentqualification.RealisticExecutionReceipt
	if err := readJSONFile(path, &receipt); err == nil {
		group, groupErr := agentqualification.CurrentRealisticWorkflowGroup()
		if groupErr != nil {
			return nil, "", groupErr
		}
		manifest, manifestErr := agentqualification.CurrentRealisticFixtureManifest()
		if manifestErr != nil {
			return nil, "", manifestErr
		}
		if receipt.ConfigurationID != state.Candidate.ConfigurationID || receipt.CaseID != state.Workspace.CaseID || receipt.RunFamilyID != state.Controller.RunFamilyID || receipt.ProviderCalls != state.Controller.ProviderCalls || receipt.ControllerResultSHA256 == "" {
			return nil, "", fmt.Errorf("persisted realistic execution receipt does not match completed controller checkpoint")
		}
		var sealedController agentqualification.RealisticControllerResult
		controllerPath := filepath.Join(state.EvidenceRoot, filepath.FromSlash(receipt.ControllerResultArtifactPath))
		if err := readJSONFile(controllerPath, &sealedController); err != nil || !reflect.DeepEqual(sealedController, state.Controller) {
			return nil, "", fmt.Errorf("persisted controller result does not match completed run checkpoint")
		}
		if reasons := agentqualification.VerifyRealisticExecutionReceipt(receipt, group, manifest, state.Workspace.InitialCopy, state.Workspace.Root, state.EvidenceRoot); len(reasons) != 0 {
			return nil, "", fmt.Errorf("persisted realistic execution receipt failed recomputation: %v", reasons)
		}
		return &receipt, path, nil
	} else if !os.IsNotExist(err) {
		return nil, "", err
	}
	return agentqualification.SealRealisticExecutionReceipt(state.EvidenceRoot, state.Workspace, state.Candidate, state.Controller)
}

func writeOrVerifyJSONExclusive(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if existing, readErr := os.ReadFile(path); readErr == nil {
		var left, right any
		if json.Unmarshal(existing, &left) != nil || json.Unmarshal(raw, &right) != nil || !reflect.DeepEqual(left, right) {
			return fmt.Errorf("persisted write-once artifact disagrees with recomputed value: %s", path)
		}
		return nil
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	return writeExclusiveFile(path, raw)
}

func finalizeRealisticFailure(ctx context.Context, artifactRoot string, state realisticRunState, campaign agentqualification.RealisticCampaignState, matrix agentqualification.RealisticRunMatrix, executionErr error) (map[string]any, error) {
	return finalizeRealisticFailureWithRecorder(ctx, artifactRoot, state, campaign, matrix, executionErr, recordRealisticObservationAtomic)
}

func finalizeRealisticFailureWithRecorder(ctx context.Context, artifactRoot string, state realisticRunState, campaign agentqualification.RealisticCampaignState, matrix agentqualification.RealisticRunMatrix, executionErr error, record realisticObservationRecorder) (map[string]any, error) {
	controller := state.Controller
	outcome := "candidate_fail"
	failureClass := controller.FailureClass
	if failureClass == "" {
		failureClass = "response_or_tool_protocol_failure"
	}
	if failureClass == "test_invalidity" || failureClass == "provider_transport_failure" {
		outcome = "invalid"
	} else if failureClass == "credential_failure" || containsString(matrix.GlobalHaltReasons, failureClass) {
		outcome = "global_halt"
	}
	zeroCallAllowed := controller.ProviderCalls == 0 && (outcome == "invalid" && failureClass == "test_invalidity" || outcome == "global_halt" && containsString([]string{"test_invalidity", "budget_breach", "credential_failure"}, failureClass))
	if controller.ProviderCalls < 1 && !zeroCallAllowed {
		return nil, fmt.Errorf("candidate failure occurred before a provider call: %w", executionErr)
	}
	observation := agentqualification.RealisticObservation{ClaimID: state.ClaimID, ConfigurationID: state.Candidate.ConfigurationID, CaseID: state.Workspace.CaseID, Outcome: outcome, FailureClass: failureClass, ProviderCalls: controller.ProviderCalls, TotalTokens: controller.InputTokens + controller.OutputTokens + controller.ReasoningTokens, CostMicros: controller.CostMicros}
	observation, observationPath, err := agentqualification.SealRealisticFailure(state.EvidenceRoot, observation, controller)
	if err != nil {
		return nil, fmt.Errorf("seal realistic failure before campaign record: %w", err)
	}
	if observationPath == "" || observation.FailureEvidenceSHA256 == "" || observation.RawProviderReceiptsSHA256 == "" || observation.ControllerResultSHA256 == "" || observation.ProviderAccountingSHA256 == "" {
		return nil, fmt.Errorf("realistic failure seal is incomplete")
	}
	if err := agentqualification.VerifyRealisticPaidFailureSeal(observationPath, observation); err != nil {
		return nil, fmt.Errorf("verify realistic failure seal before campaign record: %w", err)
	}
	next, err := record(ctx, artifactRoot, matrix, observation)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": outcome, "candidate_id": state.Candidate.ConfigurationID, "case_id": state.Workspace.CaseID, "failure_class": failureClass, "error": errorString(executionErr), "failure_evidence": observationPath, "campaign": next}, nil
}

func splitExactCandidateIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

// boundedRealisticCohort narrows only the candidate population for a fresh
// follow-up campaign. It cannot add or alter a configuration, case, grader,
// execution order, retry policy, or budget from the frozen matrix.
func boundedRealisticCohort(matrix agentqualification.RealisticRunMatrix, cohort []string) (agentqualification.RealisticRunMatrix, error) {
	if len(cohort) == 0 {
		return matrix, nil
	}
	wanted := map[string]bool{}
	for _, id := range cohort {
		if id == "" || wanted[id] {
			return matrix, fmt.Errorf("realistic cohort candidate ids must be non-empty and unique")
		}
		wanted[id] = true
	}
	filtered := make([]agentqualification.RealisticCandidate, 0, len(cohort))
	for _, candidate := range matrix.Candidates {
		if wanted[candidate.ConfigurationID] {
			filtered = append(filtered, candidate)
			delete(wanted, candidate.ConfigurationID)
		}
	}
	if len(wanted) != 0 {
		return matrix, fmt.Errorf("realistic cohort contains a candidate outside the frozen matrix")
	}
	matrix.Candidates = filtered
	return matrix, nil
}

func loadRealisticCampaign(root string) (agentqualification.RealisticCampaignState, error) {
	if !filepath.IsAbs(root) {
		return agentqualification.RealisticCampaignState{}, fmt.Errorf("realistic artifact root must be absolute")
	}
	path := filepath.Join(root, "campaign-state.json")
	var state agentqualification.RealisticCampaignState
	if err := readJSONFile(path, &state); err != nil {
		if os.IsNotExist(err) {
			return agentqualification.NewRealisticCampaignState(), nil
		}
		return state, err
	}
	return state, nil
}

func refreshRealisticAuthoritativeSpend(ctx context.Context, state agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error) {
	snapshot, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		return state, err
	}
	state.LatestProviderSpendMicros = snapshot.SpentMicros
	state.AuthoritativeDeltaMicros = snapshot.SpentMicros - state.BaselineProviderSpendMicros
	if state.AuthoritativeDeltaMicros < 0 {
		return state, fmt.Errorf("authoritative OpenRouter spend moved backward")
	}
	return state, nil
}

func ensureRealisticCampaignBaseline(ctx context.Context, root string) (agentqualification.RealisticCampaignState, error) {
	current, err := loadRealisticCampaign(root)
	if err != nil || current.BaselineProviderSpendRecorded {
		return current, err
	}
	snapshot, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		return current, err
	}
	return mutateRealisticCampaign(root, func(state agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error) {
		if !state.BaselineProviderSpendRecorded {
			state.BaselineProviderSpendMicros = snapshot.SpentMicros
			state.LatestProviderSpendMicros = snapshot.SpentMicros
			state.BaselineProviderSpendRecorded = true
		}
		return state, nil
	})
}

func claimRealisticObservation(root string, matrix agentqualification.RealisticRunMatrix, candidateID, caseID, claimID string) (agentqualification.RealisticCampaignState, error) {
	return mutateRealisticCampaign(root, func(state agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error) {
		return agentqualification.ClaimRealisticAssignment(state, matrix, candidateID, caseID, claimID)
	})
}

func ensureRealisticObservationClaim(root string, matrix agentqualification.RealisticRunMatrix, candidateID, caseID, claimID string) (agentqualification.RealisticCampaignState, error) {
	return mutateRealisticCampaign(root, func(state agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error) {
		for _, observation := range state.Observations {
			if observation.ConfigurationID == candidateID && observation.CaseID == caseID {
				if observation.ClaimID != claimID {
					return state, fmt.Errorf("saved realistic observation conflicts with recorded claim")
				}
				return state, nil
			}
		}
		existing := state.ActiveClaims[candidateID+":"+caseID]
		if existing == claimID {
			return state, nil
		}
		if existing != "" {
			return state, fmt.Errorf("saved realistic observation no longer owns the exclusive claim")
		}
		return agentqualification.ClaimRealisticAssignment(state, matrix, candidateID, caseID, claimID)
	})
}

func recordRealisticObservationAtomic(ctx context.Context, root string, matrix agentqualification.RealisticRunMatrix, observation agentqualification.RealisticObservation) (agentqualification.RealisticCampaignState, error) {
	// A crash after the campaign write but before the CLI response must not
	// require another provider-account query or duplicate accounting.
	if existing, err := loadRealisticCampaign(root); err != nil {
		return existing, err
	} else {
		for _, recorded := range existing.Observations {
			if recorded.ConfigurationID == observation.ConfigurationID && recorded.CaseID == observation.CaseID {
				if reflect.DeepEqual(recorded, observation) {
					return existing, nil
				}
				return existing, fmt.Errorf("recorded realistic observation disagrees with sealed artifact")
			}
		}
	}
	snapshot, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		return agentqualification.RealisticCampaignState{}, err
	}
	return mutateRealisticCampaign(root, func(state agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error) {
		state.LatestProviderSpendMicros = snapshot.SpentMicros
		state.AuthoritativeDeltaMicros = snapshot.SpentMicros - state.BaselineProviderSpendMicros
		if state.AuthoritativeDeltaMicros < 0 {
			return state, fmt.Errorf("authoritative OpenRouter spend moved backward")
		}
		next, recordErr := agentqualification.RecordRealisticObservation(state, matrix, observation)
		if recordErr != nil {
			return state, recordErr
		}
		if next.AuthoritativeDeltaMicros > matrix.MaxIncrementalSpendMicros {
			next.GlobalHaltReason = "budget_breach"
		}
		return next, nil
	})
}

func mutateRealisticCampaign(root string, update func(agentqualification.RealisticCampaignState) (agentqualification.RealisticCampaignState, error)) (agentqualification.RealisticCampaignState, error) {
	release, err := acquireRealisticCampaignLock(root)
	if err != nil {
		return agentqualification.RealisticCampaignState{}, err
	}
	defer release()
	state, err := loadRealisticCampaign(root)
	if err != nil {
		return state, err
	}
	expectedRevision := state.Revision
	next, err := update(state)
	if err != nil {
		return state, err
	}
	if next.Revision != expectedRevision {
		return state, fmt.Errorf("realistic campaign CAS revision drifted")
	}
	next.Revision++
	if err := writeJSONAtomic(filepath.Join(root, "campaign-state.json"), next); err != nil {
		return state, err
	}
	return next, nil
}

func inspectRealisticCampaign(root string, inspect func(agentqualification.RealisticCampaignState) error) error {
	release, err := acquireRealisticCampaignLock(root)
	if err != nil {
		return err
	}
	defer release()
	state, err := loadRealisticCampaign(root)
	if err != nil {
		return err
	}
	return inspect(state)
}

func acquireRealisticCampaignLock(root string) (func(), error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("realistic artifact root must be absolute")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(root, "campaign-state.lock")
	deadline := time.Now().Add(3 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, _ = file.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
			_ = file.Sync()
			_ = file.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("realistic campaign update lock timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func writeJSONAtomic(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temp.Write(raw); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func writeJSONExclusive(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeExclusiveFile(path, raw)
}

func writeExclusiveFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(raw); err != nil {
		return err
	}
	return file.Sync()
}

func readJSONFile(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

func newRunFamilyID(candidateID, caseID string) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return safeArtifactName(candidateID) + "-" + strings.ToLower(caseID) + "-" + hex.EncodeToString(raw), nil
}

func safeArtifactName(value string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(value)
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
