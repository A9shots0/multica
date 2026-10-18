package agentqualification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func roleQualificationV6Cell(t *testing.T) RoleQualificationExecutionCellV3 {
	t.Helper()
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range plan.Cells {
		if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" && cell.CandidateClass == "quality_checker" {
			return cell
		}
	}
	t.Fatal("no OpenCode cell")
	return RoleQualificationExecutionCellV3{}
}

func roleQualificationV6Capsule(t *testing.T, cell RoleQualificationExecutionCellV3) (OpenCodeStructuredV33Capsule, json.RawMessage) {
	t.Helper()
	schema, err := json.Marshal(cell.Request.CandidatePacket.ResponseSchema)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]any{}
	for name, field := range cell.Request.CandidatePacket.ResponseSchema.Properties {
		switch field.Type {
		case "array":
			values[name] = []string{"bounded evidence"}
		case "boolean":
			values[name] = false
		default:
			values[name] = "bounded evidence"
		}
	}
	structured, _ := json.Marshal(values)
	prompt := "frozen role qualification packet"
	body, _ := json.Marshal(map[string]any{
		"messageID": "msg_v6_test",
		"model":     map[string]string{"providerID": "openrouter", "modelID": strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")},
		"agent":     "qualification",
		"tools":     map[string]bool{"*": false},
		"format":    map[string]any{"type": "json_schema", "schema": json.RawMessage(schema), "retryCount": 0},
		"variant":   cell.ConfigurationClass.Effort,
		"parts":     []map[string]string{{"type": "text", "text": prompt}},
	})
	config := []byte("{}")
	capsule := OpenCodeStructuredV33Capsule{
		BindingID: cell.CellID, OpenCodeVersion: OpenCodeStructuredV33Version,
		Transport: OpenCodeStructuredV33Transport, Method: "POST",
		PathTemplate: "/session/{sessionID}/message?directory={url-escaped-workspace}",
		SessionID:    "ses_v6_test", RequestMessageID: "msg_v6_test", Workspace: "C:/sealed/v6",
		ServerArgs:          []string{"serve", "--pure", "--hostname", "127.0.0.1", "--port", "<ephemeral-port>"},
		EnvironmentTemplate: []string{"OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=4000"},
		InlineConfig:        config, RequestBody: body, InlineConfigSHA256: sha256Hex(config),
		ResolvedConfigSHA256: sha256Hex(config), RequestBodySHA256: sha256Hex(body),
		PromptSHA256: sha256Hex([]byte(prompt)), SchemaSHA256: sha256Hex(schema),
		RoutePolicy: "enforced_single_route_no_fallback", RetryCount: 0,
		HTTPTimeoutSeconds:    OpenCodeStructuredV33HTTPTimeoutSeconds,
		GeneratedTokenCeiling: cell.Request.MaxOutputTokens, ToolsDenied: true,
	}
	return capsule, structured
}

func roleQualificationV6HTTP(t *testing.T, cell RoleQualificationExecutionCellV3, capsule OpenCodeStructuredV33Capsule, structured json.RawMessage) json.RawMessage {
	t.Helper()
	raw := []byte(fmt.Sprintf(`{"info":{"id":"msg_assistant_v6","sessionID":%q,"role":"assistant","time":{"created":1,"completed":2},"parentID":%q,"modelID":%q,"providerID":"openrouter","mode":"primary","agent":"qualification","path":{"cwd":"C:/sealed/v6","root":"C:/sealed/v6"},"cost":0.01,"tokens":{"total":17,"input":10,"output":2,"reasoning":3,"cache":{"read":1,"write":1}},"structured":%s,"variant":%q,"finish":"stop"},"parts":[{"id":"prt_start","sessionID":%q,"messageID":"msg_assistant_v6","type":"step-start"},{"id":"prt_finish","sessionID":%q,"messageID":"msg_assistant_v6","type":"step-finish","reason":"stop","cost":0.01,"tokens":{"total":17,"input":10,"output":2,"reasoning":3,"cache":{"read":1,"write":1}}}]}`,
		capsule.SessionID, capsule.RequestMessageID, strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/"), structured, cell.ConfigurationClass.Effort, capsule.SessionID, capsule.SessionID))
	if !json.Valid(raw) {
		t.Fatal("invalid fixture")
	}
	return raw
}

func roleQualificationV6Evidence(t *testing.T, cell RoleQualificationExecutionCellV3, outcome string) RoleQualificationOpenCodeEvidenceV6 {
	t.Helper()
	capsule, structured := roleQualificationV6Capsule(t, cell)
	raw := roleQualificationV6HTTP(t, cell, capsule, structured)
	if outcome == "structured_output_error" {
		var env map[string]any
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		info := env["info"].(map[string]any)
		delete(info, "structured")
		info["error"] = map[string]any{"name": "StructuredOutputError", "data": map[string]any{"message": "schema generation failed", "retries": 0}}
		raw, _ = json.Marshal(env)
	}
	capsuleRaw, _ := json.Marshal(capsule)
	return RoleQualificationOpenCodeEvidenceV6{
		SchemaVersion: openCodeEvidenceSchemaV6, CellID: cell.CellID,
		FrozenRequestSHA256: cell.RequestSHA256, Capsule: capsule,
		CapsuleSHA256: sha256Hex(capsuleRaw), HTTPStatus: 200,
		ContentType: "application/json", RawHTTPResponse: raw,
		RawHTTPResponseSHA: sha256Hex(raw), StartedAt: "2026-09-13T18:00:00Z",
		CompletedAt: "2026-09-13T18:00:01Z", SealedAt: "2026-09-13T18:00:02Z",
	}
}

func seedRoleQualificationV6(t *testing.T, cell RoleQualificationExecutionCellV3) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "preflight"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"), []byte(`{"sealed":"v6-test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareRoleQualificationGrantV6(root, cell.CellID, "2026-09-13T17:59:58Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareRoleQualificationAttemptV6(root, cell, "2026-09-13T17:59:59Z"); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRoleQualificationV6AcceptsOnlyStructuredSuccessAndRetainsOutput(t *testing.T) {
	cell := roleQualificationV6Cell(t)
	evidence := roleQualificationV6Evidence(t, cell, "structured")
	raw, outcome, err := roleQualificationOpenCodeProviderRawV6(cell, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != "structured" {
		t.Fatalf("outcome=%q", outcome)
	}
	var provider RoleQualificationRawProviderResponseV3
	if err := strictJSON(raw, &provider); err != nil {
		t.Fatal(err)
	}
	if provider.FailureClass != "" || len(provider.StructuredOutput) == 0 {
		t.Fatalf("valid structured output was discarded: %+v", provider)
	}
	// This is the exact regression: the parser never returns "completed".
	if outcome == "completed" {
		t.Fatal("old completed sentinel accepted")
	}
}

func TestRoleQualificationV6StructuredOutputErrorIsCandidateLocalWithRawEvidence(t *testing.T) {
	cell := roleQualificationV6Cell(t)
	root := seedRoleQualificationV6(t, cell)
	evidence := roleQualificationV6Evidence(t, cell, "structured_output_error")
	if err := SealRoleQualificationOpenCodeEvidenceV6(root, cell, evidence); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationClaimV6(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	replay, err := RoleQualificationExecutionReplayV6(root)
	if err != nil {
		t.Fatal(err)
	}
	if replay.GlobalHalt || replay.SuccessReceipts != 1 || len(replay.EliminatedBindings) != 1 {
		t.Fatalf("unexpected replay: %+v", replay)
	}
	var observation RoleQualificationObservationV6
	if err := readStrictJSONFileV3(filepath.Join(root, "observations", cell.CellID+".json"), &observation); err != nil {
		t.Fatal(err)
	}
	if observation.FailureScope != "candidate_local" || observation.FailureClass != "response_or_tool_protocol_failure" {
		t.Fatalf("wrong protocol failure: %+v", observation)
	}
	if len(evidence.RawHTTPResponse) == 0 || evidence.RawHTTPResponseSHA != sha256Hex(evidence.RawHTTPResponse) {
		t.Fatal("raw HTTP evidence not preserved")
	}
}

func TestRoleQualificationV6ReplayRederivesFromRawHTTPAndCapsule(t *testing.T) {
	cell := roleQualificationV6Cell(t)
	root := seedRoleQualificationV6(t, cell)
	evidence := roleQualificationV6Evidence(t, cell, "structured")
	if err := SealRoleQualificationOpenCodeEvidenceV6(root, cell, evidence); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRoleQualificationClaimV6(root, cell.CellID); err != nil {
		t.Fatal(err)
	}
	if _, err := RoleQualificationExecutionReplayV6(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "opencode-http-receipts", cell.CellID+".json")
	raw, _ := os.ReadFile(path)
	raw = bytes.Replace(raw, []byte("msg_assistant_v6"), []byte("msg_tampered_____"), 1)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RoleQualificationExecutionReplayV6(root); err == nil {
		t.Fatal("tampered raw HTTP evidence replayed")
	}
}

type noCallProviderV6 struct{ calls int }

func (p *noCallProviderV6) Call(context.Context, RoleQualificationExecutionCellV3) RoleQualificationProviderResultV6 {
	p.calls++
	return RoleQualificationProviderResultV6{}
}

func TestRoleQualificationV6ClaudeClassHeldBeforeProviderCall(t *testing.T) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	var cell RoleQualificationExecutionCellV3
	for _, c := range plan.Cells {
		if c.ConfigurationClass.RuntimeProvider == "claude_code_subscription" {
			cell = c
			break
		}
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "preflight"), 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	pf := RoleQualificationPreflightV6{SchemaVersion: preflightSchemaV6, ExecutionFreezeSHA256: RoleQualificationExecutionFreezeV6SHA256(), V3FreezeSHA256: RoleQualificationExecutionFreezeV3SHA256(), ConfigurationClassID: cell.ConfigurationClass.ClassID, RequestedModel: cell.ConfigurationClass.Model, RequestedProvider: cell.ConfigurationClass.ServingProvider, RequestedEffort: cell.ConfigurationClass.Effort, Transport: RoleQualificationClaudeTransportV3, RuntimeVersion: "2.1.270", RuntimeExecutablePath: "C:/sealed/claude.cmd", RuntimeExecutableSHA256: strings.Repeat("a", 64), ResolvedConfigSHA256: strings.Repeat("b", 64), ExactRouteValidated: true, FallbacksDisabled: true, CredentialState: "authenticated", ObservedAt: now.Format(time.RFC3339Nano), Ready: true}
	// Direct write keeps this test focused on the pre-call hold rather than v4 identity fixtures.
	if err := writeOnceRoleQualificationJSONV3(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"), pf); err != nil {
		t.Fatal(err)
	}
	provider := &noCallProviderV6{}
	_, err = ExecuteRoleQualificationBatchV6(context.Background(), root, cell.ConfigurationClass.ClassID, now, provider)
	if err == nil || !strings.Contains(err.Error(), "held incomplete") || provider.calls != 0 {
		t.Fatalf("Claude hold failed err=%v calls=%d", err, provider.calls)
	}
}
