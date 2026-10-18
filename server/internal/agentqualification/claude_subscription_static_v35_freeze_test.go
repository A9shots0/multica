package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/multica-ai/multica/server/internal/qualificationarchive"
	"strings"
	"testing"
)

func TestClaudeSubscriptionStaticV35FreezeBindsPreparedReviewAndAllExecutionInputs(t *testing.T) {
	freeze, err := historicalContracts(t).CurrentClaudeSubscriptionStaticV35Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if !freeze.ProviderCallsAllowed || freeze.PreparedDefinitionSHA256 != claudeSubscriptionStaticV35PreparedDefinitionSHA256 || freeze.PreparedAntecedentSHA256 != claudeSubscriptionStaticV35PreparedAntecedentSHA256 || freeze.PreparedReviewBinding != claudeSubscriptionStaticV35PreparedReviewBinding || len(ClaudeSubscriptionStaticV35FreezeSHA256()) != 64 {
		t.Fatalf("unexpected v35 freeze: %+v", freeze)
	}
	commands := map[string]string{
		"claude-subscription-static-v35/main.go":           "../../cmd/claude-subscription-static-v35/main.go",
		"claude-subscription-static-v35/lifecycle.go":      "../../cmd/claude-subscription-static-v35/lifecycle.go",
		"claude-subscription-static-v35/lifecycle_test.go": "../../cmd/claude-subscription-static-v35/lifecycle_test.go",
	}
	for name, path := range commands {
		assertClaudeV35FrozenFile(t, path, freeze.CommandSources[name], name)
	}
	assertClaudeV35FrozenFile(t, "../../../docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-STATIC-V35-PREVALIDATION.md", freeze.Documents["docs/silver-mediums/ITEM-14-CLAUDE-SUBSCRIPTION-STATIC-V35-PREVALIDATION.md"], "prevalidation document")
}

func TestClaudeSubscriptionStaticV35FreezeRejectsUnlockAndBindingDrift(t *testing.T) {
	freeze, err := historicalContracts(t).CurrentClaudeSubscriptionStaticV35Freeze()
	if err != nil {
		t.Fatal(err)
	}
	freeze.ProviderCallsAllowed = false
	if historicalContracts(t).ValidateClaudeSubscriptionStaticV35Freeze(freeze) == nil {
		t.Fatal("v35 freeze accepted relock drift")
	}
	freeze, _ = historicalContracts(t).CurrentClaudeSubscriptionStaticV35Freeze()
	freeze.PreparedReviewBinding = "drift"
	if historicalContracts(t).ValidateClaudeSubscriptionStaticV35Freeze(freeze) == nil {
		t.Fatal("v35 freeze accepted prepared review-binding drift")
	}
}

func assertClaudeV35FrozenFile(t *testing.T, path, expected, label string) {
	t.Helper()
	logical := strings.TrimPrefix(path, "../../../")
	if strings.HasPrefix(path, "../../cmd/") {
		logical = "server/cmd/" + strings.TrimPrefix(path, "../../cmd/")
	}
	raw, err := qualificationarchive.Read("claude-v35", logical, expected)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != expected {
		t.Fatalf("v35 frozen %s drifted", label)
	}
}
