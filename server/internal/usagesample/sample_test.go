package usagesample

import (
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestSampleDigestAndShadowDecision(t *testing.T) {
	live := agent.UsageSample{
		Sequence: 2, BackendAttempt: 1, Provider: "codex", Model: "gpt-test", ProviderCalls: 2, SourceEvent: "usage",
		Usage:              agent.TokenUsage{InputTokens: 10},
		ModelContextWindow: 100, ModelContextWindowKind: agent.ContextMeasurementExact,
		ContextTokens: 70, ContextTokensMeasurementKind: agent.ContextMeasurementExact,
	}
	sample, err := New("task:1", 1, live)
	if err != nil {
		t.Fatal(err)
	}
	digest1, _ := sample.Digest()
	digest2, _ := sample.Digest()
	if digest1 != digest2 {
		t.Fatalf("digest changed: %q != %q", digest1, digest2)
	}
	decision, ratio := ShadowDecision(sample)
	if decision != "would_request_checkpoint" || ratio == nil || *ratio != 700_000 {
		t.Fatalf("decision=%q ratio=%v", decision, ratio)
	}
}

func TestProxyNeverClaimsOccupancyDecision(t *testing.T) {
	sample, err := New("task:1", 1, agent.UsageSample{
		Sequence: 1, BackendAttempt: 1, Provider: "codex", Model: "gpt-test", ProviderCalls: 1, SourceEvent: "usage",
		ModelContextWindow: 100, ModelContextWindowKind: agent.ContextMeasurementExact,
		ContextTokens: 99, ContextTokensMeasurementKind: agent.ContextMeasurementProxy,
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, ratio := ShadowDecision(sample)
	if decision != "unknown" || ratio != nil {
		t.Fatalf("proxy decision=%q ratio=%v", decision, ratio)
	}
}

func TestUnknownMeasurementsCannotCarryFabricatedValues(t *testing.T) {
	_, err := New("task:1", 1, agent.UsageSample{
		Sequence: 1, BackendAttempt: 1, Provider: "claude", Model: "test", ProviderCalls: 1, SourceEvent: "assistant",
		ContextTokens: 42, ContextTokensMeasurementKind: agent.ContextMeasurementUnknown,
		ModelContextWindowKind: agent.ContextMeasurementUnknown,
	})
	if err == nil {
		t.Fatal("expected unknown non-zero context to fail")
	}
}
