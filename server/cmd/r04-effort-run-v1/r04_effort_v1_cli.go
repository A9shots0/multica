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

//go:embed r04_effort_v1_cli.go
var r04EffortV1CampaignExecutorSource []byte

//go:embed main.go
var topologyMainSource []byte

const r04EffortV1CampaignFile = "campaign.json"

const (
	topologyReviewedExecuteTokens       int64 = 60000
	topologyReviewedExecuteGenerated    int64 = 12000
	topologyReviewedCorrectionTokens    int64 = 40000
	topologyReviewedCorrectionGenerated int64 = 10000
)

var r04EffortV1ReadSpendSnapshot = readOpenRouterSpendSnapshot

type r04EffortV1ObservationRuntime struct {
	ClaimID       string                                           `json:"claim_id"`
	Observation   agentqualification.TopologyExperimentObservation `json:"observation"`
	Workspace     agentqualification.RealisticCaseWorkspace        `json:"workspace"`
	Controller    agentqualification.RealisticControllerResult     `json:"controller"`
	StageReceipts []agentqualification.TopologyStageReceipt        `json:"stage_receipts,omitempty"`
	PendingReview *r04EffortV1PendingReview                        `json:"pending_review,omitempty"`
}

type r04EffortV1PendingReview struct {
	RunID string `json:"run_id"`
	Stage string `json:"stage"`
}

type topologyReviewerTransportError struct {
	cause                 error
	accountingTrustworthy bool
}

func (err topologyReviewerTransportError) Error() string { return err.cause.Error() }
func (err topologyReviewerTransportError) Unwrap() error { return err.cause }

func r04EffortV1Plan(artifactRoot string) (map[string]any, error) {
	definition, err := agentqualification.CurrentR04EffortExperimentV1()
	if err != nil {
		return nil, err
	}
	prevalidation, err := agentqualification.PrevalidateR04EffortV1Routes(definition)
	if err != nil {
		return nil, err
	}
	state, err := loadR04EffortV1Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	next, err := agentqualification.NextR04EffortV1Observation(state, definition)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "plan_only", "provider_called": false, "provider_ready": agentqualification.R04EffortExperimentV1ProviderReady(definition), "route_prevalidation": prevalidation, "next_observation": next, "campaign": state}, nil
}

func executeR04EffortV1Observation(ctx context.Context, artifactRoot, observationID string) (map[string]any, error) {
	if err := verifyPreservedTopologyCommandChain(); err != nil {
		return nil, err
	}
	if err := agentqualification.RequireCompleteR04EffortFreezeV1(); err != nil {
		return nil, err
	}
	requiredExecutor, err := agentqualification.RequiredR04EffortV1CampaignExecutorSHA256()
	if err != nil || sha256Hex(r04EffortV1CampaignExecutorSource) != requiredExecutor {
		return nil, fmt.Errorf("topology campaign executor is not the frozen reviewed source")
	}
	requiredMain, err := agentqualification.RequiredR04EffortV1MainSHA256()
	if err != nil || sha256Hex(topologyMainSource) != requiredMain {
		return nil, fmt.Errorf("topology CLI admission source is not frozen")
	}
	definition, err := agentqualification.CurrentR04EffortExperimentV1()
	if err != nil {
		return nil, err
	}
	if !agentqualification.R04EffortExperimentV1ProviderReady(definition) {
		return nil, fmt.Errorf("topology provider execution is locked until one independent review and freeze")
	}
	state, err := loadR04EffortV1Campaign(artifactRoot)
	if err != nil {
		return nil, err
	}
	next, err := agentqualification.NextR04EffortV1Observation(state, definition)
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
	state, err = prevalidateAndClaimR04EffortV1Observation(state, definition, observationID, claimID, agentqualification.PrevalidateR04EffortV1Routes)
	if err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(filepath.Join(artifactRoot, r04EffortV1CampaignFile), state); err != nil {
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
	runtimeState := r04EffortV1ObservationRuntime{ClaimID: claimID, Observation: *next, Workspace: workspace}
	if err := writeJSONExclusive(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
		return nil, err
	}
	started := time.Now()
	result, runErr := runR04EffortV1Arm(ctx, definition, *next, observationRoot, &runtimeState)
	result.EndToEndMillis = time.Since(started).Milliseconds()
	if runErr != nil {
		result = classifyTopologyExecutionError(result, runErr)
	}
	state, err = agentqualification.RecordR04EffortV1Observation(state, definition, result)
	if err != nil {
		return nil, err
	}
	if err := writeOrVerifyJSONExclusive(filepath.Join(observationRoot, "result.json"), result); err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(filepath.Join(artifactRoot, r04EffortV1CampaignFile), state); err != nil {
		return nil, err
	}
	return map[string]any{"status": result.Outcome, "provider_called": result.ProviderCalls > 0, "observation": result, "campaign": state}, nil
}

type r04EffortV1Prevalidator func(agentqualification.TopologyExperimentDefinition) (agentqualification.R04EffortV1Prevalidation, error)

func prevalidateAndClaimR04EffortV1Observation(state agentqualification.TopologyCampaignState, definition agentqualification.TopologyExperimentDefinition, observationID, claimID string, prevalidate r04EffortV1Prevalidator) (agentqualification.TopologyCampaignState, error) {
	if prevalidate == nil {
		return state, fmt.Errorf("topology v9 preclaim route validator is unavailable")
	}
	if _, err := prevalidate(definition); err != nil {
		return state, fmt.Errorf("topology v9 preclaim route validation failed: %w", err)
	}
	return agentqualification.ClaimR04EffortV1Observation(state, definition, observationID, claimID)
}

func recoverR04EffortV1Observation(ctx context.Context, artifactRoot string) (map[string]any, error) {
	definition, err := agentqualification.CurrentR04EffortExperimentV1()
	if err != nil {
		return nil, err
	}
	state, err := loadR04EffortV1Campaign(artifactRoot)
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
		state, err = agentqualification.RecordR04EffortV1Observation(state, definition, sealed)
		if err != nil {
			return nil, err
		}
		if err := writeJSONAtomic(filepath.Join(artifactRoot, r04EffortV1CampaignFile), state); err != nil {
			return nil, err
		}
		return map[string]any{"status": "recovered_sealed_result", "provider_called": false, "observation": sealed, "campaign": state}, nil
	}
	var runtimeState r04EffortV1ObservationRuntime
	if err := readJSONFile(filepath.Join(observationRoot, "run-state.json"), &runtimeState); err != nil {
		return nil, fmt.Errorf("evidence corruption: stranded claim has no durable run state: %w", err)
	}
	stageReceipts, exact := reconstructR04EffortV1CrashReceipts(observationRoot, runtimeState)
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
		stageReceipts = applyR04EffortV1AmbiguousCost(stageReceipts, runtimeState.PendingReview, runtimeState.ClaimID, ambiguousCost)
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
	state, err = agentqualification.RecordR04EffortV1Observation(state, definition, result)
	if err != nil {
		return nil, err
	}
	if err := writeJSONExclusive(resultPath, result); err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(filepath.Join(artifactRoot, r04EffortV1CampaignFile), state); err != nil {
		return nil, err
	}
	return map[string]any{"status": "recovered_as_invalid", "provider_called": false, "observation": result, "campaign": state}, nil
}

func reconstructR04EffortV1CrashReceipts(observationRoot string, runtimeState r04EffortV1ObservationRuntime) ([]agentqualification.TopologyStageReceipt, bool) {
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
			if runtimeState.PendingReview != nil && runtimeState.PendingReview.Stage == stageName {
				receipts = append(receipts, agentqualification.TopologyStageReceipt{RunID: runtimeState.PendingReview.RunID, Stage: stageName, ProviderCalls: 1})
				exact = false
			}
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
	if runtimeState.PendingReview != nil && !hasStage(runtimeState.PendingReview.Stage) {
		exact = false
	}
	return receipts, exact
}

func applyR04EffortV1AmbiguousCost(receipts []agentqualification.TopologyStageReceipt, pending *r04EffortV1PendingReview, claimID string, costMicros int64) []agentqualification.TopologyStageReceipt {
	if costMicros <= 0 {
		return receipts
	}
	if pending == nil {
		for index := range receipts {
			if receipts[index].Stage == "execute" {
				receipts[index].CostMicros += costMicros
				return receipts
			}
		}
		return append(receipts, agentqualification.TopologyStageReceipt{RunID: claimID + ":execute", Stage: "execute", ProviderCalls: 1, CostMicros: costMicros})
	}
	for index := range receipts {
		if receipts[index].Stage == pending.Stage && receipts[index].RunID == pending.RunID {
			receipts[index].CostMicros += costMicros
			return receipts
		}
	}
	return append(receipts, agentqualification.TopologyStageReceipt{RunID: pending.RunID, Stage: pending.Stage, ProviderCalls: 1, CostMicros: costMicros})
}

func verifyPreservedTopologyCommandChain() error {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("cannot locate r04-effort-v1 admission source")
	}
	return verifyPreservedTopologyCommandChainAt(filepath.Join(filepath.Dir(current), ".."))
}

func verifyPreservedTopologyCommandChainAt(commandRoot string) error {
	v9, err := agentqualification.CurrentTopologyFreezeV9()
	if err != nil {
		return fmt.Errorf("topology-v9 frozen components drifted: %w", err)
	}
	v8, err := agentqualification.CurrentTopologyFreezeV8()
	if err != nil {
		return fmt.Errorf("topology-v8 frozen components drifted: %w", err)
	}
	v7, err := agentqualification.CurrentTopologyFreezeV7()
	if err != nil {
		return fmt.Errorf("topology-v7 frozen components drifted: %w", err)
	}
	v6, err := agentqualification.CurrentTopologyFreezeV6()
	if err != nil {
		return fmt.Errorf("topology-v6 frozen components drifted: %w", err)
	}
	v5, err := agentqualification.CurrentTopologyFreezeV5()
	if err != nil {
		return fmt.Errorf("topology-v5 frozen components drifted: %w", err)
	}
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
		{"topology-v9", filepath.Join(commandRoot, "topology-run-v9"), "topology_v9_cli.go", v9.CampaignExecutorSHA256, v9.MainSHA256},
		{"topology-v8", filepath.Join(commandRoot, "topology-run-v8"), "topology_v8_cli.go", v8.CampaignExecutorSHA256, v8.MainSHA256},
		{"topology-v7", filepath.Join(commandRoot, "topology-run-v7"), "topology_v7_cli.go", v7.CampaignExecutorSHA256, v7.MainSHA256},
		{"topology-v6", filepath.Join(commandRoot, "topology-run-v6"), "topology_v6_cli.go", v6.CampaignExecutorSHA256, v6.MainSHA256},
		{"topology-v5", filepath.Join(commandRoot, "topology-run-v5"), "topology_v5_cli.go", v5.CampaignExecutorSHA256, v5.MainSHA256},
		{"topology-v4", filepath.Join(commandRoot, "topology-run-v4"), "topology_v4_cli.go", v4.CampaignExecutorSHA256, v4.MainSHA256},
		{"topology-v3", filepath.Join(commandRoot, "topology-run-v3"), "topology_v3_cli.go", v3.CampaignExecutorSHA256, v3.MainSHA256},
		{"topology-v2", filepath.Join(commandRoot, "topology-run-v2"), "topology_v2_cli.go", v2.CampaignExecutorSHA256, v2.MainSHA256},
	}
	for _, check := range checks {
		if err := verifyTopologyCommandSourcesAt(check.commandDir, check.executorFile, check.executorSHA256, check.mainSHA256); err != nil {
			return fmt.Errorf("%s frozen command source drifted: %w", check.label, err)
		}
	}
	v1ExecutorPath := filepath.Join(commandRoot, "screening-run", "topology_cli.go")
	if err := verifyTopologyCommandSourceAt(v1ExecutorPath, v1.CampaignExecutorSHA256); err != nil {
		return fmt.Errorf("topology-v1 frozen command source drifted: %w", err)
	}
	if err := verifyR04EffortV1CurrentScreeningMainAt(commandRoot); err != nil {
		return err
	}
	return nil
}

func verifyR04EffortV1CurrentScreeningMainAt(commandRoot string) error {
	expected, err := agentqualification.RequiredR04EffortV1CurrentScreeningMainSHA256()
	if err != nil {
		return err
	}
	path := filepath.Join(commandRoot, "screening-run", "main.go")
	if err := verifyTopologyCommandSourceAt(path, expected); err != nil {
		return fmt.Errorf("r04-effort-v1 current screening entrypoint drifted: %w", err)
	}
	return nil
}

func verifyTopologyCommandSourceAt(path, expected string) error {
	raw, err := os.ReadFile(path)
	if err != nil || sha256Hex(raw) != expected {
		return fmt.Errorf("%s", path)
	}
	return nil
}

func verifyTopologyCommandSourcesAt(commandDir, executorFile, executorExpected, mainExpected string) error {
	for path, expected := range map[string]string{filepath.Join(commandDir, executorFile): executorExpected, filepath.Join(commandDir, "main.go"): mainExpected} {
		if err := verifyTopologyCommandSourceAt(path, expected); err != nil {
			return err
		}
	}
	return nil
}

func runR04EffortV1Arm(ctx context.Context, definition agentqualification.TopologyExperimentDefinition, observation agentqualification.TopologyExperimentObservation, observationRoot string, runtimeState *r04EffortV1ObservationRuntime) (agentqualification.TopologyExperimentResult, error) {
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
	// The shared provider performs a conservative $0.35 pre-call reservation.
	// Add that reservation outside this arm's actual-cost ceiling; the topology
	// stage and campaign ledgers still enforce the exact paid totals afterward.
	provider := openCodeRealisticProvider{candidate: definition.Executor, baselineSpendMicros: spendSnapshot.SpentMicros, maxSpendMicros: arm.CostCeilingMicros + 350000}
	checkpointPath := filepath.Join(observationRoot, "execute-controller.json")
	checkpoint := func(controller agentqualification.RealisticControllerResult) error {
		runtimeState.Controller = controller
		if err := writeJSONAtomic(checkpointPath, controller); err != nil {
			return err
		}
		return writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState)
	}
	stageStarted := time.Now()
	controller, controllerErr := agentqualification.RunR04EffortCandidateV1(ctx, definition, runtimeState.Workspace, runtimeState.ClaimID+":execute", task, provider, checkpoint)
	runtimeState.Controller = controller
	result.StageReceipts = append(result.StageReceipts, topologyControllerStage("execute", runtimeState.ClaimID+":execute", controller, time.Since(stageStarted), false))
	runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
	if writeErr := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); writeErr != nil {
		return result, writeErr
	}
	controllerAmbiguous := r04EffortV1ControllerAccountingAmbiguous(controller)
	if controllerErr != nil && controllerAmbiguous {
		if err := reconcileR04EffortV1AmbiguousStageSpend(ctx, spendSnapshot.SpentMicros, 0, &result.StageReceipts[len(result.StageReceipts)-1]); err != nil {
			return aggregateTopologyResult(result), err
		}
		result.AccountingStatus = "ambiguous"
		runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
		if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
			return aggregateTopologyResult(result), err
		}
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
	if controllerErr != nil && controllerAmbiguous {
		result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
		if err != nil {
			return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
		}
		result = classifyR04EffortV1AmbiguousObservation(result, "executor")
		return aggregateTopologyResult(result), nil
	}
	if controllerErr != nil && controller.ProviderCalls == 0 && controller.TerminalState == "invalid" && controller.FailureClass == "test_invalidity" {
		result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
		if err != nil {
			return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
		}
		result.HiddenGradeSHA256 = sha256Hex([]byte("not_run:runtime_preflight_invalid"))
		result.Outcome, result.FailureClass = "invalid", "runtime_preflight_invalid"
		result.PostBatchCorrection = controllerErr.Error()
		return aggregateTopologyResult(result), nil
	}
	if isR04EffortV1ExecutorProtocolFailure(currentControllerResult(controller, controllerErr)) {
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
	if false { // unreachable legacy v9 review path; the v1 definition rejects reviewed arms
		finalReviewPass := true
		_ = finalReviewPass
		request, requestErr := topologyReviewRequestFor(observation, runtimeState.Workspace, currentController, currentReceipt, currentEvidenceRoot, runtimeState.ClaimID+":review-1")
		if requestErr != nil {
			return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", requestErr)
		}
		if err := persistR04EffortV1PendingReview(observationRoot, runtimeState, request.ReviewRunID, "review_1"); err != nil {
			return aggregateTopologyResult(result), err
		}
		verdict, reviewStage, reviewErr := executeTopologyReviewCall(ctx, definition, request, filepath.Join(observationRoot, "review-1"))
		result.StageReceipts = append(result.StageReceipts, reviewStage)
		runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
		runtimeState.PendingReview = nil
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
			correction, correctionErr := agentqualification.RunTopologyCorrectionV9(ctx, definition, runtimeState.Workspace, correctionRunID, correctionTask, 8, topologyReviewedCorrectionTokens, topologyReviewedCorrectionGenerated, provider, correctionCheckpoint)
			result.StageReceipts = append(result.StageReceipts, topologyControllerStage("correction", correctionRunID, correction, time.Since(stageStarted), true))
			runtimeState.Controller = correction
			runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
			if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
				return aggregateTopologyResult(result), err
			}
			correctionAmbiguous := r04EffortV1ControllerAccountingAmbiguous(correction)
			if correctionErr != nil && correctionAmbiguous {
				accountedBefore := r04EffortV1StageCostBeforeLast(result.StageReceipts)
				if err := reconcileR04EffortV1AmbiguousStageSpend(ctx, spendSnapshot.SpentMicros, accountedBefore, &result.StageReceipts[len(result.StageReceipts)-1]); err != nil {
					return aggregateTopologyResult(result), err
				}
				result.AccountingStatus = "ambiguous"
				runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
				if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
					return aggregateTopologyResult(result), err
				}
			}
			if correctionErr != nil && isTopologyGlobalError(correctionErr) {
				return aggregateTopologyResult(result), correctionErr
			}
			if correctionErr != nil && correctionAmbiguous {
				result.FinalTreeSHA256, err = agentqualification.RealisticFixtureTreeSHA256(os.DirFS(runtimeState.Workspace.Root), ".")
				if err != nil {
					return aggregateTopologyResult(result), fmt.Errorf("evidence corruption: %w", err)
				}
				result = classifyR04EffortV1AmbiguousObservation(result, "correction")
				return aggregateTopologyResult(result), nil
			}
			if correctionErr != nil {
				return aggregateTopologyResult(result), correctionErr
			}
			if isR04EffortV1ExecutorProtocolFailure(currentControllerResult(correction, correctionErr)) {
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
			if err := persistR04EffortV1PendingReview(observationRoot, runtimeState, finalRequest.ReviewRunID, "review_2"); err != nil {
				return aggregateTopologyResult(result), err
			}
			finalVerdict, finalStage, finalErr := executeTopologyReviewCall(ctx, definition, finalRequest, filepath.Join(observationRoot, "review-2"))
			result.StageReceipts = append(result.StageReceipts, finalStage)
			runtimeState.StageReceipts = append([]agentqualification.TopologyStageReceipt(nil), result.StageReceipts...)
			runtimeState.PendingReview = nil
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
	if grade.Verdict != "PASS" || currentController.TerminalState != "completed" || controllerErr != nil {
		result.Outcome, result.FailureClass = "fail", "competence_failure"
	}
	result = aggregateTopologyResult(result)
	if result.ProviderCalls > arm.MaxProviderTurns || result.TotalTokens > arm.TotalTokenReservation || result.CostMicros > arm.CostCeilingMicros {
		result.Outcome, result.FailureClass = "global_halt", "uncontrolled_spending"
	}
	return result, nil
}

func persistR04EffortV1PendingReview(observationRoot string, runtimeState *r04EffortV1ObservationRuntime, runID, stage string) error {
	if runtimeState == nil || runID == "" || stage != "review_1" && stage != "review_2" || runtimeState.PendingReview != nil {
		return fmt.Errorf("topology v9 pending review marker is invalid")
	}
	runtimeState.PendingReview = &r04EffortV1PendingReview{RunID: runID, Stage: stage}
	if err := writeJSONAtomic(filepath.Join(observationRoot, "run-state.json"), runtimeState); err != nil {
		runtimeState.PendingReview = nil
		return err
	}
	return nil
}

func currentControllerResult(controller agentqualification.RealisticControllerResult, controllerErr error) agentqualification.RealisticControllerResult {
	if controllerErr != nil && controller.TerminalState == "" {
		controller.TerminalState = "failed"
	}
	return controller
}

func isR04EffortV1ExecutorProtocolFailure(controller agentqualification.RealisticControllerResult) bool {
	return controller.TerminalState == "failed" && controller.FailureClass == "response_or_tool_protocol_failure"
}

func sealTopologyControllerIfCompleted(evidenceRoot string, workspace agentqualification.RealisticCaseWorkspace, candidate agentqualification.RealisticCandidate, controller agentqualification.RealisticControllerResult) (*agentqualification.RealisticExecutionReceipt, string, error) {
	if controller.TerminalState != "completed" {
		return nil, "", nil
	}
	definition, err := agentqualification.CurrentR04EffortExperimentV1()
	if err != nil {
		return nil, "", err
	}
	return agentqualification.SealR04EffortV1ExecutionReceiptForFrozenExecutor(definition.Executor, candidate, evidenceRoot, workspace, controller)
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
	spendBefore, spendErr := r04EffortV1ReadSpendSnapshot(ctx, time.Now().UTC())
	if spendErr != nil {
		if strings.Contains(strings.ToLower(spendErr.Error()), "credential") {
			return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("credential failure: %w", spendErr)
		}
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("provider preflight transport failure: %w", spendErr)
	}
	plan, err := agentqualification.BuildOpenCodeTopologyV9ReviewPlanForFrozenReviewer(definition.Reviewer, definition.Reviewer, request)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("evidence corruption: %w", err)
	}
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	runtimeState.Env = replaceEnv(runtimeState.Env, "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX", "5000")
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
		stage, trustworthy, sealErr := sealTopologyReviewTransportFailure(root, routing, rawJSONL, rawExport, stage, started, err)
		if sealErr != nil {
			return agentqualification.TopologyReviewVerdict{}, stage, fmt.Errorf("evidence corruption: %w", sealErr)
		}
		if reconcileErr := reconcileR04EffortV1AmbiguousStageSpend(ctx, spendBefore.SpentMicros, 0, &stage); reconcileErr != nil {
			return agentqualification.TopologyReviewVerdict{}, stage, reconcileErr
		}
		return agentqualification.TopologyReviewVerdict{}, stage, topologyReviewerTransportError{cause: err, accountingTrustworthy: trustworthy}
	}
	parsed, candidateJSON, sealedStage, err := sealAndParseTopologyReviewResponse(root, definition, routing, rawJSONL, rawExport, stage, started)
	stage = sealedStage
	if err != nil {
		return agentqualification.TopologyReviewVerdict{}, stage, err
	}
	_ = parsed
	if stage.InputTokens+stage.OutputTokens+stage.ReasoningTokens > definition.ReviewerTotalTokenReservation || stage.OutputTokens+stage.ReasoningTokens > definition.ReviewerGeneratedTokenCeiling || stage.CostMicros > 100000 {
		return agentqualification.TopologyReviewVerdict{}, stage, agentqualification.ErrTopologyStageBudgetExceeded
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

func r04EffortV1ControllerAccountingAmbiguous(controller agentqualification.RealisticControllerResult) bool {
	return controller.PendingProviderCall != nil || controller.AmbiguousProviderCalls > 0 || controller.ProviderAccountingStatus == "ambiguous" || controller.ProviderTokenAccountingStatus == "unknown" || controller.ProviderCostAccountingStatus == "unknown"
}

func classifyR04EffortV1AmbiguousObservation(result agentqualification.TopologyExperimentResult, stage string) agentqualification.TopologyExperimentResult {
	result.Outcome = "invalid"
	result.FailureClass = "transport_or_harness_imperfection"
	result.AccountingStatus = "ambiguous"
	result.HiddenGradeSHA256 = sha256Hex([]byte("not_run:" + stage + "_transport_ambiguous"))
	result.PostBatchCorrection = strings.Title(stage) + " provider outcome was ambiguous; the affected observation was preserved without retry and authoritative provider spend was reconciled before campaign continuation."
	return result
}

func r04EffortV1StageCostBeforeLast(stages []agentqualification.TopologyStageReceipt) int64 {
	var total int64
	for index := 0; index+1 < len(stages); index++ {
		total += stages[index].CostMicros
	}
	return total
}

// reconcileR04EffortV1AmbiguousStageSpend attributes the authoritative provider
// account delta to this campaign before an ambiguous observation can continue.
// Token accounting remains ambiguous; only paid spend is reconciled.
func reconcileR04EffortV1AmbiguousStageSpend(ctx context.Context, baselineMicros, alreadyAccountedMicros int64, stage *agentqualification.TopologyStageReceipt) error {
	if stage == nil || baselineMicros < 0 || alreadyAccountedMicros < 0 {
		return fmt.Errorf("evidence corruption: invalid topology v9 spend reconciliation input")
	}
	spendAfter, err := r04EffortV1ReadSpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "credential") {
			return fmt.Errorf("credential failure: %w", err)
		}
		return fmt.Errorf("uncontrolled spending: cannot reconcile ambiguous provider spend: %w", err)
	}
	delta := spendAfter.SpentMicros - baselineMicros
	if delta < alreadyAccountedMicros {
		return fmt.Errorf("evidence corruption: authoritative provider spend is below already-accounted campaign spend")
	}
	stage.CostMicros = delta - alreadyAccountedMicros
	return nil
}

func sealTopologyReviewTransportFailure(root string, routing realisticOpenCodeRoutingEvidence, rawJSONL, rawExport []byte, stage agentqualification.TopologyStageReceipt, started time.Time, transportErr error) (agentqualification.TopologyStageReceipt, bool, error) {
	receipt := map[string]any{"jsonl": string(rawJSONL), "export_raw": string(rawExport), "routing": routing, "transport_error": transportErr.Error()}
	if json.Valid(rawExport) {
		receipt["export"] = json.RawMessage(rawExport)
	}
	rawReceipt, err := json.Marshal(receipt)
	if err != nil {
		return stage, false, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return stage, false, err
	}
	if err := writeBytesExclusive(filepath.Join(root, "provider-receipt.json"), rawReceipt); err != nil {
		return stage, false, err
	}
	partial := realisticPartialProviderResponse(rawJSONL, rawExport, transportErr)
	stage.WallTimeMillis = time.Since(started).Milliseconds()
	stage.ProviderCalls = 1
	if !trustworthyTopologyReviewPartialAccounting(rawJSONL, partial) {
		return stage, false, nil
	}
	stage.InputTokens = partial.InputTokens
	stage.OutputTokens = partial.OutputTokens
	stage.ReasoningTokens = partial.ReasoningTokens
	stage.CostMicros = partial.CostMicros
	return stage, true, nil
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
	resolvedHash, err := agentqualification.ValidateR04EffortV1ResolvedConfig(plan, resolvedConfig)
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
	var reviewerTransportErr topologyReviewerTransportError
	switch {
	case errors.Is(err, agentqualification.ErrRealisticBudgetBreach):
		result.Outcome, result.FailureClass = "global_halt", "budget_breach"
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
	case errors.As(err, &reviewerTransportErr):
		result.Outcome, result.FailureClass = "invalid", "transport_or_harness_imperfection"
		result.PostBatchCorrection = reviewerTransportErr.Error()
		if reviewerTransportErr.accountingTrustworthy {
			result.AccountingStatus = "exact"
		} else {
			result.AccountingStatus = "ambiguous"
		}
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

func loadR04EffortV1Campaign(root string) (agentqualification.TopologyCampaignState, error) {
	var state agentqualification.TopologyCampaignState
	path := filepath.Join(root, r04EffortV1CampaignFile)
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
	return errors.Is(err, agentqualification.ErrRealisticBudgetBreach) || strings.Contains(message, "credential") || strings.Contains(message, "uncontrolled spending") || strings.Contains(message, "budget breach") || strings.Contains(message, "unsafe external") || strings.Contains(message, "evidence integrity") || strings.Contains(message, "evidence corruption")
}

func sha256Hex(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
