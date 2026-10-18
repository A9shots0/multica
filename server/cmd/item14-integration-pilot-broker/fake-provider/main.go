package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const model = "claude-sonnet-4-6"

// launchMarker is empty in the frozen fixture. Provider-free integration tests
// may bind it at build time with -X so they can count exact child launches
// without adding an ambient environment key to the isolated child.
var launchMarker string
var transcriptMode string

func main() {
	if err := validateArgs(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, forbidden := range []string{"MULTICA_TOKEN", "MULTICA_SERVER_URL", "MULTICA_TASK_ID", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"} {
		if os.Getenv(forbidden) != "" {
			fmt.Fprintln(os.Stderr, "forbidden environment key was exposed")
			os.Exit(3)
		}
	}
	line, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "missing provider input")
		os.Exit(4)
	}
	var input struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &input) != nil || len(input.Message.Content) != 1 {
		fmt.Fprintln(os.Stderr, "invalid provider input")
		os.Exit(5)
	}
	prompt := input.Message.Content[0].Text
	result := ""
	for _, candidate := range []string{"ITEM14_ROUTE_APPROVED", "ITEM14_EXECUTION_APPROVED", "ITEM14_REVIEW_PASS", "ITEM14_REVIEW_FAIL", "ITEM14_CORRECTION_APPROVED"} {
		if strings.Contains(prompt, candidate) {
			result = candidate
			break
		}
	}
	if result == "" {
		fmt.Fprintln(os.Stderr, "unknown bounded prompt")
		os.Exit(6)
	}
	if launchMarker != "" {
		marker, err := os.OpenFile(launchMarker, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot record provider-free launch")
			os.Exit(7)
		}
		_, _ = fmt.Fprintln(marker, result)
		_ = marker.Close()
	}
	encoder := json.NewEncoder(os.Stdout)
	_ = encoder.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": "fake-item14-session"})
	if transcriptMode != "usage_only" {
		_ = encoder.Encode(map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"role": "assistant", "model": model,
				"content": []map[string]any{{"type": "text", "text": result}},
				"usage":   map[string]any{"input_tokens": 1, "output_tokens": 1},
			},
		})
	}
	_ = encoder.Encode(map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result": result, "session_id": "fake-item14-session", "num_turns": 1,
		"modelUsage": map[string]any{
			model:                       map[string]any{"inputTokens": 1, "outputTokens": 1},
			"claude-haiku-4-5-20251001": map[string]any{"inputTokens": 1, "outputTokens": 1},
		},
	})
}

func validateArgs(args []string) error {
	requiredPairs := map[string]string{
		"--tools": "", "--permission-prompts": "none", "--output-format": "stream-json",
		"--input-format": "stream-json", "--max-turns": "1", "--model": model, "--effort": "medium",
	}
	for flag, want := range requiredPairs {
		found := false
		for i := 0; i+1 < len(args); i++ {
			if args[i] == flag && args[i+1] == want {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing exact %s", flag)
		}
	}
	for _, flag := range []string{"--safe-mode", "--restricted", "--strict-mcp-config", "--no-session-persistence", "--disable-slash-commands", "--no-chrome"} {
		found := false
		for _, arg := range args {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing exact %s", flag)
		}
	}
	for _, arg := range args {
		if arg == "--fallback-model" || arg == "--dangerously-skip-permissions" || arg == "--permission-mode" {
			return fmt.Errorf("unsafe provider argument")
		}
	}
	return nil
}
