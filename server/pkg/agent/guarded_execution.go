package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	GuardedEnvironmentPolicyExactV1 = "exact_allowlist_v1"
	GuardedToolPolicyNoneV1         = "tool_free_v1"
)

// ResolveGuardedExecutablePath resolves without executing the provider. It
// returns the native file whose bytes will actually be launched, including
// OpenCode's Windows npm-shim handoff.
func ResolveGuardedExecutablePath(provider, configured string) (string, error) {
	if provider != "claude" && provider != "opencode" {
		return "", fmt.Errorf("guarded tool-free execution is unsupported for provider %q", provider)
	}
	if strings.TrimSpace(configured) == "" {
		configured = provider
	}
	resolved, err := exec.LookPath(configured)
	if err != nil {
		return "", fmt.Errorf("resolve guarded %s executable: %w", provider, err)
	}
	if runtime.GOOS == "windows" {
		switch provider {
		case "claude":
			if native := resolveClaudeNativeFromShim(resolved, os.Stat); native != "" {
				resolved = native
			}
		case "opencode":
			if native := resolveOpenCodeNativeFromShim(resolved, os.Stat); native != "" {
				resolved = native
			}
		}
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve guarded executable absolute path: %w", err)
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve guarded executable symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("guarded executable is not a regular file: %q", resolved)
	}
	return resolved, nil
}

// resolveClaudeNativeFromShim returns the native executable bundled by the
// global Claude Code npm package. Guarded execution must stage the native bytes
// themselves: copying only a .cmd/.ps1 launcher would leave its relative
// node_modules target outside the isolated executable directory.
func resolveClaudeNativeFromShim(shimPath string, statFn func(string) (os.FileInfo, error)) string {
	ext := strings.ToLower(filepath.Ext(shimPath))
	if ext != "" && ext != ".cmd" && ext != ".ps1" {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(shimPath), "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
	info, err := statFn(candidate)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return candidate
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c Config) childEnvironment() ([]string, error) {
	if c.EnvironmentPolicy == "" {
		return buildEnv(c.Env), nil
	}
	if c.EnvironmentPolicy != GuardedEnvironmentPolicyExactV1 {
		return nil, fmt.Errorf("unsupported child environment policy %q", c.EnvironmentPolicy)
	}
	expected := make(map[string]struct{}, len(c.AllowedEnvKeys))
	for _, key := range c.AllowedEnvKeys {
		if strings.TrimSpace(key) == "" || strings.Contains(key, "=") {
			return nil, fmt.Errorf("invalid guarded environment key %q", key)
		}
		if _, duplicate := expected[key]; duplicate {
			return nil, fmt.Errorf("duplicate guarded environment key %q", key)
		}
		expected[key] = struct{}{}
	}
	if len(expected) == 0 || len(c.Env) != len(expected) {
		return nil, fmt.Errorf("guarded child environment does not match exact allowlist")
	}
	if c.provider == "claude" && c.RequiredToolPolicy == GuardedToolPolicyNoneV1 && c.Env["CLAUDE_CODE_MAX_RETRIES"] != "0" {
		return nil, fmt.Errorf("guarded Claude execution must disable API-request retries")
	}
	keys := make([]string, 0, len(c.Env))
	for key, value := range c.Env {
		if _, ok := expected[key]; !ok || strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("guarded child environment contains unapproved key or value %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+c.Env[key])
	}
	return env, nil
}

func validateGuardedToolFreeOptions(cfg Config, opts ExecOptions) error {
	if cfg.RequiredToolPolicy == "" && opts.ToolPolicy == "" {
		return nil
	}
	if cfg.RequiredToolPolicy != GuardedToolPolicyNoneV1 || opts.ToolPolicy != GuardedToolPolicyNoneV1 {
		return fmt.Errorf("guarded execution requires the bound tool-free policy")
	}
	if strings.TrimSpace(opts.Model) == "" || strings.TrimSpace(opts.ThinkingLevel) == "" ||
		opts.MaxTurns != 1 || opts.ResumeSessionID != "" || opts.ResumeExpected ||
		len(opts.ExtraArgs) != 0 || len(opts.CustomArgs) != 0 ||
		opts.ServiceTier != "" || opts.ClaudeSettingsPath != "" {
		return fmt.Errorf("guarded tool-free execution contains unbound launch options")
	}
	return nil
}

func isEmptyManagedMCP(raw json.RawMessage) bool {
	var decoded struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || decoded.MCPServers == nil || len(decoded.MCPServers) != 0 {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

// GuardedOpenCodeConfigV1 returns the complete inline config paired with the
// opencode_run_json_tool_free_v1 adapter. Empty plugin, instruction, and MCP
// state plus global and agent-local deny-all tool policy are explicit.
func GuardedOpenCodeConfigV1(model, effort string) (string, error) {
	if strings.TrimSpace(model) == "" || strings.TrimSpace(effort) == "" {
		return "", fmt.Errorf("guarded OpenCode config requires exact model and effort")
	}
	config := map[string]any{
		"$schema":       "https://opencode.ai/config.json",
		"autoupdate":    false,
		"share":         "disabled",
		"snapshot":      false,
		"plugin":        []string{},
		"instructions":  []string{},
		"mcp":           map[string]any{},
		"permission":    "deny",
		"tools":         map[string]bool{"*": false},
		"compaction":    map[string]bool{"auto": false, "prune": false},
		"default_agent": "guarded",
		"agent": map[string]any{
			"guarded": map[string]any{
				"description": "Bound tool-free guarded execution",
				"mode":        "primary", "model": model, "variant": effort,
				"prompt": "Answer only from supplied task context. Do not use tools, subagents, files, network access, or prior sessions.",
				"steps":  1, "permission": "deny", "tools": map[string]bool{"*": false},
			},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
