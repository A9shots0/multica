package agentqualification

import (
	"slices"
	"testing"
)

func TestClaudeSubscriptionStaticV35ExactRepeatIsReviewLocked(t *testing.T) {
	d, err := ClaudeSubscriptionStaticV35()
	if err != nil {
		t.Fatal(err)
	}
	if d.ProviderCallsAllowed || d.Status != "prepared_for_independent_review" || d.Caps.MaxRetries != 0 {
		t.Fatalf("v35 not review locked: %+v", d)
	}
	want := []string{"T12:v4", "T11:v4", "T08:v3", "T16:v3", "T04:v1", "T09:v2"}
	for i, c := range d.Cases {
		if c.CaseID+":"+c.CaseVersion != want[i] {
			t.Fatalf("order=%v", d.Cases)
		}
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
		if !adjacentV35(p.ArgsTemplate, "--model", "claude-sonnet-4-6") || !adjacentV35(p.ArgsTemplate, "--effort", "medium") || !adjacentV35(p.ArgsTemplate, "--tools", "") {
			t.Fatalf("route drift: %+v", p.ArgsTemplate)
		}
	}
}

func TestClaudeSubscriptionStaticV35AntecedentIsCompleteV34(t *testing.T) {
	a, err := ClaudeSubscriptionStaticV35AntecedentManifest()
	if err != nil {
		t.Fatal(err)
	}
	if a.PriorLedgerSHA256 != "21c3089b90b841513d3107477df339fe874aec9c93d71e3d8fbe37aac8207163" || a.PriorFreezeSHA256 != "424041cd761a56b72db4bb92c48f7518901f0975f608d37e3cb687ba0a92f9a3" || a.PriorInputTokens != 35643 || a.PriorOutputTokens != 4082 || a.PriorProviderReportedCostMicros != 253410 {
		t.Fatalf("antecedent drift: %+v", a)
	}
	if ClaudeSubscriptionStaticV35ReviewBindingSHA256() == "" {
		t.Fatal("missing review binding")
	}
}

func adjacentV35(v []string, l, r string) bool {
	for i := 0; i+1 < len(v); i++ {
		if v[i] == l && v[i+1] == r {
			return true
		}
	}
	return false
}
