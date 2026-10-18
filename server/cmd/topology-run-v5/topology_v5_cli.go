package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed topology_v5_cli.go
var topologyV5CampaignExecutorSource []byte

//go:embed main.go
var topologyMainSource []byte

const topologyV5CampaignFile = "campaign.json"

const (
	topologyReviewedExecuteTokens       int64 = 60000
	topologyReviewedExecuteGenerated    int64 = 14000
	topologyReviewedCorrectionTokens    int64 = 36000
	topologyReviewedCorrectionGenerated int64 = 8000
)

type topologyV5ObservationRuntime struct {
	ClaimID       string                                           `json:"claim_id"`
	Observation   agentqualification.TopologyExperimentObservation `json:"observation"`
	Workspace     agentqualification.RealisticCaseWorkspace        `json:"workspace"`
	Controller    agentqualification.RealisticControllerResult     `json:"controller"`
	StageReceipts []agentqualification.TopologyStageReceipt        `json:"stage_receipts,omitempty"`
}

func topologyV5Plan(artifactRoot string) (map[string]any, error) {
	definition, err := agentqualification.CurrentTopologyExperimentV5()
	if err != nil {
		return nil, err
	}
	state, err := loadTopologyV5Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	next, err := agentqualification.NextTopologyV5Observation(state, definition)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "plan_only", "provider_called": false, "provider_ready": agentqualification.TopologyExperimentV5ProviderReady(definition), "next_observation": next, "campaign": state}, nil
}

func executeTopologyV5Observation(ctx context.Context, artifactRoot, observationID string) (map[string]any, error) {
	if err := verifyPreservedTopologyCommandChain(); err != nil {
		return nil, err
	}
	if err := agentqualification.RequireCompleteTopologyFreezeV5(); err != nil {
		return nil, err
	}
	requiredExecutor, err := agentqualification.RequiredTopologyV5CampaignExecutorSHA256()
	if err != nil || sha256Hex(topologyV5CampaignExecutorSource) != requiredExecutor {
		return nil, fmt.Errorf("topology campaign executor is not the frozen reviewed source")
	}
	requiredMain, err := agentqualification.RequiredTopologyV5MainSHA256()
	if err != nil || sha256Hex(topologyMainSource) != requiredMain {
		return nil, fmt.Errorf("topology CLI admission source is not frozen")
	}
	definition, err := agentqualification.CurrentTopologyExperimentV5()
	if err != nil {
		return nil, err
	}
	if !agentqualification.TopologyExperimentV5ProviderReady(definition) {
		return nil, fmt.Errorf("topology provider execution is locked until one independent review and freeze")
	}
	state, err := loadTopologyV5Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	next, err := agentqualification.NextTopologyV5Observation(state, definition)
	if err != nil || next == nil {
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "batch_complete", "provider_called": false, "campaign": state}, nil
	}
	if next.ObservationID != observationID {
		return nil, fmt.Errorf("next frozen observation is %s", next.ObservationID)
	}
	if state.SpendAccountingStatus == "" {
		spend, spendErr := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
		if spendErr != nil {
			return nil, spendErr
		}
		state.BaselineProviderSpendMicros = spend.SpentMicros
		state.SpendAccountingStatus = "exact"
	}
	claimID, err := newRunFamilyID(definition.Executor.ConfigurationID, observationID)
	if err != nil {
		return nil, err
	}
	state, err = agentqualification.ClaimTopologyV5Observation(state, definition, observationID, claimID)
	if err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(filepath.Join(artifactRoot, topologyV5CampaignFile), state); err != nil {
		return nil, err
	}
	observationRoot := filepath.Join(artifactRoot, "observations", observationID)
	workspaceParent := filepath.Join(observationRoot, "workspaces")
	if err := os.MkdirAll(workspaceParent, 0o700); err != nil {
		return nil, err
	}
	workspace, err := agentqualification.PrepareRealisticCaseWorkspaceForRun(workspaceParent, next.CaseID, claimID)
	if err != nil {
		return nil, err
	}
	runtimeState := topologyV5ObservationRuntime{ClaimID: claimID, Observation: *next, Workspace: workspace}
	if err := writeJSONExclusive(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
		return nil, err
	}
	started := time.Now()
	result, runErr := runTopologyV5Arm(ctx, definition, *next, observationRoot, &runtimeState)
	result.EndToEndMillis = time.Since(started).Milliseconds()
	if runErr != nil {
		result = classifyTopologyExecutionError(result, runErr)
	}
	state, err = agentqualification.RecordTopologyV5Observation(state, definition, result)
	if err != nil {
		return nil, err
	}
	if err := writeOrVerifyJSONExclusive(filepath.Join(observationRoot, "result.json"), result); err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(filepath.Join(artifactRoot, topologyV5CampaignFile), state); err != nil {
		return nil, err
	}
	return map[string]any{"status": result.Outcome, "provider_called": result.ProviderCalls > 0, "observation": result, "campaign": state}, nil
}

func recoverTopologyV5Observation(ctx context.Context, artifactRoot string) (map[string]any, error) {
	definition, err := agentqualification.CurrentTopologyExperimentV5()
	if err != nil {
		return nil, err
	}
	state, err := loadTopologyV5Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	if state.ActiveClaim == "" {
		return map[string]any{"status": "nothing_to_recover", "provider_called": false, "campaign": state}, nil
	}
	if len(state.Observations) >= len(definition.ExecutionOrder) {
		return nil, fmt.Errorf("evidence corruption: active claim exists after final observation")
	}
	observation := definition.ExecutionOrder[len(state.Observations)]
	observationRoot := filepath.Join(artifactRoot, "observations", observation.ObservationID)
	resultPath := filepath.Join(observationRoot, "result.json")
	var sealed agentqualification.TopologyExperimentResult
	if err := readJSONFile(resultPath, &sealed); err == nil {
		state, err = agentqualification.RecordTopologyV5Observation(state, definition, sealed)
		if err != nil {
			return nil, err
		}
		if err := writeJSONAtomic(filepath.Join(artifactRoot, topologyV5CampaignFile), state); err != nil {
			return nil, err
		}
		return map[string]any{"status": "recovered_sealed_result", "provider_called": false, "observation": sealed, "campaign": state}, nil
	}
	var runtimeState topologyV5ObservationRuntime
	if err := readJSONFile(filepath.Join(observationRoot, "run-state.json"), &runtimeState); err != nil {
		return nil, fmt.Errorf("evidence corruption: stranded claim has no durable run state: %w", err)
	}
	stageReceipts, exact := reconstructTopologyV5CrashReceipts(observationRoot, runtimeState)
	ambiguousCost := int64(0)
	if !exact {
		spend, spendErr := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
		if spendErr != nil {
			return nil, spendErr
		}
		accountedSpend := state.BaselineProviderSpendMicros + state.TotalCostMicros
		if state.SpendAccountingStatus != "exact" || spend.SpentMicros < accountedSpend {
			return nil, fmt.Errorf("evidence corruption: provider spend baseline cannot reconcile")
		}
		ambiguousCost = spend.SpentMicros - accountedSpend
	}
	task, err := runtimeState.Workspace.Read("TASK.md")
	if err != nil {
		return nil, err
	}
	finalHash, err := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
	if err != nil {
		return nil, err
	}
	result := agentqualification.TopologyExperimentResult{
		ClaimID: state.ActiveClaim, ObservationID: observation.ObservationID, CaseID: observation.CaseID, Repeat: observation.Repeat, ArmID: observation.ArmID,
		Outcome: "invalid", FailureClass: "transport_or_harness_imperfection", InitialTreeSHA256: runtimeState.Workspace.StartingTreeSHA256,
		TaskSHA256: sha256Hex(task), FixtureSHA256: runtimeState.Workspace.Fixture.TreeSHA256, FinalTreeSHA256: finalHash,
		HiddenGradeSHA256: sha256Hex([]byte("crash-stranded-observation-invalid")), StageReceipts: stageReceipts, CostMicros: ambiguousCost, AccountingStatus: "ambiguous",
		PostBatchCorrection: "Crash interrupted the observation; preserved all durable artifacts, reconciled provider spend conservatively, made no retry, and excluded this observation from comparison.",
	}
	result = aggregateTopologyResult(result)
	if exact {
		result.AccountingStatus = "exact"
	}
	state, err = agentqualification.RecordTopologyV5Observation(state, definition, result)
	if err != nil {
		return nil, err
	}
	if err := writeJSONExclusive(resultPath, result); err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(filepath.Join(artifactRoot, topologyV5CampaignFile), state); err != nil {
		return nil, err
	}
	return map[string]any{"status": "recovered_as_invalid", "provider_called": false, "observation": result, "campaign": state}, nil
}

func reconstructTopologyV5CrashReceipts(observationRoot string, runtimeState topologyV5ObservationRuntime) ([]agentqualification.TopologyStageReceipt, bool) {
	receipts := append([]agentqualification.TopologyStageReceipt(nil), runtimeState.StageReceipts...)
	hasStage := func(stage string) bool {
		for _, receipt := range receipts {
			if receipt.Stage == stage {
				return true
			}
		}
		return false
	}
	if !hasStage("execute") && (runtimeState.Controller.ProviderCalls > 0 || runtimeState.Controller.InputTokens+runtimeState.Controller.OutputTokens+runtimeState.Controller.ReasoningTokens > 0 || runtimeState.Controller.CostMicros > 0) {
		receipts = append(receipts, topologyControllerStage("execute", runtimeState.ClaimID+":execute", runtimeState.Controller, 0, false))
	}
	exact := runtimeState.Controller.PendingProviderCall == nil && runtimeState.Controller.AmbiguousProviderCalls == 0
	if !hasStage("correction") {
		var correction agentqualification.RealisticControllerResult
		if err := readJSONFile(filepath.Join(observationRoot, "correction-controller.json"), &correction); err == nil {
			receipts = append(receipts, topologyControllerStage("correction", runtimeState.ClaimID+":correction", correction, 0, true))
			if correction.PendingProviderCall != nil || correction.AmbiguousProviderCalls > 0 {
				exact = false
			}
		}
	}
	for index, stageName := range []string{"review_1", "review_2"} {
		if hasStage(stageName) {
			continue
		}
		path := filepath.Join(observationRoot, fmt.Sprintf("review-%d", index+1), "provider-receipt.json")
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			exact = false
			continue
		}
		var sealed realisticRawProviderReceipt
		if json.Unmarshal(raw, &sealed) != nil {
			exact = false
			continue
		}
		parsed, _, parseErr := agentqualification.ParseOpenCodeRunEnvelope([]byte(sealed.JSONL), sealed.Export)
		if parseErr != nil || parsed.Usage.InputTokens == nil || parsed.Usage.OutputTokens == nil || parsed.Usage.ReasoningTokens == nil || parsed.Usage.ProviderReportedCostMicros == nil {
			exact = false
			continue
		}
		receipts = append(receipts, agentqualification.TopologyStageReceipt{RunID: runtimeState.ClaimID + fmt.Sprintf(":review-%d", index+1), Stage: stageName, ProviderCalls: 1, InputTokens: *parsed.Usage.InputTokens, OutputTokens: *parsed.Usage.OutputTokens, ReasoningTokens: *parsed.Usage.ReasoningTokens, CostMicros: *parsed.Usage.ProviderReportedCostMicros})
	}
	return receipts, exact
}

func verifyPreservedTopologyCommandChain() error {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("cannot locate topology-v5 admission source")
	}
	return verifyPreservedTopologyCommandChainAt(filepath.Join(filepath.Dir(current), ".."))
}

func verifyPreservedTopologyCommandChainAt(commandRoot string) error {
	v4, err := agentqualification.CurrentTopologyFreezeV4()
	if err != nil {
		return fmt.Errorf("topology-v4 frozen components drifted: %w", err)
	}
	v3, err := agentqualification.CurrentTopologyFreezeV3()
	if err != nil {
		return fmt.Errorf("topology-v3 frozen components drifted: %w", err)
	}
	v2, err := agentqualification.CurrentTopologyFreezeV2()
	if err != nil {
		return fmt.Errorf("topology-v2 frozen components drifted: %w", err)
	}
	v1, err := agentqualification.CurrentTopologyFreezeBundle()
	if err != nil {
		return fmt.Errorf("topology-v1 frozen components drifted: %w", err)
	}
	checks := []struct {
		label, commandDir, executorFile, executorSHA256, mainSHA256 string
	}{
		{"topology-v4", filepath.Join(commandRoot, "topology-run-v4"), "topology_v4_cli.go", v4.CampaignExecutorSHA256, v4.MainSHA256},
		{"topology-v3", filepath.Join(commandRoot, "topology-run-v3"), "topology_v3_cli.go", v3.CampaignExecutorSHA256, v3.MainSHA256},
		{"topology-v2", filepath.Join(commandRoot, "topology-run-v2"), "topology_v2_cli.go", v2.CampaignExecutorSHA256, v2.MainSHA256},
		{"topology-v1", filepath.Join(commandRoot, "screening-run"), "topology_cli.go", v1.CampaignExecutorSHA256, v1.MainSHA256},
	}
	for _, check := range checks {
		if err := verifyTopologyCommandSourcesAt(check.commandDir, check.executorFile, check.executorSHA256, check.mainSHA256); err != nil {
			return fmt.Errorf("%s frozen command source drifted: %w", check.label, err)
		}
	}
	return nil
}

func verifyTopologyCommandSourcesAt(commandDir, executorFile, executorExpected, mainExpected string) error {
	for path, expected := range map[string]string{filepath.Join(commandDir, executorFile): executorExpected, filepath.Join(commandDir, "main.go"): mainExpected} {
		raw, readErr := os.ReadFile(path)
		if readErr != nil || sha256Hex(raw) != expected {
			return fmt.Errorf("%s", path)
		}
	}
	return nil
}

func runTopologyV5Arm(ctx context.Context, definition agentqualification.TopologyExperimentDefinition, observation agentqualification.TopologyExperimentObservation, observationRoot string, runtimeState *topologyV5ObservationRuntime) (agentqualification.TopologyExperimentResult, error) {
	result := agentqualification.TopologyExperimentResult{ClaimID: runtimeState.ClaimID, ObservationID: observation.ObservationID, CaseID: observation.CaseID, Repeat: observation.Repeat, ArmID: observation.ArmID, InitialTreeSHA256: runtimeState.Workspace.StartingTreeSHA256, FixtureSHA256: runtimeState.Workspace.Fixture.TreeSHA256, AccountingStatus: "exact"}
	task, err := runtimeState.Workspace.Read("TASK.md")
	if err != nil {
		return result, err
	}
	result.TaskSHA256 = sha256Hex(task)
	arm := topologyArmByID(definition, observation.ArmID)
	spendSnapshot, spendErr := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if spendErr != nil {
		if strings.Contains(strings.ToLower(spendErr.Error()), "credential") {
			return aggregateTopologyResult(result), fmt.Errorf("credential failure: %w", spendErr)
		}
		return aggregateTopologyResult(result), fmt.Errorf("provider preflight transport failure: %w", spendErr)
	}
	// The shared provider performs a conservative $0.25 pre-call reservation.
	// Add that reservation outside this arm's actual-cost ceiling; the topology
	// stage and campaign ledgers still enforce the exact paid totals afterward.
	provider := openCodeRealisticProvider{candidate: definition.Executor, baselineSpendMicros: spendSnapshot.SpentMicros, maxSpendMicros: arm.CostCeilingMicros + 250000}
	checkpointPath := filepath.Join(observationRoot, "execute-controller.json")
	checkpoint := func(controller agentqualification.RealisticControllerResult) error {
		runtimeState.Controller = controller
		if err := writeJSONAtomic(checkpointPath, controller); err != nil {
			return err
		}
		return writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState)
	}
	executeTokenCap, executeGeneratedCap := arm.TotalTokenReservation, arm.GeneratedTokenCeiling
	if arm.Reviewed {
		executeTokenCap, executeGeneratedCap = topologyReviewedExecuteTokens, topologyReviewedExecuteGenerated
	}
	stageStarted := time.Now()
	controller, controllerErr := agentqualification.RunTopologyCandidateV5(ctx, definition, runtimeState.Workspace, runtimeState.ClaimID+":execute", task, 15, executeTokenCap, executeGeneratedCap, provider, checkpoint)
	runtimeState.Controller = controller
	result.StageReceipts = append(result.StageReceipts, topologyControllerStage("execute", runtimeState.ClaimID+":execute", controller, time.Since(stageStarted), false))
	runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
	if writeErr := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); writeErr != nil {
		return result, writeErr
	}
	if controllerErr != nil && errors.Is(controllerErr, agentqualification.ErrTopologyStageBudgetExceeded) {
		result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
		if err != nil {
			return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
		}
		return aggregateTopologyResult(result), controllerErr
	}
	if controllerErr != nil && isTopologyGlobalError(controllerErr) {
		return aggregateTopologyResult(result), controllerErr
	}
	if isTopologyV5ExecutorProtocolFailure(currentControllerResult(controller, controllerErr)) {
		result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
		if err != nil {
			return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
		}
		result.HiddenGradeSHA256 = sha256Hex([]byte("not_run:executor_response_or_tool_protocol_failure"))
		result.Outcome, result.FailureClass = "fail", "response_or_tool_protocol_failure"
		return aggregateTopologyResult(result), nil
	}
	currentController := controller
	currentEvidenceRoot := filepath.Join(observationRoot, "evidence", "execute")
	currentReceipt, _, sealErr := sealTopologyControllerIfCompleted(currentEvidenceRoot, runtimeState.Workspace, definition.Executor, currentController)
	if sealErr != nil {
		return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", sealErr)
	}
	finalReviewPass := true
	if arm.Reviewed {
		request, requestErr := topologyReviewRequestFor(observation, runtimeState.Workspace, currentController, currentReceipt, currentEvidenceRoot, runtimeState.ClaimID+":review-1")
		if requestErr != nil {
			return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", requestErr)
		}
		verdict, reviewStage, reviewErr := executeTopologyReviewCall(ctx, definition, request, filepath.Join(observationRoot, "review-1"))
		result.StageReceipts = append(result.StageReceipts, reviewStage)
		runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
		if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
			return aggregateTopologyResult(result), err
		}
		if reviewErr != nil {
			if strings.Contains(strings.ToLower(reviewErr.Error()), "reviewer competence failure") {
				finalReviewPass = false
			} else {
				return aggregateTopologyResult(result), reviewErr
			}
		}
		if reviewErr == nil {
			finalReviewPass = verdict.Verdict == "PASS"
		}
		if reviewErr == nil && verdict.Verdict == "FAIL" {
			packet, packetErr := agentqualification.BuildTopologyCorrectionPacket(request, verdict)
			if packetErr != nil {
				return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", packetErr)
			}
			correctionTask, packetErr := agentqualification.TopologyCorrectionTask(packet)
			if packetErr != nil {
				return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", packetErr)
			}
			if err := writeJSONExclusive(filepath.Join(observationRoot, "correction-packet.json"), packet); err != nil {
				return aggregateTopologyResult(result), err
			}
			correctionRunID := runtimeState.ClaimID + ":correction"
			correctionCheckpoint := func(value agentqualification.RealisticControllerResult) error {
				return writeJSONAtomic(filepath.Join(observationRoot, "correction-controller.json"), value)
			}
			stageStarted = time.Now()
			correction, correctionErr := agentqualification.RunTopologyCandidateV5(ctx, definition, runtimeState.Workspace, correctionRunID, correctionTask, 8, topologyReviewedCorrectionTokens, topologyReviewedCorrectionGenerated, provider, correctionCheckpoint)
			result.StageReceipts = append(result.StageReceipts, topologyControllerStage("correction", correctionRunID, correction, time.Since(stageStarted), true))
			runtimeState.Controller = correction
			runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
			if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
				return aggregateTopologyResult(result), err
			}
			if correctionErr != nil && isTopologyGlobalError(correctionErr) {
				return aggregateTopologyResult(result), correctionErr
			}
			if isTopologyV5ExecutorProtocolFailure(currentControllerResult(correction, correctionErr)) {
				result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
				if err != nil {
					return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
				}
				result.HiddenGradeSHA256 = sha256Hex([]byte("not_run:correction_response_or_tool_protocol_failure"))
				result.Outcome, result.FailureClass = "fail", "response_or_tool_protocol_failure"
				return aggregateTopologyResult(result), nil
			}
			currentController = correction
			currentEvidenceRoot = filepath.Join(observationRoot, "evidence", "correction")
			currentReceipt, _, sealErr = sealTopologyControllerIfCompleted(currentEvidenceRoot, runtimeState.Workspace, definition.Executor, currentController)
			if sealErr != nil {
				return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", sealErr)
			}
			finalRequest, requestErr := topologyReviewRequestFor(observation, runtimeState.Workspace, currentController, currentReceipt, currentEvidenceRoot, runtimeState.ClaimID+":review-2")
			if requestErr != nil {
				return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", requestErr)
			}
			finalVerdict, finalStage, finalErr := executeTopologyReviewCall(ctx, definition, finalRequest, filepath.Join(observationRoot, "review-2"))
			result.StageReceipts = append(result.StageReceipts, finalStage)
			runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
			if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
				return aggregateTopologyResult(result), err
			}
			if finalErr != nil {
				if strings.Contains(strings.ToLower(finalErr.Error()), "reviewer competence failure") {
					finalReviewPass = false
				} else {
					return aggregateTopologyResult(result), finalErr
				}
			} else {
				finalReviewPass = finalVerdict.Verdict == "PASS"
			}
		}
	}
	group, err := agentqualification.CurrentRealisticWorkflowGroup()
	if err != nil {
		return aggregateTopologyResult(result), err
	}
	manifest, err := agentqualification.CurrentRealisticFixtureManifest()
	if err != nil {
		return aggregateTopologyResult(result), err
	}
	grade, err := agentqualification.GradeVerifiedRealisticWorkspace(runtimeState.Workspace.CaseID, runtimeState.Workspace.Root, runtimeState.Workspace.InitialCopy, currentEvidenceRoot, agentqualification.RealisticIndependentEvidence{ToolReceipt: currentReceipt}, group, manifest)
	if err != nil {
		return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
	}
	sealedGrade := agentqualification.BlindGrade{SchemaVersion: "silver-mediums.realistic-grade.v1", CaseID: runtimeState.Workspace.CaseID, GraderVersion: "realistic-hidden-verifier-v1", Verdict: grade.Verdict, FailedChecks: grade.FailedChecks}
	gradeRaw, _ := json.MarshalIndent(sealedGrade, "", "  ")
	if err := writeJSONExclusive(filepath.Join(observationRoot, "hidden-grade.json"), sealedGrade); err != nil {
		return aggregateTopologyResult(result), err
	}
	result.HiddenGradeSHA256 = sha256Hex(gradeRaw)
	result.StageReceipts = append(result.StageReceipts, agentqualification.TopologyStageReceipt{RunID: runtimeState.ClaimID + ":hidden-grade", Stage: "hidden_grade"})
	result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
	if err != nil {
		return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
	}
	result.Outcome = "pass"
	if grade.Verdict != "PASS" || !finalReviewPass || currentController.TerminalState != "completed" || controllerErr != nil {
		result.Outcome, result.FailureClass = "fail", "competence_failure"
	}
	result = aggregateTopologyResult(result)
	if result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros {
		result.Outcome, result.FailureClass = "global_halt", "uncontrolled_spending"
	}
	return result, nil
}

func currentControllerResult(controller agentqualification.RealisticControllerResult, controllerErr error) agentqualification.RealisticControllerResult {
	if controllerErr != nil && controller.TerminalState == "" {
		controller.TerminalState = "failed"
	}
	return controller
}

func isTopologyV5ExecutorProtocolFailure(controller agentqualification.RealisticControllerResult) bool {
	return controller.TerminalState == "failed" && controller.FailureClass == "response_or_tool_protocol_failure"
}

func sealTopologyControllerIfCompleted(evidenceRoot string, workspace agentqualification.RealisticCaseWorkspace, candidate agentqualification.RealisticCandidate, controller agentqualification.RealisticControllerResult) (*agentqualification.RealisticExecutionReceipt, string, error) {
	if controller.TerminalState != "completed" {
		return nil, "", nil
	}
	return agentqualification.SealRealisticExecutionReceipt(evidenceRoot, workspace, candidate, controller)
}

func topologyReviewRequestFor(observation agentqualification.TopologyExperimentObservation, workspace agentqualification.RealisticCaseWorkspace, controller agentqualification.RealisticControllerResult, receipt *agentqualification.RealisticExecutionReceipt, evidenceRoot, runID string) (agentqualification.TopologyReviewRequest, error) {
	paths, err := workspace.Paths()
	if err != nil {
		return agentqualification.TopologyReviewRequest{}, err
	}
	evidenceIDs := []string{}
	evidenceItems := []agentqualification.TopologyEvidenceItem{}
	seenEvidence := map[string]struct{}{}
	appendEvidence := func(id, kind, content string) {
		if id == "" {
			return
		}
		if _, exists := seenEvidence[id]; exists {
			return
		}
		seenEvidence[id] = struct{}{}
		if len(content) > 12000 {
			content = content[:12000] + "\n[bounded by harness]"
		}
		evidenceIDs = append(evidenceIDs, id)
		evidenceItems = append(evidenceItems, agentqualification.TopologyEvidenceItem{EvidenceID: id, Kind: kind, Content: content, SHA256: sha256Hex([]byte(content))})
	}
	if receipt != nil {
		for _, item := range receipt.Evidence {
			raw, readErr := os.ReadFile(filepath.Join(evidenceRoot, filepath.FromSlash(item.ArtifactPath)))
			if readErr != nil {
				return agentqualification.TopologyReviewRequest{}, readErr
			}
			appendEvidence(item.EvidenceID, item.Kind, string(raw))
		}
	}
	for _, item := range controller.ToolResults {
		appendEvidence(item.OutputHash, "controller_tool_result", item.Output)
	}
	for _, event := range controller.ToolEvents {
		for _, relative := range event.TouchedPaths {
			raw, readErr := workspace.Read(relative)
			if readErr == nil {
				appendEvidence("artifact:"+filepath.ToSlash(relative), "produced_artifact", string(raw))
			}
		}
	}
	if len(evidenceIDs) == 0 {
		appendEvidence("no_executor_evidence", "controller_state", "No inspectable executor evidence was produced. terminal_state="+controller.TerminalState+" failure_class="+controller.FailureClass)
	}
	treeHash, err := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(workspace.Root), ".")
	if err != nil {
		return agentqualification.TopologyReviewRequest{}, err
	}
	summary := strings.TrimSpace(controller.FinalSummary)
	if summary == "" {
		summary = "Executor did not complete; terminal state=" + controller.TerminalState + ", failure class=" + controller.FailureClass
	}
	return agentqualification.BuildTopologyReviewRequest(observation.ObservationID, runID, observation.CaseID, treeHash, summary, paths, evidenceIDs, evidenceItems)
}

func executeTopologyReviewCall(ctx context.Context, definition agentqualification.TopologyExperimentDefinition, request agentqualification.TopologyReviewRequest, root string) (agentqualification.TopologyReviewVerdict, agentqualification.TopologyStageReceipt, error) {
	stage := agentqualification.TopologyStageReceipt{RunID: request.ReviewRunID, Stage: strings.Replace(request.ReviewRunID[strings.LastIndex(request.ReviewRunID, ":")+1:], "review-", "review_", 1)}
	started := time.Now()
	_, spendErr := readOpenRouterSpendSnapshot(ctx, time.Now().UTC())
	if spendErr != nil {
		if strings.Contains(strings.ToLower(spendErr.Error()), "credential") {
			return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("credential failure: %w", spendErr)
		}
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("provider preflight transport failure: %w", spendErr)
	}
	plan, err := agentqualification.BuildOpenCodeTopologyReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, request)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("evidence corruption: %w", err)
	}
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	runtimeState.Env = replaceEnv(runtimeState.Env, "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX", "4000")
	resolved, err := runtimeState.resolvedConfig(ctx)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	routing, err := topologyOpenCodeRoutingEvidenceFor(plan, resolved)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("evidence corruption: %w", err)
	}
	rawJSONL, rawExport, err := topologyRunOnce(runtimeState, plan)
	if err != nil {
		partialReceipt, _ := json.Marshal(map[string]any{"jsonl": string(rawJSONL), "export_raw": string(rawExport), "routing": routing, "transport_error": err.Error()})
		if len(partialReceipt) > 0 {
			_ = os.MkdirAll(root, 0o700)
			_ = writeBytesExclusive(filepath.Join(root, "provider-receipt.json"), partialReceipt)
		}
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	parsed, candidateJSON, sealedStage, err := sealAndParseTopologyReviewResponse(root, definition, routing, rawJSONL, rawExport, stage, started)
	stage = sealedStage
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	_ = parsed
	if stage.InputTokens+stage.OutputTokens+stage.ReasoningTokens > definition.ReviewerTotalTokenReservation || stage.OutputTokens+stage.ReasoningTokens > definition.ReviewerGeneratedTokenCeiling {
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("uncontrolled spending: reviewer token ceiling breached")
	}
	verdict, err := agentqualification.ParseTopologyReviewVerdict(candidateJSON)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("reviewer competence failure: %v", err)
	}
	if validationErr := agentqualification.ValidateTopologyReviewVerdict(request, verdict); validationErr != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("reviewer competence failure: %v", validationErr)
	}
	if err := writeJSONExclusive(filepath.Join(root, "verdict.json"), verdict); err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	return verdict, stage, nil
}

func sealAndParseTopologyReviewResponse(root string, definition agentqualification.TopologyExperimentDefinition, routing realisticOpenCodeRoutingEvidence, rawJSONL, rawExport []byte, stage agentqualification.TopologyStageReceipt, started time.Time) (agentqualification.ProviderExecutionResult, []byte, agentqualification.TopologyStageReceipt, error) {
	rawReceipt, err := json.Marshal(realisticRawProviderReceipt{JSONL: string(rawJSONL), Export: append(json.RawMessage(nil), rawExport...), Routing: routing})
	if err != nil {
		return agentqualification.ProviderExecutionResult{}, nil, stage, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return agentqualification.ProviderExecutionResult{}, nil, stage, err
	}
	if err := writeBytesExclusive(filepath.Join(root, "provider-receipt.json"), rawReceipt); err != nil {
		return agentqualification.ProviderExecutionResult{}, nil, stage, err
	}
	parsed, candidateJSON, err := agentqualification.ParseOpenCodeRunEnvelope(rawJSONL, rawExport)
	if err != nil {
		return parsed, nil, stage, fmt.Errorf("evidence corruption: %w", err)
	}
	if parsed.Usage.InputTokens == nil || parsed.Usage.OutputTokens == nil || parsed.Usage.ReasoningTokens == nil || parsed.Usage.ProviderReportedCostMicros == nil {
		return parsed, candidateJSON, stage, fmt.Errorf("evidence corruption: reviewer usage is incomplete")
	}
	stage.ProviderCalls = 1
	stage.InputTokens, stage.OutputTokens, stage.ReasoningTokens, stage.CostMicros = *parsed.Usage.InputTokens, *parsed.Usage.OutputTokens, *parsed.Usage.ReasoningTokens, *parsed.Usage.ProviderReportedCostMicros
	stage.WallTimeMillis = time.Since(started).Milliseconds()
	if parsed.ReportedModel != definition.Reviewer.Model || parsed.ReportedServingProvider != "" && !strings.EqualFold(parsed.ReportedServingProvider, definition.Reviewer.ServingProvider) {
		return parsed, candidateJSON, stage, fmt.Errorf("evidence corruption: reviewer route drifted")
	}
	return parsed, candidateJSON, stage, nil
}

func topologyOpenCodeRoutingEvidenceFor(plan agentqualification.OpenCodeScreeningPlan, resolvedConfig []byte) (realisticOpenCodeRoutingEvidence, error) {
	validationPlan := plan
	validationPlan.Transport = agentqualification.OpenCodeToolFreeTransportV5
	resolvedHash, err := agentqualification.ValidateOpenCodeResolvedConfig(validationPlan, resolvedConfig)
	if err != nil {
		return realisticOpenCodeRoutingEvidence{}, err
	}
	inlineHash := sha256.Sum256(plan.InlineConfig)
	if hex.EncodeToString(inlineHash[:]) != plan.InlineConfigSHA256 || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks || strings.TrimSpace(plan.ServingProvider) == "" {
		return realisticOpenCodeRoutingEvidence{}, fmt.Errorf("topology route plan is not hash-bound and no-fallback")
	}
	return realisticOpenCodeRoutingEvidence{ResolvedConfigSHA256: resolvedHash, InlineConfigSHA256: plan.InlineConfigSHA256, DirectProvider: "openrouter", DirectProviderEvidence: "opencode_export_provider_id", RequestedUpstreamProvider: plan.ServingProvider, ProviderOrder: []string{plan.ServingProvider}, AllowFallbacks: false, ActualUpstreamProviderState: "unavailable_from_provider_metadata"}, nil
}

func topologyRunOnce(runtimeState openCodeCommandRuntime, plan agentqualification.OpenCodeScreeningPlan) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := runtimeState.command(ctx, "--pure", "run", "--format", "json", "--model", plan.Model, "--variant", plan.Effort, "--agent", "qualification", "--title", "qualification-"+plan.CandidateID, "--dir", runtimeState.WorkDir)
	command.Stdin = strings.NewReader(plan.Prompt)
	rawJSONL, err := command.Output()
	if err != nil {
		return rawJSONL, nil, err
	}
	sessionID, err := sessionIDFromOpenCodeJSONL(rawJSONL)
	if err != nil {
		return rawJSONL, nil, err
	}
	rawExport, err := runtimeState.command(ctx, "--pure", "export", sessionID).Output()
	return rawJSONL, rawExport, err
}

func topologyControllerStage(stage, runID string, controller agentqualification.RealisticControllerResult, duration time.Duration, correction bool) agentqualification.TopologyStageReceipt {
	return agentqualification.TopologyStageReceipt{RunID: runID, Stage: stage, ProviderCalls: controller.ProviderCalls, InputTokens: controller.InputTokens, OutputTokens: controller.OutputTokens, ReasoningTokens: controller.ReasoningTokens, CostMicros: controller.CostMicros, WallTimeMillis: duration.Milliseconds(), Correction: correction}
}

func aggregateTopologyResult(result agentqualification.TopologyExperimentResult) agentqualification.TopologyExperimentResult {
	result.ProviderCalls, result.TotalTokens, result.CostMicros = 0, 0, 0
	for _, stage := range result.StageReceipts {
		result.ProviderCalls += stage.ProviderCalls
		result.TotalTokens += stage.InputTokens + stage.OutputTokens + stage.ReasoningTokens
		result.CostMicros += stage.CostMicros
	}
	return result
}

func classifyTopologyExecutionError(result agentqualification.TopologyExperimentResult, err error) agentqualification.TopologyExperimentResult {
	result = aggregateTopologyResult(result)
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, agentqualification.ErrRealisticBudgetBreach):
		result.Outcome, result.FailureClass = "global_halt", "uncontrolled_spending"
	case errors.Is(err, agentqualification.ErrTopologyStageBudgetExceeded):
		result.Outcome, result.FailureClass = "invalid", "stage_budget_exhaustion"
		result.PostBatchCorrection = "The paid response that crossed the stage reservation was sealed and counted; the next turn was stopped before another provider call. This observation is excluded without retry."
		result.AccountingStatus = "exact"
	case strings.Contains(message, "credential failure"):
		result.Outcome, result.FailureClass = "global_halt", "credential_failure"
	case strings.Contains(message, "uncontrolled spending"):
		result.Outcome, result.FailureClass = "global_halt", "uncontrolled_spending"
	case strings.Contains(message, "unsafe external"):
		result.Outcome, result.FailureClass = "global_halt", "unsafe_external_action"
	case strings.Contains(message, "evidence corruption"):
		result.Outcome, result.FailureClass = "global_halt", "evidence_corruption"
	case strings.Contains(message, "competence failure"):
		result.Outcome, result.FailureClass = "fail", "competence_failure"
	default:
		result.Outcome, result.FailureClass, result.PostBatchCorrection, result.AccountingStatus = "invalid", "transport_or_harness_imperfection", err.Error(), "ambiguous"
	}
	if result.AccountingStatus == "" {
		result.AccountingStatus = "exact"
	}
	if result.Outcome == "global_halt" && result.FailureClass == "credential_failure" {
		result.AccountingStatus = "ambiguous"
	}
	if result.FinalTreeSHA256 == "" && result.InitialTreeSHA256 != "" {
		result.FinalTreeSHA256 = result.InitialTreeSHA256
	}
	if result.HiddenGradeSHA256 == "" {
		result.HiddenGradeSHA256 = sha256Hex([]byte("unavailable:" + err.Error()))
	}
	return result
}

func reclassifyTopologyPreventiveBudgetHalt(artifactRoot string) (map[string]any, error) {
	definition, err := agentqualification.CurrentTopologyExperimentV5()
	if err != nil {
		return nil, err
	}
	state, err := loadTopologyV5Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	if state.GlobalHaltReason != "uncontrolled_spending" || len(state.Observations) == 0 {
		return nil, fmt.Errorf("campaign has no preventive budget halt to reclassify")
	}
	index := len(state.Observations) - 1
	prior := state.Observations[index]
	arm := topologyArmByID(definition, prior.ArmID)
	if prior.Outcome != "global_halt" || prior.FailureClass != "uncontrolled_spending" || prior.TotalTokens >= arm.TotalTokenReservation || prior.CostMicros >= arm.CostCeilingMicros {
		return nil, fmt.Errorf("record is a genuine or unrecognized budget halt")
	}
	corrected := prior
	corrected.Outcome = "invalid"
	corrected.FailureClass = "preventive_budget_reservation_harness_imperfection"
	corrected.PostBatchCorrection = "Shared provider reservation was counted as actual arm spend. Observation preserved without retry; later observations separate the $0.25 pre-call reserve from exact paid arm limits."
	correction := map[string]any{"schema_version": "silver-mediums.topology-classification-correction.v1", "observation_id": prior.ObservationID, "prior": prior, "corrected": corrected, "provider_called": false}
	root := filepath.Join(artifactRoot, "observations", prior.ObservationID)
	if err := writeJSONExclusive(filepath.Join(root, "classification-correction.json"), correction); err != nil {
		return nil, err
	}
	state.Observations[index] = corrected
	state.GlobalHaltReason = ""
	state.PostBatchCorrections = append(state.PostBatchCorrections, prior.ObservationID+":"+corrected.PostBatchCorrection)
	state.Revision++
	if err := writeJSONAtomic(filepath.Join(artifactRoot, topologyV5CampaignFile), state); err != nil {
		return nil, err
	}
	return map[string]any{"status": "reclassified_invalid_no_retry", "provider_called": false, "correction": correction, "campaign": state}, nil
}

func reclassifyTopologyPrecallHarnessHalt(artifactRoot string) (map[string]any, error) {
	definition, err := agentqualification.CurrentTopologyExperimentV5()
	if err != nil {
		return nil, err
	}
	state, err := loadTopologyV5Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	if state.GlobalHaltReason != "evidence_corruption" || len(state.Observations) == 0 {
		return nil, fmt.Errorf("campaign has no pre-call harness halt to reclassify")
	}
	index := len(state.Observations) - 1
	prior := state.Observations[index]
	arm := topologyArmByID(definition, prior.ArmID)
	lastStage := prior.StageReceipts[len(prior.StageReceipts)-1]
	if prior.Outcome != "global_halt" || prior.FailureClass != "evidence_corruption" || !arm.Reviewed || lastStage.Stage != "review_1" || lastStage.ProviderCalls != 0 || prior.TotalTokens >= arm.TotalTokenReservation || prior.CostMicros >= arm.CostCeilingMicros {
		return nil, fmt.Errorf("record is not the validated pre-call topology transport registration defect")
	}
	corrected := prior
	corrected.Outcome = "invalid"
	corrected.FailureClass = "topology_transport_registration_harness_imperfection"
	corrected.PostBatchCorrection = "Topology reviewer configuration validated correctly, but the generic route validator rejected the new named transport before any reviewer call. Observation preserved without retry; later reviewed observations use the same config validator through its registered tool-free validation identity."
	correction := map[string]any{"schema_version": "silver-mediums.topology-classification-correction.v1", "observation_id": prior.ObservationID, "prior": prior, "corrected": corrected, "provider_called": false}
	root := filepath.Join(artifactRoot, "observations", prior.ObservationID)
	if err := writeJSONExclusive(filepath.Join(root, "transport-classification-correction.json"), correction); err != nil {
		return nil, err
	}
	state.Observations[index] = corrected
	state.GlobalHaltReason = ""
	state.PostBatchCorrections = append(state.PostBatchCorrections, prior.ObservationID+":"+corrected.PostBatchCorrection)
	state.Revision++
	if err := writeJSONAtomic(filepath.Join(artifactRoot, topologyV5CampaignFile), state); err != nil {
		return nil, err
	}
	return map[string]any{"status": "reclassified_invalid_no_retry", "provider_called": false, "correction": correction, "campaign": state}, nil
}

func loadTopologyV5Campaign(root string) (agentqualification.TopologyCampaignState, error) {
	var state agentqualification.TopologyCampaignState
	path := filepath.Join(root, topologyV5CampaignFile)
	if err := readJSONFile(path, &state); err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	return state, nil
}

func topologyArmByID(definition agentqualification.TopologyExperimentDefinition, armID string) agentqualification.TopologyExperimentArm {
	for _, arm := range definition.Arms {
		if arm.ArmID == armID {
			return arm
		}
	}
	return agentqualification.TopologyExperimentArm{}
}

func replaceEnv(values []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(values)+1)
	for _, item := range values {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func writeBytesExclusive(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func isTopologyGlobalError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return errors.Is(err, agentqualification.ErrRealisticBudgetBreach) || strings.Contains(message, "credential") || strings.Contains(message, "evidence integrity")
}

func sha256Hex(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
