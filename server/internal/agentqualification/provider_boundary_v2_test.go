package agentqualification

import "testing"

func TestOpenRouterProviderBoundaryV2AllowsOnlyEstablishedSmallNonClaudeRoutes(t *testing.T) {
	for _, route := range OpenRouterEstablishedSmallModelRoutesV2() {
		if err := ValidateOpenRouterEstablishedSmallModelRouteV2(route.Model, route.ServingProvider); err != nil {
			t.Fatalf("eligible route rejected: %+v: %v", route, err)
		}
	}
}

func TestOpenRouterProviderBoundaryV2RejectsClaudeOpenAIAlterationAndMismatchedProviders(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		servingProvider string
	}{
		{name: "Claude model", model: "openrouter/anthropic/claude-sonnet-4.6", servingProvider: "anthropic"},
		{name: "Claude name with disguised provider", model: "openrouter/example/claude-small", servingProvider: "example"},
		{name: "Anthropic provider", model: "openrouter/example/small", servingProvider: "anthropic"},
		{name: "OpenAI model", model: "openrouter/openai/gpt-5-mini", servingProvider: "openai"},
		{name: "OpenAI provider", model: "openrouter/example/small", servingProvider: "openai"},
		{name: "unqualified model", model: "openrouter/inclusionai/ling-3.0-flash", servingProvider: "novita"},
		{name: "altered model", model: "openrouter/qwen/qwen3-coder-next:free", servingProvider: "alibaba"},
		{name: "mismatched provider", model: "openrouter/qwen/qwen3-coder-next", servingProvider: "other"},
		{name: "case-altered provider", model: "openrouter/qwen/qwen3-coder-next", servingProvider: "ALIBABA"},
		{name: "direct model", model: "qwen/qwen3-coder-next", servingProvider: "alibaba"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateOpenRouterEstablishedSmallModelRouteV2(test.model, test.servingProvider); err == nil {
				t.Fatalf("forbidden route passed: %q via %q", test.model, test.servingProvider)
			}
		})
	}
}
