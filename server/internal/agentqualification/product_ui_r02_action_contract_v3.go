package agentqualification

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

const ProductUIR02ActionContractV3Schema = "silver-mediums.product-ui-r02-action-contract.v3"

type ProductUIR02ActionContractV3 struct {
	SchemaVersion                       string   `json:"schema_version"`
	ContractID                          string   `json:"contract_id"`
	Status                              string   `json:"status"`
	SourceReliabilityResultSHA256       string   `json:"source_reliability_result_sha256"`
	InstructionPath                     string   `json:"instruction_path"`
	InstructionSHA256                   string   `json:"instruction_sha256"`
	SkillPath                           string   `json:"skill_path"`
	SkillSHA256                         string   `json:"skill_sha256"`
	CaseID                              string   `json:"case_id"`
	MaximumActions                      int      `json:"maximum_actions"`
	ExactActionSequence                 []string `json:"exact_action_sequence"`
	AllowedEditPaths                    []string `json:"allowed_edit_paths"`
	ControllerOwnedEvidencePath         string   `json:"controller_owned_evidence_path"`
	ForbiddenPayloadPatterns            []string `json:"forbidden_payload_patterns"`
	CompleteLiteralEditContentsRequired bool     `json:"complete_literal_edit_contents_required"`
	ResponsiveCSSRequired               bool     `json:"responsive_css_required"`
	PreRepairCheckForbidden             bool     `json:"pre_repair_check_forbidden"`
	BrowserRequests                     int      `json:"browser_requests"`
	ProviderCallsAllowed                bool     `json:"provider_calls_allowed"`
	CredentialReadsAllowed              bool     `json:"credential_reads_allowed"`
	NativeToolsAllowed                  bool     `json:"native_tools_allowed"`
	RepositoryAccessAllowed             bool     `json:"repository_access_allowed"`
	ProductionAccessAllowed             bool     `json:"production_access_allowed"`
	OutboundActionsAllowed              bool     `json:"outbound_actions_allowed"`
	PhoneFarmWindowsAccessAllowed       bool     `json:"phone_farm_windows_access_allowed"`
	Retries                             int      `json:"retries"`
	Fallbacks                           bool     `json:"fallbacks"`
	RuntimeQualified                    bool     `json:"runtime_qualified"`
}

//go:embed product_ui_r02_action_contract_v3.json
var productUIR02ActionContractV3JSON []byte

//go:embed role_artifacts/v3/shared_windows_executor_r02.instructions.md role_artifacts/v3/product_ui_r02.skill.md
var productUIR02ActionArtifactsV3 embed.FS

func CurrentProductUIR02ActionContractV3() (ProductUIR02ActionContractV3, error) {
	var contract ProductUIR02ActionContractV3
	if err := strictJSON(productUIR02ActionContractV3JSON, &contract); err != nil {
		return contract, fmt.Errorf("decode Product/UI R02 action contract v3: %w", err)
	}
	return contract, ValidateProductUIR02ActionContractV3(contract)
}

func ProductUIR02ActionContractV3SHA256() string {
	sum := sha256.Sum256(productUIR02ActionContractV3JSON)
	return hex.EncodeToString(sum[:])
}

func ValidateProductUIR02ActionContractV3(contract ProductUIR02ActionContractV3) error {
	if contract.SchemaVersion != ProductUIR02ActionContractV3Schema || contract.ContractID != "item-14-product-ui-r02-provider-free-correction-candidate-20260914" || contract.Status != "prepared_provider_free_not_runtime_qualified" || contract.SourceReliabilityResultSHA256 != PrimarySubscriptionReliabilityTrialResultV2SHA256() {
		return fmt.Errorf("Product/UI R02 action contract v3 identity drifted")
	}
	wantSequence := []string{"edit:report/report.js", "edit:report/styles.css", "run_check:node tests/report.test.mjs", "run_check:local-static-preview report", "browser", "finish"}
	wantEdits := []string{"report/report.js", "report/styles.css"}
	wantPatterns := []string{"...", "TODO", "placeholder", "controller will", "replace token"}
	if contract.CaseID != "R02" || contract.MaximumActions != 6 || !slices.Equal(contract.ExactActionSequence, wantSequence) || !slices.Equal(contract.AllowedEditPaths, wantEdits) || contract.ControllerOwnedEvidencePath != "EVIDENCE.json" || !slices.Equal(contract.ForbiddenPayloadPatterns, wantPatterns) || !contract.CompleteLiteralEditContentsRequired || !contract.ResponsiveCSSRequired || !contract.PreRepairCheckForbidden || contract.BrowserRequests != 1 {
		return fmt.Errorf("Product/UI R02 action contract v3 payload boundary drifted")
	}
	for _, artifact := range []struct{ path, want string }{{contract.InstructionPath, contract.InstructionSHA256}, {contract.SkillPath, contract.SkillSHA256}} {
		if !validSHA256(artifact.want) || !strings.HasPrefix(artifact.path, "role_artifacts/v3/") {
			return fmt.Errorf("Product/UI R02 action artifact identity invalid")
		}
		raw, err := productUIR02ActionArtifactsV3.ReadFile(artifact.path)
		if err != nil || hashEmbedded(raw) != artifact.want {
			return fmt.Errorf("Product/UI R02 action artifact drifted")
		}
	}
	if contract.ProviderCallsAllowed || contract.CredentialReadsAllowed || contract.NativeToolsAllowed || contract.RepositoryAccessAllowed || contract.ProductionAccessAllowed || contract.OutboundActionsAllowed || contract.PhoneFarmWindowsAccessAllowed || contract.Retries != 0 || contract.Fallbacks || contract.RuntimeQualified {
		return fmt.Errorf("Product/UI R02 action contract v3 expanded authority or qualification")
	}
	return nil
}

func ValidateProductUIR02ActionPlanV3(actions []RealisticCandidateAction) error {
	contract, err := CurrentProductUIR02ActionContractV3()
	if err != nil {
		return err
	}
	if len(actions) != contract.MaximumActions {
		return fmt.Errorf("R02 v3 action count invalid")
	}
	wantKinds := []string{"edit", "edit", "run_check", "run_check", "browser", "finish"}
	for index, action := range actions {
		if action.Kind != wantKinds[index] {
			return fmt.Errorf("R02 v3 action sequence invalid at %d", index)
		}
	}
	if actions[0].Path != "report/report.js" || actions[1].Path != "report/styles.css" || actions[2].Check != "node tests/report.test.mjs" || actions[3].Check != "local-static-preview report" || strings.TrimSpace(actions[5].Summary) == "" {
		return fmt.Errorf("R02 v3 exact action payload invalid")
	}
	if actions[4].Path != "" || actions[4].Check != "" || actions[4].Contents != "" || actions[5].Path != "" || actions[5].Check != "" || actions[5].Contents != "" {
		return fmt.Errorf("R02 v3 Browser or finish fields expanded")
	}
	for _, action := range actions[:2] {
		if strings.TrimSpace(action.Contents) == "" {
			return fmt.Errorf("R02 v3 literal edit missing")
		}
		lower := strings.ToLower(action.Contents)
		for _, pattern := range contract.ForbiddenPayloadPatterns {
			if strings.Contains(lower, strings.ToLower(pattern)) {
				return fmt.Errorf("R02 v3 edit contains forbidden placeholder pattern")
			}
		}
	}
	javascript := actions[0].Contents
	if !strings.Contains(javascript, "published_posts") || !strings.Contains(javascript, ".reduce(") || !strings.Contains(javascript, ".map(") || strings.Contains(javascript, "metric_rows") || strings.Contains(javascript, ".slice(0,3)") {
		return fmt.Errorf("R02 v3 JavaScript truth or four-card contract missing")
	}
	css := strings.ToLower(actions[1].Contents)
	if !(strings.Contains(css, "minmax(") && (strings.Contains(css, "width: min(") || strings.Contains(css, "max-width:"))) || strings.Contains(css, "width: 980px") || strings.Contains(css, "repeat(4,") {
		return fmt.Errorf("R02 v3 responsive CSS contract missing")
	}
	return nil
}
