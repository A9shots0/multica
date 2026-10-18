package agentqualification

import (
	"fmt"
	"strings"
)

// OpenRouterEstablishedSmallModelRouteV2 is an execution allowlist, not a
// model or role selection. It contains only the small non-Claude candidates
// that passed the frozen v26 reliability screen. Historical artifacts remain
// immutable, but new OpenRouter execution must pass this gate.
type OpenRouterEstablishedSmallModelRouteV2 struct {
	Model           string `json:"model"`
	ServingProvider string `json:"serving_provider"`
}

var openRouterEstablishedSmallModelRoutesV2 = []OpenRouterEstablishedSmallModelRouteV2{
	{Model: "openrouter/google/gemini-3.5-flash-lite", ServingProvider: "google-ai-studio"},
	{Model: "openrouter/nex-agi/nex-n2.5-pro:free", ServingProvider: "nex-agi"},
	{Model: "openrouter/qwen/qwen3-coder-next", ServingProvider: "alibaba"},
	{Model: "openrouter/upstage/solar-pro4", ServingProvider: "upstage"},
}

func OpenRouterEstablishedSmallModelRoutesV2() []OpenRouterEstablishedSmallModelRouteV2 {
	return append([]OpenRouterEstablishedSmallModelRouteV2(nil), openRouterEstablishedSmallModelRoutesV2...)
}

// ValidateOpenRouterEstablishedSmallModelRouteV2 is the mandatory provider
// boundary for new OpenRouter execution. Claude/Anthropic belongs exclusively
// to the Claude Code subscription lane, while OpenAI belongs exclusively to a
// Codex subscription lane. Neither may be reached through OpenRouter.
func ValidateOpenRouterEstablishedSmallModelRouteV2(model, servingProvider string) error {
	model = strings.TrimSpace(model)
	servingProvider = strings.TrimSpace(servingProvider)
	lowerModel := strings.ToLower(model)
	lowerServingProvider := strings.ToLower(servingProvider)

	if !strings.HasPrefix(lowerModel, "openrouter/") {
		return fmt.Errorf("OpenRouter boundary requires an openrouter model route")
	}
	if strings.Contains(lowerModel, "/anthropic/") || strings.Contains(lowerModel, "claude") || lowerServingProvider == "anthropic" {
		return fmt.Errorf("Claude and Anthropic models are forbidden through OpenRouter")
	}
	if strings.Contains(lowerModel, "/openai/") || lowerServingProvider == "openai" {
		return fmt.Errorf("OpenAI models are forbidden through OpenRouter")
	}
	for _, route := range openRouterEstablishedSmallModelRoutesV2 {
		if model == route.Model && servingProvider == route.ServingProvider {
			return nil
		}
	}
	return fmt.Errorf("OpenRouter model and serving-provider route is outside the established small non-Claude pool")
}
