package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type ExecutionAdapterIdentity struct {
	Provider                   string `json:"provider"`
	Runtime                    string `json:"runtime"`
	RuntimeVersion             string `json:"runtime_version"`
	Endpoint                   string `json:"endpoint"`
	BillingBoundary            string `json:"billing_boundary"`
	AccountLane                string `json:"account_lane"`
	ToolPolicy                 string `json:"tool_policy"`
	ProviderEventPolicyVersion string `json:"provider_event_policy_version"`
	ExecutorSHA256             string `json:"executor_sha256"`
	IsolationPolicySHA256      string `json:"isolation_policy_sha256"`
	EnvironmentAllowlistSHA256 string `json:"environment_allowlist_sha256"`
}

type ProviderExecutionResult struct {
	ObservationID              string                `json:"observation_id"`
	ReportedModel              string                `json:"reported_model"`
	ReportedModelVersion       string                `json:"reported_model_version,omitempty"`
	ReportedModelVersionStatus string                `json:"reported_model_version_status"`
	ReportedServingProvider    string                `json:"reported_serving_provider,omitempty"`
	ProviderResponseID         string                `json:"provider_response_id"`
	StartedAt                  string                `json:"started_at"`
	CompletedAt                string                `json:"completed_at"`
	TerminalState              string                `json:"terminal_state"`
	ObservedProviderEventTypes []string              `json:"observed_provider_event_types"`
	ObservedToolEventTypes     []string              `json:"observed_tool_event_types,omitempty"`
	RawTraceSHA256             string                `json:"raw_trace_sha256"`
	OutputSHA256               string                `json:"output_sha256"`
	Usage                      ExecutionReceiptUsage `json:"usage"`
}

type ExecutionPlanBinding struct {
	BatchID      string `json:"batch_id"`
	CandidateID  string `json:"candidate_id"`
	CaseID       string `json:"case_id"`
	Model        string `json:"model"`
	Effort       string `json:"reasoning_effort"`
	PromptSHA256 string `json:"prompt_sha256"`
	SchemaSHA256 string `json:"output_schema_sha256"`
}

func DirectExecutionPlanBinding(plan DirectScreeningPlan) ExecutionPlanBinding {
	return ExecutionPlanBinding{BatchID: plan.BatchID, CandidateID: plan.CandidateID, CaseID: plan.CaseID, Model: plan.Model, Effort: plan.Effort, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256}
}

func ClaudeExecutionPlanBinding(batchID string, plan ClaudeScreeningPlan) ExecutionPlanBinding {
	return ExecutionPlanBinding{BatchID: batchID, CandidateID: plan.CandidateID, CaseID: plan.CaseID, Model: plan.Model, Effort: plan.Effort, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256}
}

func OpenCodeExecutionPlanBinding(batchID string, plan OpenCodeScreeningPlan) ExecutionPlanBinding {
	return ExecutionPlanBinding{BatchID: batchID, CandidateID: plan.CandidateID, CaseID: plan.CaseID, Model: plan.Model, Effort: plan.Effort, PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256}
}

func ScreeningReservationSHA256(reservation ScreeningReservation) (string, error) {
	raw, err := json.Marshal(reservation)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// BuildExecutionReceipt is the sole provider-neutral normalization boundary.
// It cannot make a provider call. It rejects any drift between the admitted
// reservation, seeded assignment, immutable prompt/schema, adapter identity,
// and provider result before a write-once receipt can be sealed.
func BuildExecutionReceipt(reservation ScreeningReservation, assignment ScreeningAssignment, plan ExecutionPlanBinding, adapter ExecutionAdapterIdentity, result ProviderExecutionResult) (ExecutionReceipt, error) {
	if reservation.SchemaVersion != ScreeningReservationSchemaVersion || reservation.State != "reserved" {
		return ExecutionReceipt{}, fmt.Errorf("reservation is not an admitted immutable claim")
	}
	if reservation.BatchID != assignment.BatchID || reservation.BatchID != plan.BatchID || reservation.EvaluationBinding != assignment.EvaluationBinding || reservation.CandidateID != assignment.Configuration.ID || reservation.CaseID != assignment.CaseID || plan.CandidateID != assignment.Configuration.ID || plan.CaseID != assignment.CaseID {
		return ExecutionReceipt{}, fmt.Errorf("reservation, assignment, and plan binding mismatch")
	}
	if plan.Model != assignment.Configuration.Model || plan.Effort != assignment.Configuration.ReasoningEffort || !validSHA256(plan.PromptSHA256) || !validSHA256(plan.SchemaSHA256) {
		return ExecutionReceipt{}, fmt.Errorf("plan configuration or content binding mismatch")
	}
	if result.ReportedModel != plan.Model {
		return ExecutionReceipt{}, fmt.Errorf("provider-reported model does not match admitted model")
	}
	configurationHash, err := ConfigurationSHA256(assignment.Configuration)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	reservationHash, err := ScreeningReservationSHA256(reservation)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	actualServingProvider := result.ReportedServingProvider
	servingProviderEvidence := "provider_reported_metadata"
	if result.ReportedServingProvider != "" && assignment.Configuration.ServingProvider != "" && !strings.EqualFold(result.ReportedServingProvider, assignment.Configuration.ServingProvider) {
		return ExecutionReceipt{}, fmt.Errorf("provider-reported serving provider does not match admitted provider")
	}
	if actualServingProvider == "" && assignment.Configuration.ServingProvider != "" && assignment.Configuration.ProviderFallbacks != nil && !*assignment.Configuration.ProviderFallbacks {
		actualServingProvider = assignment.Configuration.ServingProvider
		servingProviderEvidence = "enforced_single_route_no_fallback"
	}
	receipt := ExecutionReceipt{
		SchemaVersion: ExecutionReceiptSchemaVersion, BatchID: reservation.BatchID, ObservationID: result.ObservationID,
		EvaluationBinding: reservation.EvaluationBinding, ReservationSHA256: reservationHash, ConfigurationSHA256: configurationHash,
		PromptSHA256: plan.PromptSHA256, SchemaSHA256: plan.SchemaSHA256, AdmissionInputSHA256: reservation.AdmissionInputSHA256,
		Provider: adapter.Provider, Runtime: adapter.Runtime, RuntimeVersion: adapter.RuntimeVersion, Endpoint: adapter.Endpoint,
		BillingBoundary: adapter.BillingBoundary, AccountLane: adapter.AccountLane, RequestedModel: plan.Model, RequestedEffort: plan.Effort,
		ReportedModel: result.ReportedModel, ReportedModelVersion: result.ReportedModelVersion, ReportedModelVersionStatus: result.ReportedModelVersionStatus,
		RequestedServingProvider: assignment.Configuration.ServingProvider, ProviderFallbacks: assignment.Configuration.ProviderFallbacks, ReportedServingProvider: result.ReportedServingProvider,
		ActualServingProvider: actualServingProvider, ServingProviderEvidence: servingProviderEvidence,
		ProviderResponseID: result.ProviderResponseID, StartedAt: result.StartedAt, CompletedAt: result.CompletedAt, TerminalState: result.TerminalState,
		ToolPolicy: adapter.ToolPolicy, ProviderEventPolicyVersion: adapter.ProviderEventPolicyVersion,
		ObservedProviderEventTypes: append([]string(nil), result.ObservedProviderEventTypes...), ObservedToolEventTypes: append([]string(nil), result.ObservedToolEventTypes...),
		RawTraceSHA256: result.RawTraceSHA256, OutputSHA256: result.OutputSHA256, ExecutorSHA256: adapter.ExecutorSHA256,
		IsolationPolicySHA256: adapter.IsolationPolicySHA256, EnvironmentAllowlistSHA256: adapter.EnvironmentAllowlistSHA256, Usage: result.Usage,
	}
	if reasons := ValidateExecutionReceipt(receipt); len(reasons) != 0 {
		return ExecutionReceipt{}, fmt.Errorf("normalized execution receipt invalid: %v", reasons)
	}
	return receipt, nil
}
