package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// decodeDefaultMcpServers is a small local helper mirroring the pattern
// pkg/agent/browser_mcp_config_test.go uses to inspect an assembled
// {"mcpServers": {...}} document without pulling in that package's test
// helpers.
func decodeDefaultMcpServers(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	if len(raw) == 0 {
		return map[string]json.RawMessage{}
	}
	var top struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("unmarshal mcp config: %v\n%s", err, raw)
	}
	return top.McpServers
}

func TestApplyDefaultAgentMcpDefaultsNeverInventsBrowserPermission(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "opencode", "some-future-runtime"} {
		for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`{"mcpServers":{}}`)} {
			got, err := applyDefaultAgentMcpDefaults(provider, raw)
			if err != nil {
				t.Fatal(err)
			}
			servers := decodeDefaultMcpServers(t, got)
			if _, ok := servers["playwright"]; ok {
				t.Fatalf("%s gained an unauthorized browser: %s", provider, got)
			}
			_, fetch := servers["fetch"]
			if fetch == providersWithNativeWebTools[provider] {
				t.Fatalf("%s incorrect fetch capability: %s", provider, got)
			}
		}
	}
}

func TestApplyDefaultAgentMcpDefaultsRespectsOwnerBrowserServerByName(t *testing.T) {
	raw := json.RawMessage(`{"mcpServers":{"playwright":{"command":"node","args":["@playwright/mcp","--headless","--isolated"]}}}`)
	got, err := applyDefaultAgentMcpDefaults("claude", raw)
	if err != nil {
		t.Fatalf("applyDefaultAgentMcpDefaults: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("owner-supplied browser server changed:\n got  %s\n want %s", got, raw)
	}
}

func TestApplyDefaultAgentMcpDefaultsRespectsOwnerBrowserServerByShape(t *testing.T) {
	// A differently-named entry whose args still identify it as playwright
	// (matching hardenWindowsBrowserMcpConfig's own detection) must count as
	// "already configured" too, not just an entry literally named
	// "playwright".
	raw := json.RawMessage(`{"mcpServers":{"browser":{"command":"npx","args":["@playwright/mcp@latest","--headless"]}}}`)
	got, err := applyDefaultAgentMcpDefaults("claude", raw)
	if err != nil {
		t.Fatalf("applyDefaultAgentMcpDefaults: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("owner-supplied browser-shaped server changed:\n got  %s\n want %s", got, raw)
	}
}

func TestApplyDefaultAgentMcpDefaultsRespectsOwnerFetchServer(t *testing.T) {
	raw := json.RawMessage(`{"mcpServers":{"fetch":{"command":"uvx","args":["mcp-server-fetch","--ignore-robots-txt"]}}}`)
	got, err := applyDefaultAgentMcpDefaults("opencode", raw)
	if err != nil {
		t.Fatalf("applyDefaultAgentMcpDefaults: %v", err)
	}
	servers := decodeDefaultMcpServers(t, got)
	fetchRaw, ok := servers["fetch"]
	if !ok {
		t.Fatalf("owner fetch server dropped: %s", got)
	}
	var fetchEntry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(fetchRaw, &fetchEntry); err != nil {
		t.Fatalf("unmarshal fetch entry: %v", err)
	}
	if fetchEntry.Command != "uvx" || len(fetchEntry.Args) != 2 || fetchEntry.Args[1] != "--ignore-robots-txt" {
		t.Fatalf("owner fetch server overwritten: %s", fetchRaw)
	}
	// A configured fetch server must not implicitly authorize a browser.
	if _, ok := servers["playwright"]; ok {
		t.Fatalf("unexpected browser alongside owner's fetch server: %s", got)
	}
}

func TestApplyDefaultAgentMcpDefaultsPreservesUnrelatedOwnerServers(t *testing.T) {
	raw := json.RawMessage(`{"mcpServers":{"jira":{"command":"jira-mcp","args":["serve"]}}}`)
	got, err := applyDefaultAgentMcpDefaults("codex", raw)
	if err != nil {
		t.Fatalf("applyDefaultAgentMcpDefaults: %v", err)
	}
	servers := decodeDefaultMcpServers(t, got)
	if _, ok := servers["jira"]; !ok {
		t.Fatalf("unrelated owner server dropped: %s", got)
	}
	if _, ok := servers["playwright"]; ok {
		t.Fatalf("unexpected browser alongside owner's jira server: %s", got)
	}
}

func TestApplyDefaultAgentMcpDefaultsNoopWhenAlreadyComplete(t *testing.T) {
	raw := json.RawMessage(`{"mcpServers":{"playwright":{"command":"npx","args":["@playwright/mcp@latest"]}}}`)
	got, err := applyDefaultAgentMcpDefaults("claude", raw)
	if err != nil {
		t.Fatalf("applyDefaultAgentMcpDefaults: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("nothing to add should return the input unchanged:\n got  %s\n want %s", got, raw)
	}
}

func TestApplyDefaultAgentMcpDefaultsLeavesMalformedConfigUntouched(t *testing.T) {
	raw := json.RawMessage(`not json`)
	got, err := applyDefaultAgentMcpDefaults("claude", raw)
	if err != nil {
		t.Fatalf("applyDefaultAgentMcpDefaults: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("malformed config changed: got %s want %s", got, raw)
	}
}

// TestMcpConfigWithRuntimeDefaultsPullsNativeLocalConfigWhenUnmanaged verifies
// that when an agent has no owner-managed mcp_config at all (the common case
// this feature targets), the runtime's own local MCP servers are preserved
// as the base layer before the browser default is injected — so gaining a
// default browser server never silently drops an owner's own
// ~/.claude.json-configured servers the way handing Claude a bare
// {"mcpServers":{"playwright":...}} via --mcp-config would (see
// writeMcpConfigToTempRoot/claude.go: --mcp-config replaces inheritance).
func TestMcpConfigWithRuntimeDefaultsPullsNativeLocalConfigWhenUnmanaged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	claudeConfig := `{"mcpServers":{"existing-native":{"command":"native-server","args":["serve"]}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claudeConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	got := mcpConfigWithRuntimeDefaults("claude", nil, nil)
	servers := decodeDefaultMcpServers(t, got)
	if _, ok := servers["existing-native"]; !ok {
		t.Fatalf("expected native ~/.claude.json server to survive default injection: %s", got)
	}
	if _, ok := servers["playwright"]; ok {
		t.Fatalf("unexpected unconfigured browser: %s", got)
	}
	if _, ok := servers["fetch"]; ok {
		t.Fatalf("claude has a native web tool; should not get a fetch default: %s", got)
	}
}

// TestMcpConfigWithRuntimeDefaultsDoesNotMergeNativeConfigWhenAlreadyManaged
// verifies the inverse: once the agent (or a remote MCP broker) has already
// produced a managed config, the native-local merge step is skipped — the
// owner's managed set is authoritative, matching the existing "managed
// config replaces inheritance" contract for Claude/Codex/Cursor/OpenCode.
// No browser is added on top.
func TestMcpConfigWithRuntimeDefaultsDoesNotMergeNativeConfigWhenAlreadyManaged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	claudeConfig := `{"mcpServers":{"native-only":{"command":"native-server","args":["serve"]}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claudeConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	managed := json.RawMessage(`{"mcpServers":{"foo":{"command":"foo-server","args":["serve"]}}}`)
	got := mcpConfigWithRuntimeDefaults("claude", managed, nil)
	servers := decodeDefaultMcpServers(t, got)
	if _, ok := servers["foo"]; !ok {
		t.Fatalf("expected owner-managed server to survive: %s", got)
	}
	if _, ok := servers["native-only"]; ok {
		t.Fatalf("managed config must not silently absorb unrelated native-local servers: %s", got)
	}
	if _, ok := servers["playwright"]; ok {
		t.Fatalf("unexpected unconfigured browser: %s", got)
	}
}
