package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type providerReceipt struct {
	ResultText string
	Models     []string
	ResultLine []byte
}

func runProvider(ctx context.Context, executable, profileKind, profilePath, prompt string, output io.Writer) (returned providerReceipt, runErr error) {
	// Store only broker-owned error text and response metadata. No raw provider
	// diagnostics, credentials, prompts or reasoning are retained in this receipt.
	frames := []map[string]any{}
	defer func() {
		if taskID := os.Getenv("MULTICA_TASK_ID"); len(taskID) == 36 {
			record := map[string]any{"task_id": taskID, "frames": frames, "provider_result_seen": len(returned.ResultLine) > 0, "models": returned.Models}
			if runErr != nil {
				record["broker_error"] = runErr.Error()
			}
			raw, _ := json.MarshalIndent(record, "", "  ")
			_ = writeNew(filepath.Join(stateDirectory, taskID+".provider-receipt.json"), raw)
		}
	}()
	if strings.TrimSpace(executable) == "" {
		return providerReceipt{}, fmt.Errorf("provider executable is empty")
	}
	args := []string{
		"-p", "--safe-mode", "--restricted", "--tools", "",
		"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
		"--no-session-persistence", "--permission-prompts", "none",
		"--disable-slash-commands", "--no-chrome",
		"--output-format", "stream-json", "--input-format", "stream-json",
		"--verbose", "--max-turns", "1",
		"--model", exactClaudeModel, "--effort", exactReasoningEffort,
	}
	input, err := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": prompt}}},
	})
	if err != nil {
		return providerReceipt{}, err
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	workspace, cleanup, err := freshProviderWorkspace()
	if err != nil {
		return providerReceipt{}, err
	}
	defer cleanup()
	cmd.Dir = workspace
	childEnv, err := providerChildEnvironment(cmd.Dir, executable, profileKind, profilePath)
	if err != nil {
		return providerReceipt{}, err
	}
	cmd.Env = environmentList(childEnv)
	cmd.Stdin = bytes.NewReader(append(input, '\n'))
	cmd.Stderr = io.Discard
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return providerReceipt{}, err
	}
	if err := cmd.Start(); err != nil {
		return providerReceipt{}, err
	}

	receipt := providerReceipt{}
	models := map[string]struct{}{}
	var assistantText strings.Builder
	primaryAssistantSeen := false
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			_ = cmd.Process.Kill()
			return providerReceipt{}, fmt.Errorf("provider emitted non-JSON output")
		}
		frames = append(frames, safeFrameMetadata(event))
		assistantPart, primary, inspectErr := inspectProviderEvent(event, models)
		if inspectErr != nil {
			_ = cmd.Process.Kill()
			return providerReceipt{}, inspectErr
		}
		if primary {
			primaryAssistantSeen = true
			assistantText.WriteString(assistantPart)
			if assistantPart == "" {
				continue
			}
			// inspectProviderEvent removes non-text reasoning blocks. Forward only
			// the cleaned response to the native adapter.
			line, _ = json.Marshal(event)
		}
		if event["type"] == "result" {
			if len(receipt.ResultLine) != 0 {
				_ = cmd.Process.Kill()
				return providerReceipt{}, fmt.Errorf("provider emitted duplicate result frames")
			}
			result, _ := event["result"].(string)
			isError, _ := event["is_error"].(bool)
			if isError || strings.TrimSpace(result) == "" {
				return providerReceipt{}, fmt.Errorf("provider returned an error or empty result")
			}
			receipt.ResultText = result
			receipt.ResultLine = line
			continue
		}
		if _, err := output.Write(append(line, '\n')); err != nil {
			_ = cmd.Process.Kill()
			return providerReceipt{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()
		return providerReceipt{}, err
	}
	if err := cmd.Wait(); err != nil {
		return providerReceipt{}, fmt.Errorf("provider process failed")
	}
	if len(receipt.ResultLine) == 0 {
		return providerReceipt{}, fmt.Errorf("provider result frame is missing")
	}
	if !primaryAssistantSeen || strings.TrimSpace(assistantText.String()) != strings.TrimSpace(receipt.ResultText) {
		return providerReceipt{}, fmt.Errorf("result is not bound to one exact primary assistant response")
	}
	for model := range models {
		if model != exactClaudeModel && model != acceptedBackgroundModel {
			return providerReceipt{}, fmt.Errorf("unapproved provider model was observed")
		}
		receipt.Models = append(receipt.Models, model)
	}
	sort.Strings(receipt.Models)
	return receipt, nil
}

func freshProviderWorkspace() (string, func(), error) {
	parent, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", nil, err
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		return "", nil, err
	}
	root, err := os.MkdirTemp(parent, "silv38-provider-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		info, statErr := os.Lstat(root)
		resolved, resolveErr := filepath.EvalSymlinks(root)
		if statErr == nil && resolveErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && resolved == root && filepath.Dir(root) == parent && strings.HasPrefix(filepath.Base(root), "silv38-provider-") {
			_ = os.RemoveAll(root)
		}
	}
	// Keep fixture credentials and scratch files outside the actual working directory.
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	return workspace, cleanup, nil
}

func inspectProviderEvent(event map[string]any, models map[string]struct{}) (string, bool, error) {
	typeName, _ := event["type"].(string)
	switch typeName {
	case "system":
		if subtype, _ := event["subtype"].(string); subtype != "init" {
			return "", false, fmt.Errorf("provider emitted an unapproved system event")
		}
		if model, _ := event["model"].(string); model != "" && model != exactClaudeModel {
			return "", false, fmt.Errorf("provider initialized an unapproved primary model")
		}
		return "", false, nil
	case "rate_limit_event":
		return "", false, nil
	case "result":
		usage, ok := event["modelUsage"].(map[string]any)
		if !ok || len(usage) == 0 {
			return "", false, fmt.Errorf("provider result is missing model usage")
		}
		for model := range usage {
			models[model] = struct{}{}
		}
		return "", false, nil
	case "assistant":
	default:
		return "", false, fmt.Errorf("provider emitted an unapproved event type")
	}
	message, ok := event["message"].(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("provider assistant message is missing")
	}
	model, _ := message["model"].(string)
	if apiError, _ := event["is_api_error_message"].(bool); apiError {
		return "", false, fmt.Errorf("provider API failure: %s", safeAPIError(message))
	}
	if model != exactClaudeModel {
		return "", false, fmt.Errorf("provider assistant used the wrong primary model")
	}
	models[model] = struct{}{}
	content, ok := message["content"].([]any)
	if !ok || len(content) == 0 {
		return "", false, fmt.Errorf("provider assistant content is missing")
	}
	var textContent strings.Builder
	cleanContent := []any{}
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			return "", false, fmt.Errorf("provider assistant emitted an unapproved content block")
		}
		if block["type"] == "thinking" || block["type"] == "redacted_thinking" {
			continue
		}
		if block["type"] != "text" {
			return "", false, fmt.Errorf("provider assistant emitted an unapproved content block")
		}
		text, ok := block["text"].(string)
		if !ok || text == "" {
			return "", false, fmt.Errorf("provider assistant text is invalid")
		}
		textContent.WriteString(text)
		cleanContent = append(cleanContent, block)
	}
	message["content"] = cleanContent
	return textContent.String(), true, nil
}

func safeFrameMetadata(event map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"type", "subtype", "model", "is_error", "is_api_error_message", "api_error_status"} {
		if value, ok := event[key]; ok {
			result[key] = value
		}
	}
	if message, ok := event["message"].(map[string]any); ok {
		result["assistant_model"] = message["model"]
		kinds := []any{}
		if blocks, ok := message["content"].([]any); ok {
			for _, item := range blocks {
				if block, ok := item.(map[string]any); ok {
					kinds = append(kinds, block["type"])
				}
			}
		}
		result["content_types"] = kinds
		if value, ok := message["usage"]; ok {
			result["usage"] = value
		}
	}
	if value, ok := event["modelUsage"]; ok {
		result["model_usage"] = value
	}
	return result
}

func safeAPIError(message map[string]any) string {
	blocks, _ := message["content"].([]any)
	for _, item := range blocks {
		block, _ := item.(map[string]any)
		text, _ := block["text"].(string)
		text = strings.ToLower(text)
		for _, pair := range [][2]string{{"hit your limit", "usage_limit"}, {"rate limit", "rate_limit"}, {"401", "authentication"}, {"403", "access_denied"}, {"unauthorized", "authentication"}, {"connection refused", "connection_refused"}, {"model", "model_error"}} {
			if strings.Contains(text, pair[0]) {
				return pair[1]
			}
		}
	}
	return "unclassified_api_error"
}

func providerChildEnvironment(tempDir, executable, profileKind, profilePath string) (map[string]string, error) {
	home := ""
	switch profileKind {
	case "task_local_fixture":
		home = filepath.Join(filepath.Dir(tempDir), "fixture-profile")
	case "claude_code_subscription":
		if !filepath.IsAbs(profilePath) || filepath.Clean(profilePath) != profilePath {
			return nil, fmt.Errorf("Claude subscription profile path is not exact and absolute")
		}
		home = profilePath
	default:
		return nil, fmt.Errorf("provider profile kind is not controller-bound")
	}
	configDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return nil, err
	}
	if profileKind == "task_local_fixture" {
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return nil, err
		}
	}
	env := map[string]string{
		"HOME": home, "USERPROFILE": home,
		"APPDATA":      filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA": filepath.Join(home, "AppData", "Local"),
		"SYSTEMROOT":   os.Getenv("SYSTEMROOT"), "PATH": filepath.Dir(executable),
		"PWD": tempDir, "TEMP": tempDir, "TMP": tempDir, "TMPDIR": tempDir,
		"CLAUDE_CONFIG_DIR": configDir, "CLAUDE_CODE_TMPDIR": tempDir,
		"CLAUDE_CODE_MAX_RETRIES": "0", "CLAUDE_CODE_SAFE_MODE": "1",
	}
	return env, nil
}

func validateProviderExecutable(path, expectedSHA256 string) error {
	if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("provider executable path is not exact and absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("provider executable is not a non-symlink regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if expectedSHA256 != hex.EncodeToString(h.Sum(nil)) {
		return fmt.Errorf("provider executable digest does not match controller authority")
	}
	return nil
}

func environmentList(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func requireLoopbackServer() (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(os.Getenv("MULTICA_SERVER_URL")), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() != "8080" || u.Path != "" {
		return "", fmt.Errorf("pilot broker requires an explicit loopback Multica server")
	}
	return raw, nil
}

func fatal(stage string, err error) {
	fmt.Fprintln(os.Stderr, stage+": "+err.Error())
	os.Exit(1)
}
