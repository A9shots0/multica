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

type item14OpenRouterDirectPreflightV4 struct {
	SchemaVersion                 string            `json:"schema_version"`
	DefinitionSHA256              string            `json:"definition_sha256"`
	AntecedentResponseSHA256      string            `json:"antecedent_response_sha256"`
	SuiteSHA256                   string            `json:"suite_sha256"`
	Model                         string            `json:"model"`
	APIModel                      string            `json:"api_model"`
	ExpectedEndpointModel         string            `json:"expected_endpoint_model"`
	ServingProvider               string            `json:"serving_provider"`
	ProviderFallbacks             bool              `json:"provider_fallbacks"`
	ProviderOrderExact            bool              `json:"provider_order_exact"`
	ProviderOnlyExact             bool              `json:"provider_only_exact"`
	TopLevelProviderRequired      bool              `json:"top_level_provider_required"`
	ProviderAttemptEvidencePolicy string            `json:"provider_attempt_evidence_policy"`
	RetryCount                    int               `json:"retry_count"`
	RedirectCount                 int               `json:"redirect_count"`
	ProviderCallsMade             int               `json:"provider_calls_made"`
	CredentialReads               int               `json:"credential_reads"`
	RequestSHA256ByCase           map[string]string `json:"request_sha256_by_workflow"`
	Ready                         bool              `json:"ready"`
}

type item14OpenRouterDirectTerminalV4 struct {
	SchemaVersion             string                                                        `json:"schema_version"`
	Status                    string                                                        `json:"status"`
	FailureClass              string                                                        `json:"failure_class"`
	Failure                   string                                                        `json:"failure"`
	RunID                     string                                                        `json:"run_id"`
	DefinitionSHA256          string                                                        `json:"definition_sha256"`
	AntecedentResponseSHA256  string                                                        `json:"antecedent_response_sha256"`
	WorkflowID                string                                                        `json:"workflow_id"`
	ProviderRequestCount      int                                                           `json:"provider_request_count"`
	CompletedObservationCount int                                                           `json:"completed_observation_count"`
	RemainingWorkflowCount    int                                                           `json:"remaining_workflow_count"`
	ResponseAccounting        agentqualification.OpenRouterDirectShadowResponseAccountingV4 `json:"response_accounting"`
	BeforeSpentMicros         int64                                                         `json:"before_spent_micros"`
	AfterSpentMicros          int64                                                         `json:"after_spent_micros"`
	PostSpendObserved         bool                                                          `json:"post_spend_observed"`
	ObservedIncrementMicros   int64                                                         `json:"observed_increment_micros"`
	CredentialReads           int                                                           `json:"credential_reads"`
	CredentialCopyPersisted   bool                                                          `json:"credential_copy_persisted"`
	CredentialMemoryZeroed    bool                                                          `json:"credential_memory_zeroed"`
	AuthorityExhausted        bool                                                          `json:"authority_exhausted"`
	RetryAllowed              bool                                                          `json:"retry_allowed"`
}

func item14OpenRouterDirectPlanV4(optional ...agentqualification.QualificationContracts) (map[string]any, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, _, plans, err := loadItem14OpenRouterDirectPlansV4(q)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "frozen_not_launched", "provider_called": false, "credential_read": false, "definition": definition, "definition_sha256": agentqualification.OpenRouterDirectShadowDefinitionV4SHA256(), "plans": directPlanSummariesV2(plans)}, nil
}

func item14OpenRouterDirectPrepareV4(optional ...agentqualification.QualificationContracts) (item14OpenRouterDirectPreflightV4, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, _, plans, err := loadItem14OpenRouterDirectPlansV4(q)
	if err != nil {
		return item14OpenRouterDirectPreflightV4{}, err
	}
	hashes := map[string]string{}
	for _, plan := range plans {
		if err := q.ValidateOpenRouterDirectShadowPlanV4(definition, plan); err != nil {
			return item14OpenRouterDirectPreflightV4{}, err
		}
		hashes[plan.WorkflowID] = plan.RequestSHA256
	}
	return item14OpenRouterDirectPreflightV4{SchemaVersion: "silver-mediums.openrouter-direct-shadow-preflight.v4", DefinitionSHA256: agentqualification.OpenRouterDirectShadowDefinitionV4SHA256(), AntecedentResponseSHA256: definition.AntecedentV3ResponseSHA256, SuiteSHA256: definition.SuiteSHA256, Model: definition.Candidate.Model, APIModel: definition.Candidate.APIModel, ExpectedEndpointModel: definition.ExpectedEndpointModel, ServingProvider: definition.Candidate.ServingProvider, ProviderFallbacks: false, ProviderOrderExact: true, ProviderOnlyExact: true, TopLevelProviderRequired: true, ProviderAttemptEvidencePolicy: definition.ProviderAttemptEvidencePolicy, RetryCount: 0, RedirectCount: 0, ProviderCallsMade: 0, CredentialReads: 0, RequestSHA256ByCase: hashes, Ready: len(hashes) == 3}, nil
}

func item14OpenRouterDirectFakeV4() (agentqualification.OpenRouterDirectShadowRunV4, error) {
	tempRoot, err := os.MkdirTemp("", "multica-item14-openrouter-direct-v4-fake-")
	if err != nil {
		return agentqualification.OpenRouterDirectShadowRunV4{}, err
	}
	defer os.RemoveAll(tempRoot)
	parent := filepath.Join(tempRoot, ".qualification-evidence")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return agentqualification.OpenRouterDirectShadowRunV4{}, err
	}
	return executeItem14OpenRouterDirectV4(context.Background(), filepath.Join(parent, "item14-openrouter-direct-shadow-v4"), newFakeItem14OpenRouterDirectDependenciesV4())
}

func executeItem14OpenRouterDirectRealV4(ctx context.Context, root string) (agentqualification.OpenRouterDirectShadowRunV4, error) {
	return executeItem14OpenRouterDirectV4(ctx, root, item14OpenRouterDirectDependenciesV2{OpenCredential: openOpenRouterCredentialOnceV2, SpendSnapshot: readOpenRouterSpendSnapshotWithCredentialV2, InvokeOnce: invokeOpenRouterDirectOnceV2})
}

func executeItem14OpenRouterDirectV4(ctx context.Context, root string, deps item14OpenRouterDirectDependenciesV2) (run agentqualification.OpenRouterDirectShadowRunV4, returnErr error) {
	if deps.ContractError != nil {
		return run, deps.ContractError
	}
	if deps.Contracts.Historical() && !deps.Fake {
		return run, fmt.Errorf("historical inputs cannot authorize real execution")
	}
	q := deps.Contracts

	definition, suite, plans, err := loadItem14OpenRouterDirectPlansV4(q)
	if err != nil {
		return run, err
	}
	if deps.SpendSnapshot == nil || deps.InvokeOnce == nil || (!deps.Fake && deps.OpenCredential == nil) {
		return run, fmt.Errorf("Item 14 OpenRouter direct v4 dependencies incomplete")
	}
	preflight, err := item14OpenRouterDirectPrepareV4(q)
	if err != nil || !preflight.Ready {
		return run, fmt.Errorf("Item 14 OpenRouter direct v4 preflight failed: %w", err)
	}
	if !filepath.IsAbs(root) {
		return run, fmt.Errorf("Item 14 OpenRouter direct v4 root must be absolute")
	}
	if _, statErr := os.Stat(root); statErr == nil || !os.IsNotExist(statErr) {
		return run, fmt.Errorf("Item 14 OpenRouter direct v4 root already exists or cannot be proven absent")
	}
	claim := map[string]any{"schema_version": "silver-mediums.openrouter-direct-shadow-claim.v4", "run_id": definition.RunID, "definition_sha256": agentqualification.OpenRouterDirectShadowDefinitionV4SHA256(), "antecedent_response_sha256": definition.AntecedentV3ResponseSHA256, "suite_sha256": definition.SuiteSHA256, "model": definition.Candidate.Model, "expected_endpoint_model": definition.ExpectedEndpointModel, "serving_provider": definition.Candidate.ServingProvider, "max_provider_calls": 3, "retry_count": 0, "claimed_at": time.Now().UTC().Format(time.RFC3339Nano), "replay_policy": "any_claim_without_a_terminal_run_is_ambiguous_and_never_replayed"}
	if err := writeExclusiveJSON(filepath.Join(filepath.Dir(root), "item14-openrouter-direct-authority-v4.claim.json"), claim); err != nil {
		return run, err
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(root, "provider-free-preflight.json"), preflight); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(root, "run-claim.json"), claim); err != nil {
		return run, err
	}
	credentialReads := 0
	credential := []byte("fake-no-credential")
	if !deps.Fake {
		credential, err = deps.OpenCredential()
		credentialReads = 1
		if err != nil {
			return run, err
		}
	}
	defer func() {
		for index := range credential {
			credential[index] = 0
		}
	}()
	before, err := deps.SpendSnapshot(ctx, credential, time.Now().UTC())
	if err != nil {
		return run, err
	}
	if before.SpentMicros > definition.CumulativeCeilingMicros-definition.TotalPriceReservationMicros {
		return run, fmt.Errorf("OpenRouter cumulative spend leaves insufficient v4 reservation")
	}
	if err := writeExclusiveJSON(filepath.Join(root, "spend-before.json"), before); err != nil {
		return run, err
	}
	observations := make([]agentqualification.OpenRouterDirectShadowObservationV2Record, 0, 3)
	lastSpend := before
	for index, plan := range plans {
		workflowDir := filepath.Join(root, fmt.Sprintf("%02d-%s", index+1, plan.WorkflowID))
		if err := os.Mkdir(workflowDir, 0o700); err != nil {
			return run, err
		}
		workflowClaim := map[string]any{"schema_version": "silver-mediums.openrouter-direct-shadow-workflow-claim.v4", "run_id": definition.RunID, "workflow_id": plan.WorkflowID, "sequence": index + 1, "request_sha256": plan.RequestSHA256, "model": plan.Model, "expected_endpoint_model": definition.ExpectedEndpointModel, "serving_provider": plan.ServingProvider, "max_provider_calls": 1, "retry_count": 0, "claimed_at": time.Now().UTC().Format(time.RFC3339Nano)}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "claim.json"), workflowClaim); err != nil {
			return run, err
		}
		current, err := deps.SpendSnapshot(ctx, credential, time.Now().UTC())
		if err != nil {
			return run, err
		}
		used := current.SpentMicros - before.SpentMicros
		if used < 0 || used+definition.PerCallReservationMicros > definition.SliceIncrementalCeilingMicros || current.SpentMicros+definition.PerCallReservationMicros > definition.CumulativeCeilingMicros {
			return run, fmt.Errorf("OpenRouter preventive direct v4 reservation denied")
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "spend-before.json"), current); err != nil {
			return run, err
		}
		started := time.Now().UTC()
		raw, requests, invokeErr := deps.InvokeOnce(ctx, credential, definition.Endpoint, plan)
		completed := time.Now().UTC()
		if len(raw) > 0 {
			if err := writeExclusiveBytes(filepath.Join(workflowDir, "provider-response.json"), raw); err != nil {
				return run, err
			}
		}
		post, postErr := deps.SpendSnapshot(ctx, credential, time.Now().UTC())
		if postErr == nil {
			postErr = writeExclusiveJSON(filepath.Join(workflowDir, "spend-after.json"), post)
			lastSpend = post
		}
		if postErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "post_exposure_accounting_failure", "provider_calls_at_most": requests, "retry_allowed": false, "error": postErr.Error()})
			if sealErr := sealItem14OpenRouterDirectTerminalV4(root, "post_exposure_accounting_failure", plan.WorkflowID, postErr, raw, len(observations)+requests, len(observations), credentialReads, before, nil, credential, q); sealErr != nil {
				return run, fmt.Errorf("%w; terminal settlement failed: %v", postErr, sealErr)
			}
			return run, postErr
		}
		if invokeErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "transport_or_http_failure", "provider_calls_at_most": requests, "retry_allowed": false, "error": invokeErr.Error()})
			if sealErr := sealItem14OpenRouterDirectTerminalV4(root, "transport_or_http_failure", plan.WorkflowID, invokeErr, raw, len(observations)+requests, len(observations), credentialReads, before, &post, credential, q); sealErr != nil {
				return run, fmt.Errorf("%w; terminal settlement failed: %v", invokeErr, sealErr)
			}
			return run, invokeErr
		}
		observation, observationErr := q.BuildOpenRouterDirectShadowObservationV4(definition, suite, plan, raw, requests, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano), completed.Sub(started).Milliseconds())
		if observationErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "identity_provider_or_evidence_failure", "provider_calls": requests, "retry_allowed": false, "error": observationErr.Error()})
			if sealErr := sealItem14OpenRouterDirectTerminalV4(root, "identity_provider_or_evidence_failure", plan.WorkflowID, observationErr, raw, len(observations)+requests, len(observations), credentialReads, before, &post, credential, q); sealErr != nil {
				return run, fmt.Errorf("%w; terminal settlement failed: %v", observationErr, sealErr)
			}
			return run, observationErr
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "observation.json"), observation); err != nil {
			return run, err
		}
		observations = append(observations, observation)
	}
	run, err = q.BuildOpenRouterDirectShadowRunV4(definition, suite, observations, deps.Fake)
	if err != nil {
		return run, err
	}
	if err := verifyCredentialAbsentFromEvidenceV2(root, credential); err != nil {
		return run, err
	}
	for index := range credential {
		credential[index] = 0
	}
	credential = nil
	increment := lastSpend.SpentMicros - before.SpentMicros
	accounting := item14OpenRouterDirectAccountingV2{SchemaVersion: "silver-mediums.openrouter-direct-shadow-accounting.v4", Provider: before.Provider, ProjectLane: before.ProjectLane, WindowID: before.WindowID, BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: lastSpend.SpentMicros, ObservedIncrementMicros: increment, ReceiptCostMicros: run.CurrentCostMicros, SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeCeilingMicros, BeforeObservedAt: before.ObservedAt, AfterObservedAt: lastSpend.ObservedAt, Source: lastSpend.Source, CredentialReads: credentialReads, CredentialCopyPersisted: false, CredentialMemoryZeroed: true}
	if increment < 0 || increment > definition.SliceIncrementalCeilingMicros || lastSpend.SpentMicros > definition.CumulativeCeilingMicros || run.CurrentCostMicros > definition.SliceIncrementalCeilingMicros {
		_ = writeExclusiveJSON(filepath.Join(root, "accounting.json"), accounting)
		return run, fmt.Errorf("OpenRouter direct v4 spend boundary exceeded")
	}
	if err := writeExclusiveJSON(filepath.Join(root, "accounting.json"), accounting); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(root, "run.json"), run); err != nil {
		return run, err
	}
	return run, nil
}

func sealItem14OpenRouterDirectTerminalV4(root, failureClass, workflowID string, failure error, raw []byte, providerRequestCount, completedObservationCount, credentialReads int, before agentqualification.APISpendSnapshot, after *agentqualification.APISpendSnapshot, credential []byte, q agentqualification.QualificationContracts) error {
	if err := verifyCredentialAbsentFromEvidenceV2(root, credential); err != nil {
		return err
	}
	responseAccounting, err := agentqualification.InspectOpenRouterDirectShadowResponseAccountingV4(raw)
	if err != nil {
		return err
	}
	for index := range credential {
		credential[index] = 0
	}
	definition, err := q.CurrentOpenRouterDirectShadowDefinitionV4()
	if err != nil {
		return err
	}
	afterSpentMicros := before.SpentMicros
	postSpendObserved := after != nil
	if postSpendObserved {
		afterSpentMicros = after.SpentMicros
	}
	remainingWorkflowCount := len(definition.WorkflowIDs) - completedObservationCount
	if providerRequestCount > completedObservationCount {
		remainingWorkflowCount--
	}
	terminal := item14OpenRouterDirectTerminalV4{
		SchemaVersion:             "silver-mediums.openrouter-direct-shadow-terminal.v4",
		Status:                    "failed_closed_global_halt",
		FailureClass:              failureClass,
		Failure:                   failure.Error(),
		RunID:                     definition.RunID,
		DefinitionSHA256:          agentqualification.OpenRouterDirectShadowDefinitionV4SHA256(),
		AntecedentResponseSHA256:  definition.AntecedentV3ResponseSHA256,
		WorkflowID:                workflowID,
		ProviderRequestCount:      providerRequestCount,
		CompletedObservationCount: completedObservationCount,
		RemainingWorkflowCount:    remainingWorkflowCount,
		ResponseAccounting:        responseAccounting,
		BeforeSpentMicros:         before.SpentMicros,
		AfterSpentMicros:          afterSpentMicros,
		PostSpendObserved:         postSpendObserved,
		ObservedIncrementMicros:   afterSpentMicros - before.SpentMicros,
		CredentialReads:           credentialReads,
		CredentialCopyPersisted:   false,
		CredentialMemoryZeroed:    true,
		AuthorityExhausted:        true,
		RetryAllowed:              false,
	}
	if err := writeExclusiveJSON(filepath.Join(root, "terminal-failure-receipt.json"), terminal); err != nil {
		return err
	}
	if after != nil {
		accounting := item14OpenRouterDirectAccountingV2{
			SchemaVersion: "silver-mediums.openrouter-direct-shadow-accounting.v4", Provider: before.Provider, ProjectLane: before.ProjectLane,
			WindowID: before.WindowID, BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: after.SpentMicros,
			ObservedIncrementMicros: after.SpentMicros - before.SpentMicros, ReceiptCostMicros: responseAccounting.ProviderReportedCostMicros,
			SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeCeilingMicros,
			BeforeObservedAt: before.ObservedAt, AfterObservedAt: after.ObservedAt, Source: after.Source, CredentialReads: credentialReads,
			CredentialCopyPersisted: false, CredentialMemoryZeroed: true,
		}
		if err := writeExclusiveJSON(filepath.Join(root, "accounting.json"), accounting); err != nil {
			return err
		}
	}
	return nil
}

func loadItem14OpenRouterDirectPlansV4(optional ...agentqualification.QualificationContracts) (agentqualification.OpenRouterDirectShadowDefinitionV4, agentqualification.LocalShadowWorkflowSuiteV1, []agentqualification.OpenRouterDirectShadowPlanV2, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, err := q.CurrentOpenRouterDirectShadowDefinitionV4()
	if err != nil {
		return definition, agentqualification.LocalShadowWorkflowSuiteV1{}, nil, err
	}
	suite, err := q.CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		return definition, suite, nil, err
	}
	plans, err := q.BuildOpenRouterDirectShadowPlansV4(definition, suite)
	return definition, suite, plans, err
}

func newFakeItem14OpenRouterDirectDependenciesV4() item14OpenRouterDirectDependenciesV2 {
	q, contractErr := agentqualification.HistoricalQualificationContracts()

	calls := 0
	return item14OpenRouterDirectDependenciesV2{Contracts: q, ContractError: contractErr, Fake: true, SpendSnapshot: func(_ context.Context, _ []byte, at time.Time) (agentqualification.APISpendSnapshot, error) {
		return agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "direct_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 110000000 + int64(calls*10), ObservedAt: at.Format(time.RFC3339Nano), Source: "fake_authoritative_api_usage"}, nil
	}, InvokeOnce: func(_ context.Context, _ []byte, _ string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
		calls++
		return fakeOpenRouterDirectResponseV4(plan), 1, nil
	}}
}

func fakeOpenRouterDirectResponseV4(plan agentqualification.OpenRouterDirectShadowPlanV2) []byte {
	q, contractErr := agentqualification.HistoricalQualificationContracts()
	if contractErr != nil {
		return nil
	}

	definition, suite, _, err := loadItem14OpenRouterDirectPlansV4(q)
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
	contractIDs := []string{}
	seen := map[string]struct{}{}
	for _, assignment := range assignments {
		stageIDs = append(stageIDs, assignment.Stage)
		for _, contractID := range assignment.EvidenceContractIDs {
			if _, ok := seen[contractID]; !ok {
				seen[contractID] = struct{}{}
				contractIDs = append(contractIDs, contractID)
			}
		}
	}
	contract := agentqualification.OpenRouterLocalShadowResponseV1{WorkflowID: workflow.ID, SuiteSHA256: definition.SuiteSHA256, StageIDs: stageIDs, EvidenceContractIDs: contractIDs, BoundaryVerdict: "accepted_frozen_provider_free_contract"}
	content, _ := json.Marshal(contract)
	value := map[string]any{"id": "gen-fake-v4-" + plan.WorkflowID, "object": "chat.completion", "model": plan.APIModel, "provider": "Alibaba", "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "cost": json.Number("0.000010"), "prompt_tokens_details": map[string]any{"cached_tokens": 0}, "completion_tokens_details": map[string]any{"reasoning_tokens": 0}}, "openrouter_metadata": map[string]any{"requested": plan.APIModel, "strategy": "direct", "attempt": 1, "endpoints": map[string]any{"total": 4, "available": []any{map[string]any{"provider": "Alibaba", "model": definition.ExpectedEndpointModel, "selected": true}}}}}
	raw, _ := json.Marshal(value)
	return raw
}
