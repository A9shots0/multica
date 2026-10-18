package agentqualification

import (
	"context"
	"testing"
)

type topologyCapturingProvider struct {
	requests []RealisticProviderRequest
}

func (provider *topologyCapturingProvider) Next(_ context.Context, request RealisticProviderRequest) (RealisticProviderResponse, error) {
	provider.requests = append(provider.requests, request)
	return RealisticProviderResponse{}, nil
}

func TestTopologyTaskProviderReplacesTaskAndCapsCalls(t *testing.T) {
	inner := &topologyCapturingProvider{}
	provider := &topologyTaskProvider{inner: inner, task: "bounded correction", maxCalls: 1, maxTotalTokens: 100, maxGeneratedTokens: 50}
	request := RealisticProviderRequest{Task: "full issue history", PreviousResults: []RealisticToolResult{{Output: "secret history"}}}
	if _, err := provider.Next(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(inner.requests) != 1 || inner.requests[0].Task != "bounded correction" || len(inner.requests[0].PreviousResults) != 1 {
		t.Fatal("correction provider did not preserve the fresh correction-run history")
	}
	if _, err := provider.Next(context.Background(), request); err != ErrTopologyStageBudgetExceeded {
		t.Fatal("correction provider did not enforce its exact call ceiling")
	}
}

func TestTopologyTaskProviderStopsStageTokenOverrun(t *testing.T) {
	inner := topologyFixedUsageProvider{response: RealisticProviderResponse{InputTokens: 8, OutputTokens: 3}}
	provider := &topologyTaskProvider{inner: inner, task: "task", maxCalls: 2, maxTotalTokens: 10, maxGeneratedTokens: 5}
	if response, err := provider.Next(context.Background(), RealisticProviderRequest{}); err != nil || response.InputTokens != 8 {
		t.Fatalf("paid overrun response was not returned for durable accounting: response=%+v err=%v", response, err)
	}
	if _, err := provider.Next(context.Background(), RealisticProviderRequest{}); err != ErrTopologyStageBudgetExceeded {
		t.Fatalf("next call did not stop before another provider request: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("budget exhaustion made another paid call: %d", provider.calls)
	}
}

type topologyFixedUsageProvider struct{ response RealisticProviderResponse }

func (provider topologyFixedUsageProvider) Next(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
	return provider.response, nil
}
