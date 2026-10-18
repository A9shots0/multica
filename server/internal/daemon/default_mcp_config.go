package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/multica-ai/multica/server/pkg/agent"
)

// Runtimes without a native web tool receive a plain fetch default. A browser
// is a separate execution surface and requires explicit owner configuration.
var providersWithNativeWebTools = map[string]bool{
	"claude": true,
	"codex":  true,
}

// hasManagedMcpConfigValue mirrors pkg/agent's hasManagedMcpConfig three-state
// check (unexported there): only SQL NULL / JSON null mean "nothing managed
// yet"; any object, including `{}`, counts as a present, managed config.
// Duplicated here rather than exported across the package boundary for a
// two-line check used only by the default-injection path below.
func hasManagedMcpConfigValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

// applyDefaultAgentMcpDefaults supplies a plain fetch tool to runtimes that
// lack one. Never infer permission to launch a standalone browser from a
// missing MCP entry: the owner may require a different browser surface, such
// as an external supervising session's in-app Browser. An explicitly supplied
// browser server remains unchanged. No browser availability is promised here.
//
// Defaults only fill a gap, never override: an owner-supplied browser-shaped
// server (any name matching agent.HasBrowserMcpServer) or an owner-supplied
// "fetch" server is always left exactly as configured. A malformed raw config
// is returned unchanged so the next consumer (execenv, the provider CLI)
// surfaces the same parse error it would have without this step, rather than
// this step silently discarding the owner's config.
func applyDefaultAgentMcpDefaults(provider string, raw json.RawMessage) (json.RawMessage, error) {
	top := map[string]json.RawMessage{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		if err := json.Unmarshal(trimmed, &top); err != nil {
			return raw, nil
		}
	}

	servers := map[string]any{}
	if serversRaw, ok := top["mcpServers"]; ok {
		if err := json.Unmarshal(serversRaw, &servers); err != nil {
			return raw, nil
		}
	}

	changed := false
	if !providersWithNativeWebTools[provider] && !agent.HasNamedMcpServer(raw, agent.DefaultFetchMcpServerName) {
		servers[agent.DefaultFetchMcpServerName] = agent.DefaultFetchMcpServerEntry()
		changed = true
	}
	if !changed {
		return raw, nil
	}

	serversRaw, err := json.Marshal(servers)
	if err != nil {
		return nil, fmt.Errorf("marshal default mcp servers: %w", err)
	}
	top["mcpServers"] = serversRaw

	out, err := json.Marshal(top)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp config with defaults: %w", err)
	}
	return out, nil
}

// mcpConfigWithRuntimeDefaults resolves the final, task-local MCP config for
// dispatch: it starts from effectiveMcpConfig (the agent's own config already
// merged with any remote-MCP broker servers), and — only when that config is
// not already "managed" in the API's three-state sense (see
// hasManagedMcpConfigValue) — first folds in whatever this runtime already
// discovers locally (mergeRuntimeAndAgentMcpConfig with an empty managed
// object), so injecting a fetch default cannot silently drop an owner's own
// ~/.claude.json-style MCP servers the way passing a bare default straight to
// a "managed config replaces inheritance" runtime (Claude, Codex, Cursor,
// OpenCode, OpenClaw, CodeArts) would. It then adds the fetch default when needed.
//
// Skipped entirely for guarded/bounded tool-free execution, which requires an
// exact, owner-authorized MCP set (including "none at all") and must never
// gain extra servers — see claudeBackend.Execute's isEmptyManagedMCP check.
func mcpConfigWithRuntimeDefaults(provider string, effectiveMcpConfig json.RawMessage, taskLog *slog.Logger) json.RawMessage {
	base := effectiveMcpConfig
	if !hasManagedMcpConfigValue(base) {
		if withNative, err := mergeRuntimeAndAgentMcpConfig(provider, json.RawMessage(`{}`)); err != nil {
			if taskLog != nil {
				taskLog.Warn("mcp_config: could not load runtime-local servers for default injection base",
					"provider", provider, "error", err)
			}
		} else {
			base = withNative
		}
	}
	withDefaults, err := applyDefaultAgentMcpDefaults(provider, base)
	if err != nil {
		if taskLog != nil {
			taskLog.Warn("mcp_config: default fetch injection failed; dispatching without it",
				"provider", provider, "error", err)
		}
		return effectiveMcpConfig
	}
	return withDefaults
}
