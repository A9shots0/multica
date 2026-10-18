package service

import (
	"strings"
	"testing"
)

func TestHistoricalWorkerPreambleDoesNotCertifyItsReport(t *testing.T) {
	const report = "The worker delivered a reviewable result.\n\nI cannot proceed on this runtime."
	got := ChatWorkBodyForDisplay(report, 4000)
	if strings.Contains(got, "reviewable result") || !strings.Contains(got, "cannot proceed") {
		t.Fatalf("historical report misrepresented: %q", got)
	}
}

func TestChatWorkSummaryKeepsConciseResultWithoutMachineReceipt(t *testing.T) {
	const answer = "Clients concept delivered. The list now leads; owner acceptance remains pending."
	output := answer + "\n<development-memory>" + strings.Repeat("x", 9000) + "</development-memory>"
	if got := chatWorkResultSummary(output, 4000); got != answer {
		t.Fatalf("concise task answer was lost: %q", got)
	}
	if got := ChatWorkBodyForDisplay(strings.Repeat("Detailed report. ", 600), 4000); got != longerChatWorkUpdateNotice || strings.Contains(got, "task completed") {
		t.Fatalf("display budget misrepresented task outcome: %q", got)
	}
}

func TestChatWorkSummaryRejectsRawDumpAndIncompleteReceipt(t *testing.T) {
	for _, output := range []string{
		strings.Repeat("tool call and process narration\n", 1000),
		"Result. <development-memory>unfinished",
		"<development-memory>{}</development-memory>",
	} {
		if got := chatWorkResultSummary(output, 4000); got != "" {
			t.Fatalf("unsafe or empty final answer reached compact result: %q", got)
		}
	}
}

func TestChatWorkIndexDoesNotTurnLongUpdateIntoCompletionFailure(t *testing.T) {
	if got := ChatWorkBodyForDisplay("The worker started.", 600); got != "The worker started." {
		t.Fatalf("short update changed: %q", got)
	}
	if got := ChatWorkBodyForDisplay(strings.Repeat("中", 601), 600); got != longerChatWorkUpdateNotice {
		t.Fatalf("long catch-up update claimed a different task state: %q", got)
	}
}

func TestChatWorkBodyHidesReceiptsBeforeApplyingDisplayBudget(t *testing.T) {
	const answer = "Two layouts are attached. Choose the composition that fits the work."
	for _, tag := range []string{"development-memory", "design-task-delivery", "design-review-verdict", "design-checkpoint"} {
		t.Run(tag, func(t *testing.T) {
			for _, size := range []int{20, 5000} {
				body := answer + "\n<" + tag + ">" + strings.Repeat("x", size) + "</" + tag + ">"
				if got := ChatWorkBodyForDisplay(body, 600); got != answer {
					t.Fatalf("receipt leaked or displaced the review text: %q", got)
				}
			}
		})
	}
	for _, body := range []string{"<design-checkpoint>{}</design-checkpoint>", "Visible text <design-checkpoint>unfinished"} {
		if got := ChatWorkBodyForDisplay(body, 600); got != longerChatWorkUpdateNotice {
			t.Fatalf("empty/incomplete receipt reached Chat: %q", got)
		}
	}
}
