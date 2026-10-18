package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestBuildReceiptsBindsThreeExactTemporaryRoles(t *testing.T) {
	root := t.TempDir()
	broker := filepath.Join(root, "broker.exe")
	provider := filepath.Join(root, "provider.exe")
	for _, path := range []string{broker, provider} {
		if err := writeFixture(path); err != nil {
			t.Fatal(err)
		}
	}
	issued := time.Date(2026, 9, 15, 5, 40, 0, 0, time.UTC)
	opts := options{
		workspaceID: "4d89b87d-6c66-48ba-b671-f7c6ed176dc6",
		issueID:     "01a0a390-d524-79a0-a3bf-5781d57f72ee", runtimeID: "dc924e46-34a9-4916-9fc2-847f13a8a6d0",
		leaderID: "7d622153-c9a9-4b57-912c-bebef8ce06b0", executorID: "eabbf54b-06e9-4709-8c4d-9ab4cbff1246",
		reviewerID: "c59782cf-15fb-4293-9110-08d89184994f", brokerPath: broker, providerPath: provider,
		profilePath: filepath.Join(root, "profile"), issuedAt: issued, expiresAt: issued.Add(3 * time.Hour),
	}
	receipts, err := buildReceipts(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 3 {
		t.Fatalf("receipt count = %d, want 3", len(receipts))
	}
	wantNames := []string{"Item 14 Replay Intake", "Item 14 Replay Executor", "Item 14 Replay Reviewer"}
	wantKinds := [][]string{{"leader"}, {"implementation", "correction"}, {"review"}}
	for i, receipt := range receipts {
		if receipt.ID != agentqualification.WorkflowBrokerRuntimeReceiptIDV1(opts.issueID, receipt.ExecutorAgentID) ||
			receipt.WorkspaceID != opts.workspaceID || receipt.IssueID != opts.issueID || receipt.RuntimeID != opts.runtimeID ||
			receipt.RoleID != replayRoleID || receipt.RuntimeProvider != "claude" || receipt.Model != replayModel ||
			receipt.ReasoningEffort != replayEffort || receipt.ChildExecution.Adapter != agentqualification.GuardedClaudeWorkflowBrokerAdapterV1 ||
			receipt.ChildExecution.ExecutableSHA256 == "" || receipt.ChildExecution.ProviderExecutableSHA256 == "" ||
			receipt.WorkflowBroker == nil || receipt.WorkflowBroker.AgentName != wantNames[i] ||
			receipt.WorkflowBroker.ProviderProfileKind != agentqualification.GuardedWorkflowBrokerProfileClaudeV1 ||
			!sameStrings(receipt.WorkflowBroker.AllowedTaskKinds, wantKinds[i]) || receipt.ExpiresAt.Sub(receipt.IssuedAt) != 3*time.Hour {
			t.Fatalf("receipt %d drifted: %#v", i, receipt)
		}
	}
}

func TestValidateLoopbackDatabaseURLIsExact(t *testing.T) {
	if err := validateLoopbackDatabaseURL("postgres://multica:multica@127.0.0.1:15434/multica?sslmode=disable"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"postgres://multica:multica@localhost:15434/multica",
		"postgres://multica:multica@127.0.0.1:15432/multica",
		"postgres://multica:multica@127.0.0.1:15434/other",
		"https://127.0.0.1:15434/multica",
	} {
		if err := validateLoopbackDatabaseURL(raw); err == nil {
			t.Fatalf("unsafe database URL accepted: %s", raw)
		}
	}
}

func TestParseOptionsRejectsUnrecognizedMode(t *testing.T) {
	if _, err := parseOptions([]string{"-mode", "launch"}); err == nil {
		t.Fatal("unrecognized authority mode was accepted")
	}
}

func TestExactSingleAttemptDefaultFailsClosed(t *testing.T) {
	if !isExactSingleAttemptDefault("1") {
		t.Fatal("exact one-attempt default was rejected")
	}
	for _, value := range []string{"", "2", "1::integer", "0", "1; DROP TABLE agent_task_queue"} {
		if isExactSingleAttemptDefault(value) {
			t.Fatalf("unsafe retry default was accepted: %q", value)
		}
	}
}

func TestExactLaunchContractPreflightRejectsDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "launch-contract.json")
	originalPath, originalHash := replayLaunchContractPath, replayLaunchContractSHA
	t.Cleanup(func() {
		replayLaunchContractPath, replayLaunchContractSHA = originalPath, originalHash
	})
	replayLaunchContractPath = path

	writeContract := func(contract replayLaunchContractV1) {
		raw, err := json.Marshal(contract)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		replayLaunchContractSHA = hex.EncodeToString(sum[:])
	}

	contract := expectedReplayLaunchContractV1()
	writeContract(contract)
	if err := validateReplayLaunchContract(path); err != nil {
		t.Fatalf("exact launch contract rejected: %v", err)
	}

	contract.HostAttestation = ""
	writeContract(contract)
	if err := validateReplayLaunchContract(path); err == nil {
		t.Fatal("launch contract with missing host attestation was accepted")
	}

	contract = expectedReplayLaunchContractV1()
	contract.MaxConcurrentTasks = 2
	writeContract(contract)
	if err := validateReplayLaunchContract(path); err == nil {
		t.Fatal("launch contract with concurrency drift was accepted")
	}

	contract = expectedReplayLaunchContractV1()
	contract.DaemonAuthMode = "pat"
	writeContract(contract)
	if err := validateReplayLaunchContract(path); err == nil {
		t.Fatal("launch contract with non-daemon authentication was accepted")
	}

	contract = expectedReplayLaunchContractV1()
	contract.GuardedPackageRequired = false
	writeContract(contract)
	if err := validateReplayLaunchContract(path); err == nil {
		t.Fatal("launch contract without a guarded package requirement was accepted")
	}
}

func writeFixture(path string) error {
	return os.WriteFile(path, []byte("fixture"), 0o600)
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
