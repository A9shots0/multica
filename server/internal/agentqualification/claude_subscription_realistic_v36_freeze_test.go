package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeSubscriptionRealisticV36Freeze(t *testing.T) {
	freeze, err := CurrentClaudeSubscriptionRealisticV36Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if !freeze.ProviderCallsAllowed || freeze.PreparedDefinitionSHA256 != ClaudeSubscriptionRealisticV36SHA256() || freeze.ExecutionOrder[0] != "R03:v2" || freeze.ExecutionOrder[1] != "R04:v1" {
		t.Fatalf("freeze=%+v", freeze)
	}
	if err := RequireClaudeSubscriptionRealisticV36Freeze(); err != nil {
		t.Fatal(err)
	}
	_, current, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	for name, want := range freeze.CommandSources {
		raw, err := os.ReadFile(filepath.Join(repoRoot, "server", "cmd", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		if hex.EncodeToString(hash[:]) != want {
			t.Fatalf("command source drift: %s", name)
		}
	}
	for name, want := range freeze.Documents {
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		if hex.EncodeToString(hash[:]) != want {
			t.Fatalf("document drift: %s", name)
		}
	}
}
