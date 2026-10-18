package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"slices"
)

const IntegrationReplayResultV1Schema = "silver-mediums.integration-replay-result.v1"

type IntegrationReplayTaskV1 struct {
	Stage       string `json:"stage"`
	TaskID      string `json:"task_id"`
	AgentID     string `json:"agent_id"`
	Status      string `json:"status"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	DurationMS  int    `json:"duration_ms"`
}

type IntegrationReplayTransportV1 struct {
	Lane                                string   `json:"lane"`
	RuntimeProvider                     string   `json:"runtime_provider"`
	RequestedPrimaryModel               string   `json:"requested_primary_model"`
	ReasoningEffort                     string   `json:"reasoning_effort"`
	ObservedModels                      []string `json:"observed_models"`
	ExactSingleModelProven              bool     `json:"exact_single_model_proven"`
	UnderlyingHTTPRequestCountCertified bool     `json:"underlying_http_request_count_certified"`
	AutomaticRetries                    int      `json:"automatic_retries"`
	Fallbacks                           bool     `json:"fallbacks"`
	Tools                               int      `json:"tools"`
	ProviderChildRepositoryAccesses     int      `json:"provider_child_repository_accesses"`
}

type IntegrationReplayUsageV1 struct {
	Rows                 int    `json:"rows"`
	SonnetRows           int    `json:"sonnet_rows"`
	HaikuRows            int    `json:"haiku_rows"`
	InputTokens          int    `json:"input_tokens"`
	OutputTokens         int    `json:"output_tokens"`
	CacheReadTokens      int    `json:"cache_read_tokens"`
	CacheWriteTokens     int    `json:"cache_write_tokens"`
	TotalAccountedTokens int    `json:"total_accounted_tokens"`
	DatabaseCostReported bool   `json:"database_cost_reported"`
	UIEstimatedCostUSD   string `json:"ui_estimated_cost_usd"`
}

type IntegrationReplayOwnerProjectionV1 struct {
	IssueStatus           string `json:"issue_status"`
	PlanState             string `json:"plan_state"`
	PictureRevision       int    `json:"picture_revision"`
	ScopeRevision         int    `json:"scope_revision"`
	OutcomeState          string `json:"outcome_state"`
	ExecutionState        string `json:"execution_state"`
	ReleaseState          string `json:"release_state"`
	AttentionState        string `json:"attention_state"`
	Milestone             string `json:"milestone"`
	BoardIssueInboxPassed bool   `json:"board_issue_inbox_passed"`
}

type IntegrationReplayArtifactV1 struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type IntegrationReplayReportV1 struct {
	Roots               int                           `json:"roots"`
	RequiredPathChecks  int                           `json:"required_path_checks"`
	CompletedPathChecks int                           `json:"completed_path_checks"`
	Present             int                           `json:"present"`
	Absent              int                           `json:"absent"`
	Artifacts           []IntegrationReplayArtifactV1 `json:"artifacts"`
}

type IntegrationReplaySourceBindingsV1 struct {
	LaunchContractPath    string `json:"launch_contract_path"`
	LaunchContractSHA256  string `json:"launch_contract_sha256"`
	BrokerPath            string `json:"broker_path"`
	BrokerSHA256          string `json:"broker_sha256"`
	AuthorityHelperPath   string `json:"authority_helper_path"`
	AuthorityHelperSHA256 string `json:"authority_helper_sha256"`
	ControllerImageDigest string `json:"controller_image_digest"`
}

type IntegrationReplayCleanupV1 struct {
	RuntimeOffline        bool `json:"runtime_offline"`
	DaemonTokens          int  `json:"daemon_tokens"`
	CredentialCopyAbsent  bool `json:"credential_copy_absent"`
	ProviderProfileAbsent bool `json:"provider_profile_absent"`
	DaemonProfileAbsent   bool `json:"daemon_profile_absent"`
	ChildTempRootsAbsent  bool `json:"child_temp_roots_absent"`
	MatchingProcesses     int  `json:"matching_processes"`
	ActiveReceipts        int  `json:"active_receipts"`
}

type IntegrationReplayResultV1 struct {
	SchemaVersion            string                             `json:"schema_version"`
	ResultID                 string                             `json:"result_id"`
	Status                   string                             `json:"status"`
	EvidenceStage            string                             `json:"evidence_stage"`
	ObservedAt               string                             `json:"observed_at"`
	IssueID                  string                             `json:"issue_id"`
	IssueKey                 string                             `json:"issue_key"`
	WorkflowKind             string                             `json:"workflow_kind"`
	Tasks                    []IntegrationReplayTaskV1          `json:"tasks"`
	WorkflowEvents           []string                           `json:"workflow_events"`
	ReviewOutcome            string                             `json:"review_outcome"`
	CorrectionCount          int                                `json:"correction_count"`
	ProviderTaskCeiling      int                                `json:"provider_task_ceiling"`
	ProviderTaskInvocations  int                                `json:"provider_task_invocations"`
	Receipts                 int                                `json:"receipts"`
	RevokedReceipts          int                                `json:"revoked_receipts"`
	CapabilityGrants         int                                `json:"capability_grants"`
	LaunchConsumptions       int                                `json:"launch_consumptions"`
	DistinctPackageHashes    int                                `json:"distinct_package_hashes"`
	DistinctDaemons          int                                `json:"distinct_daemons"`
	GuardRoleID              string                             `json:"guard_role_id"`
	GuardRoleInterpretation  string                             `json:"guard_role_interpretation"`
	Transport                IntegrationReplayTransportV1       `json:"transport"`
	Usage                    IntegrationReplayUsageV1           `json:"usage"`
	OwnerProjection          IntegrationReplayOwnerProjectionV1 `json:"owner_projection"`
	Report                   IntegrationReplayReportV1          `json:"report"`
	SourceBindings           IntegrationReplaySourceBindingsV1  `json:"source_bindings"`
	Cleanup                  IntegrationReplayCleanupV1         `json:"cleanup"`
	ProductionActions        int                                `json:"production_actions"`
	OutboundActions          int                                `json:"outbound_actions"`
	PhoneFarmWindowsAccesses int                                `json:"phone_farm_windows_accesses"`
	PermanentSelections      int                                `json:"permanent_selections"`
}

//go:embed integration_replay_result_v1.json
var integrationReplayResultV1JSON []byte

func CurrentIntegrationReplayResultV1() (IntegrationReplayResultV1, error) {
	var result IntegrationReplayResultV1
	if err := strictJSON(integrationReplayResultV1JSON, &result); err != nil {
		return result, fmt.Errorf("decode integration replay result: %w", err)
	}
	return result, ValidateIntegrationReplayResultV1(result)
}

func IntegrationReplayResultV1SHA256() string {
	sum := sha256.Sum256(integrationReplayResultV1JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateIntegrationReplayResultV1(result IntegrationReplayResultV1) error {
	if result.SchemaVersion != IntegrationReplayResultV1Schema || result.ResultID != "item-14-cef-7-real-integration-replay-20260915" || result.Status != "accepted_local_runtime_verified_integration_observation" || result.EvidenceStage != "local-only" || result.ObservedAt != "2026-09-15T13:15:00Z" || result.IssueID != "8f3111b6-c9b5-4919-986a-6a043837cdb3" || result.IssueKey != "CEF-7" || result.WorkflowKind != "controller_bound_read_only_windows_repository_inventory" {
		return fmt.Errorf("integration replay identity drifted")
	}
	wantTasks := []IntegrationReplayTaskV1{
		{Stage: "task_scoped_intake_leader", TaskID: "01a0a52c-fe4a-79a4-8cd7-6ce364016ef2", AgentID: "7fc31137-43c4-4031-9036-9c01f0ed7b0e", Status: "completed", Attempt: 1, MaxAttempts: 1, DurationMS: 6458},
		{Stage: "shared_windows_executor_fixture", TaskID: "01a0a52d-27ac-7dab-99ee-8dc30d819583", AgentID: "12df8369-55e8-4c1a-9995-42530d75c64b", Status: "completed", Attempt: 1, MaxAttempts: 1, DurationMS: 18332},
		{Stage: "task_scoped_independent_reviewer", TaskID: "01a0a52d-78a9-7544-af88-e58fa8e47398", AgentID: "8b74703a-b5ca-4247-87c5-6f684e87caf6", Status: "completed", Attempt: 1, MaxAttempts: 1, DurationMS: 9068},
	}
	if !slices.Equal(result.Tasks, wantTasks) || !slices.Equal(result.WorkflowEvents, []string{"implementation_ready", "review_passed"}) || result.ReviewOutcome != "ITEM14_REVIEW_PASS" || result.CorrectionCount != 0 {
		return fmt.Errorf("integration replay workflow drifted")
	}
	if result.ProviderTaskCeiling != 5 || result.ProviderTaskInvocations != 3 || result.Receipts != 3 || result.RevokedReceipts != 3 || result.CapabilityGrants != 3 || result.LaunchConsumptions != 3 || result.DistinctPackageHashes != 3 || result.DistinctDaemons != 1 || result.GuardRoleID != "data_sync_integrity" || result.GuardRoleInterpretation != "transport_guard_label_only_not_permanent_role_qualification" {
		return fmt.Errorf("integration replay accounting drifted")
	}
	transport := result.Transport
	if transport.Lane != "claude_code_subscription_composite" || transport.RuntimeProvider != "claude" || transport.RequestedPrimaryModel != "claude-sonnet-4-6" || transport.ReasoningEffort != "medium" || !slices.Equal(transport.ObservedModels, []string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"}) || transport.ExactSingleModelProven || transport.UnderlyingHTTPRequestCountCertified || transport.AutomaticRetries != 0 || transport.Fallbacks || transport.Tools != 0 || transport.ProviderChildRepositoryAccesses != 0 {
		return fmt.Errorf("integration replay transport drifted")
	}
	if result.Usage != (IntegrationReplayUsageV1{Rows: 6, SonnetRows: 3, HaikuRows: 3, InputTokens: 2887, OutputTokens: 88, CacheReadTokens: 7763, CacheWriteTokens: 2958, TotalAccountedTokens: 13696, DatabaseCostReported: false, UIEstimatedCostUSD: "0.02"}) {
		return fmt.Errorf("integration replay usage drifted")
	}
	if result.OwnerProjection != (IntegrationReplayOwnerProjectionV1{IssueStatus: "done", PlanState: "done", PictureRevision: 1, ScopeRevision: 1, OutcomeState: "satisfied", ExecutionState: "completed", ReleaseState: "not_required", AttentionState: "resolved", Milestone: "complete", BoardIssueInboxPassed: true}) {
		return fmt.Errorf("integration replay owner projection drifted")
	}
	wantArtifacts := []IntegrationReplayArtifactV1{
		{Path: ".qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/EVIDENCE.md", SHA256: "0310bf784a2f5da817921cd638ac531b9f04fecce3969875ec5d05c3f3d9e87f"},
		{Path: ".qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", SHA256: "48382cc93f8d178bbc737b63bbe74b591b00a40cf7fb3f338fed801dec01ce7e"},
		{Path: ".qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/inventory.json", SHA256: "4f6309ba8d864d513c4c2a70e6f7e0f599051a9ac1681a5e08eb376fa48ea4b9"},
	}
	if result.Report.Roots != 10 || result.Report.RequiredPathChecks != 444 || result.Report.CompletedPathChecks != 444 || result.Report.Present != 439 || result.Report.Absent != 5 || !slices.Equal(result.Report.Artifacts, wantArtifacts) {
		return fmt.Errorf("integration replay report drifted")
	}
	wantBindings := IntegrationReplaySourceBindingsV1{LaunchContractPath: ".qualification-evidence/item14-integration-replay-20260915-v3/EXACT-LAUNCH-CONTRACT.json", LaunchContractSHA256: "837411cfb5a4d4a0cf43d66bc468da0f7fbe69b7314e33c12c77dcea43577469", BrokerPath: "server/bin/item14-integration-pilot-broker-v3.exe", BrokerSHA256: "366718905d210c833d10a1116e4ef4033613be65046fa647625be88e09b4d4da", AuthorityHelperPath: "server/bin/item14-integration-pilot-authority-v7-real.exe", AuthorityHelperSHA256: "26bde00be5ed07d7a0692dd51a6625039e798ba9a343df6f1ee925e2246b3d0b", ControllerImageDigest: "sha256:53ce0736da07fbc00c0fc7c7be8c2b8414e5afea63100aa921fee881b5f37080"}
	if result.SourceBindings != wantBindings {
		return fmt.Errorf("integration replay source bindings drifted")
	}
	wantCleanup := IntegrationReplayCleanupV1{RuntimeOffline: true, DaemonTokens: 0, CredentialCopyAbsent: true, ProviderProfileAbsent: true, DaemonProfileAbsent: true, ChildTempRootsAbsent: true, MatchingProcesses: 0, ActiveReceipts: 0}
	if result.Cleanup != wantCleanup || result.ProductionActions != 0 || result.OutboundActions != 0 || result.PhoneFarmWindowsAccesses != 0 || result.PermanentSelections != 0 {
		return fmt.Errorf("integration replay cleanup or safety boundary drifted")
	}
	return nil
}

func VerifyIntegrationReplayResultV1(sourceFS fs.FS) error {
	result, err := CurrentIntegrationReplayResultV1()
	if err != nil {
		return err
	}
	artifacts := append([]IntegrationReplayArtifactV1(nil), result.Report.Artifacts...)
	artifacts = append(artifacts,
		IntegrationReplayArtifactV1{Path: result.SourceBindings.LaunchContractPath, SHA256: result.SourceBindings.LaunchContractSHA256},
		IntegrationReplayArtifactV1{Path: result.SourceBindings.BrokerPath, SHA256: result.SourceBindings.BrokerSHA256},
		IntegrationReplayArtifactV1{Path: result.SourceBindings.AuthorityHelperPath, SHA256: result.SourceBindings.AuthorityHelperSHA256},
	)
	for _, artifact := range artifacts {
		raw, err := fs.ReadFile(sourceFS, artifact.Path)
		if err != nil {
			return fmt.Errorf("read integration replay artifact %s: %w", artifact.Path, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != artifact.SHA256 {
			return fmt.Errorf("integration replay artifact %s hash drifted", artifact.Path)
		}
	}
	return nil
}
