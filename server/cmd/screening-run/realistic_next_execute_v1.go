package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
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

const realisticNextCampaignSchema = "silver-mediums.realistic-next-campaign-result.v1"

//go:embed realistic_next_execute_v1.go
var realisticNextCommandSource []byte

//go:embed realistic_next_execute_v1_test.go
var realisticNextCommandTestSource []byte

//go:embed main.go
var realisticNextMainSource []byte

//go:embed realistic_execute.go
var realisticNextTranslatorSource []byte

//go:embed openrouter_execute.go
var realisticNextRuntimeSource []byte

type realisticNextProviderFactory func(agentqualification.RealisticCandidate, int64, int64, int64) agentqualification.RealisticProvider

const realisticNextClaimSchema = "silver-mediums.realistic-next-provider-claim.v1"

type realisticNextProviderClaim struct {
	SchemaVersion             string `json:"schema_version"`
	MatrixID                  string `json:"matrix_id"`
	CandidateID               string `json:"candidate_id"`
	CaseID                    string `json:"case_id"`
	RunFamilyID               string `json:"run_family_id"`
	Status                    string `json:"status"`
	InputMicrosPerMillion     int64  `json:"input_micros_per_million"`
	GeneratedMicrosPerMillion int64  `json:"generated_micros_per_million"`
	ReservedCostMicros        int64  `json:"reserved_cost_micros"`
}

type realisticNextObservationRecord struct {
	CandidateID   string                                     `json:"candidate_id"`
	CaseID        string                                     `json:"case_id"`
	Status        string                                     `json:"status"`
	FailureClass  string                                     `json:"failure_class,omitempty"`
	SkipReason    string                                     `json:"skip_reason,omitempty"`
	WorkspaceRoot string                                     `json:"workspace_root,omitempty"`
	Run           *agentqualification.RealisticNextRunResult `json:"run,omitempty"`
}

type realisticNextCampaignResult struct {
	SchemaVersion   string                           `json:"schema_version"`
	MatrixID        string                           `json:"matrix_id"`
	Status          string                           `json:"status"`
	HaltReason      string                           `json:"halt_reason,omitempty"`
	ProviderCalls   int                              `json:"provider_calls"`
	InputTokens     int64                            `json:"input_tokens"`
	OutputTokens    int64                            `json:"output_tokens"`
	ReasoningTokens int64                            `json:"reasoning_tokens"`
	CostMicros      int64                            `json:"cost_micros"`
	Retries         int                              `json:"retries"`
	Observations    []realisticNextObservationRecord `json:"observations"`
}

func realisticNextPlan() (map[string]any, error) {
	group, err := agentqualification.CurrentRealisticNextGroup()
	if err != nil {
		return nil, err
	}
	matrix, err := agentqualification.CurrentRealisticNextRunMatrix()
	if err != nil {
		return nil, err
	}
	providerReady := agentqualification.RequireRealisticNextReviewedFreeze() == nil
	plans := make([]map[string]any, 0, len(matrix.Candidates)*len(matrix.ExecutionOrder))
	for _, caseID := range matrix.ExecutionOrder {
		for _, candidate := range matrix.Candidates {
			request := realisticNextPlanRequest(candidate.ConfigurationID, caseID)
			plan, err := agentqualification.BuildOpenCodeRealisticNextActionPlan(candidate, request)
			if err != nil {
				return nil, err
			}
			plans = append(plans, map[string]any{"candidate_id": candidate.ConfigurationID, "case_id": caseID, "model": plan.Model, "effort": plan.Effort, "serving_provider": plan.ServingProvider, "provider_fallbacks": *plan.ProviderFallbacks, "prompt_sha256": plan.PromptSHA256, "schema_sha256": plan.SchemaSHA256, "inline_config_sha256": plan.InlineConfigSHA256})
		}
	}
	return map[string]any{"status": "plan_only", "provider_called": false, "provider_ready": providerReady, "group_status": group.Status, "matrix_status": matrix.Status, "stage_policy": matrix.StagePolicy, "max_incremental_spend_micros": matrix.MaxIncrementalSpendMicros, "max_total_tokens": matrix.MaxTotalTokens, "per_observation_total_token_reservation": matrix.PerObservationTotalTokenReservation, "per_observation_generated_token_ceiling": matrix.PerObservationGeneratedTokenCeiling, "retries": matrix.Retries, "plans": plans}, nil
}

func realisticNextPlanRequest(runFamilyID, caseID string) agentqualification.RealisticProviderRequest {
	checks := []string{agentqualification.R05V1SimulateCheck, agentqualification.R05V1VerifyCheck}
	if caseID == "R06" {
		checks = []string{agentqualification.R06V1ContractCheck, agentqualification.R06V1ReleaseCRM, agentqualification.R06V1ReleaseLinkHub, agentqualification.R06V1ReleaseCanopy, agentqualification.R06V1AuthenticatedVerify, agentqualification.R06V1Rollback, agentqualification.R06V1VerifyBaseline}
	}
	return agentqualification.RealisticProviderRequest{ProviderCallID: runFamilyID + "-plan", ProviderTurn: 1, CaseID: caseID, CaseVersion: "v1", RunFamilyID: runFamilyID, Attempt: 1, Task: "sealed provider-free plan", AllowedActions: []string{"read", "search", "edit", "run_check", "finish"}, AllowedChecks: checks, MaxSteps: 20}
}

func executeRealisticNextMatrix(ctx context.Context, artifactRoot string) (realisticNextCampaignResult, error) {
	if err := agentqualification.RequireRealisticNextReviewedFreeze(); err != nil {
		return realisticNextCampaignResult{}, err
	}
	if err := requireRealisticNextCommandSources(); err != nil {
		return realisticNextCampaignResult{}, err
	}
	matrix, err := agentqualification.CurrentRealisticNextRunMatrix()
	if err != nil {
		return realisticNextCampaignResult{}, err
	}
	baseline, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		return realisticNextCampaignResult{}, err
	}
	factory := func(candidate agentqualification.RealisticCandidate, baselineMicros, maxMicros, reservationMicros int64) agentqualification.RealisticProvider {
		return openCodeRealisticNextProvider{candidate: candidate, baselineSpendMicros: baselineMicros, maxSpendMicros: maxMicros, reservationMicros: reservationMicros}
	}
	return runRealisticNextMatrix(ctx, artifactRoot, matrix, baseline.SpentMicros, factory)
}

func requireRealisticNextCommandSources() error {
	required, err := agentqualification.RequiredRealisticNextCommandSources()
	if err != nil {
		return err
	}
	actual := map[string][]byte{
		"realistic_next_execute_v1.go":      realisticNextCommandSource,
		"realistic_next_execute_v1_test.go": realisticNextCommandTestSource,
		"main.go":                           realisticNextMainSource,
		"realistic_execute.go":              realisticNextTranslatorSource,
		"openrouter_execute.go":             realisticNextRuntimeSource,
	}
	if len(required) != len(actual) {
		return fmt.Errorf("realistic-next frozen command source set drifted")
	}
	for name, raw := range actual {
		hash := fmt.Sprintf("%x", sha256.Sum256(raw))
		if required[name] != hash {
			return fmt.Errorf("realistic-next frozen command source drifted: %s", name)
		}
	}
	return nil
}

func runRealisticNextMatrix(ctx context.Context, artifactRoot string, matrix agentqualification.RealisticNextRunMatrix, baselineSpendMicros int64, factory realisticNextProviderFactory) (realisticNextCampaignResult, error) {
	result := realisticNextCampaignResult{SchemaVersion: realisticNextCampaignSchema, MatrixID: matrix.MatrixID, Status: "running", Retries: 0}
	if !filepath.IsAbs(artifactRoot) || factory == nil || matrix.Status != "frozen_reviewed" || matrix.Retries != 0 || matrix.ProviderFallbacks {
		return result, fmt.Errorf("realistic-next execution requires an absolute artifact root and frozen no-retry/no-fallback matrix")
	}
	finalPath := filepath.Join(artifactRoot, "campaign-result.json")
	if err := readJSONFile(finalPath, &result); err == nil {
		if err := validateStoredRealisticNextCampaign(artifactRoot, matrix, result); err != nil {
			return result, err
		}
		return result, nil
	} else if !os.IsNotExist(err) {
		return result, err
	}
	passedR05 := map[string]bool{}
	for _, caseID := range matrix.ExecutionOrder {
		for _, candidate := range matrix.Candidates {
			existing, found, err := loadRealisticNextObservation(artifactRoot, matrix, candidate, caseID)
			if err != nil {
				result.Status, result.HaltReason = "global_halt", "evidence_integrity_failure"
				return finalizeRealisticNextCampaign(finalPath, result)
			}
			if found {
				if err := appendRealisticNextObservation(&result, existing, matrix); err != nil {
					result.Status, result.HaltReason = "global_halt", "evidence_integrity_failure"
					return finalizeRealisticNextCampaign(finalPath, result)
				}
				if caseID == "R05" && existing.Status == "passed" {
					passedR05[candidate.ConfigurationID] = true
				}
				if existing.Run != nil && existing.Run.GlobalHalt {
					result.Status, result.HaltReason = "global_halt", existing.Run.HaltReason
					return finalizeRealisticNextCampaign(finalPath, result)
				}
				continue
			}
			claimPath := realisticNextClaimPath(artifactRoot, candidate.ConfigurationID, caseID)
			if _, err := os.Stat(claimPath); err == nil {
				result.Status, result.HaltReason = "global_halt", "ambiguous_transport"
				return finalizeRealisticNextCampaign(finalPath, result)
			} else if !os.IsNotExist(err) {
				return result, err
			}
			if caseID == "R06" && !passedR05[candidate.ConfigurationID] {
				record := realisticNextObservationRecord{CandidateID: candidate.ConfigurationID, CaseID: caseID, Status: "skipped", SkipReason: "candidate_did_not_produce_a_valid_R05_pass"}
				if err := persistRealisticNextObservation(artifactRoot, record); err != nil {
					return result, err
				}
				result.Observations = append(result.Observations, record)
				continue
			}
			reservationMicros, err := realisticNextWorstCaseCostMicros(candidate, matrix)
			if err != nil {
				result.Status, result.HaltReason = "global_halt", "evidence_integrity_failure"
				return finalizeRealisticNextCampaign(finalPath, result)
			}
			if !realisticNextAdmitObservation(result, matrix, reservationMicros) {
				result.Status, result.HaltReason = "global_halt", "budget_breach"
				return finalizeRealisticNextCampaign(finalPath, result)
			}
			workspaceParent := filepath.Join(artifactRoot, "workspaces", safeArtifactName(candidate.ConfigurationID), strings.ToLower(caseID))
			if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
				return result, err
			}
			workspace, err := prepareRealisticNextWorkspace(workspaceParent, caseID)
			if err != nil {
				return result, err
			}
			runFamilyID := realisticNextRunFamilyID(matrix.MatrixID, candidate.ConfigurationID, caseID)
			price, _ := realisticNextPinnedPrice(candidate)
			claim := realisticNextProviderClaim{SchemaVersion: realisticNextClaimSchema, MatrixID: matrix.MatrixID, CandidateID: candidate.ConfigurationID, CaseID: caseID, RunFamilyID: runFamilyID, Status: "provider_call_reserved", InputMicrosPerMillion: price.InputMicrosPerMillion, GeneratedMicrosPerMillion: price.GeneratedMicrosPerMillion, ReservedCostMicros: reservationMicros}
			if err := writeJSONExclusive(claimPath, claim); err != nil {
				if os.IsExist(err) {
					result.Status, result.HaltReason = "global_halt", "ambiguous_transport"
					return finalizeRealisticNextCampaign(finalPath, result)
				}
				return result, err
			}
			provider := factory(candidate, baselineSpendMicros, matrix.MaxIncrementalSpendMicros, reservationMicros)
			limits := realisticNextObservationLimits(matrix, result)
			run, err := agentqualification.RunRealisticNextCandidate(ctx, workspace, runFamilyID, provider, limits, func(controller agentqualification.RealisticControllerResult) error {
				return writeJSONAtomic(filepath.Join(workspaceParent, "run-state.json"), controller)
			})
			if err != nil {
				return result, err
			}
			realisticNextApplyPostRunCaps(&run, matrix)
			receiptRoot := filepath.Join(artifactRoot, "receipts", safeArtifactName(candidate.ConfigurationID), strings.ToLower(caseID))
			receiptPath, receiptHash, err := agentqualification.SealRealisticNextReceipt(receiptRoot, run)
			if err != nil {
				return result, err
			}
			run.ReceiptPath, run.ReceiptHash = receiptPath, receiptHash
			record := realisticNextObservationRecord{CandidateID: candidate.ConfigurationID, CaseID: caseID, Status: run.Controller.TerminalState, FailureClass: run.Controller.FailureClass, WorkspaceRoot: workspace.Root, Run: &run}
			if err := persistRealisticNextObservation(artifactRoot, record); err != nil {
				return result, err
			}
			if err := appendRealisticNextObservation(&result, record, matrix); err != nil {
				return result, err
			}
			if caseID == "R05" && run.Controller.TerminalState == "passed" {
				passedR05[candidate.ConfigurationID] = true
			}
			if run.GlobalHalt || realisticNextCampaignCapReached(result, matrix) {
				result.Status = "global_halt"
				result.HaltReason = run.HaltReason
				if result.HaltReason == "" {
					result.HaltReason = "budget_breach"
				}
				return finalizeRealisticNextCampaign(finalPath, result)
			}
		}
	}
	result.Status = "complete"
	return finalizeRealisticNextCampaign(finalPath, result)
}

func prepareRealisticNextWorkspace(parent, caseID string) (agentqualification.RealisticNextWorkspace, error) {
	if caseID == "R05" {
		return agentqualification.PrepareRealisticNextR05V1(parent)
	}
	if caseID == "R06" {
		return agentqualification.PrepareRealisticNextR06V1(parent)
	}
	return agentqualification.RealisticNextWorkspace{}, fmt.Errorf("unsupported realistic-next case %s", caseID)
}

func realisticNextObservationLimits(matrix agentqualification.RealisticNextRunMatrix, campaign realisticNextCampaignResult) agentqualification.RealisticNextLimits {
	remainingCost := matrix.MaxIncrementalSpendMicros - campaign.CostMicros
	remainingTokens := matrix.MaxTotalTokens - campaign.InputTokens - campaign.OutputTokens - campaign.ReasoningTokens
	return agentqualification.RealisticNextLimits{MaxSteps: 20, MaxInputTokens: remainingTokens, MaxOutputTokens: matrix.PerObservationGeneratedTokenCeiling, MaxReasoningTokens: matrix.PerObservationGeneratedTokenCeiling, MaxCostMicros: remainingCost}
}

func realisticNextApplyPostRunCaps(run *agentqualification.RealisticNextRunResult, matrix agentqualification.RealisticNextRunMatrix) {
	total := run.Controller.InputTokens + run.Controller.OutputTokens + run.Controller.ReasoningTokens
	generated := run.Controller.OutputTokens + run.Controller.ReasoningTokens
	if total > matrix.PerObservationTotalTokenReservation || generated > matrix.PerObservationGeneratedTokenCeiling {
		run.Controller.TerminalState, run.Controller.FailureClass = "invalid", "budget_breach"
		run.GlobalHalt, run.HaltReason = true, "budget_breach"
	}
}

func realisticNextCampaignCapReached(result realisticNextCampaignResult, matrix agentqualification.RealisticNextRunMatrix) bool {
	return result.CostMicros >= matrix.MaxIncrementalSpendMicros || result.InputTokens+result.OutputTokens+result.ReasoningTokens >= matrix.MaxTotalTokens
}

type realisticNextPrice struct {
	InputMicrosPerMillion     int64
	GeneratedMicrosPerMillion int64
}

func realisticNextPinnedPrice(candidate agentqualification.RealisticCandidate) (realisticNextPrice, error) {
	switch candidate.Model {
	case "openrouter/anthropic/claude-sonnet-5":
		return realisticNextPrice{InputMicrosPerMillion: 3_000_000, GeneratedMicrosPerMillion: 15_000_000}, nil
	case "openrouter/openai/gpt-5.4":
		return realisticNextPrice{InputMicrosPerMillion: 2_500_000, GeneratedMicrosPerMillion: 15_000_000}, nil
	default:
		return realisticNextPrice{}, fmt.Errorf("no pinned price for %s", candidate.Model)
	}
}

func realisticNextWorstCaseCostMicros(candidate agentqualification.RealisticCandidate, matrix agentqualification.RealisticNextRunMatrix) (int64, error) {
	price, err := realisticNextPinnedPrice(candidate)
	if err != nil {
		return 0, err
	}
	total := matrix.PerObservationTotalTokenReservation
	generated := matrix.PerObservationGeneratedTokenCeiling
	if total <= 0 || generated < 0 || generated > total || price.InputMicrosPerMillion <= 0 || price.GeneratedMicrosPerMillion <= 0 {
		return 0, fmt.Errorf("invalid realistic-next price or token reservation")
	}
	// Maximize cost within total-token and generated-token ceilings. Reasoning
	// tokens are conservatively priced as generated output.
	generatedAtMax := int64(0)
	if price.GeneratedMicrosPerMillion > price.InputMicrosPerMillion {
		generatedAtMax = generated
	}
	inputAtMax := total - generatedAtMax
	numerator := inputAtMax*price.InputMicrosPerMillion + generatedAtMax*price.GeneratedMicrosPerMillion
	return (numerator + 999_999) / 1_000_000, nil
}

func realisticNextAdmitObservation(result realisticNextCampaignResult, matrix agentqualification.RealisticNextRunMatrix, reservationMicros int64) bool {
	usedTokens := result.InputTokens + result.OutputTokens + result.ReasoningTokens
	return reservationMicros > 0 && result.CostMicros <= matrix.MaxIncrementalSpendMicros-reservationMicros && usedTokens <= matrix.MaxTotalTokens-matrix.PerObservationTotalTokenReservation
}

func realisticNextRunFamilyID(matrixID, candidateID, caseID string) string {
	digest := sha256.Sum256([]byte(matrixID + "\x00" + candidateID + "\x00" + caseID))
	return safeArtifactName(candidateID) + "-" + strings.ToLower(caseID) + "-" + fmt.Sprintf("%x", digest[:8])
}

func realisticNextClaimPath(root, candidateID, caseID string) string {
	return filepath.Join(root, "observations", safeArtifactName(candidateID), strings.ToLower(caseID), "provider-claim.json")
}

func realisticNextObservationPath(root, candidateID, caseID string) string {
	return filepath.Join(root, "observations", safeArtifactName(candidateID), strings.ToLower(caseID), "result.json")
}

func loadRealisticNextObservation(root string, matrix agentqualification.RealisticNextRunMatrix, candidate agentqualification.RealisticCandidate, caseID string) (realisticNextObservationRecord, bool, error) {
	var record realisticNextObservationRecord
	path := realisticNextObservationPath(root, candidate.ConfigurationID, caseID)
	if err := readJSONFile(path, &record); err != nil {
		if os.IsNotExist(err) {
			return record, false, nil
		}
		return record, false, err
	}
	if record.CandidateID != candidate.ConfigurationID || record.CaseID != caseID || record.Status == "" {
		return record, false, fmt.Errorf("completed observation identity drifted")
	}
	if record.Status == "skipped" {
		if record.Run != nil || record.SkipReason == "" {
			return record, false, fmt.Errorf("skipped observation is malformed")
		}
		return record, true, nil
	}
	if record.Run == nil || record.Run.Controller.RunFamilyID != realisticNextRunFamilyID(matrix.MatrixID, candidate.ConfigurationID, caseID) {
		return record, false, fmt.Errorf("completed observation run identity drifted")
	}
	if record.Status != record.Run.Controller.TerminalState || record.FailureClass != record.Run.Controller.FailureClass {
		return record, false, fmt.Errorf("completed observation terminal state drifted")
	}
	var claim realisticNextProviderClaim
	if err := readJSONFile(realisticNextClaimPath(root, candidate.ConfigurationID, caseID), &claim); err != nil {
		return record, false, fmt.Errorf("completed observation claim missing: %w", err)
	}
	price, err := realisticNextPinnedPrice(candidate)
	if err != nil {
		return record, false, err
	}
	reservation, err := realisticNextWorstCaseCostMicros(candidate, matrix)
	if err != nil {
		return record, false, err
	}
	if claim.SchemaVersion != realisticNextClaimSchema || claim.MatrixID != matrix.MatrixID || claim.CandidateID != candidate.ConfigurationID || claim.CaseID != caseID || claim.RunFamilyID != record.Run.Controller.RunFamilyID || claim.Status != "provider_call_reserved" || claim.InputMicrosPerMillion != price.InputMicrosPerMillion || claim.GeneratedMicrosPerMillion != price.GeneratedMicrosPerMillion || claim.ReservedCostMicros != reservation {
		return record, false, fmt.Errorf("completed observation claim drifted")
	}
	if err := verifyRealisticNextSealedRun(root, candidate.ConfigurationID, caseID, *record.Run); err != nil {
		return record, false, err
	}
	return record, true, nil
}

func verifyRealisticNextSealedRun(root, candidateID, caseID string, run agentqualification.RealisticNextRunResult) error {
	expectedRoot := filepath.Join(root, "receipts", safeArtifactName(candidateID), strings.ToLower(caseID))
	expectedRoot, _ = filepath.Abs(expectedRoot)
	receiptPath, err := filepath.Abs(run.ReceiptPath)
	if err != nil || receiptPath == expectedRoot || !strings.HasPrefix(receiptPath, expectedRoot+string(os.PathSeparator)) || run.ReceiptHash == "" {
		return fmt.Errorf("completed observation receipt path is invalid")
	}
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != run.ReceiptHash {
		return fmt.Errorf("completed observation receipt hash drifted")
	}
	var sealed struct {
		Schema string                                    `json:"schema"`
		Run    agentqualification.RealisticNextRunResult `json:"run"`
	}
	if err := json.Unmarshal(raw, &sealed); err != nil || sealed.Schema != agentqualification.RealisticNextHarnessVersion {
		return fmt.Errorf("completed observation receipt is malformed")
	}
	want := run
	want.ReceiptPath, want.ReceiptHash = "", ""
	if !reflect.DeepEqual(sealed.Run, want) {
		return fmt.Errorf("completed observation does not match sealed receipt")
	}
	return nil
}

func appendRealisticNextObservation(result *realisticNextCampaignResult, record realisticNextObservationRecord, matrix agentqualification.RealisticNextRunMatrix) error {
	result.Observations = append(result.Observations, record)
	if record.Run == nil {
		return nil
	}
	run := record.Run
	result.ProviderCalls += run.Controller.ProviderCalls
	result.InputTokens += run.Controller.InputTokens
	result.OutputTokens += run.Controller.OutputTokens
	result.ReasoningTokens += run.Controller.ReasoningTokens
	result.CostMicros += run.Controller.CostMicros
	if realisticNextCampaignCapReached(*result, matrix) && (result.CostMicros > matrix.MaxIncrementalSpendMicros || result.InputTokens+result.OutputTokens+result.ReasoningTokens > matrix.MaxTotalTokens) {
		return fmt.Errorf("reconstructed campaign accounting exceeds frozen caps")
	}
	return nil
}

func validateStoredRealisticNextCampaign(root string, matrix agentqualification.RealisticNextRunMatrix, stored realisticNextCampaignResult) error {
	if stored.SchemaVersion != realisticNextCampaignSchema || stored.MatrixID != matrix.MatrixID || stored.Retries != 0 || stored.Status != "complete" && stored.Status != "global_halt" {
		return fmt.Errorf("stored realistic-next campaign identity drifted")
	}
	if stored.Status == "complete" && (stored.HaltReason != "" || len(stored.Observations) != len(matrix.ExecutionOrder)*len(matrix.Candidates)) || stored.Status == "global_halt" && stored.HaltReason == "" {
		return fmt.Errorf("stored realistic-next campaign completion shape drifted")
	}
	rebuilt := realisticNextCampaignResult{SchemaVersion: realisticNextCampaignSchema, MatrixID: matrix.MatrixID, Status: stored.Status, HaltReason: stored.HaltReason, Retries: 0}
	seen := map[string]bool{}
	for index, record := range stored.Observations {
		caseIndex := index / len(matrix.Candidates)
		candidateIndex := index % len(matrix.Candidates)
		if caseIndex >= len(matrix.ExecutionOrder) || record.CaseID != matrix.ExecutionOrder[caseIndex] || record.CandidateID != matrix.Candidates[candidateIndex].ConfigurationID {
			return fmt.Errorf("stored realistic-next campaign order drifted")
		}
		key := record.CandidateID + "\x00" + record.CaseID
		if seen[key] {
			return fmt.Errorf("stored realistic-next campaign duplicated an observation")
		}
		seen[key] = true
		var candidate agentqualification.RealisticCandidate
		foundCandidate := false
		for _, allowed := range matrix.Candidates {
			if allowed.ConfigurationID == record.CandidateID {
				candidate, foundCandidate = allowed, true
				break
			}
		}
		if !foundCandidate || record.CaseID != "R05" && record.CaseID != "R06" {
			return fmt.Errorf("stored realistic-next campaign contains an unknown binding")
		}
		sealed, found, err := loadRealisticNextObservation(root, matrix, candidate, record.CaseID)
		if err != nil || !found || !reflect.DeepEqual(sealed, record) {
			return fmt.Errorf("stored realistic-next campaign observation is not sealed")
		}
		if err := appendRealisticNextObservation(&rebuilt, sealed, matrix); err != nil {
			return err
		}
	}
	if rebuilt.ProviderCalls != stored.ProviderCalls || rebuilt.InputTokens != stored.InputTokens || rebuilt.OutputTokens != stored.OutputTokens || rebuilt.ReasoningTokens != stored.ReasoningTokens || rebuilt.CostMicros != stored.CostMicros {
		return fmt.Errorf("stored realistic-next campaign accounting drifted")
	}
	return nil
}

func persistRealisticNextObservation(root string, record realisticNextObservationRecord) error {
	return writeJSONExclusive(realisticNextObservationPath(root, record.CandidateID, record.CaseID), record)
}

func finalizeRealisticNextCampaign(path string, result realisticNextCampaignResult) (realisticNextCampaignResult, error) {
	if err := writeJSONExclusive(path, result); err != nil {
		return result, err
	}
	return result, nil
}

type openCodeRealisticNextProvider struct {
	candidate           agentqualification.RealisticCandidate
	baselineSpendMicros int64
	maxSpendMicros      int64
	reservationMicros   int64
}

func (provider openCodeRealisticNextProvider) Next(ctx context.Context, request agentqualification.RealisticProviderRequest) (agentqualification.RealisticProviderResponse, error) {
	spend, err := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		if errors.Is(err, errOpenRouterCredentialFailure) {
			return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticCredentialFailure, err)
		}
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	if provider.baselineSpendMicros < 0 || provider.maxSpendMicros <= 0 || provider.reservationMicros <= 0 || spend.SpentMicros-provider.baselineSpendMicros+provider.reservationMicros > provider.maxSpendMicros {
		return agentqualification.RealisticProviderResponse{}, agentqualification.ErrRealisticBudgetBreach
	}
	plan, err := agentqualification.BuildOpenCodeRealisticNextActionPlan(provider.candidate, request)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	resolvedConfig, err := runtimeState.resolvedConfig(ctx)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	routing, err := realisticOpenCodeRoutingEvidenceFor(plan, resolvedConfig)
	if err != nil {
		return agentqualification.RealisticProviderResponse{}, fmt.Errorf("%w: %v", agentqualification.ErrRealisticProviderPreflight, err)
	}
	rawJSONL, rawExport, err := runtimeState.runOnce(plan)
	if err != nil {
		return realisticPartialProviderResponse(rawJSONL, rawExport, err), err
	}
	return translateRealisticOpenCodeArtifact(provider.candidate, routing, rawJSONL, rawExport)
}
