package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultBrowserMcpServerName is the server name Multica injects when a
// dispatched task has no browser-capable MCP server configured. It
// deliberately matches the name hardenWindowsBrowserMcpConfig already
// recognizes (lowerName == "playwright"), so the Windows hardening below
// applies to the injected default exactly as it would to an owner-supplied
// playwright entry — there is no separate "default" code path to harden.
const DefaultBrowserMcpServerName = "playwright"

// DefaultFetchMcpServerName is the server name Multica injects, for runtimes
// with no native web-search/fetch tool of their own, when a dispatched task
// has neither an owner-supplied "fetch" server nor a remote-MCP equivalent.
const DefaultFetchMcpServerName = "fetch"

// DefaultBrowserMcpServerEntry returns the default, owner-config-shaped
// playwright MCP server entry. It is launched via `npx`, which needs Node/npm
// on the agent host but no API key or paid resource — the same zero-secret
// shape an owner would hand-configure. Args match what
// hardenWindowsBrowserMcpConfig already detects (@playwright/mcp) so the
// existing Windows hardening (headless launch config, Edge fallback probing)
// keeps applying unchanged.
func DefaultBrowserMcpServerEntry() map[string]any {
	return map[string]any{
		"command":                     "npx",
		"args":                        []any{"-y", "@playwright/mcp@latest"},
		"default_tools_approval_mode": "approve",
	}
}

// DefaultFetchMcpServerEntry returns a minimal, key-free web-fetch MCP server
// for runtimes that have neither a native web tool nor an owner-configured
// MCP entry (OpenCode and similar MCP-only runtimes). It reuses the
// `uvx mcp-server-fetch` shape already exercised elsewhere in this codebase's
// tests (see execenv/prepare_rollback_test.go) rather than inventing a new
// package; it needs `uv` on the agent host but no API key.
func DefaultFetchMcpServerEntry() map[string]any {
	return map[string]any{
		"command": "uvx",
		"args":    []any{"mcp-server-fetch"},
	}
}

// HasBrowserMcpServer reports whether raw's mcpServers already contains a
// playwright- or chrome-devtools-shaped entry, using the same name/args
// detection hardenWindowsBrowserMcpConfig uses to find servers to harden.
// A malformed or absent config reports false (nothing to detect), matching
// this file's existing "leave anything we can't understand alone" posture.
func HasBrowserMcpServer(raw json.RawMessage) bool {
	servers, ok := mcpServersMap(raw)
	if !ok {
		return false
	}
	for name, serverRaw := range servers {
		var entry map[string]any
		if err := json.Unmarshal(serverRaw, &entry); err != nil {
			continue
		}
		args, _ := stringSlice(entry["args"])
		lowerName := strings.ToLower(name)
		switch {
		case lowerName == "playwright" || lowerName == "chrome-devtools":
			return true
		case argsContain(args, "@playwright/mcp") || argsContain(args, `@playwright\mcp`):
			return true
		case argsContain(args, "chrome-devtools-mcp"):
			return true
		}
	}
	return false
}

// HasNamedMcpServer reports whether raw's mcpServers already has an entry
// with the given name (case-insensitive), so a default injection can skip a
// name an owner (or a remote MCP broker) already used.
func HasNamedMcpServer(raw json.RawMessage, name string) bool {
	servers, ok := mcpServersMap(raw)
	if !ok {
		return false
	}
	name = strings.ToLower(name)
	for existing := range servers {
		if strings.ToLower(existing) == name {
			return true
		}
	}
	return false
}

// mcpServersMap extracts the top-level mcpServers object from a raw MCP
// config document. Returns ok=false for empty/null input or anything that
// does not parse as {"mcpServers": {...}} — callers treat that as "nothing
// detected" rather than an error, mirroring hardenWindowsBrowserMcpConfig.
func mcpServersMap(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return nil, false
	}
	serversRaw, ok := top["mcpServers"]
	if !ok {
		return nil, false
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(serversRaw, &servers); err != nil {
		return nil, false
	}
	return servers, true
}

var (
	browserMcpGOOS = runtime.GOOS
	browserMcpStat = os.Stat
	browserMcpEnv  = os.Getenv
)

func hardenBrowserMcpConfig(raw json.RawMessage, tempDir string) ([]byte, error) {
	if browserMcpGOOS != "windows" {
		return raw, nil
	}
	return hardenWindowsBrowserMcpConfig(raw, tempDir)
}

func hardenWindowsBrowserMcpConfig(raw json.RawMessage, tempDir string) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return raw, nil
	}
	serversRaw, ok := top["mcpServers"]
	if !ok {
		return raw, nil
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(serversRaw, &servers); err != nil {
		return raw, nil
	}

	changed := false
	for name, serverRaw := range servers {
		var entry map[string]any
		if err := json.Unmarshal(serverRaw, &entry); err != nil {
			continue
		}
		args, ok := stringSlice(entry["args"])
		if !ok {
			continue
		}

		lowerName := strings.ToLower(name)
		switch {
		case lowerName == "playwright" || argsContain(args, "@playwright/mcp") || argsContain(args, `@playwright\mcp`):
			nextArgs, err := hardenWindowsPlaywrightMcpArgs(args, tempDir)
			if err != nil {
				return nil, err
			}
			if !sameStringSlice(args, nextArgs) {
				entry["args"] = nextArgs
				servers[name], changed = mustMarshalRaw(entry), true
			}
		case lowerName == "chrome-devtools" || argsContain(args, "chrome-devtools-mcp"):
			if path, ok := windowsChromiumFallbackExecutable(); ok && shouldPinChromeDevToolsExecutable(args) {
				entry["args"] = append(args, "--executablePath="+path)
				servers[name], changed = mustMarshalRaw(entry), true
			}
		}
	}
	if !changed {
		return raw, nil
	}

	top["mcpServers"] = mustMarshalRaw(servers)
	data, err := json.Marshal(top)
	if err != nil {
		return nil, fmt.Errorf("marshal hardened mcp config: %w", err)
	}
	return data, nil
}

func hardenWindowsPlaywrightMcpArgs(args []string, tempDir string) ([]string, error) {
	if hasFlag(args, "--config") || hasFlag(args, "--cdp-endpoint") || hasFlag(args, "--extension") {
		return args, nil
	}
	configPath := filepath.Join(tempDir, "playwright-windows-browser.json")
	config := map[string]any{
		"browser": map[string]any{
			"launchOptions": map[string]any{
				"args": []string{"--disable-gpu"},
			},
		},
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal playwright mcp browser config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write playwright mcp browser config: %w", err)
	}
	return append(args, "--config", configPath), nil
}

func windowsChromiumFallbackExecutable() (string, bool) {
	if path := strings.TrimSpace(browserMcpEnv("MULTICA_CHROME_DEVTOOLS_EXECUTABLE_PATH")); path != "" {
		return path, true
	}
	for _, root := range []string{
		browserMcpEnv("ProgramFiles(x86)"),
		browserMcpEnv("ProgramFiles"),
		browserMcpEnv("LocalAppData"),
	} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		path := windowsPathJoin(root, "Microsoft", "Edge", "Application", "msedge.exe")
		if _, err := browserMcpStat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func windowsPathJoin(root string, elems ...string) string {
	root = strings.TrimRight(root, `\/`)
	if root == "" {
		return ""
	}
	return root + `\` + strings.Join(elems, `\`)
}

func shouldPinChromeDevToolsExecutable(args []string) bool {
	for _, flag := range []string{
		"--executablePath",
		"--executable-path",
		"-e",
		"--channel",
		"--browserUrl",
		"--browser-url",
		"-u",
		"--wsEndpoint",
		"--ws-endpoint",
		"-w",
		"--autoConnect",
		"--auto-connect",
	} {
		if hasFlag(args, flag) {
			return false
		}
	}
	return true
}

func stringSlice(v any) ([]string, bool) {
	raw, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func argsContain(args []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, arg := range args {
		if strings.Contains(strings.ToLower(arg), needle) {
			return true
		}
	}
	return false
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustMarshalRaw(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
