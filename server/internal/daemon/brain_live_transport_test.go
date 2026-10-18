package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in transport smoke: this calls the actual configured shared service,
// but never resolves or launches an installed agent/provider executable.
func TestOwnerBrainLiveTransport(t *testing.T) {
	if os.Getenv("MULTICA_RUN_BRAIN_TRANSPORT_SMOKE") != "1" {
		t.Skip("requires the isolated owner-review fixture")
	}
	raw, err := os.ReadFile(os.Getenv("MULTICA_BRAIN_CLAIM_FIXTURE"))
	if err != nil {
		t.Fatal("protected claim fixture unavailable")
	}
	var claim struct {
		Task Task `json:"task"`
	}
	if json.Unmarshal(raw, &claim) != nil || !brainReferenceBound(claim.Task.BrainContext) {
		t.Fatal("claim lacks durable brain binding")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, broker, err := startTaskBrainMCP(ctx, claim.Task, NewClient("http://127.0.0.1:3230"), nil)
	if err != nil || broker == nil {
		t.Fatal("task broker unavailable")
	}
	defer broker.Close()
	var parsed struct {
		Servers map[string]struct {
			URL string `json:"url"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(config, &parsed) != nil {
		t.Fatal("invalid broker configuration")
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		resp, err := http.Post(parsed.Servers["silver-mediums-brain"].URL, "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal("broker HTTP request failed")
		}
		defer resp.Body.Close()
		var packet struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if json.NewDecoder(resp.Body).Decode(&packet) != nil || packet.Result.IsError || len(packet.Result.Content) != 1 {
			t.Fatal("broker call failed: " + name)
		}
		var result map[string]any
		if json.Unmarshal([]byte(packet.Result.Content[0].Text), &result) != nil {
			t.Fatal("invalid tool result")
		}
		return result
	}
	contextResult := call("brain_context", map[string]any{"q": "task authority"})
	project, _ := contextResult["project"].(map[string]any)
	if project["id"] != "multica" {
		t.Fatal("context escaped mapped project")
	}
	full := call("brain_read", map[string]any{"id": "7c2f7568-d49b-5c60-9c7d-053cb1417f2b", "revision": "9d879306a6d2ba15b32e809b43ecbf99e184ce05bc56f4f3d9dcbe6a5e889570"})
	doc, _ := full["document"].(map[string]any)
	body, _ := doc["body"].(string)
	if doc["project"] != "multica" || doc["truncated"] != false || body == "" {
		t.Fatal("immutable reference incomplete")
	}
	input := map[string]any{"eventId": os.Getenv("MULTICA_BRAIN_SMOKE_EVENT_ID"), "summary": "Isolated Multica owner review: real task-scoped MCP context and full immutable read succeeded. Provider execution and production deployment were not tested. Acceptance remains unestablished.", "evidenceRefs": []string{"C:/Dev/_work/brain-completion-evidence/owner-review-tests.log"}}
	for i := 0; i < 2; i++ {
		receipt := call("brain_record_event", input)
		if receipt["accepted"] != false || (receipt["queued"] != true && receipt["delivered"] != true) {
			t.Fatal("handoff state missing or falsely accepted")
		}
	}
	t.Log("Real task claim, scoped MCP context, immutable full read and idempotent queued handoff passed; no provider executable invoked.")
}
