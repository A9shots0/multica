package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const CodexSubscriptionBoundaryV1Schema = "silver-mediums.codex-subscription-boundary.v1"

type CodexSubscriptionCapturedRequestV1 struct {
	Method                            string   `json:"method"`
	Path                              string   `json:"path"`
	RequestCount                      int      `json:"request_count"`
	Model                             string   `json:"model"`
	WireAPI                           string   `json:"wire_api"`
	RequestMaxRetries                 int      `json:"request_max_retries"`
	StreamMaxRetries                  int      `json:"stream_max_retries"`
	Stream                            bool     `json:"stream"`
	Store                             bool     `json:"store"`
	ToolChoice                        string   `json:"tool_choice"`
	TopLevelToolsField                string   `json:"top_level_tools_field"`
	AdditionalToolsMessageCount       int      `json:"additional_tools_message_count"`
	AdditionalToolNames               []string `json:"additional_tool_names"`
	PreventiveToolFree                bool     `json:"preventive_tool_free"`
	AmbientRuntimeInstructionsPresent bool     `json:"ambient_runtime_instructions_present"`
}

type CodexSubscriptionBoundaryV1 struct {
	SchemaVersion            string                             `json:"schema_version"`
	EvidenceID               string                             `json:"evidence_id"`
	CapturedAt               string                             `json:"captured_at"`
	Status                   string                             `json:"status"`
	CLIVersion               string                             `json:"cli_version"`
	ExecutionMode            string                             `json:"execution_mode"`
	CredentialMode           string                             `json:"credential_mode"`
	FreshProfile             bool                               `json:"fresh_profile"`
	ExternalEmptyWorkDir     bool                               `json:"external_empty_work_directory"`
	UserConfigIgnored        bool                               `json:"user_config_ignored"`
	ProjectRulesIgnored      bool                               `json:"project_rules_ignored"`
	ProjectDocumentsDisabled bool                               `json:"project_documents_disabled"`
	ProjectDocumentObserved  bool                               `json:"project_document_observed"`
	RealCredentialReads      int                                `json:"real_credential_reads"`
	RealProviderRequests     int                                `json:"real_provider_requests"`
	CapturedRequest          CodexSubscriptionCapturedRequestV1 `json:"captured_request"`
	DisabledFeatures         []string                           `json:"disabled_features"`
	EnvironmentControls      []string                           `json:"environment_controls"`
	Decision                 string                             `json:"decision"`
}

//go:embed codex_subscription_boundary_v1.json
var codexSubscriptionBoundaryV1JSON []byte

func CurrentCodexSubscriptionBoundaryV1() (CodexSubscriptionBoundaryV1, error) {
	var evidence CodexSubscriptionBoundaryV1
	if err := json.Unmarshal(codexSubscriptionBoundaryV1JSON, &evidence); err != nil {
		return evidence, fmt.Errorf("decode Codex subscription boundary v1: %w", err)
	}
	if err := ValidateCodexSubscriptionBoundaryV1(evidence); err != nil {
		return CodexSubscriptionBoundaryV1{}, err
	}
	return evidence, nil
}

func CodexSubscriptionBoundaryV1SHA256() string {
	sum := sha256.Sum256(codexSubscriptionBoundaryV1JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateCodexSubscriptionBoundaryV1(evidence CodexSubscriptionBoundaryV1) error {
	if evidence.SchemaVersion != CodexSubscriptionBoundaryV1Schema ||
		strings.TrimSpace(evidence.EvidenceID) == "" ||
		evidence.Status != "held_unsuitable_for_model_only" ||
		evidence.CLIVersion != "codex-cli 0.153.2" {
		return fmt.Errorf("Codex boundary identity drifted")
	}
	if evidence.ExecutionMode != "loopback_fake_responses_provider" ||
		evidence.CredentialMode != "dummy_environment_value_only" ||
		!evidence.FreshProfile || !evidence.ExternalEmptyWorkDir ||
		!evidence.UserConfigIgnored || !evidence.ProjectRulesIgnored ||
		!evidence.ProjectDocumentsDisabled || evidence.ProjectDocumentObserved ||
		evidence.RealCredentialReads != 0 || evidence.RealProviderRequests != 0 {
		return fmt.Errorf("Codex provider-free isolation evidence drifted")
	}
	request := evidence.CapturedRequest
	if request.Method != "POST" || request.Path != "/v1/responses" ||
		request.RequestCount != 1 || request.Model != "gpt-5.6-luna" ||
		request.WireAPI != "responses" || request.RequestMaxRetries != 0 ||
		request.StreamMaxRetries != 0 || !request.Stream || request.Store ||
		request.ToolChoice != "auto" || request.TopLevelToolsField != "absent" ||
		request.AdditionalToolsMessageCount != 1 || request.PreventiveToolFree ||
		!request.AmbientRuntimeInstructionsPresent {
		return fmt.Errorf("captured Codex request facts drifted")
	}
	wantTools := []string{"functions.exec", "functions.request_user_input", "functions.wait"}
	if !slices.Equal(request.AdditionalToolNames, wantTools) {
		return fmt.Errorf("captured additional tools drifted")
	}
	for _, required := range []string{
		"shell_tool", "unified_exec", "code_mode", "code_mode_host",
		"default_mode_request_user_input", "request_permissions_tool",
		"view_image", "browser_use", "computer_use", "multi_agent", "plugins",
		"skill_search", "workspace_dependencies",
	} {
		if !slices.Contains(evidence.DisabledFeatures, required) {
			return fmt.Errorf("required disabled feature %s is missing", required)
		}
	}
	for _, required := range []string{
		"fresh_codex_home", "app_tool_pipe_cleared", "desktop_session_identity_cleared",
		"real_provider_key_variables_cleared", "web_search_disabled",
		"project_root_markers_empty", "project_doc_max_bytes_zero",
	} {
		if !slices.Contains(evidence.EnvironmentControls, required) {
			return fmt.Errorf("required environment control %s is missing", required)
		}
	}
	if !strings.Contains(evidence.Decision, "cannot be used") ||
		!strings.Contains(evidence.Decision, "no tool definitions") {
		return fmt.Errorf("fail-closed Codex decision is incomplete")
	}
	return nil
}
