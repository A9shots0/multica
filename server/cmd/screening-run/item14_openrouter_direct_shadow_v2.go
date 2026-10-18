package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const (
	openRouterDirectCreditsURLV2    = "https://openrouter.ai/api/v1/credits"
	openRouterDirectResponseMaxV2   = 2 * 1024 * 1024
	openRouterDirectCredentialMaxV2 = 16 * 1024
)

type item14OpenRouterDirectPreflightV2 struct {
	SchemaVersion        string            `json:"schema_version"`
	DefinitionSHA256     string            `json:"definition_sha256"`
	SuiteSHA256          string            `json:"suite_sha256"`
	Model                string            `json:"model"`
	APIModel             string            `json:"api_model"`
	ServingProvider      string            `json:"serving_provider"`
	Endpoint             string            `json:"endpoint"`
	ProviderFallbacks    bool              `json:"provider_fallbacks"`
	ProviderOrderExact   bool              `json:"provider_order_exact"`
	ProviderOnlyExact    bool              `json:"provider_only_exact"`
	RouterMetadata       bool              `json:"router_metadata_required"`
	RetryCount           int               `json:"retry_count"`
	RedirectCount        int               `json:"redirect_count"`
	ProviderCallsMade    int               `json:"provider_calls_made"`
	CredentialReads      int               `json:"credential_reads"`
	RequestSHA256ByCase  map[string]string `json:"request_sha256_by_workflow"`
	ResponseFailureScope string            `json:"response_failure_scope"`
	Ready                bool              `json:"ready"`
}

type item14OpenRouterDirectClaimV2 struct {
	SchemaVersion    string `json:"schema_version"`
	RunID            string `json:"run_id"`
	DefinitionSHA256 string `json:"definition_sha256"`
	SuiteSHA256      string `json:"suite_sha256"`
	Model            string `json:"model"`
	APIModel         string `json:"api_model"`
	ServingProvider  string `json:"serving_provider"`
	MaxProviderCalls int    `json:"max_provider_calls"`
	RetryCount       int    `json:"retry_count"`
	ClaimedAt        string `json:"claimed_at"`
	ReplayPolicy     string `json:"replay_policy"`
}

type item14OpenRouterDirectAccountingV2 struct {
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
	CredentialReads               int    `json:"credential_reads"`
	CredentialCopyPersisted       bool   `json:"credential_copy_persisted"`
	CredentialMemoryZeroed        bool   `json:"credential_memory_zeroed"`
}

type item14OpenRouterDirectDependenciesV2 struct {
	Contracts      agentqualification.QualificationContracts
	ContractError  error
	Fake           bool
	OpenCredential func() ([]byte, error)
	SpendSnapshot  func(context.Context, []byte, time.Time) (agentqualification.APISpendSnapshot, error)
	InvokeOnce     func(context.Context, []byte, string, agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error)
}

func item14OpenRouterDirectPlanV2(optional ...agentqualification.QualificationContracts) (map[string]any, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, _, plans, err := loadItem14OpenRouterDirectPlansV2(q)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status": "frozen_not_launched", "provider_called": false, "credential_read": false,
		"definition": definition, "definition_sha256": agentqualification.OpenRouterDirectShadowDefinitionV2SHA256(),
		"plans": directPlanSummariesV2(plans),
	}, nil
}

func item14OpenRouterDirectPrepareV2(optional ...agentqualification.QualificationContracts) (item14OpenRouterDirectPreflightV2, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, _, plans, err := loadItem14OpenRouterDirectPlansV2(q)
	if err != nil {
		return item14OpenRouterDirectPreflightV2{}, err
	}
	hashes := make(map[string]string, len(plans))
	for _, plan := range plans {
		if err := q.ValidateOpenRouterDirectShadowPlanV2(definition, plan); err != nil {
			return item14OpenRouterDirectPreflightV2{}, err
		}
		hashes[plan.WorkflowID] = plan.RequestSHA256
	}
	preflight := item14OpenRouterDirectPreflightV2{
		SchemaVersion: "silver-mediums.openrouter-direct-shadow-preflight.v2", DefinitionSHA256: agentqualification.OpenRouterDirectShadowDefinitionV2SHA256(), SuiteSHA256: definition.SuiteSHA256,
		Model: definition.Candidate.Model, APIModel: definition.Candidate.APIModel, ServingProvider: definition.Candidate.ServingProvider, Endpoint: definition.Endpoint,
		ProviderFallbacks: false, ProviderOrderExact: true, ProviderOnlyExact: true, RouterMetadata: true, RetryCount: 0, RedirectCount: 0, ProviderCallsMade: 0, CredentialReads: 0,
		RequestSHA256ByCase: hashes, ResponseFailureScope: definition.ResponseContractFailurePolicy, Ready: len(hashes) == definition.MaxProviderCalls,
	}
	if !preflight.Ready {
		return preflight, fmt.Errorf("Item 14 OpenRouter direct shadow preflight incomplete")
	}
	return preflight, nil
}

func item14OpenRouterDirectFakeV2() (agentqualification.OpenRouterDirectShadowRunV2, error) {
	tempRoot, err := os.MkdirTemp("", "multica-item14-openrouter-direct-fake-")
	if err != nil {
		return agentqualification.OpenRouterDirectShadowRunV2{}, err
	}
	defer os.RemoveAll(tempRoot)
	evidenceParent := filepath.Join(tempRoot, ".qualification-evidence")
	if err := os.MkdirAll(evidenceParent, 0o700); err != nil {
		return agentqualification.OpenRouterDirectShadowRunV2{}, err
	}
	return executeItem14OpenRouterDirectV2(context.Background(), filepath.Join(evidenceParent, "item14-openrouter-direct-shadow-v2"), newFakeItem14OpenRouterDirectDependenciesV2())
}

func executeItem14OpenRouterDirectRealV2(ctx context.Context, artifactRoot string) (agentqualification.OpenRouterDirectShadowRunV2, error) {
	dependencies := item14OpenRouterDirectDependenciesV2{
		OpenCredential: openOpenRouterCredentialOnceV2,
		SpendSnapshot:  readOpenRouterSpendSnapshotWithCredentialV2,
		InvokeOnce:     invokeOpenRouterDirectOnceV2,
	}
	return executeItem14OpenRouterDirectV2(ctx, artifactRoot, dependencies)
}

func executeItem14OpenRouterDirectV2(ctx context.Context, artifactRoot string, dependencies item14OpenRouterDirectDependenciesV2) (run agentqualification.OpenRouterDirectShadowRunV2, returnErr error) {
	if dependencies.ContractError != nil {
		return run, dependencies.ContractError
	}
	if dependencies.Contracts.Historical() && !dependencies.Fake {
		return run, fmt.Errorf("historical inputs cannot authorize real execution")
	}
	q := dependencies.Contracts

	definition, suite, plans, err := loadItem14OpenRouterDirectPlansV2(q)
	if err != nil {
		return run, err
	}
	if dependencies.SpendSnapshot == nil || dependencies.InvokeOnce == nil || (!dependencies.Fake && dependencies.OpenCredential == nil) {
		return run, fmt.Errorf("Item 14 OpenRouter direct shadow dependencies are incomplete")
	}
	preflight, err := item14OpenRouterDirectPrepareV2(q)
	if err != nil {
		return run, err
	}
	if !filepath.IsAbs(artifactRoot) {
		return run, fmt.Errorf("Item 14 OpenRouter direct shadow evidence root must be absolute")
	}
	if _, statErr := os.Stat(artifactRoot); statErr == nil || !os.IsNotExist(statErr) {
		return run, fmt.Errorf("Item 14 OpenRouter direct shadow evidence root already exists or cannot be proven absent")
	}
	claim := item14OpenRouterDirectClaimV2{
		SchemaVersion: "silver-mediums.openrouter-direct-shadow-claim.v2", RunID: definition.RunID, DefinitionSHA256: agentqualification.OpenRouterDirectShadowDefinitionV2SHA256(), SuiteSHA256: definition.SuiteSHA256,
		Model: definition.Candidate.Model, APIModel: definition.Candidate.APIModel, ServingProvider: definition.Candidate.ServingProvider, MaxProviderCalls: definition.MaxProviderCalls, RetryCount: 0,
		ClaimedAt: time.Now().UTC().Format(time.RFC3339Nano), ReplayPolicy: "any_claim_without_a_terminal_run_is_ambiguous_and_never_replayed",
	}
	authorityClaimPath := filepath.Join(filepath.Dir(artifactRoot), "item14-openrouter-direct-authority-v2.claim.json")
	if err := writeExclusiveJSON(authorityClaimPath, claim); err != nil {
		return run, err
	}
	if err := os.Mkdir(artifactRoot, 0o700); err != nil {
		return run, fmt.Errorf("create write-once OpenRouter direct shadow root: %w", err)
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
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "credits_failure", "provider_calls": 0, "retry_allowed": false, "error": err.Error()})
		return run, err
	}
	if before.SpentMicros > definition.CumulativeOpenRouterCeilingMicros-definition.TotalPriceReservationMicros {
		err = fmt.Errorf("OpenRouter cumulative spend leaves insufficient room for the direct-shadow reservation")
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "preventive_spend_denial", "provider_calls": 0, "retry_allowed": false, "spent_micros": before.SpentMicros})
		return run, err
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
		workflowClaim := map[string]any{
			"schema_version": "silver-mediums.openrouter-direct-shadow-workflow-claim.v2", "run_id": definition.RunID, "workflow_id": plan.WorkflowID,
			"sequence": index + 1, "prompt_sha256": plan.PromptSHA256, "schema_sha256": plan.SchemaSHA256, "request_sha256": plan.RequestSHA256,
			"model": plan.Model, "api_model": plan.APIModel, "serving_provider": plan.ServingProvider, "max_provider_calls": 1, "retry_count": 0, "claimed_at": time.Now().UTC().Format(time.RFC3339Nano),
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "claim.json"), workflowClaim); err != nil {
			return run, err
		}
		currentSpend, err := dependencies.SpendSnapshot(ctx, credential, time.Now().UTC())
		if err != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "pre_call_credits_failure", "provider_calls": 0, "retry_allowed": false, "error": err.Error()})
			return run, err
		}
		usedIncrement := currentSpend.SpentMicros - before.SpentMicros
		if usedIncrement < 0 || usedIncrement+definition.PerCallReservationMicros > definition.SliceIncrementalCeilingMicros || currentSpend.SpentMicros+definition.PerCallReservationMicros > definition.CumulativeOpenRouterCeilingMicros {
			err = fmt.Errorf("OpenRouter preventive direct-shadow reservation denied")
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "preventive_per_call_spend_denial", "provider_calls": 0, "retry_allowed": false, "spent_micros": currentSpend.SpentMicros})
			return run, err
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
		postSpend, postSpendErr := dependencies.SpendSnapshot(ctx, credential, time.Now().UTC())
		if postSpendErr == nil {
			postSpendErr = writeExclusiveJSON(filepath.Join(workflowDir, "spend-after.json"), postSpend)
			lastSpend = postSpend
		}
		if postSpendErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "post_exposure_accounting_failure", "provider_calls_at_most": 1, "retry_allowed": false, "error": postSpendErr.Error()})
			return run, postSpendErr
		}
		if invokeErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "transport_or_http_failure", "provider_calls_at_most": requests, "retry_allowed": false, "error": invokeErr.Error()})
			return run, invokeErr
		}
		observation, observationErr := q.BuildOpenRouterDirectShadowObservationV2(definition, suite, plan, raw, requests, started.Format(time.RFC3339Nano), completed.Format(time.RFC3339Nano), completed.Sub(started).Milliseconds())
		if observationErr != nil {
			_ = writeExclusiveJSON(filepath.Join(workflowDir, "global-halt.json"), map[string]any{"class": "identity_provider_or_evidence_failure", "provider_calls": requests, "retry_allowed": false, "error": observationErr.Error()})
			return run, observationErr
		}
		if err := writeExclusiveJSON(filepath.Join(workflowDir, "observation.json"), observation); err != nil {
			return run, err
		}
		observations = append(observations, observation)
	}
	run, err = q.BuildOpenRouterDirectShadowRunV2(definition, suite, observations, dependencies.Fake)
	if err != nil {
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "global-halt.json"), map[string]any{"class": "aggregate_evidence_integrity_failure", "provider_calls": len(observations), "retry_allowed": false, "error": err.Error()})
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
	accounting := item14OpenRouterDirectAccountingV2{
		SchemaVersion: "silver-mediums.openrouter-direct-shadow-accounting.v2", Provider: before.Provider, ProjectLane: before.ProjectLane, WindowID: before.WindowID,
		BeforeSpentMicros: before.SpentMicros, AfterSpentMicros: lastSpend.SpentMicros, ObservedIncrementMicros: increment, ReceiptCostMicros: run.TotalCostMicros,
		SliceIncrementalCeilingMicros: definition.SliceIncrementalCeilingMicros, CumulativeCeilingMicros: definition.CumulativeOpenRouterCeilingMicros,
		BeforeObservedAt: before.ObservedAt, AfterObservedAt: lastSpend.ObservedAt, Source: lastSpend.Source, CredentialReads: credentialReads,
		CredentialCopyPersisted: false, CredentialMemoryZeroed: true,
	}
	if increment < 0 || increment > definition.SliceIncrementalCeilingMicros || lastSpend.SpentMicros > definition.CumulativeOpenRouterCeilingMicros || run.TotalCostMicros > definition.SliceIncrementalCeilingMicros {
		_ = writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting)
		return run, fmt.Errorf("Item 14 OpenRouter direct shadow spend evidence exceeded its boundary")
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "accounting.json"), accounting); err != nil {
		return run, err
	}
	if err := writeExclusiveJSON(filepath.Join(artifactRoot, "run.json"), run); err != nil {
		return run, err
	}
	return run, nil
}

func loadItem14OpenRouterDirectPlansV2(optional ...agentqualification.QualificationContracts) (agentqualification.OpenRouterDirectShadowDefinitionV2, agentqualification.LocalShadowWorkflowSuiteV1, []agentqualification.OpenRouterDirectShadowPlanV2, error) {
	q := agentqualification.QualificationContracts{}
	if len(optional) > 0 {
		q = optional[0]
	}

	definition, err := q.CurrentOpenRouterDirectShadowDefinitionV2()
	if err != nil {
		return definition, agentqualification.LocalShadowWorkflowSuiteV1{}, nil, err
	}
	suite, err := q.CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		return definition, suite, nil, err
	}
	plans, err := q.BuildOpenRouterDirectShadowPlansV2(definition, suite)
	return definition, suite, plans, err
}

func directPlanSummariesV2(plans []agentqualification.OpenRouterDirectShadowPlanV2) []map[string]any {
	result := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		result = append(result, map[string]any{"workflow_id": plan.WorkflowID, "model": plan.Model, "api_model": plan.APIModel, "serving_provider": plan.ServingProvider, "provider_fallbacks": false, "request_sha256": plan.RequestSHA256})
	}
	return result
}

func newFakeItem14OpenRouterDirectDependenciesV2() item14OpenRouterDirectDependenciesV2 {
	q, contractErr := agentqualification.HistoricalQualificationContracts()

	calls := 0
	return item14OpenRouterDirectDependenciesV2{Contracts: q, ContractError: contractErr,
		Fake: true,
		SpendSnapshot: func(_ context.Context, _ []byte, observedAt time.Time) (agentqualification.APISpendSnapshot, error) {
			return agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "direct_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: 110000000 + int64(calls*10), ObservedAt: observedAt.Format(time.RFC3339Nano), Source: "fake_authoritative_api_usage"}, nil
		},
		InvokeOnce: func(_ context.Context, _ []byte, _ string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
			calls++
			return fakeOpenRouterDirectResponseV2(plan), 1, nil
		},
	}
}

func fakeOpenRouterDirectResponseV2(plan agentqualification.OpenRouterDirectShadowPlanV2) []byte {
	q, contractErr := agentqualification.HistoricalQualificationContracts()
	if contractErr != nil {
		return nil
	}

	definition, suite, _, err := loadItem14OpenRouterDirectPlansV2(q)
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
	contractIDs := make([]string, 0)
	seenContracts := map[string]struct{}{}
	for _, assignment := range assignments {
		stageIDs = append(stageIDs, assignment.Stage)
		for _, contractID := range assignment.EvidenceContractIDs {
			if _, exists := seenContracts[contractID]; exists {
				continue
			}
			seenContracts[contractID] = struct{}{}
			contractIDs = append(contractIDs, contractID)
		}
	}
	contract := agentqualification.OpenRouterLocalShadowResponseV1{WorkflowID: workflow.ID, SuiteSHA256: definition.SuiteSHA256, StageIDs: stageIDs, EvidenceContractIDs: contractIDs, BoundaryVerdict: "accepted_frozen_provider_free_contract"}
	content, _ := json.Marshal(contract)
	value := map[string]any{
		"id": "gen-fake-" + plan.WorkflowID, "object": "chat.completion", "model": plan.APIModel,
		"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}},
		"usage":   map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "cost": json.Number("0.000010"), "prompt_tokens_details": map[string]any{"cached_tokens": 0}, "completion_tokens_details": map[string]any{"reasoning_tokens": 0}},
		"openrouter_metadata": map[string]any{
			"requested": plan.APIModel, "strategy": "direct", "attempt": 1,
			"endpoints": map[string]any{"total": 1, "available": []any{map[string]any{"provider": "Alibaba", "model": plan.APIModel, "selected": true}}},
			"attempts":  []any{map[string]any{"provider": "Alibaba", "model": plan.APIModel, "status": 200}},
		},
	}
	raw, _ := json.Marshal(value)
	return raw
}

func openOpenRouterCredentialOnceV2() ([]byte, error) {
	profile, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(profile, ".local", "share", "opencode", "auth.json"))
	if err != nil {
		return nil, fmt.Errorf("OpenRouter credential source unavailable: %w", err)
	}
	defer func() {
		for index := range raw {
			raw[index] = 0
		}
	}()
	if len(raw) > openRouterDirectCredentialMaxV2 {
		return nil, fmt.Errorf("OpenRouter credential source exceeded its size limit")
	}
	var auth struct {
		OpenRouter struct {
			Type string          `json:"type"`
			Key  json.RawMessage `json:"key"`
		} `json:"openrouter"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&auth); err != nil || decoder.Decode(&struct{}{}) != io.EOF || auth.OpenRouter.Type != "api" {
		return nil, fmt.Errorf("OpenRouter credential source is malformed")
	}
	keyRaw := bytes.TrimSpace(auth.OpenRouter.Key)
	defer func() {
		for index := range auth.OpenRouter.Key {
			auth.OpenRouter.Key[index] = 0
		}
	}()
	if len(keyRaw) < 3 || keyRaw[0] != '"' || keyRaw[len(keyRaw)-1] != '"' || bytes.ContainsRune(keyRaw[1:len(keyRaw)-1], '\\') {
		return nil, fmt.Errorf("OpenRouter credential source contains an unsupported key encoding")
	}
	credential := append([]byte(nil), keyRaw[1:len(keyRaw)-1]...)
	if len(bytes.TrimSpace(credential)) != len(credential) || bytes.ContainsAny(credential, "\x00\r\n") {
		for index := range credential {
			credential[index] = 0
		}
		return nil, fmt.Errorf("OpenRouter credential is malformed")
	}
	return credential, nil
}

func readOpenRouterSpendSnapshotWithCredentialV2(ctx context.Context, credential []byte, observedAt time.Time) (agentqualification.APISpendSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterDirectCreditsURLV2, nil)
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	request.Header.Set("Authorization", "Bearer "+string(credential))
	transport := newOpenRouterDirectHTTPTransportV2()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	request.Header.Del("Authorization")
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	defer response.Body.Close()
	raw, err := readOpenRouterDirectLimitedV2(response.Body)
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	if response.StatusCode != http.StatusOK {
		return agentqualification.APISpendSnapshot{}, fmt.Errorf("OpenRouter credits read failed with status %d", response.StatusCode)
	}
	var credits struct {
		Data struct {
			TotalUsage json.Number `json:"total_usage"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&credits); err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	spentMicros, err := ceilingUSDMicros(credits.Data.TotalUsage.String())
	if err != nil {
		return agentqualification.APISpendSnapshot{}, err
	}
	return agentqualification.APISpendSnapshot{Provider: "openrouter", ProjectLane: "direct_openrouter", Currency: "USD", WindowID: "openrouter_account_lifetime", SpentMicros: spentMicros, ObservedAt: observedAt.Format(time.RFC3339Nano), Source: "authoritative_api_usage"}, nil
}

func invokeOpenRouterDirectOnceV2(ctx context.Context, credential []byte, endpoint string, plan agentqualification.OpenRouterDirectShadowPlanV2) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	request.Body = io.NopCloser(bytes.NewReader(plan.RequestBody))
	request.ContentLength = int64(len(plan.RequestBody))
	request.GetBody = nil
	request.Header.Set("Authorization", "Bearer "+string(credential))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-OpenRouter-Metadata", "enabled")
	base := newOpenRouterDirectHTTPTransportV2()
	defer base.CloseIdleConnections()
	gate := &openRouterDirectRoundTripperV2{endpoint: endpoint, plan: plan, base: base}
	client := &http.Client{Transport: gate, Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	request.Header.Del("Authorization")
	requests := int(gate.requests.Load())
	if err != nil {
		return nil, requests, fmt.Errorf("OpenRouter direct request failed without retry: %w", err)
	}
	if response == nil || response.Body == nil {
		return nil, requests, fmt.Errorf("OpenRouter direct request returned no response")
	}
	defer response.Body.Close()
	raw, err := readOpenRouterDirectLimitedV2(response.Body)
	if err != nil {
		return raw, requests, err
	}
	if response.StatusCode != http.StatusOK {
		return raw, requests, fmt.Errorf("OpenRouter direct request returned status %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return raw, requests, fmt.Errorf("OpenRouter direct request returned non-JSON content")
	}
	return raw, requests, nil
}

type openRouterDirectRoundTripperV2 struct {
	endpoint string
	plan     agentqualification.OpenRouterDirectShadowPlanV2
	base     http.RoundTripper
	requests atomic.Int32
}

func (gate *openRouterDirectRoundTripperV2) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || !gate.requests.CompareAndSwap(0, 1) {
		return nil, fmt.Errorf("OpenRouter direct transport denied a duplicate request")
	}
	if request.Method != http.MethodPost || request.URL.String() != gate.endpoint || request.GetBody != nil || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Accept") != "application/json" || request.Header.Get("X-OpenRouter-Metadata") != "enabled" || strings.TrimSpace(request.Header.Get("Authorization")) == "" || request.Header.Get("Cookie") != "" {
		return nil, fmt.Errorf("OpenRouter direct request envelope drifted")
	}
	raw, err := readOpenRouterDirectLimitedV2(request.Body)
	if err != nil {
		return nil, err
	}
	if agentqualification.GuardedPromptSHA256V1(string(raw)) != gate.plan.RequestSHA256 {
		return nil, fmt.Errorf("OpenRouter direct request body drifted")
	}
	request.Body = io.NopCloser(bytes.NewReader(raw))
	request.ContentLength = int64(len(raw))
	request.GetBody = nil
	return gate.base.RoundTrip(request)
}

func newOpenRouterDirectHTTPTransportV2() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: -1}
	return &http.Transport{
		Proxy: nil, DialContext: dialer.DialContext, ForceAttemptHTTP2: false, DisableKeepAlives: true, DisableCompression: true,
		MaxConnsPerHost: 1, ResponseHeaderTimeout: 60 * time.Second, TLSHandshakeTimeout: 10 * time.Second, MaxResponseHeaderBytes: 64 * 1024,
		ExpectContinueTimeout: 0, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

func readOpenRouterDirectLimitedV2(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, openRouterDirectResponseMaxV2+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > openRouterDirectResponseMaxV2 {
		return nil, fmt.Errorf("OpenRouter direct payload exceeded its limit")
	}
	return raw, nil
}

func verifyCredentialAbsentFromEvidenceV2(root string, credential []byte) error {
	if len(credential) == 0 {
		return fmt.Errorf("OpenRouter credential absence check has no reference value")
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(raw, credential) {
			return fmt.Errorf("credential material appeared in evidence")
		}
		return nil
	})
}
