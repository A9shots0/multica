package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type item14OpenRouterDirectPreflightV3 struct {
	item14OpenRouterDirectPreflightV2
	ExpectedEndpointModel string `json:"expected_endpoint_model"`
	ProviderSchemaParam   bool   `json:"provider_structured_output_parameter_allowed"`
	ResponseValidation    string `json:"response_validation"`
}

func item14OpenRouterDirectPlanV3(optional ...agentqualification.QualificationContracts) (map[string]any, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, _, plans, err := loadItem14OpenRouterDirectPlansV3(q)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "frozen_not_launched", "provider_called": false, "credential_read": false, "definition": definition, "definition_sha256": agentqualification.OpenRouterDirectShadowDefinitionV3SHA256(), "plans": directPlanSummariesV2(plans)}, nil
}

func item14OpenRouterDirectPrepareV3(optional ...agentqualification.QualificationContracts) (item14OpenRouterDirectPreflightV3, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, _, plans, err := loadItem14OpenRouterDirectPlansV3(q)
	if err != nil {
		return item14OpenRouterDirectPreflightV3{}, err
	}
	hashes := make(map[string]string, len(plans))
	for _, plan := range plans {
		if err := q.ValidateOpenRouterDirectShadowPlanV3(definition, plan); err != nil {
			return item14OpenRouterDirectPreflightV3{}, err
		}
		hashes[plan.WorkflowID] = plan.RequestSHA256
	}
	return item14OpenRouterDirectPreflightV3{item14OpenRouterDirectPreflightV2: item14OpenRouterDirectPreflightV2{
		SchemaVersion: "silver-mediums.openrouter-direct-shadow-preflight.v3", DefinitionSHA256: agentqualification.OpenRouterDirectShadowDefinitionV3SHA256(), SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, APIModel: definition.Candidate.APIModel,
		ServingProvider: definition.Candidate.ServingProvider, Endpoint: definition.Endpoint, ProviderFallbacks: false, ProviderOrderExact: true, ProviderOnlyExact: true, RouterMetadata: true, RetryCount: 0, RedirectCount: 0, ProviderCallsMade: 0, CredentialReads: 0,
		RequestSHA256ByCase: hashes, ResponseFailureScope: definition.ResponseContractFailurePolicy, Ready: len(hashes) == definition.MaxProviderCalls,
	}, ExpectedEndpointModel: definition.ExpectedEndpointModel, ProviderSchemaParam: false, ResponseValidation: definition.ResponseValidation}, nil
}

func item14OpenRouterDirectFakeV3() (agentqualification.OpenRouterDirectShadowRunV3, error) {
	tempRoot, err := os.MkdirTemp("", "multica-item14-openrouter-direct-v3-fake-")
	if err != nil {
		return agentqualification.OpenRouterDirectShadowRunV3{}, err
	}
	defer os.RemoveAll(tempRoot)
	parent := filepath.Join(tempRoot, ".qualification-evidence")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return agentqualification.OpenRouterDirectShadowRunV3{}, err
	}
	return executeItem14OpenRouterDirectV3(context.Background(), filepath.Join(parent, "item14-openrouter-direct-shadow-v3"), newFakeItem14OpenRouterDirectDependenciesV3())
}

func executeItem14OpenRouterDirectRealV3(ctx context.Context, artifactRoot string) (agentqualification.OpenRouterDirectShadowRunV3, error) {
	return executeItem14OpenRouterDirectV3(ctx, artifactRoot, item14OpenRouterDirectDependenciesV2{OpenCredential: openOpenRouterCredentialOnceV2, SpendSnapshot: readOpenRouterSpendSnapshotWithCredentialV2, InvokeOnce: invokeOpenRouterDirectOnceV2})
}

func executeItem14OpenRouterDirectV3(ctx context.Context, artifactRoot string, dependencies item14OpenRouterDirectDependenciesV2) (run agentqualification.OpenRouterDirectShadowRunV3, returnErr error) {
	if dependencies.ContractError != nil {
		return run, dependencies.ContractError
	}
	if dependencies.Contracts.Historical() && !dependencies.Fake {
		return run, fmt.Errorf("historical inputs cannot authorize real execution")
	}
	q := dependencies.Contracts

	definition, suite, plans, err := loadItem14OpenRouterDirectPlansV3(q)
	if err != nil {
		return run, err
	}
	if dependencies.SpendSnapshot == nil || dependencies.InvokeOnce == nil || (!dependencies.Fake && dependencies.OpenCredential == nil) {
		return run, fmt.Errorf("Item 14 OpenRouter direct v3 dependencies incomplete")
	}
	preflight, err := item14OpenRouterDirectPrepareV3(q)
	if err != nil || !preflight.Ready {
		return run, fmt.Errorf("Item 14 OpenRouter direct v3 preflight failed: %w", err)
	}
	if !filepath.IsAbs(artifactRoot) {
		return run, fmt.Errorf("Item 14 OpenRouter direct v3 evidence root must be absolute")
	}
	if _, statErr := os.Stat(artifactRoot); statErr == nil || !os.IsNotExist(statErr) {
		return run, fmt.Errorf("Item 14 OpenRouter direct v3 evidence root already exists or cannot be proven absent")
	}
	claim := map[string]any{
		"schema_version": "silver-mediums.openrouter-direct-shadow-claim.v3", "run_id": definition.RunID, "definition_sha256": agentqualification.OpenRouterDirectShadowDefinitionV3SHA256(), "suite_sha256": definition.SuiteSHA256,
		"antecedent_v2_terminal_response_sha256": definition.AntecedentV2TerminalResponseSHA256, "model": definition.Candidate.Model, "api_model": definition.Candidate.APIModel, "expected_endpoint_model": definition.ExpectedEndpointModel,
		"serving_provider": definition.Candidate.ServingProvider, "max_provider_calls": 4, "retry_count": 0, "claimed_at": time.Now().UTC().Format(time.RFC3339Nano), "replay_policy": "any_claim_without_a_terminal_run_is_ambiguous_and_never_replayed",
	}
	if err := writeExclusiveJSON(filepath.Join(filepath.Dir(artifactRoot), "item14-openrouter-direct-authority-v3.claim.json"), claim); err != nil {
		return run, err
	}
	if err := os.Mkdir(artifactRoot, 0o700); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "provider-free-preflight.json"), preflight); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "run-claim.json"), claim); err != nil {
		return run, err
	}
	credentialReads := 0
	credential := []byte("fake-no-credential")
	if !dependencies.Fake {
		credential, err = dependencies.OpenCredential()
		credentialReads = 1
		if err != nil {
			_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "credential_failure", "provider_calls": 0, "retry_allowed": false, "error": err.Error()})
			return run, err
		}
	}
	defer func() {
		for index := range credential {
			credential[index] = 0
		}
	}()
	before, err := dependencies.SpendSnapshot(ctx, credential, time.Now().UTC())
	if err != nil {
		return run, err
	}
	if before.SpentMicros > definition.CumulativeOpenRouterCeilingMicros-definition.TotalPriceReservationMicros {
		return run, fmt.Errorf("OpenRouter cumulative spend leaves insufficient v3 reservation")
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "spend-before.json"), before); err != nil {
		return run, err
	}
	observations := make([]agentqualification.OpenRouterDirectShadowObservationV2Record, 0, len(plans))
	lastSpend := before
	for index, plan := range plans {
		workflowDir := filepath.Join(artifactRoot, fmt.Sprintf("%02d-%s", index+1, plan.WorkflowID))
		if err := os.Mkdir(workflowDir, 0o700); err != nil {
			return run, err
		}
		workflowClaim := map[string]any{"schema_version": "silver-mediums.openrouter-direct-shadow-workflow-claim.v3", "run_id": definition.RunID, "workflow_id": plan.WorkflowID, "sequence": index + 1, "request_sha256": plan.RequestSHA256, "model": plan.Model, "expected_endpoint_model": definition.ExpectedEndpointModel, "serving_provider": plan.ServingProvider, "max_provider_calls": 1, "retry_count": 0, "claimed_at": time.Now().UTC().Format(time.RFC3339Nano)}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "claim.json"), workflowClaim); err != nil {
			return run, err
		}
		currentSpend, err := dependencies.SpendSnapshot(ctx, credential, time.Now().UTC())
		if err != nil {
			return run, err
		}
		used := currentSpend.SpentMicros - before.SpentMicros
		if used < 0 || used+definition.PerCallReservationMicros > definition.SliceIncrementalCeilingMicros || currentSpend.SpentMicros+definition.PerCallReservationMicros > definition.CumulativeOpenRouterCeilingMicros {
			return run, fmt.Errorf("OpenRouter preventive direct v3 reservation denied")
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "spend-before.json"), currentSpend); err != nil {
			return run, err
		}
		started := time.Now().UTC()
		raw, requests, invokeErr := dependencies.InvokeOnce(ctx, credential, definition.Endpoint, plan)
		completed := time.Now().UTC()
		if len(raw) > 0 {
			if err := writeExclusiveBytes(filepath.Join(workflowDir, "provider-response.json"), raw); err != nil {
				return run, err
			}
		}
		postSpend, postErr := dependencies.SpendSnapshot(ctx, credential, time.Now().UTC())
		if postErr == nil {
			postErr = writeExclusiveJSON(filepath.Join(workflowDir, "spend-after.json"), postSpend)
			lastSpend = postSpend
		}
		if postErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "post_exposure_accounting_failure", "provider_calls_at_most": requests, "retry_allowed": false, "error": postErr.Error()})
			return run, postErr
		}
		if invokeErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "transport_or_http_failure", "provider_calls_at_most": requests, "retry_allowed": false, "error": invokeErr.Error()})
			return run, invokeErr
		}
		observation, observationErr := q.BuildOpenRouterDirectShadowObservationV3(definition, suite, plan, raw, requests, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano), completed.Sub(started).Milliseconds())
		if observationErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "identity_provider_or_evidence_failure", "provider_calls": requests, "retry_allowed": false, "error": observationErr.Error()})
			return run, observationErr
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "observation.json"), observation); err != nil {
			return run, err
		}
		observations = append(observations, observation)
	}
	run, err = q.BuildOpenRouterDirectShadowRunV3(definition, suite, observations, dependencies.Fake)
	if err != nil {
		return run, err
	}
	if err := verifyCredentialAbsentFromEvidenceV2(artifactRoot, credential); err != nil {
		return run, err
	}
	for index := range credential {
		credential[index] = 0
	}
	credential = nil
	increment := lastSpend.SpentMicros - before.SpentMicros
	accounting := item14OpenRouterDirectAccountingV2{SchemaVersion: "silver-mediums.openrouter-direct-shadow-accounting.v3", Provider: before.Provider, ProjectLane: before.ProjectLane, WindowID: before.WindowID, BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: lastSpend.SpentMicros, ObservedIncrementMicros: increment, ReceiptCostMicros: run.TotalCostMicros, SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeOpenRouterCeilingMicros, BeforeObservedAt: before.ObservedAt, AfterObservedAt: lastSpend.ObservedAt, Source: lastSpend.Source, CredentialReads: credentialReads, CredentialCopyPersisted: false, CredentialMemoryZeroed: true}
	if increment < 0 || increment > definition.SliceIncrementalCeilingMicros || lastSpend.SpentMicros > definition.CumulativeOpenRouterCeilingMicros || run.TotalCostMicros > definition.SliceIncrementalCeilingMicros {
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting)
		return run, fmt.Errorf("OpenRouter direct v3 spend boundary exceeded")
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "run.json"), run); err != nil {
		return run, err
	}
	return run, nil
}

func loadItem14OpenRouterDirectPlansV3(optional ...agentqualification.QualificationContracts) (agentqualification.OpenRouterDirectShadowDefinitionV3, agentqualification.LocalShadowWorkflowSuiteV1, []agentqualification.OpenRouterDirectShadowPlanV2, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, err := q.CurrentOpenRouterDirectShadowDefinitionV3()
	if err != nil {
		return definition, agentqualification.LocalShadowWorkflowSuiteV1{}, nil, err
	}
	suite, err := q.CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		return definition, suite, nil, err
	}
	plans, err := q.BuildOpenRouterDirectShadowPlansV3(definition, suite)
	return definition, suite, plans, err
}

func newFakeItem14OpenRouterDirectDependenciesV3() item14OpenRouterDirectDependenciesV2 {
	q, contractErr := agentqualification.HistoricalQualificationContracts()

	calls := 0
	return item14OpenRouterDirectDependenciesV2{Contracts: q, ContractError: contractErr,
		Fake: true,
		SpendSnapshot: func(_ context.Context, _ []byte, observedAt time.Time) (agentqualification.APISpendSnapshot, error) {
			return agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "direct_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 110000000 + int64(calls*10), ObservedAt: observedAt.Format(time.RFC3339Nano), Source: "fake_authoritative_api_usage"}, nil
		},
		InvokeOnce: func(_ context.Context, _ []byte, _ string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
			calls++
			return fakeOpenRouterDirectResponseV3(plan), 1, nil
		},
	}
}

func fakeOpenRouterDirectResponseV3(plan agentqualification.OpenRouterDirectShadowPlanV2) []byte {
	q, contractErr := agentqualification.HistoricalQualificationContracts()
	if contractErr != nil {
		return nil
	}

	definition, suite, _, err := loadItem14OpenRouterDirectPlansV3(q)
	if err != nil {
		return nil
	}
	var workflow agentqualification.LocalShadowWorkflowV1
	for _, candidate := range suite.Workflows {
		if candidate.ID == plan.WorkflowID {
			workflow = candidate
			break
		}
	}
	assignments := append([]agentqualification.LocalShadowStageAssignmentV1{workflow.Intake}, workflow.Stages...)
	stageIDs := make([]string, 0, len(assignments))
	contractIDs, seen := make([]string, 0), map[string]struct{}{}
	for _, assignment := range assignments {
		stageIDs = append(stageIDs, assignment.Stage)
		for _, contractID := range assignment.EvidenceContractIDs {
			if _, exists := seen[contractID]; !exists {
				seen[contractID] = struct{}{}
				contractIDs = append(contractIDs, contractID)
			}
		}
	}
	contract := agentqualification.OpenRouterLocalShadowResponseV1{WorkflowID: workflow.ID, SuiteSHA256: definition.SuiteSHA256, StageIDs: stageIDs, EvidenceContractIDs: contractIDs, BoundaryVerdict: "accepted_frozen_provider_free_contract"}
	content, _ := json.Marshal(contract)
	value := map[string]any{"id": "gen-fake-v3-" + plan.WorkflowID, "object": "chat.completion", "model": plan.APIModel, "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "cost": json.Number("0.000010"), "prompt_tokens_details": map[string]any{"cached_tokens": 0}, "completion_tokens_details": map[string]any{"reasoning_tokens": 0}}, "openrouter_metadata": map[string]any{"requested": plan.APIModel, "strategy": "direct", "attempt": 1, "endpoints": map[string]any{"total": 1, "available": []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "selected": true}}}, "attempts": []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "status": 200}}}}
	raw, _ := json.Marshal(value)
	return raw
}
