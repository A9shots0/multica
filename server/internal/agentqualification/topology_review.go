package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const OpenCodeTopologyReviewTransport = "opencode_run_json_topology_review_v1"

type TopologyReviewRequest struct {
	ReviewRunID       string                 `json:"review_run_id"`
	ObservationID     string                 `json:"observation_id"`
	CaseID            string                 `json:"case_id"`
	CaseVersion       string                 `json:"case_version"`
	RequiredActions   []string               `json:"required_actions"`
	RequiredEvidence  []string               `json:"required_evidence"`
	HardDisqualifiers []string               `json:"hard_disqualifiers"`
	ArtifactManifest  []string               `json:"artifact_manifest"`
	EvidenceIDs       []string               `json:"evidence_ids"`
	EvidenceItems     []TopologyEvidenceItem `json:"evidence_items"`
	ExecutorSummary   string                 `json:"executor_summary"`
	WorkspaceTreeHash string                 `json:"workspace_tree_sha256"`
}

type TopologyEvidenceItem struct {
	EvidenceID string `json:"evidence_id"`
	Kind       string `json:"kind"`
	Content    string `json:"content"`
	SHA256     string `json:"sha256"`
}

type TopologyReviewVerdict struct {
	Verdict               string   `json:"verdict"`
	FailedCriteria        []string `json:"failed_criteria"`
	Evidence              []string `json:"evidence"`
	RequiredCorrection    string   `json:"required_correction"`
	OwnerDecisionRequired bool     `json:"owner_decision_required"`
}

type TopologyCorrectionPacket struct {
	ObservationID       string   `json:"observation_id"`
	CaseID              string   `json:"case_id"`
	CaseVersion         string   `json:"case_version"`
	FailedCriteria      []string `json:"failed_criteria"`
	Evidence            []string `json:"evidence"`
	RequiredCorrection  string   `json:"required_correction"`
	WorkspaceTreeSHA256 string   `json:"workspace_tree_sha256"`
}

func ParseTopologyReviewVerdict(raw []byte) (TopologyReviewVerdict, error) {
	var verdict TopologyReviewVerdict
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&verdict); err != nil {
		return verdict, fmt.Errorf("decode topology review verdict: %w", err)
	}
	if decoder.More() {
		return verdict, fmt.Errorf("topology review returned multiple values")
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return verdict, fmt.Errorf("topology review returned trailing data")
	}
	return verdict, nil
}

func BuildTopologyCorrectionPacket(request TopologyReviewRequest, verdict TopologyReviewVerdict) (TopologyCorrectionPacket, error) {
	if verdict.Verdict != "FAIL" {
		return TopologyCorrectionPacket{}, fmt.Errorf("only a failed review can create a correction packet")
	}
	if err := ValidateTopologyReviewVerdict(request, verdict); err != nil {
		return TopologyCorrectionPacket{}, err
	}
	return TopologyCorrectionPacket{
		ObservationID: request.ObservationID, CaseID: request.CaseID, CaseVersion: request.CaseVersion,
		FailedCriteria: append([]string(nil), verdict.FailedCriteria...), Evidence: append([]string(nil), verdict.Evidence...), RequiredCorrection: verdict.RequiredCorrection, WorkspaceTreeSHA256: request.WorkspaceTreeHash,
	}, nil
}

func TopologyCorrectionTask(packet TopologyCorrectionPacket) ([]byte, error) {
	if packet.ObservationID == "" || packet.CaseID == "" || packet.CaseVersion == "" || len(packet.FailedCriteria) == 0 || len(packet.Evidence) == 0 || strings.TrimSpace(packet.RequiredCorrection) == "" || !validSHA256(packet.WorkspaceTreeSHA256) {
		return nil, fmt.Errorf("topology correction packet is incomplete")
	}
	raw, err := json.Marshal(packet)
	if err != nil {
		return nil, err
	}
	return []byte("Correct only the failed public criteria in this sealed synthetic workspace. Use the listed evidence and current workspace; do not reconstruct chat or repeat completed work. Do not deploy, message, access credentials, access Phone Farm, or touch unrelated files. Finish with a truthful summary of the bounded correction and remaining open outcome.\n\n<correction_packet>\n" + string(raw) + "\n</correction_packet>\n"), nil
}

func BuildTopologyReviewRequest(observationID, reviewRunID, caseID, treeHash, summary string, artifacts, evidenceIDs []string, evidenceItems []TopologyEvidenceItem) (TopologyReviewRequest, error) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		return TopologyReviewRequest{}, err
	}
	var selected RealisticWorkflowCase
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID == caseID {
			selected = candidateCase
			break
		}
	}
	request := TopologyReviewRequest{
		ReviewRunID: reviewRunID, ObservationID: observationID, CaseID: selected.CaseID, CaseVersion: selected.CaseVersion,
		RequiredActions: append([]string(nil), selected.RequiredActions...), RequiredEvidence: append([]string(nil), selected.RequiredEvidence...), HardDisqualifiers: append([]string(nil), selected.HardDisqualifiers...),
		ArtifactManifest: append([]string(nil), artifacts...), EvidenceIDs: append([]string(nil), evidenceIDs...), ExecutorSummary: summary, WorkspaceTreeHash: treeHash,
		EvidenceItems: append([]TopologyEvidenceItem(nil), evidenceItems...),
	}
	if request.ReviewRunID == "" || request.ObservationID == "" || request.CaseID == "" || request.ExecutorSummary == "" || !validSHA256(request.WorkspaceTreeHash) || len(request.ArtifactManifest) == 0 || len(request.EvidenceIDs) == 0 || len(request.EvidenceItems) == 0 {
		return TopologyReviewRequest{}, fmt.Errorf("topology review request is incomplete")
	}
	itemIDs := map[string]struct{}{}
	for _, item := range request.EvidenceItems {
		hash := sha256.Sum256([]byte(item.Content))
		if item.EvidenceID == "" || item.Kind == "" || item.SHA256 != hex.EncodeToString(hash[:]) || !contains(request.EvidenceIDs, item.EvidenceID) {
			return TopologyReviewRequest{}, fmt.Errorf("topology review evidence is incomplete or unbound")
		}
		itemIDs[item.EvidenceID] = struct{}{}
	}
	for _, evidenceID := range request.EvidenceIDs {
		if _, ok := itemIDs[evidenceID]; !ok {
			return TopologyReviewRequest{}, fmt.Errorf("topology review evidence id has no inspectable content")
		}
	}
	sort.Strings(request.ArtifactManifest)
	sort.Strings(request.EvidenceIDs)
	return request, nil
}

func ValidateTopologyReviewVerdict(request TopologyReviewRequest, verdict TopologyReviewVerdict) error {
	if verdict.OwnerDecisionRequired {
		return fmt.Errorf("synthetic topology review cannot require an owner decision")
	}
	if verdict.Verdict != "PASS" && verdict.Verdict != "FAIL" {
		return fmt.Errorf("topology review verdict is invalid")
	}
	criteria := append(append([]string(nil), request.RequiredActions...), request.RequiredEvidence...)
	evidence := make(map[string]struct{}, len(request.EvidenceIDs)+len(request.ArtifactManifest))
	for _, value := range append(append([]string(nil), request.EvidenceIDs...), request.ArtifactManifest...) {
		evidence[value] = struct{}{}
	}
	if verdict.Verdict == "PASS" {
		if len(verdict.FailedCriteria) != 0 || strings.TrimSpace(verdict.RequiredCorrection) != "" || len(verdict.Evidence) == 0 {
			return fmt.Errorf("passing topology review contains a correction or lacks evidence")
		}
		for _, required := range request.RequiredEvidence {
			if !contains(request.EvidenceIDs, required) || !contains(verdict.Evidence, required) {
				return fmt.Errorf("passing topology review does not cover every required evidence item")
			}
		}
	} else if len(verdict.FailedCriteria) == 0 || len(verdict.Evidence) == 0 || strings.TrimSpace(verdict.RequiredCorrection) == "" {
		return fmt.Errorf("failing topology review lacks a bounded correction packet")
	}
	for _, criterion := range verdict.FailedCriteria {
		if !contains(criteria, criterion) {
			return fmt.Errorf("topology review invented a criterion")
		}
	}
	for _, item := range verdict.Evidence {
		if _, ok := evidence[item]; !ok {
			return fmt.Errorf("topology review fabricated evidence")
		}
	}
	correction := strings.ToLower(verdict.RequiredCorrection)
	for _, forbidden := range []string{"deploy", "production", "send message", "phone farm", "credential", "ask the owner", "ask drew", "unrelated"} {
		if strings.Contains(correction, forbidden) {
			return fmt.Errorf("topology correction exceeds the sealed local boundary")
		}
	}
	return nil
}

func BuildOpenCodeTopologyReviewPlan(candidate RealisticCandidate, request TopologyReviewRequest) (OpenCodeScreeningPlan, error) {
	definition, err := CurrentTopologyExperiment()
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	if candidate != definition.Reviewer {
		return OpenCodeScreeningPlan{}, fmt.Errorf("topology reviewer configuration is not frozen")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	schema := []byte(`{"type":"object","additionalProperties":false,"required":["verdict","failed_criteria","evidence","required_correction","owner_decision_required"],"properties":{"verdict":{"type":"string","enum":["PASS","FAIL"]},"failed_criteria":{"type":"array","items":{"type":"string"}},"evidence":{"type":"array","items":{"type":"string"}},"required_correction":{"type":"string"},"owner_decision_required":{"type":"boolean"}}}`)
	prompt := "Independently review one sealed synthetic result against only the public criteria. Return exactly one JSON verdict matching the schema. Use only listed evidence identifiers. Do not edit, execute tools, infer hidden grader rules, request owner input, expand scope, deploy, message, access credentials, or access Phone Farm. PASS requires complete public evidence. FAIL must name public failed criteria and the smallest local correction.\n\n<response_schema>\n" + string(schema) + "\n</response_schema>\n\n<review_request>\n" + string(requestJSON) + "\n</review_request>\n"
	config := openCodeQualificationConfig{
		Schema: "https://opencode.ai/config.json", Autoupdate: false, Share: "disabled", Snapshot: false,
		Plugin: []string{}, Instructions: []string{}, Permission: "deny", Tools: map[string]bool{"*": false}, Compaction: map[string]bool{"auto": false, "prune": false}, DefaultAgent: "qualification",
		Agent: map[string]openCodeQualificationAgent{"qualification": {Description: "Pinned independent topology reviewer", Mode: "primary", Model: candidate.Model, Variant: candidate.Effort, Prompt: "Return one evidence-bound review verdict as bare JSON.", Steps: 2, Permission: "deny", Tools: map[string]bool{"*": false}}},
	}
	modelID := strings.TrimPrefix(candidate.Model, "openrouter/")
	if modelID == candidate.Model {
		return OpenCodeScreeningPlan{}, fmt.Errorf("topology reviewer must use exact OpenRouter model id")
	}
	config.Provider = map[string]openCodeProviderConfig{"openrouter": {Options: map[string]any{"name": "OpenRouter"}, Headers: map[string]string{"X-OpenRouter-Metadata": "enabled"}, Whitelist: []string{modelID}, Models: map[string]openCodeModelConfig{modelID: {Options: map[string]any{"provider": map[string]any{"order": []string{candidate.ServingProvider}, "allow_fallbacks": false}}}}}}
	inlineConfig, err := json.Marshal(config)
	if err != nil {
		return OpenCodeScreeningPlan{}, err
	}
	promptHash, schemaHash, configHash := sha256.Sum256([]byte(prompt)), sha256.Sum256(schema), sha256.Sum256(inlineConfig)
	fallbacks := false
	return OpenCodeScreeningPlan{
		Transport: OpenCodeTopologyReviewTransport, CandidateID: candidate.ConfigurationID, CaseID: request.CaseID, CaseVersion: request.CaseVersion,
		Model: candidate.Model, Effort: candidate.Effort, Prompt: prompt, PromptSHA256: hex.EncodeToString(promptHash[:]), Schema: schema, SchemaSHA256: hex.EncodeToString(schemaHash[:]), InlineConfig: inlineConfig, InlineConfigSHA256: hex.EncodeToString(configHash[:]),
		ArgsTemplate:        []string{"--pure", "run", "--format", "json", "--model", candidate.Model, "--variant", candidate.Effort, "--agent", "qualification", "--title", "topology-review-<run-id>", "--dir", "<ephemeral-empty-dir>", "<exact-prompt-via-stdin>"},
		EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000"},
		ServingProvider:     candidate.ServingProvider, ProviderFallbacks: &fallbacks,
	}, nil
}
