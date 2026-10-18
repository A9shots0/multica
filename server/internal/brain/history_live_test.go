package brain

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLiveBrainHistoryAndEvidence(t *testing.T) {
	if os.Getenv("MULTICA_RUN_LIVE_BRAIN_HISTORY") != "1" {
		t.Skip("requires enrolled isolated service mapping")
	}
	c, err := Load(os.Getenv("MULTICA_BRAIN_CONFIG"))
	if err != nil {
		t.Fatal("protected service config unavailable")
	}
	project, ok := c.Project(os.Getenv("MULTICA_BRAIN_SMOKE_WORKSPACE"), os.Getenv("MULTICA_BRAIN_SMOKE_PROJECT"))
	if !ok {
		t.Fatal("fixture mapping unavailable")
	}
	taskID := os.Getenv("MULTICA_BRAIN_SMOKE_HISTORY_TASK")
	if taskID == "" {
		t.Fatal("exact fixture task required")
	}
	cursor := "0"
	seen := map[string]bool{}
	complete := false
	for page := 0; page < 20; page++ {
		raw, err := c.Events(context.Background(), project, taskID, cursor, 1)
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			Next string `json:"nextCursor"`
			More bool   `json:"hasMore"`
		}
		if json.Unmarshal(raw, &response) != nil {
			t.Fatal("invalid page")
		}
		for _, event := range response.Events {
			if seen[event.ID] {
				t.Fatal("duplicate event across pages")
			}
			seen[event.ID] = true
		}
		if !response.More {
			complete = true
			break
		}
		cursor = response.Next
	}
	if !complete || len(seen) < 3 {
		t.Fatalf("bounded fixture history incomplete: count=%d", len(seen))
	}
	id, revision := os.Getenv("MULTICA_BRAIN_SMOKE_EVIDENCE_ID"), os.Getenv("MULTICA_BRAIN_SMOKE_EVIDENCE_REVISION")
	if _, err := c.Evidence(context.Background(), project, id, revision); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Evidence(context.Background(), project, id, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong revision accepted")
	}
	t.Logf("Exact task history paginated without duplication: %d events; immutable evidence full read and revision rejection passed", len(seen))
}
