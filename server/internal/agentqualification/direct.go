package agentqualification

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const DirectScreeningTransport = "direct_codex_exec_jsonl_v1"

type DirectScreeningPlan struct {
	BatchID      string   `json:"batch_id"`
	CandidateID  string   `json:"candidate_id"`
	CaseID       string   `json:"case_id"`
	Model        string   `json:"model"`
	Effort       string   `json:"reasoning_effort"`
	Prompt       string   `json:"-"`
	PromptSHA256 string   `json:"prompt_sha256"`
	Schema       []byte   `json:"-"`
	SchemaSHA256 string   `json:"output_schema_sha256"`
	ArgsTemplate []string `json:"args_template"`
}

type DirectScreeningRun struct {
	Status            string   `json:"status"`
	RawTraceSHA256    string   `json:"raw_trace_sha256"`
	EventCount        int      `json:"event_count"`
	Output            string   `json:"output,omitempty"`
	OutputSHA256      string   `json:"output_sha256,omitempty"`
	ToolEvents        int      `json:"tool_events"`
	ToolTypes         []string `json:"tool_types,omitempty"`
	InputTokens       int64    `json:"input_tokens"`
	CachedInputTokens int64    `json:"cached_input_tokens"`
	FreshInputTokens  int64    `json:"fresh_input_tokens"`
	OutputTokens      int64    `json:"output_tokens"`
	TotalTokens       int64    `json:"total_tokens"`
	InvalidReasons    []string `json:"invalid_reasons,omitempty"`
}

func BuildDirectScreeningPlan(batch ScreeningBatchDefinition, candidateID, caseID string) (DirectScreeningPlan, error) {
	var plan DirectScreeningPlan
	if err := ValidateScreeningBatch(batch); err != nil {
		return plan, err
	}
	if err := ValidateScreeningInstructionCompatibility(batch); err != nil {
		return plan, err
	}
	if batch.CandidateTransport != DirectScreeningTransport {
		return plan, fmt.Errorf("batch transport is not direct screening")
	}
	var candidate *Configuration
	for i := range batch.Candidates {
		if batch.Candidates[i].ID == candidateID {
			candidate = &batch.Candidates[i]
			break
		}
	}
	if candidate == nil {
		return plan, fmt.Errorf("candidate %s is not in batch", candidateID)
	}
	caseVersion := ""
	for _, screeningCase := range batch.Cases {
		if screeningCase.CaseID == caseID {
			caseVersion = screeningCase.CaseVersion
			break
		}
	}
	if caseVersion == "" {
		return plan, fmt.Errorf("case %s is not in batch", caseID)
	}
	packet, err := CandidateScreeningPacketVersion(caseID, caseVersion)
	if err != nil {
		return plan, err
	}
	schema, err := directOutputSchema(caseID, caseVersion)
	if err != nil {
		return plan, err
	}
	prompt := strings.TrimSpace(string(screeningCandidateInstructionsV4)) +
		"\n\n<qualification-policy>\n" + strings.TrimSpace(string(screeningSkillV2)) +
		"\n</qualification-policy>\n\n<case-packet>\n" + strings.TrimSpace(string(packet)) +
		"\n</case-packet>\n"
	plan = DirectScreeningPlan{
		BatchID:     batch.BatchID,
		CandidateID: candidate.ID,
		CaseID:      caseID,
		Model:       candidate.Model,
		Effort:      candidate.ReasoningEffort,
		Prompt:      prompt,
		Schema:      schema,
		ArgsTemplate: []string{
			"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules",
			"--skip-git-repo-check", "--sandbox", "read-only", "--model", candidate.Model,
			"--config", "model_reasoning_effort=\"" + candidate.ReasoningEffort + "\"",
			"--output-schema", "<generated-schema>", "--json", "-",
		},
	}
	plan.PromptSHA256 = sha256Hex([]byte(prompt))
	plan.SchemaSHA256 = sha256Hex(schema)
	return plan, nil
}

func directOutputSchema(caseID, caseVersion string) ([]byte, error) {
	packet, err := CandidateScreeningPacketVersion(caseID, caseVersion)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		ResponseContract struct {
			Fields map[string]string `json:"fields"`
		} `json:"response_contract"`
	}
	if err := json.Unmarshal(packet, &decoded); err != nil {
		return nil, fmt.Errorf("decode public response contract: %w", err)
	}
	required := make([]string, 0, len(decoded.ResponseContract.Fields))
	properties := make(map[string]any, len(decoded.ResponseContract.Fields))
	for name, kind := range decoded.ResponseContract.Fields {
		required = append(required, name)
		switch kind {
		case "string":
			properties[name] = map[string]any{"type": "string"}
		case "boolean":
			properties[name] = map[string]any{"type": "boolean"}
		case "integer":
			properties[name] = map[string]any{"type": "integer"}
		case "array_of_strings":
			properties[name] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
		default:
			if values, ok := responseEnumValues(kind, "enum:"); ok {
				properties[name] = map[string]any{"type": "string", "enum": values}
			} else if values, ok := responseEnumValues(kind, "array_of_enums:"); ok {
				properties[name] = map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": values}, "uniqueItems": true}
			} else {
				return nil, fmt.Errorf("unsupported public response type %s for %s", kind, name)
			}
		}
	}
	sort.Strings(required)
	return json.Marshal(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	})
}

func responseEnumValues(kind, prefix string) ([]string, bool) {
	if !strings.HasPrefix(kind, prefix) {
		return nil, false
	}
	values := strings.Split(strings.TrimPrefix(kind, prefix), "|")
	if len(values) < 2 {
		return nil, false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return nil, false
		}
	}
	return values, true
}

func ParseDirectScreeningJSONL(raw []byte) (DirectScreeningRun, error) {
	run := DirectScreeningRun{Status: "INVALID", RawTraceSHA256: sha256Hex(raw)}
	toolKinds := map[string]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		run.EventCount++
		var event map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return run, fmt.Errorf("decode direct runner event: %w", err)
		}
		typ, _ := event["type"].(string)
		item, _ := event["item"].(map[string]any)
		itemType, _ := item["type"].(string)
		if directToolEvent(typ, itemType) {
			run.ToolEvents++
			if itemType != "" {
				toolKinds[itemType] = struct{}{}
			} else {
				toolKinds[typ] = struct{}{}
			}
		}
		if typ == "item.completed" && itemType == "agent_message" {
			if text, ok := item["text"].(string); ok {
				run.Output = text
			}
		}
		if typ == "turn.completed" {
			if usage, ok := event["usage"].(map[string]any); ok {
				run.InputTokens = jsonInt64(usage["input_tokens"])
				run.CachedInputTokens = jsonInt64(usage["cached_input_tokens"])
				run.OutputTokens = jsonInt64(usage["output_tokens"])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return run, err
	}
	for kind := range toolKinds {
		run.ToolTypes = append(run.ToolTypes, kind)
	}
	sort.Strings(run.ToolTypes)
	run.FreshInputTokens = run.InputTokens - run.CachedInputTokens
	if run.FreshInputTokens < 0 {
		run.InvalidReasons = append(run.InvalidReasons, "invalid_token_accounting")
	}
	run.TotalTokens = run.InputTokens + run.OutputTokens
	if run.Output == "" {
		run.InvalidReasons = append(run.InvalidReasons, "missing_final_output")
	} else {
		run.OutputSHA256 = sha256Hex([]byte(run.Output))
		if !json.Valid([]byte(run.Output)) {
			run.InvalidReasons = append(run.InvalidReasons, "final_output_not_json")
		}
	}
	if run.ToolEvents != 0 {
		run.InvalidReasons = append(run.InvalidReasons, "tool_event_observed")
	}
	if run.InputTokens == 0 && run.OutputTokens == 0 {
		run.InvalidReasons = append(run.InvalidReasons, "missing_usage_receipt")
	}
	if len(run.InvalidReasons) == 0 {
		run.Status = "VALID"
	}
	return run, nil
}

func directToolEvent(eventType, itemType string) bool {
	combined := strings.ToLower(eventType + ":" + itemType)
	for _, marker := range []string{
		"command_execution", "file_change", "mcp_tool", "web_search", "computer_tool", "tool_call",
		"dynamic_tool", "image_generation", "collab_agent", "function_call", "server_tool", "tool_use",
	} {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

func jsonInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
