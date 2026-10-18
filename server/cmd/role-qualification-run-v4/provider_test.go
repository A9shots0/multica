package main

import (
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestV4ExecuteRuntimeMustMatchSealedPreflight(t *testing.T) {
	plan, _ := agentqualification.BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	now := time.Now().UTC()
	proof := preflightForCommandV4(cell, now)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "preflight"), 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(proof)
	if err := os.WriteFile(filepath.Join(root, "preflight", cell.ConfigurationClass.ClassID+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	p := &liveProviderV4{classID: cell.ConfigurationClass.ClassID, executable: proof.RuntimeExecutablePath, runtimeVersion: proof.RuntimeVersion, executableHash: proof.RuntimeExecutableSHA256, resolvedHash: proof.ResolvedConfigSHA256}
	if err := p.validateAgainstSealedPreflight(root); err != nil {
		t.Fatal(err)
	}
	p.resolvedHash = strings.Repeat("c", 64)
	if err := p.validateAgainstSealedPreflight(root); err == nil {
		t.Fatal("runtime drift accepted")
	}
}
func preflightForCommandV4(cell agentqualification.RoleQualificationExecutionCellV3, now time.Time) agentqualification.RoleQualificationPreflightV4 {
	return agentqualification.RoleQualificationPreflightV4{SchemaVersion: "silver-mediums.role-qualification-preflight.v4", ExecutionFreezeSHA256: agentqualification.RoleQualificationExecutionFreezeV4SHA256(), V3FreezeSHA256: agentqualification.RoleQualificationExecutionFreezeV3SHA256(), ConfigurationClassID: cell.ConfigurationClass.ClassID, RequestedModel: cell.ConfigurationClass.Model, RequestedProvider: cell.ConfigurationClass.ServingProvider, RequestedEffort: cell.ConfigurationClass.Effort, Transport: transportForCellV4(cell), RuntimeVersion: "fixture", RuntimeExecutablePath: "C:/fixture.exe", RuntimeExecutableSHA256: strings.Repeat("a", 64), ResolvedConfigSHA256: strings.Repeat("b", 64), ExactRouteValidated: true, FallbacksDisabled: true, CredentialState: "authenticated", OpenRouterSpendBaselineMicros: 0, ObservedAt: now.Format(time.RFC3339Nano), Ready: true}
}

func TestV4OpenCodeRequestInputsAreTypedToolFreeExactRoute(t *testing.T) {
	plan, _ := agentqualification.BuildRoleQualificationExecutionPlanV3()
	for _, cell := range plan.Cells {
		prompt, schema, err := promptAndSchemaV4(cell)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(prompt, cell.CaseID) {
			t.Fatal("candidate packet absent")
		}
		var typed agentqualification.RoleQualificationTypedSchemaV3
		if json.Unmarshal(schema, &typed) != nil || typed.AdditionalProperties {
			t.Fatal("typed schema drift")
		}
		for _, p := range typed.Properties {
			if p.Type != "string" && p.Type != "array" && p.Type != "boolean" {
				t.Fatal(p.Type)
			}
		}
		if cell.ConfigurationClass.RuntimeProvider != "claude_code_subscription" {
			config, err := inlineConfigV4(cell)
			if err != nil {
				t.Fatal(err)
			}
			op := openCodePlanV4(cell, config)
			if _, err := agentqualification.ValidateOpenCodeResolvedConfig(op, syntheticResolvedConfigV4(cell)); err != nil {
				t.Fatal(err)
			}
			if *op.ProviderFallbacks {
				t.Fatal("fallback enabled")
			}
		}
	}
}
func TestV4RawEnvelopePreservesActualRouteAndTypedOutput(t *testing.T) {
	plan, _ := agentqualification.BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	answer := map[string]any{"answer": "direct", "truth": "truth", "action": "action", "owner_decision_needed": false}
	structured, _ := json.Marshal(answer)
	now := mustTimeV4("2026-09-13T12:00:00Z")
	raw := rawEnvelopeV4(cell, "call", structured, "", agentqualification.RoleQualificationUsageV3{Calls: 1, InputTokens: 1, OutputTokens: 1, TotalTokens: 2, WallMillis: 1000}, now, now.Add(1e9))
	var got agentqualification.RoleQualificationRawProviderResponseV3
	if json.Unmarshal(raw, &got) != nil {
		t.Fatal("raw invalid")
	}
	if got.ActualModel != cell.ConfigurationClass.Model || got.ActualProvider != cell.ConfigurationClass.ServingProvider || got.RetryCount != 0 || got.FallbackUsed {
		t.Fatalf("raw=%+v", got)
	}
}
func syntheticResolvedConfigV4(cell agentqualification.RoleQualificationExecutionCellV3) []byte {
	model := strings.TrimPrefix(cell.ConfigurationClass.Model, "openrouter/")
	raw, _ := json.Marshal(map[string]any{"autoupdate": false, "share": "disabled", "snapshot": false, "plugin": []any{}, "instructions": []string{}, "permission": "deny", "tools": map[string]bool{"*": false}, "compaction": map[string]bool{"auto": false}, "default_agent": "qualification", "mcp": map[string]any{}, "agent": map[string]any{"qualification": map[string]any{"model": cell.ConfigurationClass.Model, "variant": cell.ConfigurationClass.Effort, "steps": 2, "permission": "deny", "tools": map[string]bool{"*": false}}}, "provider": map[string]any{"openrouter": map[string]any{"options": map[string]any{"name": "OpenRouter"}, "whitelist": []string{model}, "models": map[string]any{model: map[string]any{"options": map[string]any{"provider": map[string]any{"allow_fallbacks": false, "order": []string{cell.ConfigurationClass.ServingProvider}}}}}}}})
	return raw
}
func mustTimeV4(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
