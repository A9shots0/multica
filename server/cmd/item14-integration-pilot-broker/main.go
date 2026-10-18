package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	brokerVersion           = "2.1.271"
	exactClaudeModel        = "claude-sonnet-4-6"
	acceptedBackgroundModel = "claude-haiku-4-5-20251001"
	exactReasoningEffort    = "medium"
	leaderAgentName         = "Item 14 Replay Intake"
	executorAgentName       = "Item 14 Replay Executor"
	reviewerAgentName       = "Item 14 Replay Reviewer"
	leaderApproval          = "ITEM14_ROUTE_APPROVED"
	executorApproval        = "ITEM14_EXECUTION_APPROVED"
	reviewerPass            = "ITEM14_REVIEW_PASS"
	reviewerFail            = "ITEM14_REVIEW_FAIL"
	correctionApproval      = "ITEM14_CORRECTION_APPROVED"
	replayReportDirectory   = `C:\Dev\Silver-Mediums-Knowledge\initiatives\company-engineering-foundation\multica-integration-pilot-replay-20260914`
)

var authorizedReportDirectory = replayReportDirectory

type controlEnvelope struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Request   struct {
		Subtype string `json:"subtype"`
	} `json:"request"`
}

type providerReceipt struct {
	ResultText string
	Models     []string
	ResultLine []byte
}

type preparedRole struct {
	Inventory    *inventoryDocument
	Verification *verificationResult
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(brokerVersion)
		return
	}

	stdin := bufio.NewReader(os.Stdin)
	firstFrame, err := stdin.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		fatal("read first stdin frame", err)
	}
	if handleLocalControl(firstFrame) {
		return
	}

	role := strings.TrimSpace(os.Getenv("MULTICA_AGENT_NAME"))
	taskKind := strings.TrimSpace(os.Getenv("MULTICA_WORKFLOW_TASK_KIND"))
	prepared, safePrompt, err := prepareRole(role, taskKind)
	if err != nil {
		fatal("prepare exact pilot role", err)
	}
	if os.Getenv("ITEM14_BROKER_PREPARE_ONLY") == "1" {
		fmt.Printf("ITEM14_BROKER_PREPARED role=%s task_kind=%s provider_started=false\n", role, taskKind)
		return
	}

	providerPath := strings.TrimSpace(os.Getenv("ITEM14_PROVIDER_CHILD_PATH"))
	providerSHA256 := strings.TrimSpace(os.Getenv("ITEM14_PROVIDER_CHILD_SHA256"))
	profileKind := strings.TrimSpace(os.Getenv("ITEM14_PROVIDER_PROFILE_KIND"))
	profilePath := strings.TrimSpace(os.Getenv("ITEM14_PROVIDER_PROFILE_PATH"))
	if err := validateProviderExecutable(providerPath, providerSHA256); err != nil {
		fatal("validate exact provider child", err)
	}
	receipt, err := runProvider(context.Background(), providerPath, profileKind, profilePath, safePrompt, os.Stdout)
	if err != nil {
		fatal("run exact tool-free Claude child", err)
	}
	if err := performRoleAction(context.Background(), role, taskKind, receipt.ResultText, prepared); err != nil {
		// Preserve the provider result/usage frame, but exit non-zero so Multica
		// cannot record success when the broker action failed.
		_, _ = os.Stdout.Write(append(receipt.ResultLine, '\n'))
		fatal("apply exact pilot action", err)
	}
	if _, err := os.Stdout.Write(append(receipt.ResultLine, '\n')); err != nil {
		fatal("forward provider result", err)
	}
}

func handleLocalControl(firstFrame []byte) bool {
	var envelope controlEnvelope
	if json.Unmarshal(bytes.TrimSpace(firstFrame), &envelope) != nil ||
		envelope.Type != "control_request" || envelope.Request.Subtype != "list_models" {
		return false
	}
	requestID := envelope.RequestID
	if requestID == "" {
		requestID = "multica-list-models"
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype": "error", "request_id": requestID,
			"error": "Unsupported control request subtype: list_models",
		},
	})
	return true
}

func prepareRole(role, taskKind string) (preparedRole, string, error) {
	switch role {
	case leaderAgentName:
		if taskKind != "leader" {
			return preparedRole{}, "", fmt.Errorf("leader broker refuses successor task kind %q", taskKind)
		}
		return preparedRole{}, "Return exactly " + leaderApproval + ". This is a local routing receipt. Do not call tools, discuss alternatives, or add other text.", nil
	case executorAgentName:
		inventory, err := collectInventory(time.Now().UTC())
		if err != nil {
			return preparedRole{}, "", err
		}
		if taskKind == "correction" {
			return preparedRole{Inventory: &inventory}, fmt.Sprintf(
				"Return exactly %s. A deterministic parent broker has recollected %d authorized roots and %d complete worktree path checks. You have no tools or repository access.",
				correctionApproval, len(inventory.Roots), len(inventory.Worktrees)), nil
		}
		if taskKind != "implementation" {
			return preparedRole{}, "", fmt.Errorf("executor broker refuses task kind %q", taskKind)
		}
		return preparedRole{Inventory: &inventory}, fmt.Sprintf(
			"Return exactly %s. A deterministic parent broker observed %d authorized roots, %d worktree records, and completed %d of %d path checks. You have no tools or repository access.",
			executorApproval, len(inventory.Roots), len(inventory.Worktrees), inventory.Totals.CompletedPathChecks, inventory.Totals.RequiredPathChecks), nil
	case reviewerAgentName:
		if taskKind != "review" {
			return preparedRole{}, "", fmt.Errorf("reviewer broker requires review task, got %q", taskKind)
		}
		verification, err := verifyArtifacts(authorizedReportDirectory)
		if err != nil {
			verification = verificationResult{Pass: false, Summary: err.Error()}
		}
		return preparedRole{Verification: &verification}, fmt.Sprintf(
			"Independently classify this deterministic verification. Return exactly %s if it passes, otherwise exactly %s. No tools are available. Verification pass=%t; summary=%s",
			reviewerPass, reviewerFail, verification.Pass, verification.Summary), nil
	default:
		return preparedRole{}, "", fmt.Errorf("unrecognized temporary pilot role %q", role)
	}
}

func performRoleAction(ctx context.Context, role, taskKind, providerResult string, prepared preparedRole) error {
	result := strings.TrimSpace(providerResult)
	switch role {
	case leaderAgentName:
		if result != leaderApproval {
			return fmt.Errorf("leader receipt mismatch")
		}
		return routeToImplementer(ctx)
	case executorAgentName:
		if prepared.Inventory == nil {
			return fmt.Errorf("executor inventory is unavailable")
		}
		expected := executorApproval
		event := "implementation_ready"
		if taskKind == "correction" {
			expected = correctionApproval
			event = "correction_ready"
		}
		if result != expected {
			return fmt.Errorf("executor receipt mismatch")
		}
		evidence, err := writeArtifacts(authorizedReportDirectory, *prepared.Inventory, taskKind == "correction")
		if err != nil {
			return err
		}
		// Re-read, recompute current Git/path state, and rebuild the packet at
		// the last possible point before the controller event. A post-write
		// mutation therefore cannot ride stale hashes into workflow authority.
		finalVerification, err := verifyArtifacts(authorizedReportDirectory)
		if err != nil || !finalVerification.Pass {
			return fmt.Errorf("final artifact admission failed: %v (%s)", err, finalVerification.Summary)
		}
		inventoryRaw, err := os.ReadFile(filepath.Join(authorizedReportDirectory, "inventory.json"))
		if err != nil {
			return err
		}
		var finalInventory inventoryDocument
		if err := json.Unmarshal(inventoryRaw, &finalInventory); err != nil {
			return err
		}
		finalFiles := map[string][]byte{"inventory.json": inventoryRaw}
		for _, name := range []string{"WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", "EVIDENCE.md"} {
			finalFiles[name], err = os.ReadFile(filepath.Join(authorizedReportDirectory, name))
			if err != nil {
				return err
			}
		}
		evidence.Packet, err = buildArtifactEvidence(authorizedReportDirectory, finalInventory, finalFiles)
		if err != nil {
			return err
		}
		evidence.Reason = finalVerification.Summary
		body := map[string]any{"event": event, "reason": evidence.Reason}
		if event == "implementation_ready" {
			body["artifact_evidence"] = evidence.Packet
		}
		return postWorkflowEvent(ctx, body)
	case reviewerAgentName:
		if prepared.Verification == nil {
			return fmt.Errorf("review verification is unavailable")
		}
		if prepared.Verification.Pass && result == reviewerPass {
			return postWorkflowEvent(ctx, map[string]any{
				"event": "review_passed", "reason": prepared.Verification.Summary,
			})
		}
		if result != reviewerFail && result != reviewerPass {
			return fmt.Errorf("review receipt mismatch")
		}
		reason := prepared.Verification.Summary
		if prepared.Verification.Pass {
			reason = "provider reviewer declined an otherwise valid deterministic verification"
		}
		return postWorkflowEvent(ctx, map[string]any{
			"event": "review_failed", "reason": reason,
			"correction": map[string]any{
				"failed_criteria":     []string{"all frozen report and evidence checks must pass"},
				"evidence":            reason,
				"required_correction": "recollect the exact allowlist and replace only the three replay report artifacts",
				"candidate":           authorizedReportDirectory,
			},
		})
	default:
		return fmt.Errorf("unrecognized temporary pilot role")
	}
}

func runProvider(ctx context.Context, executable, profileKind, profilePath, prompt string, output io.Writer) (providerReceipt, error) {
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
	cmd.Dir = filepath.Join(os.TempDir(), "item14-broker-empty-workspace")
	if err := os.MkdirAll(cmd.Dir, 0o700); err != nil {
		return providerReceipt{}, err
	}
	childEnv, err := providerChildEnvironment(cmd.Dir, executable, profileKind, profilePath)
	if err != nil {
		return providerReceipt{}, err
	}
	cmd.Env = environmentList(childEnv)
	cmd.Stdin = bytes.NewReader(append(input, '\n'))
	cmd.Stderr = io.Discard
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
		assistantPart, primary, inspectErr := inspectProviderEvent(event, models)
		if inspectErr != nil {
			_ = cmd.Process.Kill()
			return providerReceipt{}, inspectErr
		}
		if primary {
			primaryAssistantSeen = true
			assistantText.WriteString(assistantPart)
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
	if model != exactClaudeModel {
		return "", false, fmt.Errorf("provider assistant used the wrong primary model")
	}
	models[model] = struct{}{}
	content, ok := message["content"].([]any)
	if !ok || len(content) == 0 {
		return "", false, fmt.Errorf("provider assistant content is missing")
	}
	var textContent strings.Builder
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok || block["type"] != "text" {
			return "", false, fmt.Errorf("provider assistant emitted an unapproved content block")
		}
		text, ok := block["text"].(string)
		if !ok || text == "" {
			return "", false, fmt.Errorf("provider assistant text is invalid")
		}
		textContent.WriteString(text)
	}
	return textContent.String(), true, nil
}

func providerChildEnvironment(tempDir, executable, profileKind, profilePath string) (map[string]string, error) {
	home := ""
	switch profileKind {
	case "task_local_fixture":
		home = filepath.Join(tempDir, "fixture-profile")
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
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "" {
		return "", fmt.Errorf("pilot broker requires an explicit loopback Multica server")
	}
	return raw, nil
}

func fatal(stage string, err error) {
	fmt.Fprintln(os.Stderr, stage+": "+err.Error())
	os.Exit(1)
}
