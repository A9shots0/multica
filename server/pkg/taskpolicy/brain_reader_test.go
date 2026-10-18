package taskpolicy

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewBrainMCPReadOnlyUsesFixedProviderAndExactToolSet(t *testing.T) {
	policy, err := NewBrainMCPReadOnly(Request{
		Mode:      BrainMCPReadOnlyMode,
		MCPServer: "brain-reader",
	})
	if err != nil {
		t.Fatal(err)
	}
	if policy.Provider != BrainMCPReadOnlyProvider {
		t.Fatalf("provider = %q", policy.Provider)
	}
	want := []string{
		"mcp__brain-reader__brain_resolve",
		"mcp__brain-reader__brain_context",
		"mcp__brain-reader__brain_read",
		"mcp__brain-reader__brain_company_projects",
		"mcp__brain-reader__brain_projects",
		"mcp__brain-reader__company_events",
	}
	if !reflect.DeepEqual(policy.Tools, want) {
		t.Fatalf("tools = %#v, want %#v", policy.Tools, want)
	}
}

func TestNewBrainMCPReadOnlyRejectsInvalidModeOrServer(t *testing.T) {
	for _, request := range []Request{
		{Mode: "other", MCPServer: "brain"},
		{Mode: BrainMCPReadOnlyMode, MCPServer: ""},
		{Mode: BrainMCPReadOnlyMode, MCPServer: "../brain"},
	} {
		if _, err := NewBrainMCPReadOnly(request); err == nil {
			t.Errorf("NewBrainMCPReadOnly(%+v) succeeded", request)
		}
	}
}

func TestValidateRequiresExactUniqueAllowlist(t *testing.T) {
	policy, err := NewBrainMCPReadOnly(Request{Mode: BrainMCPReadOnlyMode, MCPServer: "brain"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(policy); err != nil {
		t.Fatal(err)
	}

	mutated := *policy
	mutated.Tools = append(append([]string(nil), policy.Tools...), "mcp__brain__unknown")
	if err := Validate(&mutated); err == nil {
		t.Fatal("accepted an additional tool")
	}
	mutated = *policy
	mutated.Tools[0] = mutated.Tools[1]
	if err := Validate(&mutated); err == nil {
		t.Fatal("accepted a duplicate tool")
	}
	mutated = *policy
	mutated.Provider = "codex"
	if err := Validate(&mutated); err == nil {
		t.Fatal("accepted a different provider")
	}
}

func TestIsBrainMCPReadOnlyToolFailsClosed(t *testing.T) {
	if IsBrainMCPReadOnlyTool(nil, "mcp__brain__brain_read") {
		t.Fatal("nil policy allowed a tool")
	}
	policy, err := NewBrainMCPReadOnly(Request{Mode: BrainMCPReadOnlyMode, MCPServer: "brain"})
	if err != nil {
		t.Fatal(err)
	}
	if !IsBrainMCPReadOnlyTool(policy, "mcp__brain__brain_read") {
		t.Fatal("exact authorized tool was rejected")
	}
	for _, tool := range []string{"Bash", "Read", "Write", "Edit", "mcp__other__brain_read", "mcp__brain__unknown"} {
		if IsBrainMCPReadOnlyTool(policy, tool) {
			t.Errorf("unauthorized tool %q was allowed", tool)
		}
	}
}

func TestFromTaskContextReadsAndValidatesPolicy(t *testing.T) {
	context := `{"head_sha":"abc","execution_policy":{"mode":"brain_mcp_read_only","provider":"claude","mcp_server":"brain","tools":["mcp__brain__brain_resolve","mcp__brain__brain_context","mcp__brain__brain_read","mcp__brain__brain_company_projects","mcp__brain__brain_projects","mcp__brain__company_events"]}}`
	policy, err := FromTaskContext([]byte(context))
	if err != nil {
		t.Fatal(err)
	}
	if policy == nil || policy.MCPServer != "brain" {
		t.Fatalf("policy = %#v", policy)
	}
	if policy, err := FromTaskContext([]byte(`{"head_sha":"abc"}`)); err != nil || policy != nil {
		t.Fatalf("ordinary task context: policy=%#v err=%v", policy, err)
	}
	if _, err := FromTaskContext([]byte(`{"execution_policy":{"mode":"brain_mcp_read_only","provider":"claude","mcp_server":"brain","tools":["Bash"]}}`)); err == nil {
		t.Fatal("accepted malformed policy in task context")
	}
}

func TestFilterMCPConfigRetainsOnlySelectedConfiguredServerWithoutMutation(t *testing.T) {
	raw := json.RawMessage(`{"mcpServers":{"brain":{"type":"http","url":"https://brain.invalid/mcp"},"other":{"type":"http","url":"https://other.invalid/mcp"}},"globalSetting":true}`)
	before := append(json.RawMessage(nil), raw...)
	policy, err := NewBrainMCPReadOnly(Request{Mode: BrainMCPReadOnlyMode, MCPServer: "brain"})
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := FilterMCPConfig(raw, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw, before) {
		t.Fatalf("input config mutated: %s", raw)
	}
	var got struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(filtered, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.MCPServers) != 1 || len(got.MCPServers["brain"]) == 0 {
		t.Fatalf("filtered MCP servers = %s", filtered)
	}
}

func TestFilterMCPConfigFailsClosedWhenSelectedServerMissing(t *testing.T) {
	policy, err := NewBrainMCPReadOnly(Request{Mode: BrainMCPReadOnlyMode, MCPServer: "brain"})
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"mcpServers":{"other":{"type":"http","url":"https://other.invalid"}}}`),
	} {
		if _, err := FilterMCPConfig(config, policy); err == nil {
			t.Errorf("accepted config %s", config)
		}
	}
}
