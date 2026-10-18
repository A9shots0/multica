package service

import (
	"strings"
	"testing"
)

func TestDesignPracticeIsDeliveredToEveryOrdinaryAgentRole(t *testing.T) {
	for _, key := range []string{"", MikaSystemKey, "custom-specialist"} {
		if !named((&TaskService{}).BuiltinSkills(key, false), "multica-design") {
			t.Fatalf("design practice missing for agent system key %q", key)
		}
	}
	for _, notes := range []string{"", "Keep this workspace's existing product shell.\n"} {
		got := ComposeDesignPracticeInstructions(notes)
		if notes != "" && !strings.HasPrefix(got, strings.TrimRight(notes, "\n")+"\n\n") {
			t.Fatal("workspace agent instructions were overwritten")
		}
		if strings.Count(got, "## Product design practice") != 1 || !strings.Contains(got, "Non-UI work does not acquire design checkpoints") {
			t.Fatal("shared policy lost its conditional scope")
		}
	}
}

func TestDesignCheckpointReceiptDoesNotReplaceClientFacingAnswer(t *testing.T) {
	const answer = "Two wireframes are ready. I recommend A. Which direction should I develop?"
	if got := chatWorkResultSummary(answer+"\n<design-checkpoint>{\"stage\":\"wireframes\"}</design-checkpoint>", 4000); got != answer {
		t.Fatalf("checkpoint exposed machine receipt or lost the question: %q", got)
	}
	if got := chatWorkResultSummary("<design-checkpoint>unfinished", 4000); got != "" {
		t.Fatalf("incomplete checkpoint leaked into summary: %q", got)
	}
}
