package agentqualification

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDirectOutputSchemaSupportsDisclosedEnums(t *testing.T) {
	schema, err := directOutputSchema("T12", "v3")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range [][]byte{[]byte(`"enum":["team_lead","team-lead"]`), []byte(`"uniqueItems":true`), []byte(`"phonefarm_contract_fixture"`)} {
		if !bytes.Contains(schema, required) {
			t.Fatalf("generated enum schema missing %s: %s", required, schema)
		}
	}
}

func TestDirectScreeningPlanIsPinnedAndToolFree(t *testing.T) {
	batch, err := CurrentScreeningBatch()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildDirectScreeningPlan(batch, "screen.generalist.luna.medium.v5", "T12")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PromptSHA256 == "" || plan.SchemaSHA256 == "" || !json.Valid(plan.Schema) {
		t.Fatalf("unsealed plan: %+v", plan)
	}
	for _, forbidden := range []string{"multica issue", "mention://", "SILV-", "tool result"} {
		if containsText(plan.Prompt, forbidden) {
			t.Fatalf("direct prompt leaked workflow transport %q", forbidden)
		}
	}
	if !containsText(plan.Prompt, "<qualification-policy>") || !containsText(plan.Prompt, "<case-packet>") {
		t.Fatal("direct prompt omitted policy or public packet")
	}
}

func TestParseDirectScreeningJSONLAcceptsOneTurnWithoutTools(t *testing.T) {
	raw := []byte("{\"type\":\"thread.started\",\"thread_id\":\"x\"}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"{\\\"owner_outcome\\\":\\\"Restore access.\\\"}\"}}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":12000,\"cached_input_tokens\":9000,\"output_tokens\":500}}\n")
	run, err := ParseDirectScreeningJSONL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "VALID" || run.ToolEvents != 0 || run.FreshInputTokens != 3000 || run.TotalTokens != 12500 {
		t.Fatalf("unexpected direct receipt: %+v", run)
	}
	if run.RawTraceSHA256 == "" || run.EventCount != 3 {
		t.Fatalf("trace provenance missing: %+v", run)
	}
}

func TestDirectToolEventCoversNonCommandCapabilities(t *testing.T) {
	for _, kind := range []string{"dynamic_tool_call", "image_generation", "collab_agent_tool_call", "server_tool_use"} {
		if !directToolEvent("item.completed", kind) {
			t.Fatalf("tool-capable item %q was not detected", kind)
		}
	}
}

func TestParseDirectScreeningJSONLInvalidatesAnyToolEvent(t *testing.T) {
	raw := []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\"}}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"{}\"}}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":100,\"cached_input_tokens\":20,\"output_tokens\":10}}\n")
	run, err := ParseDirectScreeningJSONL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "INVALID" || run.ToolEvents != 1 || !contains(run.InvalidReasons, "tool_event_observed") {
		t.Fatalf("tool event was accepted: %+v", run)
	}
}
