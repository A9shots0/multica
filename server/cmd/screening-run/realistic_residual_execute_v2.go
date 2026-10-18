package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const realisticResidualCampaignSchemaV2 = "silver-mediums.realistic-residual-campaign-result.v2"

//go:embed realistic_residual_execute_v2.go
var realisticResidualCommandSourceV2 []byte

//go:embed realistic_residual_execute_v2_test.go
var realisticResidualCommandTestSourceV2 []byte

//go:embed realistic_residual_entry_v2.go
var realisticResidualEntrySourceV2 []byte

func realisticResidualV2Plan() (map[string]any, error) {
	matrix, err := agentqualification.CurrentRealisticResidualRunMatrixV2()
	if err != nil {
		return nil, err
	}
	providerReady := agentqualification.RequireRealisticResidualV2ReviewedFreeze() == nil
	plans := make([]map[string]any, 0, len(matrix.ExecutionOrder))
	for _, caseID := range matrix.ExecutionOrder {
		candidate := matrix.Candidates[0]
		plan, err := agentqualification.BuildOpenCodeRealisticNextActionPlan(candidate, realisticNextPlanRequest(candidate.ConfigurationID, caseID))
		if err != nil {
			return nil, err
		}
		plans = append(plans, map[string]any{
			"candidate_id": candidate.ConfigurationID, "case_id": caseID, "model": plan.Model,
			"effort": plan.Effort, "serving_provider": plan.ServingProvider,
			"provider_fallbacks": *plan.ProviderFallbacks, "prompt_sha256": plan.PromptSHA256,
			"schema_sha256": plan.SchemaSHA256, "inline_config_sha256": plan.InlineConfigSHA256,
		})
	}
	return map[string]any{
		"status": "plan_only", "provider_called": false, "provider_ready": providerReady,
		"matrix_status": matrix.Status, "matrix_id": matrix.MatrixID, "stage_policy": matrix.StagePolicy,
		"observation_exhaustion_policy":           matrix.ObservationExhaustionPolicy,
		"max_incremental_spend_micros":            matrix.MaxIncrementalSpendMicros,
		"max_total_tokens":                        matrix.MaxTotalTokens,
		"per_observation_total_token_reservation": matrix.PerObservationTotalTokenReservation,
		"per_observation_generated_token_ceiling": matrix.PerObservationGeneratedTokenCeiling,
		"retries": matrix.Retries, "plans": plans,
	}, nil
}

func executeRealisticResidualV2(ctx context.Context, artifactRoot string) (realisticNextCampaignResult, error) {
	if err := agentqualification.RequireRealisticResidualV2ReviewedFreeze(); err != nil {
		return realisticNextCampaignResult{}, err
	}
	if err := requireRealisticResidualV2CommandSources(); err != nil {
		return realisticNextCampaignResult{}, err
	}
	matrix, err := agentqualification.CurrentRealisticResidualRunMatrixV2()
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
	return runRealisticResidualV2(ctx, artifactRoot, matrix, baseline.SpentMicros, factory)
}

func requireRealisticResidualV2CommandSources() error {
	required, err := agentqualification.RequiredRealisticResidualV2CommandSources()
	if err != nil {
		return err
	}
	actual := map[string][]byte{
		"realistic_residual_execute_v2.go":      realisticResidualCommandSourceV2,
		"realistic_residual_execute_v2_test.go": realisticResidualCommandTestSourceV2,
		"realistic_residual_entry_v2.go":        realisticResidualEntrySourceV2,
		"realistic_next_execute_v1.go":          realisticNextCommandSource,
		"realistic_execute.go":                  realisticNextTranslatorSource,
		"openrouter_execute.go":                 realisticNextRuntimeSource,
	}
	if len(required) != len(actual)+2 {
		return fmt.Errorf("realistic residual v2 frozen command source set drifted")
	}
	for name, raw := range actual {
		if required[name] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			return fmt.Errorf("realistic residual v2 frozen command source drifted: %s", name)
		}
	}
	return nil
}

func runRealisticResidualV2(ctx context.Context, artifactRoot string, matrix agentqualification.RealisticResidualRunMatrixV2, baselineSpendMicros int64, factory realisticNextProviderFactory) (realisticNextCampaignResult, error) {
	result := realisticNextCampaignResult{SchemaVersion: realisticResidualCampaignSchemaV2, MatrixID: matrix.MatrixID, Status: "running", Retries: 0}
	if !filepath.IsAbs(artifactRoot) || factory == nil || matrix.Status != "frozen_reviewed" || matrix.Retries != 0 || matrix.ProviderFallbacks {
		return result, fmt.Errorf("realistic residual v2 execution requires an absolute artifact root and frozen no-retry/no-fallback matrix")
	}
	legacy := matrix.AsRealisticNextV1()
	legacy.Status = "frozen_reviewed"
	finalPath := filepath.Join(artifactRoot, "campaign-result.json")
	if err := readJSONFile(finalPath, &result); err == nil {
		if err := validateStoredRealisticResidualV2(artifactRoot, legacy, result); err != nil {
			return result, err
		}
		return result, nil
	} else if !os.IsNotExist(err) {
		return result, err
	}
	passedR05 := false
	for _, caseID := range matrix.ExecutionOrder {
		candidate := matrix.Candidates[0]
		existing, found, err := loadRealisticResidualV2Observation(artifactRoot, legacy, candidate, caseID)
		if err != nil {
			result.Status, result.HaltReason = "global_halt", "evidence_corruption_making_further_results_unusable"
			return finalizeRealisticResidualV2(finalPath, result)
		}
		if found {
			if err := appendRealisticNextObservation(&result, existing, legacy); err != nil {
				result.Status, result.HaltReason = "global_halt", "evidence_corruption_making_further_results_unusable"
				return finalizeRealisticResidualV2(finalPath, result)
			}
			if caseID == "R05" && existing.Status == "passed" {
				passedR05 = true
			}
			if existing.Run != nil && existing.Run.GlobalHalt {
				result.Status, result.HaltReason = "global_halt", residualV2GlobalReason(existing.Run.HaltReason)
				return finalizeRealisticResidualV2(finalPath, result)
			}
			continue
		}
		claimPath := realisticNextClaimPath(artifactRoot, candidate.ConfigurationID, caseID)
		if _, err := os.Stat(claimPath); err == nil {
			result.Status, result.HaltReason = "global_halt", "transport_failure"
			return finalizeRealisticResidualV2(finalPath, result)
		} else if !os.IsNotExist(err) {
			return haltRealisticResidualV2(finalPath, result, "runtime_or_config_failure")
		}
		if caseID == "R06" && !passedR05 {
			record := realisticNextObservationRecord{CandidateID: candidate.ConfigurationID, CaseID: caseID, Status: "skipped", SkipReason: "candidate_did_not_produce_a_valid_R05_pass"}
			if err := persistRealisticNextObservation(artifactRoot, record); err != nil {
				return haltRealisticResidualV2(finalPath, result, "evidence_corruption_making_further_results_unusable")
			}
			result.Observations = append(result.Observations, record)
			continue
		}
		reservationMicros, err := realisticResidualV2WorstCaseCostMicros(candidate, matrix)
		if err != nil {
			result.Status, result.HaltReason = "global_halt", "evidence_corruption_making_further_results_unusable"
			return finalizeRealisticResidualV2(finalPath, result)
		}
		if !realisticNextAdmitObservation(result, legacy, reservationMicros) {
			result.Status, result.HaltReason = "global_halt", "campaign_budget_breach"
			return finalizeRealisticResidualV2(finalPath, result)
		}
		workspaceParent := filepath.Join(artifactRoot, "workspaces", safeArtifactName(candidate.ConfigurationID), strings.ToLower(caseID))
		if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
			return haltRealisticResidualV2(finalPath, result, "runtime_or_config_failure")
		}
		workspace, err := prepareRealisticNextWorkspace(workspaceParent, caseID)
		if err != nil {
			return haltRealisticResidualV2(finalPath, result, "runtime_or_config_failure")
		}
		runFamilyID := realisticNextRunFamilyID(matrix.MatrixID, candidate.ConfigurationID, caseID)
		price, _ := realisticNextPinnedPrice(candidate)
		claim := realisticNextProviderClaim{SchemaVersion: realisticNextClaimSchema, MatrixID: matrix.MatrixID, CandidateID: candidate.ConfigurationID, CaseID: caseID, RunFamilyID: runFamilyID, Status: "provider_call_reserved", InputMicrosPerMillion: price.InputMicrosPerMillion, GeneratedMicrosPerMillion: price.GeneratedMicrosPerMillion, ReservedCostMicros: reservationMicros}
		if err := writeJSONExclusive(claimPath, claim); err != nil {
			if os.IsExist(err) {
				result.Status, result.HaltReason = "global_halt", "transport_failure"
				return finalizeRealisticResidualV2(finalPath, result)
			}
			return haltRealisticResidualV2(finalPath, result, "runtime_or_config_failure")
		}
		provider := &agentqualification.RealisticResidualBoundedProviderV2{
			Inner:              factory(candidate, baselineSpendMicros, matrix.MaxIncrementalSpendMicros, reservationMicros),
			MaxTotalTokens:     matrix.PerObservationTotalTokenReservation,
			MaxGeneratedTokens: matrix.PerObservationGeneratedTokenCeiling,
		}
		limits := realisticResidualV2ObservationLimits(matrix, result, reservationMicros)
		run, err := agentqualification.RunRealisticNextCandidate(ctx, workspace, runFamilyID, provider, limits, func(controller agentqualification.RealisticControllerResult) error {
			return writeJSONAtomic(filepath.Join(workspaceParent, "run-state.json"), controller)
		})
		if err != nil {
			return haltRealisticResidualV2(finalPath, result, "runtime_or_config_failure")
		}
		residualV2ClassifyExhaustion(&run, matrix, result)
		receiptRoot := filepath.Join(artifactRoot, "receipts", safeArtifactName(candidate.ConfigurationID), strings.ToLower(caseID))
		receiptPath, receiptHash, err := agentqualification.SealRealisticNextReceipt(receiptRoot, run)
		if err != nil {
			return haltRealisticResidualV2(finalPath, result, "evidence_corruption_making_further_results_unusable")
		}
		run.ReceiptPath, run.ReceiptHash = receiptPath, receiptHash
		record := realisticNextObservationRecord{CandidateID: candidate.ConfigurationID, CaseID: caseID, Status: run.Controller.TerminalState, FailureClass: run.Controller.FailureClass, WorkspaceRoot: workspace.Root, Run: &run}
		if err := persistRealisticNextObservation(artifactRoot, record); err != nil {
			return haltRealisticResidualV2(finalPath, result, "evidence_corruption_making_further_results_unusable")
		}
		if err := appendRealisticNextObservation(&result, record, legacy); err != nil {
			return haltRealisticResidualV2(finalPath, result, "evidence_corruption_making_further_results_unusable")
		}
		if caseID == "R05" && run.Controller.TerminalState == "passed" {
			passedR05 = true
		}
		if run.GlobalHalt || residualV2CampaignCapReached(result, matrix) {
			result.Status = "global_halt"
			result.HaltReason = residualV2GlobalReason(run.HaltReason)
			if result.HaltReason == "" {
				result.HaltReason = "campaign_budget_breach"
			}
			return finalizeRealisticResidualV2(finalPath, result)
		}
	}
	result.Status = "complete"
	return finalizeRealisticResidualV2(finalPath, result)
}

func realisticResidualV2ObservationLimits(matrix agentqualification.RealisticResidualRunMatrixV2, campaign realisticNextCampaignResult, reservationMicros int64) agentqualification.RealisticNextLimits {
	remainingCost := matrix.MaxIncrementalSpendMicros - campaign.CostMicros
	if remainingCost > reservationMicros {
		remainingCost = reservationMicros
	}
	return agentqualification.RealisticNextLimits{
		MaxSteps: 20, MaxInputTokens: matrix.PerObservationTotalTokenReservation,
		MaxOutputTokens:    matrix.PerObservationGeneratedTokenCeiling,
		MaxReasoningTokens: matrix.PerObservationGeneratedTokenCeiling, MaxCostMicros: remainingCost,
	}
}

func residualV2ClassifyExhaustion(run *agentqualification.RealisticNextRunResult, matrix agentqualification.RealisticResidualRunMatrixV2, campaign realisticNextCampaignResult) {
	if run.Controller.FailureClass == "provider_preflight_failure" {
		run.GlobalHalt = true
		run.HaltReason = "runtime_or_config_failure"
		return
	}
	observationTotal := run.Controller.InputTokens + run.Controller.OutputTokens + run.Controller.ReasoningTokens
	observationGenerated := run.Controller.OutputTokens + run.Controller.ReasoningTokens
	if run.Controller.ProviderCalls > 0 && (observationTotal > matrix.PerObservationTotalTokenReservation || observationGenerated > matrix.PerObservationGeneratedTokenCeiling) {
		totalTokens := campaign.InputTokens + campaign.OutputTokens + campaign.ReasoningTokens + observationTotal
		totalCost := campaign.CostMicros + run.Controller.CostMicros
		if totalTokens >= matrix.MaxTotalTokens || totalCost >= matrix.MaxIncrementalSpendMicros {
			run.GlobalHalt, run.HaltReason = true, "campaign_budget_breach"
			return
		}
		run.Controller.TerminalState = "failed"
		run.Controller.FailureClass = "candidate_resource_exhaustion"
		run.Controller.FinalSummary = "candidate exhausted its combined reserved observation resources"
		run.GlobalHalt, run.HaltReason = false, ""
		return
	}
	if run.HaltReason != "budget_breach" || run.Controller.ProviderCalls == 0 {
		if run.HaltReason == "budget_breach" {
			run.HaltReason = "credit_failure"
		}
		return
	}
	totalTokens := campaign.InputTokens + campaign.OutputTokens + campaign.ReasoningTokens + run.Controller.InputTokens + run.Controller.OutputTokens + run.Controller.ReasoningTokens
	totalCost := campaign.CostMicros + run.Controller.CostMicros
	if totalTokens >= matrix.MaxTotalTokens || totalCost >= matrix.MaxIncrementalSpendMicros {
		run.HaltReason = "campaign_budget_breach"
		return
	}
	run.Controller.TerminalState = "failed"
	run.Controller.FailureClass = "candidate_resource_exhaustion"
	run.Controller.FinalSummary = "candidate exhausted its reserved observation resources"
	run.GlobalHalt = false
	run.HaltReason = ""
}

func realisticResidualV2WorstCaseCostMicros(candidate agentqualification.RealisticCandidate, matrix agentqualification.RealisticResidualRunMatrixV2) (int64, error) {
	price, err := realisticNextPinnedPrice(candidate)
	if err != nil {
		return 0, err
	}
	total := matrix.PerObservationTotalTokenReservation
	// The v1 controller enforces output and reasoning independently. Reserve
	// both generated-token dimensions while also respecting the combined total.
	generated := 2 * matrix.PerObservationGeneratedTokenCeiling
	if generated > total {
		generated = total
	}
	if total <= 0 || generated < 0 || price.InputMicrosPerMillion <= 0 || price.GeneratedMicrosPerMillion <= 0 {
		return 0, fmt.Errorf("invalid residual v2 price or token reservation")
	}
	input := total - generated
	numerator := input*price.InputMicrosPerMillion + generated*price.GeneratedMicrosPerMillion
	return (numerator + 999_999) / 1_000_000, nil
}

func residualV2CampaignCapReached(result realisticNextCampaignResult, matrix agentqualification.RealisticResidualRunMatrixV2) bool {
	return result.CostMicros >= matrix.MaxIncrementalSpendMicros || result.InputTokens+result.OutputTokens+result.ReasoningTokens >= matrix.MaxTotalTokens
}

func residualV2GlobalReason(reason string) string {
	if reason == "budget_breach" {
		return "credit_failure"
	}
	if reason == "ambiguous_transport" {
		return "transport_failure"
	}
	if reason == "evidence_integrity_failure" {
		return "evidence_corruption_making_further_results_unusable"
	}
	return reason
}

func validateStoredRealisticResidualV2(root string, matrix agentqualification.RealisticNextRunMatrix, stored realisticNextCampaignResult) error {
	if stored.SchemaVersion != realisticResidualCampaignSchemaV2 || stored.MatrixID != matrix.MatrixID || stored.Retries != 0 || stored.Status != "complete" && stored.Status != "global_halt" {
		return fmt.Errorf("stored realistic residual v2 campaign identity drifted")
	}
	if stored.Status == "complete" && (stored.HaltReason != "" || len(stored.Observations) != len(matrix.ExecutionOrder)*len(matrix.Candidates)) || stored.Status == "global_halt" && stored.HaltReason == "" {
		return fmt.Errorf("stored realistic residual v2 campaign shape drifted")
	}
	rebuilt := realisticNextCampaignResult{SchemaVersion: realisticResidualCampaignSchemaV2, MatrixID: matrix.MatrixID, Status: stored.Status, HaltReason: stored.HaltReason, Retries: 0}
	seen := map[string]bool{}
	for index, record := range stored.Observations {
		caseIndex := index / len(matrix.Candidates)
		candidateIndex := index % len(matrix.Candidates)
		if caseIndex >= len(matrix.ExecutionOrder) || record.CaseID != matrix.ExecutionOrder[caseIndex] || record.CandidateID != matrix.Candidates[candidateIndex].ConfigurationID {
			return fmt.Errorf("stored realistic residual v2 campaign order drifted")
		}
		key := record.CandidateID + "\x00" + record.CaseID
		if seen[key] {
			return fmt.Errorf("stored realistic residual v2 campaign duplicated an observation")
		}
		seen[key] = true
		sealed, found, err := loadRealisticResidualV2Observation(root, matrix, matrix.Candidates[candidateIndex], record.CaseID)
		if err != nil || !found || !reflectDeepEqualObservation(sealed, record) {
			return fmt.Errorf("stored realistic residual v2 observation is not sealed")
		}
		if err := appendRealisticNextObservation(&rebuilt, sealed, matrix); err != nil {
			return err
		}
	}
	if rebuilt.ProviderCalls != stored.ProviderCalls || rebuilt.InputTokens != stored.InputTokens || rebuilt.OutputTokens != stored.OutputTokens || rebuilt.ReasoningTokens != stored.ReasoningTokens || rebuilt.CostMicros != stored.CostMicros {
		return fmt.Errorf("stored realistic residual v2 campaign accounting drifted")
	}
	return nil
}

func loadRealisticResidualV2Observation(root string, matrix agentqualification.RealisticNextRunMatrix, candidate agentqualification.RealisticCandidate, caseID string) (realisticNextObservationRecord, bool, error) {
	var record realisticNextObservationRecord
	if err := readJSONFile(realisticNextObservationPath(root, candidate.ConfigurationID, caseID), &record); err != nil {
		if os.IsNotExist(err) {
			return record, false, nil
		}
		return record, false, err
	}
	if record.CandidateID != candidate.ConfigurationID || record.CaseID != caseID || record.Status == "" {
		return record, false, fmt.Errorf("completed residual observation identity drifted")
	}
	if record.Status == "skipped" {
		if record.Run != nil || record.SkipReason == "" {
			return record, false, fmt.Errorf("skipped residual observation is malformed")
		}
		return record, true, nil
	}
	if record.Run == nil || record.Run.Controller.RunFamilyID != realisticNextRunFamilyID(matrix.MatrixID, candidate.ConfigurationID, caseID) || record.Status != record.Run.Controller.TerminalState || record.FailureClass != record.Run.Controller.FailureClass {
		return record, false, fmt.Errorf("completed residual observation drifted")
	}
	var claim realisticNextProviderClaim
	if err := readJSONFile(realisticNextClaimPath(root, candidate.ConfigurationID, caseID), &claim); err != nil {
		return record, false, fmt.Errorf("completed residual claim missing: %w", err)
	}
	residualMatrix, err := agentqualification.CurrentRealisticResidualRunMatrixV2()
	if err != nil {
		return record, false, err
	}
	residualMatrix.Status = matrix.Status
	reservation, err := realisticResidualV2WorstCaseCostMicros(candidate, residualMatrix)
	if err != nil {
		return record, false, err
	}
	price, _ := realisticNextPinnedPrice(candidate)
	if claim.SchemaVersion != realisticNextClaimSchema || claim.MatrixID != matrix.MatrixID || claim.CandidateID != candidate.ConfigurationID || claim.CaseID != caseID || claim.RunFamilyID != record.Run.Controller.RunFamilyID || claim.Status != "provider_call_reserved" || claim.InputMicrosPerMillion != price.InputMicrosPerMillion || claim.GeneratedMicrosPerMillion != price.GeneratedMicrosPerMillion || claim.ReservedCostMicros != reservation {
		return record, false, fmt.Errorf("completed residual claim drifted")
	}
	if err := verifyRealisticNextSealedRun(root, candidate.ConfigurationID, caseID, *record.Run); err != nil {
		return record, false, err
	}
	return record, true, nil
}

func reflectDeepEqualObservation(left, right realisticNextObservationRecord) bool {
	return reflect.DeepEqual(left, right)
}

func haltRealisticResidualV2(path string, result realisticNextCampaignResult, reason string) (realisticNextCampaignResult, error) {
	result.Status, result.HaltReason = "global_halt", reason
	return finalizeRealisticResidualV2(path, result)
}

func finalizeRealisticResidualV2(path string, result realisticNextCampaignResult) (realisticNextCampaignResult, error) {
	if err := writeJSONExclusive(path, result); err != nil {
		return result, err
	}
	return result, nil
}
