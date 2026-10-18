package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type r02RepeatCampaignState struct {
	SchemaVersion               string                                 `json:"schema_version"`
	AdmissionID                 string                                 `json:"admission_id"`
	AdmissionSHA256             string                                 `json:"admission_sha256"`
	BaselineProviderSpendMicros int64                                  `json:"baseline_provider_spend_micros"`
	LatestProviderSpendMicros   int64                                  `json:"latest_provider_spend_micros"`
	AuthoritativeDeltaMicros    int64                                  `json:"authoritative_delta_micros"`
	Result                      *agentqualification.R02RepeatRunRecord `json:"result,omitempty"`
}

// executeR02RepeatObservation is intentionally separate from the historical
// realistic campaign executor. Its first operation is the generation-owned
// admission gate; only its bounded in-memory matrix and dedicated ledger can
// reach the paid provider.
func executeR02RepeatObservation(ctx context.Context, artifactRoot string, resume bool) (map[string]any, error) {
	admission, admissionHash, err := agentqualification.RequireActiveR02RepeatAdmission(repositoryRootForR02Repeat())
	if err != nil {
		return nil, err
	}
	matrix, err := agentqualification.R02RepeatBoundedMatrix(admission)
	if err != nil {
		return nil, err
	}
	if len(matrix.Candidates) != 1 || matrix.Candidates[0].ConfigurationID != admission.ConfigurationID {
		return nil, fmt.Errorf("R02 repeat bounded candidate is unavailable")
	}
	if !filepath.IsAbs(artifactRoot) {
		return nil, fmt.Errorf("R02 repeat artifact root must be absolute")
	}
	campaignPath := filepath.Join(artifactRoot, "r02-repeat-campaign.json")
	var campaign r02RepeatCampaignState
	if err := readJSONFile(campaignPath, &campaign); err != nil {
		if !os.IsNotExist(err) || resume {
			return nil, err
		}
		spend, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		campaign = r02RepeatCampaignState{SchemaVersion: "silver-mediums.realistic-r02-repeat-campaign.v1", AdmissionID: admission.AdmissionID, AdmissionSHA256: admissionHash, BaselineProviderSpendMicros: spend.SpentMicros, LatestProviderSpendMicros: spend.SpentMicros}
		if err := writeJSONExclusive(campaignPath, campaign); err != nil {
			return nil, err
		}
	}
	if campaign.AdmissionID != admission.AdmissionID || campaign.AdmissionSHA256 != admissionHash || campaign.Result != nil {
		return nil, fmt.Errorf("R02 repeat campaign admission is stale or already complete")
	}

	observationRoot := filepath.Join(artifactRoot, "observations", safeArtifactName(admission.ConfigurationID), "r02")
	statePath := filepath.Join(observationRoot, "run-state.json")
	var state realisticRunState
	if resume {
		if err := readJSONFile(statePath, &state); err != nil {
			return nil, err
		}
		if state.Candidate != matrix.Candidates[0] || state.Workspace.CaseID != "R02" || state.ClaimID == "" {
			return nil, fmt.Errorf("saved R02 repeat state does not match active admission")
		}
	} else {
		if _, err := os.Stat(statePath); err == nil {
			return nil, fmt.Errorf("R02 repeat already started; use resume")
		}
		runFamilyID, err := newRunFamilyID(admission.ConfigurationID, "R02")
		if err != nil {
			return nil, err
		}
		workspaceParent := filepath.Join(observationRoot, "workspaces")
		if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
			return nil, err
		}
		workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(workspaceParent, "R02", runFamilyID)
		if err != nil {
			return nil, err
		}
		state = realisticRunState{ClaimID: runFamilyID, Candidate: matrix.Candidates[0], Workspace: workspace, Controller: agentqualification.RealisticControllerResult{CaseID: "R02", RunFamilyID: runFamilyID}, EvidenceRoot: filepath.Join(observationRoot, "evidence")}
		if err := writeJSONExclusive(statePath, state); err != nil {
			return nil, err
		}
	}

	provider := openCodeRealisticProvider{candidate: state.Candidate, baselineSpendMicros: campaign.BaselineProviderSpendMicros, maxSpendMicros: admission.MaxSpendMicros}
	browser := agentqualification.FileRealisticBrowserCollector{RunFamilyID: state.Controller.RunFamilyID, ArtifactRoot: state.EvidenceRoot}
	checkpoint := func(controller agentqualification.RealisticControllerResult) error {
		state.Controller = controller
		return writeJSONAtomic(statePath, state)
	}
	if !resume {
		state.Controller, err = agentqualification.RunRealisticCandidate(ctx, state.Workspace, state.Controller.RunFamilyID, 1, provider, browser, checkpoint)
	} else if state.Controller.TerminalState != "completed" {
		state.Controller, err = agentqualification.ResumeRealisticCandidate(ctx, state.Workspace, state.Controller, provider, browser, checkpoint)
	}
	if errors.Is(err, agentqualification.ErrRealisticCheckpointDurability) {
		return nil, err
	}
	if persistErr := writeJSONAtomic(statePath, state); persistErr != nil {
		return nil, persistErr
	}
	if state.Controller.TerminalState == "interrupted" || state.Controller.TerminalState == "awaiting_browser_evidence" {
		return map[string]any{"status": state.Controller.TerminalState, "provider_called": state.Controller.ProviderCalls > 0, "automatic_retry": false, "run_state": statePath, "browser_request": filepath.Join(state.EvidenceRoot, "browser-request.json")}, nil
	}
	record, artifacts, finalizeErr := finalizeR02RepeatObservation(state, admission, admissionHash, err)
	if finalizeErr != nil {
		return nil, finalizeErr
	}
	spend, spendErr := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if spendErr != nil {
		return nil, fmt.Errorf("R02 repeat result sealed but authoritative spend readback failed: %w", spendErr)
	}
	campaign.LatestProviderSpendMicros = spend.SpentMicros
	campaign.AuthoritativeDeltaMicros = spend.SpentMicros - campaign.BaselineProviderSpendMicros
	if campaign.AuthoritativeDeltaMicros < 0 || campaign.AuthoritativeDeltaMicros > admission.MaxSpendMicros {
		record.Outcome, record.FailureClass = "global_halt", "uncontrolled_spending"
		record.Observation.Outcome, record.Observation.FailureClass = record.Outcome, record.FailureClass
	}
	if err := agentqualification.ValidateR02RepeatRunRecord(admission, record, state.Controller); err != nil {
		return nil, err
	}
	campaign.Result = &record
	if err := writeJSONAtomic(campaignPath, campaign); err != nil {
		return nil, err
	}
	artifacts["campaign"] = campaignPath
	artifacts["status"] = record.Outcome
	artifacts["failure_class"] = record.FailureClass
	return artifacts, nil
}

func finalizeR02RepeatObservation(state realisticRunState, admission agentqualification.R02RepeatAdmission, admissionHash string, executionErr error) (agentqualification.R02RepeatRunRecord, map[string]any, error) {
	controller := state.Controller
	observation := agentqualification.RealisticObservation{ClaimID: state.ClaimID, ConfigurationID: admission.ConfigurationID, CaseID: "R02", ProviderCalls: controller.ProviderCalls, TotalTokens: controller.InputTokens + controller.OutputTokens + controller.ReasoningTokens, CostMicros: controller.CostMicros}
	result := agentqualification.R02RepeatRunRecord{SchemaVersion: agentqualification.R02RepeatRunRecordSchemaVersion, AdmissionID: admission.AdmissionID, AdmissionHash: admissionHash}
	artifacts := map[string]any{"provider_called": controller.ProviderCalls > 0, "automatic_retry": false, "candidate_id": admission.ConfigurationID, "case_id": "R02"}
	if controller.TerminalState == "completed" {
		receipt, receiptPath, err := loadOrSealRealisticExecutionReceipt(state)
		if err != nil {
			return result, nil, err
		}
		group, err := agentqualification.CurrentRealisticWorkflowGroup()
		if err != nil {
			return result, nil, err
		}
		manifest, err := agentqualification.CurrentRealisticFixtureManifest()
		if err != nil {
			return result, nil, err
		}
		grade, err := agentqualification.GradeVerifiedRealisticWorkspace("R02", state.Workspace.Root, state.Workspace.InitialCopy, state.EvidenceRoot, agentqualification.RealisticIndependentEvidence{BrowserProofs: controller.BrowserProofs, ToolReceipt: receipt}, group, manifest)
		if err != nil {
			return result, nil, err
		}
		gradePath := filepath.Join(state.EvidenceRoot, "grade.json")
		sealedGrade := agentqualification.BlindGrade{SchemaVersion: "silver-mediums.realistic-grade.v1", CaseID: "R02", GraderVersion: "realistic-hidden-verifier-v1", Verdict: grade.Verdict, FailedChecks: grade.FailedChecks}
		if err := writeOrVerifyJSONExclusive(gradePath, sealedGrade); err != nil {
			return result, nil, err
		}
		observation.Outcome = "pass"
		if grade.Verdict != "PASS" {
			observation.Outcome, observation.FailureClass = "candidate_fail", "competence_failure"
		}
		observation, observationPath, err := agentqualification.SealOrReuseRealisticObservation(state.EvidenceRoot, receiptPath, gradePath, observation, *receipt)
		if err != nil {
			return result, nil, err
		}
		result.Outcome, result.FailureClass, result.Observation = observation.Outcome, observation.FailureClass, observation
		artifacts["execution_receipt"], artifacts["grade_path"], artifacts["observation_path"] = receiptPath, gradePath, observationPath
		return result, artifacts, nil
	}
	outcome, failureClass := agentqualification.ClassifyR02RepeatController(controller, executionErr)
	if observation.TotalTokens > admission.MaxTotalTokens || controller.OutputTokens+controller.ReasoningTokens > admission.MaxGeneratedTokens || observation.CostMicros > admission.MaxSpendMicros || len(controller.ToolEvents) > admission.MaxActions {
		outcome, failureClass = "global_halt", "uncontrolled_spending"
	}
	observation.Outcome, observation.FailureClass = outcome, failureClass
	if controller.ProviderCalls > 0 {
		controller.TerminalState = "invalid"
		controller.FailureClass = failureClass
		sealed, path, err := agentqualification.SealRealisticFailure(state.EvidenceRoot, observation, controller)
		if err != nil {
			return result, nil, err
		}
		observation = sealed
		artifacts["failure_evidence"] = path
	} else {
		if controller.InputTokens != 0 || controller.OutputTokens != 0 || controller.ReasoningTokens != 0 || controller.CostMicros != 0 {
			return result, nil, fmt.Errorf("zero-call R02 repeat failure has nonzero accounting")
		}
	}
	result.Outcome, result.FailureClass, result.Observation = outcome, failureClass, observation
	artifacts["error"] = strings.TrimSpace(errorString(executionErr))
	return result, artifacts, nil
}

func repositoryRootForR02Repeat() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for current := cwd; ; current = filepath.Dir(current) {
		marker := filepath.Join(current, "server", "internal", "agentqualification", "realistic_r02_repeat_admission_v1.json")
		if _, err := os.Stat(marker); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return ""
}
