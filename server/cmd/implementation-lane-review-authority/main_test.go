package main

import (
	"strings"
	"testing"
	"time"

	aq "github.com/multica-ai/multica/server/internal/agentqualification"
)

func validContract(now time.Time) contract {
	broker := strings.Repeat("a", 64)
	return contract{
		ControllerID: "11111111-1111-4111-8111-111111111111", WorkspaceID: "22222222-2222-4222-8222-222222222222",
		OwnerID: "33333333-3333-4333-8333-333333333333", IssueID: "44444444-4444-4444-8444-444444444444",
		SquadID: "55555555-5555-4555-8555-555555555555", RuntimeID: "66666666-6666-4666-8666-666666666666",
		ImplementerID: "77777777-7777-4777-8777-777777777777", ReviewerID: "88888888-8888-4888-8888-888888888888",
		ReviewerName: "Lane Evidence Reviewer",
		BrokerPath:   `C:\Dev\multica\server\bin\implementation-lane-review-broker.exe`, BrokerSHA256: broker,
		ProviderPath:      `C:\Users\a9sho\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe`,
		ProviderSHA256:    strings.Repeat("b", 64),
		ProviderProfile:   `C:\Dev\multica\.qualification-evidence\issue-lane-local\provider-home`,
		EvidenceReceiptID: evidencePrefix + broker, IssuedAt: now, ExpiresAt: now.Add(3 * time.Hour),
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestTheHelperOnlyServesTheLoopbackProofInstance(t *testing.T) {
	c := validContract(time.Now().UTC())
	ok := map[string]string{"APP_ENV": "development", "MULTICA_WORKFLOW_CONTROLLER_ID": c.ControllerID,
		"DATABASE_URL": "postgres://multica:x@127.0.0.1:55434/multica?sslmode=disable"}
	if err := checkEnvironment(env(ok), c); err != nil {
		t.Fatalf("the proof environment was refused: %v", err)
	}
	for name, change := range map[string][2]string{
		"production":       {"APP_ENV", "production"},
		"other controller": {"MULTICA_WORKFLOW_CONTROLLER_ID", "99999999-9999-4999-8999-999999999999"},
		"remote database":  {"DATABASE_URL", "postgres://multica:x@postgres:5432/multica"},
		"no database":      {"DATABASE_URL", ""},
	} {
		values := map[string]string{}
		for k, v := range ok {
			values[k] = v
		}
		values[change[0]] = change[1]
		if err := checkEnvironment(env(values), c); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestTheContractIsExactAndBounded(t *testing.T) {
	now := time.Now().UTC()
	if err := validateContract(validContract(now), now); err != nil {
		t.Fatalf("a valid contract was refused: %v", err)
	}
	cases := map[string]func(*contract){
		"self review":       func(c *contract) { c.ReviewerID = c.ImplementerID },
		"loose digest":      func(c *contract) { c.BrokerSHA256 = "abc" },
		"unbound evidence":  func(c *contract) { c.EvidenceReceiptID = evidencePrefix + strings.Repeat("c", 64) },
		"relative broker":   func(c *contract) { c.BrokerPath = `server\bin\broker.exe` },
		"slash path":        func(c *contract) { c.ProviderProfile = "C:/Dev/multica/provider-home" },
		"long window":       func(c *contract) { c.ExpiresAt = c.IssuedAt.Add(5 * time.Hour) },
		"expired":           func(c *contract) { c.IssuedAt, c.ExpiresAt = now.Add(-3*time.Hour), now.Add(-time.Hour) },
		"future issue time": func(c *contract) { c.IssuedAt = now.Add(time.Hour); c.ExpiresAt = now.Add(2 * time.Hour) },
		"bad issue id":      func(c *contract) { c.IssueID = "not-a-uuid" },
		"no reviewer name":  func(c *contract) { c.ReviewerName = " " },
	}
	for name, mutate := range cases {
		c := validContract(now)
		mutate(&c)
		if err := validateContract(c, now); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestTheReceiptGrantsOnlyTheReviewTurn(t *testing.T) {
	c := validContract(time.Now().UTC())
	receipt, err := buildReceipt(c)
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if receipt.ID != aq.WorkflowBrokerRuntimeReceiptIDV1(c.IssueID, c.ReviewerID) || receipt.ExecutorAgentID != c.ReviewerID {
		t.Fatalf("receipt identity = %s / %s", receipt.ID, receipt.ExecutorAgentID)
	}
	template := receipt.WorkflowBroker
	if template == nil || len(template.AllowedTaskKinds) != 1 || template.AllowedTaskKinds[0] != "review" {
		t.Fatalf("allowed task kinds = %v, want only review", template)
	}
	if receipt.ChildExecution.ToolPolicy != aq.GuardedToolPolicyNoneV1 || receipt.ChildExecution.ExecutableSHA256 != c.BrokerSHA256 ||
		receipt.ChildExecution.ProviderExecutableSHA256 != c.ProviderSHA256 {
		t.Fatalf("child execution boundary = %+v", receipt.ChildExecution)
	}
	if template.ProviderProfileKind != aq.GuardedWorkflowBrokerProfileClaudeV1 || template.AgentName != c.ReviewerName ||
		template.HostAttestation != hostAttestation {
		t.Fatalf("broker template = %+v", template)
	}
	if receipt.RoleID != roleID || receipt.Model != exactModel || receipt.ReasoningEffort != exactReasoningEffort ||
		receipt.RuntimeProvider != "claude" || !receipt.Immutable {
		t.Fatalf("receipt role or runtime selector = %+v", receipt)
	}
}

func TestWindowsPathContractsAreHostIndependent(t *testing.T) {
	for _, value := range []string{`C:\Dev\broker.exe`, `a:\fixture\profile`} {
		if !canonicalWindowsAbsolutePath(value) {
			t.Fatalf("valid Windows contract refused: %s", value)
		}
	}
	for _, value := range []string{`C:broker.exe`, `/tmp/broker`, `\\host\share`, `\\?\C:\broker`, `C:\a\..\broker`, `C:\a\.\broker`, `C:\\broker`, `C:\broker `, `C:\broker.`, `C:\NUL.exe`, `C:\a/broker`, `C:\a:stream`} {
		if canonicalWindowsAbsolutePath(value) {
			t.Fatalf("noncanonical Windows contract accepted: %s", value)
		}
	}
}
