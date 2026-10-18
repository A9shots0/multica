package taskcontinuity

import (
	"strings"
	"testing"
)

func TestProviderAdaptersCarryIdenticalContractAndHash(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"codex", "claude", "copilot"} {
		adapter, err := ProviderAdapter(provider)
		if err != nil {
			t.Fatalf("ProviderAdapter(%q) error = %v", provider, err)
		}
		for _, want := range []string{
			AdapterBegin,
			AdapterEnd,
			ContractHash(),
			RuntimeInstruction,
		} {
			if !strings.Contains(adapter, want) {
				t.Errorf("ProviderAdapter(%q) missing %q", provider, want)
			}
		}
		if strings.Count(adapter, RuntimeInstruction) != 1 {
			t.Errorf("ProviderAdapter(%q) contract count = %d, want 1", provider, strings.Count(adapter, RuntimeInstruction))
		}
	}
}

func TestProviderAdapterRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	if _, err := ProviderAdapter("unknown"); err == nil {
		t.Fatal("ProviderAdapter(unknown) succeeded, want error")
	}
}

func TestUpsertProviderAdapterPreservesSurroundingInstructions(t *testing.T) {
	t.Parallel()

	first, err := UpsertProviderAdapter("# User rules\n\nKeep this.\n", "codex")
	if err != nil {
		t.Fatalf("first upsert error = %v", err)
	}
	if !strings.HasPrefix(first, "# User rules\n\nKeep this.\n") || !InstalledContractMatches(first) {
		t.Fatalf("first upsert lost surrounding content or contract:\n%s", first)
	}
	second, err := UpsertProviderAdapter(first, "claude")
	if err != nil {
		t.Fatalf("replacement upsert error = %v", err)
	}
	if !strings.Contains(second, "Execution continuity (Claude Code)") ||
		strings.Contains(second, "Execution continuity (Codex)") ||
		strings.Count(second, AdapterBegin) != 1 ||
		!InstalledContractMatches(second) {
		t.Fatalf("replacement did not preserve one current adapter:\n%s", second)
	}
}

func TestUpsertProviderAdapterRejectsMalformedMarkers(t *testing.T) {
	t.Parallel()

	for _, existing := range []string{
		AdapterBegin,
		AdapterEnd,
		AdapterEnd + "\n" + AdapterBegin,
		AdapterBegin + "\n" + AdapterBegin + "\n" + AdapterEnd,
	} {
		if _, err := UpsertProviderAdapter(existing, "codex"); err == nil {
			t.Fatalf("UpsertProviderAdapter(%q) succeeded, want error", existing)
		}
	}
}
