package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type item14OpenRouterShadowPreflight struct {
	SchemaVersion         string            `json:"schema_version"`
	DefinitionSHA256      string            `json:"definition_sha256"`
	SuiteSHA256           string            `json:"suite_sha256"`
	Model                 string            `json:"model"`
	ServingProvider       string            `json:"serving_provider"`
	ProviderFallbacks     bool              `json:"provider_fallbacks"`
	RetryCount            int               `json:"retry_count"`
	ProviderCallsMade     int               `json:"provider_calls_made"`
	CredentialReads       int               `json:"credential_reads"`
	CredentialIsolation   string            `json:"credential_isolation"`
	ResolvedConfigSHA256  map[string]string `json:"resolved_config_sha256_by_workflow"`
	TemporaryRootsRemoved int               `json:"temporary_roots_removed"`
	TemporaryRootsAbsent  bool              `json:"temporary_roots_absent"`
	Ready                 bool              `json:"ready"`
}

type item14OpenRouterShadowClaim struct {
	SchemaVersion    string `json:"schema_version"`
	RunID            string `json:"run_id"`
	DefinitionSHA256 string `json:"definition_sha256"`
	SuiteSHA256      string `json:"suite_sha256"`
	Model            string `json:"model"`
	ServingProvider  string `json:"serving_provider"`
	MaxProviderCalls int    `json:"max_provider_calls"`
	RetryCount       int    `json:"retry_count"`
	ClaimedAt        string `json:"claimed_at"`
	ReplayPolicy     string `json:"replay_policy"`
}

type item14OpenRouterWorkflowClaim struct {
	SchemaVersion      string `json:"schema_version"`
	RunID              string `json:"run_id"`
	WorkflowID         string `json:"workflow_id"`
	Sequence           int    `json:"sequence"`
	PromptSHA256       string `json:"prompt_sha256"`
	InlineConfigSHA256 string `json:"inline_config_sha256"`
	Model              string `json:"model"`
	ServingProvider    string `json:"serving_provider"`
	MaxProviderCalls   int    `json:"max_provider_calls"`
	RetryCount         int    `json:"retry_count"`
	ClaimedAt          string `json:"claimed_at"`
}

type item14OpenRouterShadowAccounting struct {
	SchemaVersion                 string `json:"schema_version"`
	Provider                      string `json:"provider"`
	ProjectLane                   string `json:"project_lane"`
	WindowID                      string `json:"window_id"`
	BeforeSpentMicros             int64  `json:"before_spent_micros"`
	AfterSpentMicros              int64  `json:"after_spent_micros"`
	ObservedIncrementMicros       int64  `json:"observed_increment_micros"`
	ReceiptCostMicros             int64  `json:"receipt_cost_micros"`
	SliceIncrementalCeilingMicros int64  `json:"slice_incremental_ceiling_micros"`
	CumulativeCeilingMicros       int64  `json:"cumulative_ceiling_micros"`
	BeforeObservedAt              string `json:"before_observed_at"`
	AfterObservedAt               string `json:"after_observed_at"`
	Source                        string `json:"source"`
}

type item14OpenRouterShadowDependencies struct {
	Contracts     agentqualification.QualificationContracts
	ContractError error
	Prepare       func(context.Context) (item14OpenRouterShadowPreflight, error)
	SpendSnapshot func(context.Context, time.Time) (agentqualification.APISpendSnapshot, error)
	InvokeOnce    func(context.Context, agentqualification.OpenCodeScreeningPlan) ([]byte, []byte, bool, error)
	Fake          bool
}

func item14OpenRouterShadowPlan(optional ...agentqualification.QualificationContracts) (map[string]any, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, suite, plans, err := loadItem14OpenRouterShadowPlans(q)
	if err != nil {
		return nil, err
	}
	planSummary := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		planSummary = append(planSummary, map[string]any{
			"workflow_id": plan.CaseID, "model": plan.Model, "reasoning_effort": plan.Effort,
			"serving_provider": plan.ServingProvider, "provider_fallbacks": *plan.ProviderFallbacks,
			"prompt_sha256": plan.PromptSHA256, "schema_sha256": plan.SchemaSHA256, "inline_config_sha256": plan.InlineConfigSHA256,
		})
	}
	return map[string]any{
		"status": "frozen_not_launched", "provider_called": false, "credential_read": false,
		"definition": definition, "definition_sha256": agentqualification.OpenRouterLocalShadowDefinitionV1SHA256(),
		"suite_id": suite.SuiteID, "plans": planSummary,
	}, nil
}

func item14OpenRouterShadowFake() (agentqualification.OpenRouterLocalShadowRunV1, error) {
	tempRoot, err := os.MkdirTemp("", "multica-item14-openrouter-fake-lifecycle-")
	if err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	defer os.RemoveAll(tempRoot)
	evidenceParent := filepath.Join(tempRoot, ".qualification-evidence")
	if err := os.MkdirAll(evidenceParent, 0o700); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	return executeItem14OpenRouterShadowWithDependencies(context.Background(), filepath.Join(evidenceParent, "item14-openrouter-local-shadow-v1"), newFakeItem14OpenRouterDependencies())
}

func item14OpenRouterShadowPrepare(ctx context.Context) (item14OpenRouterShadowPreflight, error) {
	definition, _, plans, err := loadItem14OpenRouterShadowPlans()
	if err != nil {
		return item14OpenRouterShadowPreflight{}, err
	}
	preflight := item14OpenRouterShadowPreflight{
		SchemaVersion: "silver-mediums.openrouter-local-shadow-preflight.v1", DefinitionSHA256: agentqualification.OpenRouterLocalShadowDefinitionV1SHA256(), SuiteSHA256: definition.SuiteSHA256,
		Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, ProviderFallbacks: definition.Candidate.ProviderFallbacks,
		RetryCount: definition.RetryCount, CredentialIsolation: "fresh_empty_task_local_profile", ResolvedConfigSHA256: map[string]string{}, TemporaryRootsAbsent: true,
	}
	for _, plan := range plans {
		if err := agentqualification.ValidateOpenRouterEstablishedSmallModelRouteV2(plan.Model, plan.ServingProvider); err != nil {
			return preflight, err
		}
		runtimeState, err := newOpenCodeCommandRuntime(plan)
		if err != nil {
			return preflight, err
		}
		tempRoot := filepath.Dir(runtimeState.WorkDir)
		if err := pinItem14OpenRouterOutputCeiling(&runtimeState); err != nil {
			_ = os.RemoveAll(tempRoot)
			return preflight, err
		}
		if err := isolateItem14PrepareProfile(&runtimeState); err != nil {
			_ = os.RemoveAll(tempRoot)
			return preflight, err
		}
		resolved, resolveErr := runtimeState.resolvedConfig(ctx)
		if resolveErr == nil {
			var hash string
			hash, resolveErr = agentqualification.ValidateOpenCodeResolvedConfig(plan, resolved)
			if resolveErr == nil {
				preflight.ResolvedConfigSHA256[plan.CaseID] = hash
			}
		}
		cleanupErr := os.RemoveAll(tempRoot)
		if cleanupErr == nil {
			_, statErr := os.Stat(tempRoot)
			if os.IsNotExist(statErr) {
				preflight.TemporaryRootsRemoved++
			} else {
				preflight.TemporaryRootsAbsent = false
				cleanupErr = fmt.Errorf("temporary OpenCode root still exists")
			}
		}
		if resolveErr != nil {
			return preflight, resolveErr
		}
		if cleanupErr != nil {
			return preflight, cleanupErr
		}
	}
	preflight.Ready = len(preflight.ResolvedConfigSHA256) == len(plans) && preflight.TemporaryRootsRemoved == len(plans) && preflight.TemporaryRootsAbsent
	if !preflight.Ready {
		return preflight, fmt.Errorf("Item 14 OpenRouter shadow preflight incomplete")
	}
	return preflight, nil
}

func executeItem14OpenRouterShadow(ctx context.Context, artifactRoot string) (agentqualification.OpenRouterLocalShadowRunV1, error) {
	dependencies := item14OpenRouterShadowDependencies{
		Prepare:       item14OpenRouterShadowPrepare,
		SpendSnapshot: readOpenRouterSpendSnapshot,
		InvokeOnce:    invokeRealItem14OpenRouterShadowOnce,
	}
	return executeItem14OpenRouterShadowWithDependencies(ctx, artifactRoot, dependencies)
}

func executeItem14OpenRouterShadowWithDependencies(ctx context.Context, artifactRoot string, dependencies item14OpenRouterShadowDependencies) (agentqualification.OpenRouterLocalShadowRunV1, error) {
	if dependencies.ContractError != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, dependencies.ContractError
	}
	if dependencies.Contracts.Historical() && !dependencies.Fake {
		return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("historical inputs cannot authorize real execution")
	}
	q := dependencies.Contracts

	definition, suite, plans, err := loadItem14OpenRouterShadowPlans(q)
	if err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if dependencies.Prepare == nil || dependencies.SpendSnapshot == nil || dependencies.InvokeOnce == nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("Item 14 OpenRouter shadow dependencies are incomplete")
	}
	preflight, err := dependencies.Prepare(ctx)
	if err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if !filepath.IsAbs(artifactRoot) {
		return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("Item 14 OpenRouter shadow evidence root must be absolute")
	}
	claim := item14OpenRouterShadowClaim{
		SchemaVersion: "silver-mediums.openrouter-local-shadow-claim.v1", RunID: definition.RunID, DefinitionSHA256: agentqualification.OpenRouterLocalShadowDefinitionV1SHA256(), SuiteSHA256: definition.SuiteSHA256,
		Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, MaxProviderCalls: definition.MaxProviderCalls, RetryCount: 0,
		ClaimedAt: time.Now().UTC().Format(time.RFC3339Nano), ReplayPolicy: "any_claim_without_receipt_is_ambiguous_and_never_replayed",
	}
	if _, statErr := os.Stat(artifactRoot); statErr == nil || !os.IsNotExist(statErr) {
		return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("Item 14 OpenRouter shadow evidence root already exists or cannot be proven absent")
	}
	authorityClaimPath := filepath.Join(filepath.Dir(artifactRoot), "item14-openrouter-authority-v1.claim.json")
	if err := writeExclusiveJSON(authorityClaimPath, claim); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if err := os.Mkdir(artifactRoot, 0o700); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("create write-once Item 14 OpenRouter shadow root after authority consumption: %w", err)
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "provider-free-preflight.json"), preflight); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "run-claim.json"), claim); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	before, err := dependencies.SpendSnapshot(ctx, time.Now().UTC())
	if err != nil {
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "credential_or_credits_failure", "provider_calls": 0, "retry_allowed": false, "error": err.Error()})
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if before.SpentMicros > definition.CumulativeOpenRouterCeilingMicros-definition.TotalPriceReservationMicros {
		err = fmt.Errorf("OpenRouter cumulative spend leaves insufficient room for the price-derived Item 14 reservation")
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "preventive_spend_denial", "provider_calls": 0, "retry_allowed": false, "spent_micros": before.SpentMicros, "price_reservation_micros": definition.TotalPriceReservationMicros, "ceiling_micros": definition.CumulativeOpenRouterCeilingMicros})
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "spend-before.json"), before); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}

	receipts := make([]agentqualification.OpenRouterLocalShadowReceiptV1, 0, len(plans))
	lastSpend := before
	for index, plan := range plans {
		workflowDir := filepath.Join(artifactRoot, fmt.Sprintf("%02d-%s", index+1, plan.CaseID))
		if err := os.Mkdir(workflowDir, 0o700); err != nil {
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		workflowClaim := item14OpenRouterWorkflowClaim{SchemaVersion: "silver-mediums.openrouter-local-shadow-workflow-claim.v1", RunID: definition.RunID, WorkflowID: plan.CaseID, Sequence: index + 1, PromptSHA256: plan.PromptSHA256, InlineConfigSHA256: plan.InlineConfigSHA256, Model: plan.Model, ServingProvider: plan.ServingProvider, MaxProviderCalls: 1, RetryCount: 0, ClaimedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "claim.json"), workflowClaim); err != nil {
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		currentSpend, err := dependencies.SpendSnapshot(ctx, time.Now().UTC())
		if err != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "pre_call_credits_failure", "provider_calls": 0, "retry_allowed": false, "error": err.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		usedIncrement := currentSpend.SpentMicros - before.SpentMicros
		if usedIncrement < 0 || usedIncrement+definition.PerCallReservationMicros > definition.SliceIncrementalCeilingMicros || currentSpend.SpentMicros+definition.PerCallReservationMicros > definition.CumulativeOpenRouterCeilingMicros {
			err := fmt.Errorf("OpenRouter preventive per-call spend reservation denied")
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "preventive_per_call_spend_denial", "provider_calls": 0, "retry_allowed": false, "spent_micros": currentSpend.SpentMicros, "observed_increment_micros": usedIncrement, "per_call_reservation_micros": definition.PerCallReservationMicros})
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "spend-before.json"), currentSpend); err != nil {
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		if err := agentqualification.ValidateOpenRouterEstablishedSmallModelRouteV2(plan.Model, plan.ServingProvider); err != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "provider_boundary_failure", "provider_calls": 0, "retry_allowed": false, "error": err.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		rawJSONL, rawExport, cleanupVerified, runErr := dependencies.InvokeOnce(ctx, plan)
		rawWriteErr := error(nil)
		if len(rawJSONL) > 0 {
			rawWriteErr = writeExclusiveBytes(filepath.Join(workflowDir, "provider-run.jsonl"), rawJSONL)
		}
		if rawWriteErr == nil && len(rawExport) > 0 {
			rawWriteErr = writeExclusiveBytes(filepath.Join(workflowDir, "provider-session-export.json"), rawExport)
		}
		postSpend, postSpendErr := dependencies.SpendSnapshot(ctx, time.Now().UTC())
		if postSpendErr == nil {
			postSpendErr = writeExclusiveJSON(filepath.Join(workflowDir, "spend-after.json"), postSpend)
			lastSpend = postSpend
		}
		if postSpendErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "post_exposure_accounting_failure", "provider_calls_at_most": 1, "retry_allowed": false, "cleanup_verified": cleanupVerified, "error": postSpendErr.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, postSpendErr
		}
		if postSpend.SpentMicros < before.SpentMicros || postSpend.SpentMicros-before.SpentMicros > definition.SliceIncrementalCeilingMicros || postSpend.SpentMicros > definition.CumulativeOpenRouterCeilingMicros {
			err := fmt.Errorf("post-exposure OpenRouter spend crossed the frozen boundary")
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "post_exposure_spend_breach", "provider_calls_at_most": 1, "retry_allowed": false, "spent_micros": postSpend.SpentMicros, "observed_increment_micros": postSpend.SpentMicros - before.SpentMicros})
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		if rawWriteErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "raw_evidence_write_failure", "provider_calls_at_most": 1, "retry_allowed": false, "cleanup_verified": cleanupVerified, "accounting_sealed": true, "error": rawWriteErr.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, rawWriteErr
		}
		if runErr != nil || !cleanupVerified {
			cause := runErr
			if cause == nil {
				cause = fmt.Errorf("temporary OpenCode runtime cleanup failed")
			}
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "ambiguous_transport_or_cleanup_failure", "provider_calls_at_most": 1, "retry_allowed": false, "cleanup_verified": cleanupVerified, "error": cause.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("workflow %s failed closed and is not retryable: %w", plan.CaseID, cause)
		}
		result, response, err := agentqualification.ParseOpenCodeRunArtifact(rawJSONL, rawExport)
		if err != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "evidence_integrity_failure", "provider_calls": 1, "retry_allowed": false, "cleanup_verified": true, "error": err.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		receipt, err := q.BuildOpenRouterLocalShadowReceiptV1(definition, suite, plan, result, response, rawExport)
		if err != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "identity_or_response_contract_failure", "provider_calls": 1, "retry_allowed": false, "cleanup_verified": true, "error": err.Error()})
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "receipt.json"), receipt); err != nil {
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
		receipts = append(receipts, receipt)
	}
	run, err := q.BuildOpenRouterLocalShadowRunV1(definition, suite, receipts)
	if err != nil {
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "aggregate_evidence_integrity_failure", "provider_calls": len(receipts), "retry_allowed": false, "error": err.Error()})
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if dependencies.Fake {
		run.ExecutionMode = "deterministic_fake_provider_free"
		run.ProviderRequestCount = 0
		run.SimulatedResponseCount = len(receipts)
		if err := q.ValidateOpenRouterLocalShadowRunV1(definition, suite, run); err != nil {
			return agentqualification.OpenRouterLocalShadowRunV1{}, err
		}
	}
	after := lastSpend
	increment := after.SpentMicros - before.SpentMicros
	accounting := item14OpenRouterShadowAccounting{SchemaVersion: "silver-mediums.openrouter-local-shadow-accounting.v1", Provider: before.Provider, ProjectLane: before.ProjectLane, WindowID: before.WindowID, BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: after.SpentMicros, ObservedIncrementMicros: increment, ReceiptCostMicros: run.TotalCostMicros, SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeOpenRouterCeilingMicros, BeforeObservedAt: before.ObservedAt, AfterObservedAt: after.ObservedAt, Source: after.Source}
	if increment < 0 || increment > definition.SliceIncrementalCeilingMicros || after.SpentMicros > definition.CumulativeOpenRouterCeilingMicros || run.TotalCostMicros > definition.SliceIncrementalCeilingMicros {
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting)
		return agentqualification.OpenRouterLocalShadowRunV1{}, fmt.Errorf("Item 14 OpenRouter shadow spend evidence exceeded its frozen boundary")
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "run.json"), run); err != nil {
		return agentqualification.OpenRouterLocalShadowRunV1{}, err
	}
	return run, nil
}

func invokeRealItem14OpenRouterShadowOnce(_ context.Context, plan agentqualification.OpenCodeScreeningPlan) ([]byte, []byte, bool, error) {
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		return nil, nil, false, err
	}
	tempRoot := filepath.Dir(runtimeState.WorkDir)
	if err := pinItem14OpenRouterOutputCeiling(&runtimeState); err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, nil, false, err
	}
	rawJSONL, rawExport, runErr := runtimeState.runOnce(plan)
	cleanupErr := os.RemoveAll(tempRoot)
	cleanupVerified := false
	if cleanupErr == nil {
		_, statErr := os.Stat(tempRoot)
		cleanupVerified = os.IsNotExist(statErr)
	}
	if runErr == nil && cleanupErr != nil {
		runErr = cleanupErr
	}
	return rawJSONL, rawExport, cleanupVerified, runErr
}

func newFakeItem14OpenRouterDependencies() item14OpenRouterShadowDependencies {
	q, contractErr := agentqualification.HistoricalQualificationContracts()

	exposures := 0
	return item14OpenRouterShadowDependencies{Contracts: q, ContractError: contractErr,
		Fake: true,
		Prepare: func(context.Context) (item14OpenRouterShadowPreflight, error) {
			definition, _, plans, err := loadItem14OpenRouterShadowPlans(q)
			if err != nil {
				return item14OpenRouterShadowPreflight{}, err
			}
			hashes := map[string]string{}
			for _, plan := range plans {
				hashes[plan.CaseID] = plan.InlineConfigSHA256
			}
			return item14OpenRouterShadowPreflight{SchemaVersion: "silver-mediums.openrouter-local-shadow-preflight.v1", DefinitionSHA256: agentqualification.OpenRouterLocalShadowDefinitionV1SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, ServingProvider: definition.Candidate.ServingProvider, RetryCount: 0, ProviderCallsMade: 0, CredentialReads: 0, CredentialIsolation: "fake_no_credential_source", ResolvedConfigSHA256: hashes, TemporaryRootsRemoved: len(plans), TemporaryRootsAbsent: true, Ready: true}, nil
		},
		SpendSnapshot: func(_ context.Context, observedAt time.Time) (agentqualification.APISpendSnapshot, error) {
			return agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "opencode_local_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 110000000 + int64(exposures*10), ObservedAt: observedAt.Format(time.RFC3339Nano), Source: "fake_authoritative_api_usage"}, nil
		},
		InvokeOnce: func(_ context.Context, plan agentqualification.OpenCodeScreeningPlan) ([]byte, []byte, bool, error) {
			index := exposures
			exposures++
			return fakeItem14OpenRouterRawArtifacts(plan, index)
		},
	}
}

func fakeItem14OpenRouterRawArtifacts(plan agentqualification.OpenCodeScreeningPlan, index int) ([]byte, []byte, bool, error) {
	q, contractErr := agentqualification.HistoricalQualificationContracts()
	if contractErr != nil {
		return nil, nil, false, contractErr
	}

	definition, suite, _, err := loadItem14OpenRouterShadowPlans(q)
	if err != nil {
		return nil, nil, false, err
	}
	var workflow *agentqualification.LocalShadowWorkflowV1
	for workflowIndex := range suite.Workflows {
		if suite.Workflows[workflowIndex].ID == plan.CaseID {
			workflow = &suite.Workflows[workflowIndex]
			break
		}
	}
	if workflow == nil {
		return nil, nil, false, fmt.Errorf("fake Item 14 workflow missing")
	}
	assignments := append([]agentqualification.LocalShadowStageAssignmentV1{workflow.Intake}, workflow.Stages...)
	stageIDs := make([]string, 0, len(assignments))
	contractIDs := make([]string, 0)
	seenContracts := map[string]struct{}{}
	for _, assignment := range assignments {
		stageIDs = append(stageIDs, assignment.Stage)
		for _, contractID := range assignment.EvidenceContractIDs {
			if _, ok := seenContracts[contractID]; ok {
				continue
			}
			seenContracts[contractID] = struct{}{}
			contractIDs = append(contractIDs, contractID)
		}
	}
	response := agentqualification.OpenRouterLocalShadowResponseV1{WorkflowID: workflow.ID, SuiteSHA256: definition.SuiteSHA256, StageIDs: stageIDs, EvidenceContractIDs: contractIDs, BoundaryVerdict: "accepted_frozen_provider_free_contract"}
	responseRaw, _ := json.Marshal(response)
	sessionID := fmt.Sprintf("ses_item14_fake_%d", index)
	messageID := fmt.Sprintf("msg_item14_fake_%d", index)
	timestamp := int64(1789387200000 + index*1000)
	tokens := map[string]any{"input": 100 + index, "output": 20 + index, "reasoning": 0, "cache": map[string]any{"read": 0, "write": 0}}
	events := []map[string]any{
		{"type": "step_start", "timestamp": timestamp, "sessionID": sessionID, "part": map[string]any{"id": "part-start", "sessionID": sessionID, "messageID": messageID, "type": "step-start"}},
		{"type": "text", "timestamp": timestamp + 1, "sessionID": sessionID, "part": map[string]any{"id": "part-text", "sessionID": sessionID, "messageID": messageID, "type": "text", "text": string(responseRaw), "metadata": map[string]any{"openrouter": map[string]any{"selected": true, "provider": plan.ServingProvider}}}},
		{"type": "step_finish", "timestamp": timestamp + 2, "sessionID": sessionID, "part": map[string]any{"id": "part-finish", "sessionID": sessionID, "messageID": messageID, "type": "step-finish", "cost": 0.00001, "tokens": tokens}},
	}
	var rawJSONL []byte
	for _, event := range events {
		line, _ := json.Marshal(event)
		rawJSONL = append(rawJSONL, line...)
		rawJSONL = append(rawJSONL, '\n')
	}
	export := map[string]any{
		"info": map[string]any{"id": sessionID},
		"messages": []any{map[string]any{
			"info":  map[string]any{"id": messageID, "sessionID": sessionID, "role": "assistant", "modelID": strings.TrimPrefix(plan.Model, "openrouter/"), "providerID": "openrouter", "summary": false, "finish": "stop", "cost": 0.00001, "tokens": tokens, "error": nil},
			"parts": []any{map[string]any{"id": "part-text", "sessionID": sessionID, "messageID": messageID, "type": "text", "text": string(responseRaw), "metadata": map[string]any{"openrouter": map[string]any{"selected": true, "provider": plan.ServingProvider}}}},
		}},
	}
	rawExport, _ := json.Marshal(export)
	return rawJSONL, rawExport, true, nil
}

func loadItem14OpenRouterShadowPlans(optional ...agentqualification.QualificationContracts) (agentqualification.OpenRouterLocalShadowDefinitionV1, agentqualification.LocalShadowWorkflowSuiteV1, []agentqualification.OpenCodeScreeningPlan, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, err := q.CurrentOpenRouterLocalShadowDefinitionV1()
	if err != nil {
		return definition, agentqualification.LocalShadowWorkflowSuiteV1{}, nil, err
	}
	suite, err := q.CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		return definition, suite, nil, err
	}
	plans, err := q.BuildOpenRouterLocalShadowPlansV1(definition, suite)
	return definition, suite, plans, err
}

func writeExclusiveJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeExclusiveBytes(path, append(raw, '\n'))
}

func writeExclusiveBytes(path string, raw []byte) error {
	if !filepath.IsAbs(path) || len(raw) == 0 {
		return fmt.Errorf("write-once evidence path or body invalid")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func pinItem14OpenRouterOutputCeiling(runtimeState *openCodeCommandRuntime) error {
	prefix := "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX="
	want := fmt.Sprintf("%s%d", prefix, agentqualification.OpenRouterLocalShadowGeneratedMaxV1)
	found := false
	for index, value := range runtimeState.Env {
		if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
			runtimeState.Env[index] = want
			found = true
		}
	}
	if !found {
		return fmt.Errorf("Item 14 OpenRouter output ceiling environment is missing")
	}
	return nil
}

func isolateItem14PrepareProfile(runtimeState *openCodeCommandRuntime) error {
	tempRoot := filepath.Dir(runtimeState.WorkDir)
	profile := filepath.Join(tempRoot, "prepare-profile")
	appData := filepath.Join(tempRoot, "prepare-appdata")
	localAppData := filepath.Join(tempRoot, "prepare-localappdata")
	for _, path := range []string{profile, appData, localAppData} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	blocked := map[string]struct{}{}
	for _, name := range []string{"USERPROFILE", "HOMEDRIVE", "HOMEPATH", "APPDATA", "LOCALAPPDATA", "HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		blocked[name] = struct{}{}
	}
	env := make([]string, 0, len(runtimeState.Env)+5)
	for _, value := range runtimeState.Env {
		name, _, ok := strings.Cut(value, "=")
		if ok {
			if _, remove := blocked[name]; remove {
				continue
			}
		}
		env = append(env, value)
	}
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(profile)
		homePath := strings.TrimPrefix(profile, volume)
		env = append(env, "USERPROFILE="+profile, "HOMEDRIVE="+volume, "HOMEPATH="+homePath, "APPDATA="+appData, "LOCALAPPDATA="+localAppData)
	} else {
		env = append(env, "HOME="+profile, "XDG_DATA_HOME="+localAppData, "XDG_CACHE_HOME="+appData)
	}
	runtimeState.Env = env
	return nil
}
