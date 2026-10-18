package agentqualification

import (
	"slices"
	"testing"
)

func TestClaudeSubscriptionStaticV34ReusesExactFrozenCasesAndLocksCalls(t *testing.T) {
	d, err := ClaudeSubscriptionStaticV34()
	if err != nil {
		t.Fatal(err)
	}
	if !d.ProviderCallsAllowed || d.Status != "frozen_reviewed" || d.Caps.MaxProviderInvocations != 6 || d.Caps.MaxRetries != 0 {
		t.Fatalf("unsafe v34 plan: %+v", d)
	}
	if d.Configuration.ServingProvider != "anthropic" || d.Configuration.ProviderFallbacks == nil || *d.Configuration.ProviderFallbacks {
		t.Fatalf("Claude provider route is not locked: %+v", d.Configuration)
	}
	for _, c := range d.Cases {
		p, err := BuildClaudeSubscriptionScreeningPlan(d.Configuration, c)
		if err != nil {
			t.Fatal(err)
		}
		if p.Transport != ClaudeCodeSubscriptionToolFreeTransport || slices.Contains(p.ArgsTemplate, "--bare") {
			t.Fatalf("transport drift: %+v", p.ArgsTemplate)
		}
		for _, required := range []string{"--restricted", "--tools", "--strict-mcp-config", "--no-session-persistence", "--json-schema", "--max-turns"} {
			if !slices.Contains(p.ArgsTemplate, required) {
				t.Fatalf("%s missing %s", c.CaseID, required)
			}
		}
		if !containsAdjacentV34(p.ArgsTemplate, "--model", "claude-sonnet-4-6") || !containsAdjacentV34(p.ArgsTemplate, "--effort", "medium") || !containsAdjacentV34(p.ArgsTemplate, "--tools", "") {
			t.Fatalf("exact model, effort, or tool denial drifted: %+v", p.ArgsTemplate)
		}
	}
}

func containsAdjacentV34(values []string, left, right string) bool {
	for index := 0; index+1 < len(values); index++ {
		if values[index] == left && values[index+1] == right {
			return true
		}
	}
	return false
}

func TestClaudeSubscriptionStaticV34ExactOrderAndBudgets(t *testing.T) {
	d, err := ClaudeSubscriptionStaticV34()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"T12:v4", "T11:v4", "T08:v3", "T16:v3", "T04:v1", "T09:v2"}
	for i, c := range d.Cases {
		if c.CaseID+":"+c.CaseVersion != want[i] {
			t.Fatalf("order=%v", d.Cases)
		}
	}
	if int64(len(d.Cases))*d.Caps.MaxTotalReportedTokensPerCell != d.Caps.MaxCampaignReportedTokens {
		t.Fatal("campaign cap does not bound six cells")
	}
}

func TestClaudeSubscriptionStaticV34TransportCatalogRecordsValidCalibrationButStaysHeld(t *testing.T) {
	catalog, err := CurrentTransportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range catalog.Transports {
		if transport.ID != "claude_code_subscription" {
			continue
		}
		if transport.Status != "held" || transport.UsageReceipt != "json_live_receipt_normalized" || transport.PreventiveToolPolicy != "restricted_tools_empty_stdin_live_calibrated" {
			t.Fatalf("Claude subscription transport calibration drifted: %+v", transport)
		}
		if !contains(transport.Reasons, "the corrected stdin transport produced one valid T11 v4 PASS with one invocation and zero retry") || !contains(transport.Reasons, "the lane remains held pending frozen static breadth and reliability evidence") {
			t.Fatalf("Claude subscription transport did not preserve calibration/hold: %+v", transport.Reasons)
		}
		return
	}
	t.Fatal("Claude subscription transport is missing")
}
