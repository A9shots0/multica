package service

import (
	"strings"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/pkg/redact"
)

const longerChatWorkUpdateNotice = "A longer worker update is available in the attached artifacts or linked issue."

// A compact work index is not the raw-stream completion fallback. Exceeding
// its smaller display budget does not mean that a task's final output failed.
func ChatWorkBodyForDisplay(body string, maxRunes int) string {
	body = chatWorkTextWithoutReceipts(body)
	// Older delivery receipts inferred successful work from any comment.
	// Correct the presentation without rewriting their original evidence.
	for _, pair := range [][2]string{
		{"The worker delivered a reviewable result.", "The worker left this report."},
		{"The worker delivered a reviewable result before the run was stopped.", "The worker left this report before the run was stopped."},
		{"The worker delivered a reviewable result before the run ended with an error.", "The worker left this report before the run ended with an error."},
	} {
		body = strings.ReplaceAll(body, pair[0], pair[1])
	}
	if body != "" && utf8.RuneCountInString(body) <= maxRunes {
		return body
	}
	return longerChatWorkUpdateNotice
}

// Prefer the exact completed task's concise final answer when its deliberately
// detailed issue comment exceeds Chat's budget. Never clip an execution dump.
// Machine receipts remain intact in task storage and their existing gates.
func chatWorkResultSummary(output string, maxRunes int) string {
	output = chatWorkTextWithoutReceipts(output)
	if output == "" || utf8.RuneCountInString(output) > maxRunes {
		return ""
	}
	return output
}

// This is a presentation filter only. Stored comments, task results and the
// machine receipts used by completion gates must retain their original bytes.
func chatWorkTextWithoutReceipts(output string) string {
	for _, tag := range []string{"development-memory", "design-task-delivery", "design-review-verdict", "design-checkpoint"} {
		open, close := "<"+tag+">", "</"+tag+">"
		for {
			start := strings.Index(output, open)
			if start < 0 {
				break
			}
			end := strings.Index(output[start+len(open):], close)
			if end < 0 {
				return ""
			}
			output = output[:start] + output[start+len(open)+end+len(close):]
		}
	}
	return strings.TrimSpace(redact.Text(output))
}
