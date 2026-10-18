package taskcontinuity

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const (
	AdapterBegin = "<!-- multica-agent-continuity:begin -->"
	AdapterEnd   = "<!-- multica-agent-continuity:end -->"
)

var supportedProviders = map[string]string{
	"codex":   "Codex",
	"claude":  "Claude Code",
	"copilot": "GitHub Copilot",
}

// ContractHash identifies the exact provider-neutral instruction installed on
// a host. It lets supervisors compare installations without treating prose or
// a repository checkout as evidence of deployed state.
func ContractHash() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(RuntimeInstruction)))
}

// ProviderAdapter renders a bounded Markdown block suitable for AGENTS.md,
// CLAUDE.md, or Copilot's repository instructions. Provider names affect only
// the label; every adapter carries the exact same execution contract and hash.
func ProviderAdapter(provider string) (string, error) {
	label, ok := supportedProviders[provider]
	if !ok {
		return "", fmt.Errorf("unsupported continuity provider %q", provider)
	}
	return fmt.Sprintf(
		"%s\n## Execution continuity (%s)\n\nContract: `%s`\n\n%s\n%s",
		AdapterBegin,
		label,
		ContractHash(),
		RuntimeInstruction,
		AdapterEnd,
	), nil
}

// UpsertProviderAdapter replaces the one managed block or appends it without
// disturbing any user- or repository-authored instructions around it.
func UpsertProviderAdapter(existing, provider string) (string, error) {
	adapter, err := ProviderAdapter(provider)
	if err != nil {
		return "", err
	}
	begin := strings.Index(existing, AdapterBegin)
	end := strings.Index(existing, AdapterEnd)
	switch {
	case begin == -1 && end == -1:
		trimmed := strings.TrimRight(existing, "\r\n")
		if trimmed == "" {
			return adapter + "\n", nil
		}
		return trimmed + "\n\n" + adapter + "\n", nil
	case begin == -1 || end == -1 || end < begin:
		return "", fmt.Errorf("malformed managed continuity block")
	case strings.Contains(existing[begin+len(AdapterBegin):], AdapterBegin):
		return "", fmt.Errorf("multiple managed continuity blocks")
	case strings.Contains(existing[end+len(AdapterEnd):], AdapterEnd):
		return "", fmt.Errorf("multiple managed continuity blocks")
	default:
		end += len(AdapterEnd)
		return existing[:begin] + adapter + existing[end:], nil
	}
}

// InstalledContractMatches checks the managed block's contract hash and exact
// canonical prose while allowing provider-specific headings around them.
func InstalledContractMatches(content string) bool {
	begin := strings.Index(content, AdapterBegin)
	end := strings.Index(content, AdapterEnd)
	if begin == -1 || end < begin ||
		strings.Contains(content[begin+len(AdapterBegin):], AdapterBegin) ||
		strings.Contains(content[end+len(AdapterEnd):], AdapterEnd) {
		return false
	}
	block := content[begin : end+len(AdapterEnd)]
	return strings.Contains(block, ContractHash()) &&
		strings.Count(block, RuntimeInstruction) == 1
}
