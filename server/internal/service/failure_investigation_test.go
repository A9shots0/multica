package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestProviderAuthenticationDoesNotLaunchModelFailureResearch(t *testing.T) {
	const expired = "Failed to authenticate: OAuth session expired and could not be refreshed"
	for _, reason := range []string{string(taskfailure.ReasonAgentProviderAuthOrAccess), string(taskfailure.ReasonAgentUnknown), ""} {
		if shouldInvestigateTerminalAIFailure(reason, expired) {
			t.Errorf("provider authentication with reason %q would block recovery with AI research", reason)
		}
	}
	if shouldInvestigateTerminalAIFailure(string(taskfailure.ReasonAgentProviderAuthOrAccess), "provider rejected credentials") {
		t.Fatal("canonical provider authentication reason must be respected")
	}
	for _, message := range []string{"worker produced no usable artifact", "tool failed unexpectedly", "OAuth documentation was unavailable during design work"} {
		if !shouldInvestigateTerminalAIFailure(string(taskfailure.ReasonAgentUnknown), message) {
			t.Errorf("unknown failure %q lost its investigation", message)
		}
	}
}

func TestProviderModelAccessDoesNotLaunchModelFailureResearch(t *testing.T) {
	const rejected = `{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'gpt-6.1-sol' model is not supported when using Codex with a ChatGPT account."}}`
	for _, reason := range []string{string(taskfailure.ReasonAPIInvalidRequest), string(taskfailure.ReasonAgentUnknown), string(taskfailure.ReasonAgentModelNotFoundOrUnavailable)} {
		if shouldInvestigateTerminalAIFailure(reason, rejected) {
			t.Errorf("provider model access with reason %q would block recovery with AI research", reason)
		}
	}
	if !shouldInvestigateTerminalAIFailure(string(taskfailure.ReasonAPIInvalidRequest), "malformed tool input in generated request") {
		t.Fatal("other invalid requests still need investigation")
	}
}

func TestFailureInvestigationBriefRequiresRootCauseBeforeReplacement(t *testing.T) {
	for _, required := range []string{
		"Do not jump to a replacement solution",
		"company and product context",
		"model behavior, capability limits",
		"prompts, instructions, memory",
		"tools, runtime, data, evidence",
		"human factors, psychology",
		"workflow, reviewer, QA",
		"current authoritative external research",
		"causal timeline",
		"root-cause tree",
		"why existing gates missed it",
		"verification plan that could falsify",
		"Do not silently accept",
	} {
		if !strings.Contains(failureInvestigationBrief, required) {
			t.Fatalf("failure investigation brief is missing %q", required)
		}
	}
	if len(failureInvestigationScope) != 6 {
		t.Fatalf("research scope has %d dimensions, want 6", len(failureInvestigationScope))
	}
}

func validFailureInvestigationOutput(t *testing.T, investigationID string) string {
	t.Helper()
	domains := make([]map[string]any, 0, len(failureInvestigationScope))
	for _, domain := range failureInvestigationScope {
		sources := []map[string]string{}
		if domain == "model_behavior_and_capability_limits" || domain == "human_factors_psychology_and_decision_design" {
			sources = append(sources, map[string]string{"title": "Authoritative research", "url": "https://example.org/research"})
		}
		domains = append(domains, map[string]any{
			"id": domain, "findings": []string{"The evidence establishes a task-specific finding."},
			"evidence_refs": []string{"issue:fixture"}, "external_sources": sources,
		})
	}
	report := map[string]any{
		"schema_version": 1, "investigation_id": investigationID, "summary": "The failure crossed multiple system layers.",
		"causal_timeline":         []string{"The request was received.", "The result bypassed the intended gate."},
		"root_causes":             []map[string]any{{"cause": "The completion boundary accepted unverified output.", "confidence": "confirmed", "evidence_refs": []string{"task:fixture"}}},
		"contributing_conditions": []string{"The prompt was treated as enforcement."},
		"gate_misses":             []string{"No structural receipt was validated."}, "domains": domains,
		"facts": []string{"The task completed with an output."}, "hypotheses": []string{}, "unknowns": []string{},
		"prevention_changes":      []map[string]string{{"layer": "workflow", "change": "Validate a typed receipt.", "verification": "Reject a shallow completion fixture."}},
		"verification_plan":       []map[string]string{{"test": "Submit a shallow completion.", "falsifies_if": "The investigation is marked completed."}},
		"learning_candidates":     []string{"Prompts require server-side evidence gates."},
		"replacement_work_status": "ready_for_review", "next_action": "Review the prevention changes before replacement work.",
		"owner_input": map[string]any{"required": false, "question": "", "options": []string{}},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return failureInvestigationOpenTag + string(raw) + failureInvestigationCloseTag
}

func failureInvestigationWithOwnerInput(t *testing.T, output string, required bool, question string, options []string) string {
	t.Helper()
	raw, err := extractFailureInvestigationBlock(output)
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	report["owner_input"] = map[string]any{"required": required, "question": question, "options": options}
	raw, err = json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return failureInvestigationOpenTag + string(raw) + failureInvestigationCloseTag
}

func TestValidatedFailureInvestigationFindingsRequiresResearchAndFalsifiablePrevention(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000123"
	findings, err := validatedFailureInvestigationFindings(validFailureInvestigationOutput(t, id), id)
	if err != nil {
		t.Fatalf("valid investigation rejected: %v", err)
	}
	if !json.Valid(findings) || !strings.Contains(string(findings), "verification_plan") {
		t.Fatalf("canonical findings missing verification plan: %s", findings)
	}
	if _, err := validatedFailureInvestigationFindings(
		failureInvestigationWithOwnerInput(t, validFailureInvestigationOutput(t, id), true, "Which prevention plan should we use?", []string{"Use the reviewed plan", "Change the plan first"}),
		id,
	); err != nil {
		t.Fatalf("valid owner question rejected: %v", err)
	}

	for name, output := range map[string]string{
		"shallow prose":           "We should improve the prompt and retry.",
		"wrong identity":          strings.Replace(validFailureInvestigationOutput(t, id), id, "00000000-0000-0000-0000-000000000999", 1),
		"missing research domain": strings.Replace(validFailureInvestigationOutput(t, id), `"id":"company_and_product_context"`, `"id":"model_behavior_and_capability_limits"`, 1),
		"hidden owner question":   failureInvestigationWithOwnerInput(t, validFailureInvestigationOutput(t, id), false, "Choose a plan", []string{}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validatedFailureInvestigationFindings(output, id); err == nil {
				t.Fatal("expected incomplete investigation to be rejected")
			}
		})
	}
}

func TestNormalizeFailureInvestigationSummaryIsBoundedAndNeverEmpty(t *testing.T) {
	if got := normalizeFailureInvestigationSummary(" \x00 "); !strings.Contains(got, "did not include") {
		t.Fatalf("empty summary fallback = %q", got)
	}
	got := normalizeFailureInvestigationSummary(strings.Repeat("a", maxFailureInvestigationSummaryRunes+10))
	if len([]rune(got)) != maxFailureInvestigationSummaryRunes+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("bounded summary has %d runes and suffix %q", len([]rune(got)), got[len(got)-3:])
	}
}
