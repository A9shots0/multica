package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestEventsBoundProjectTaskAndPagination(t *testing.T) {
	for _, scenario := range []string{"valid", "empty", "project", "task", "payload-project", "payload-task", "order", "cursor", "oversize", "missing-events", "empty-more"} {
		t.Run(scenario, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if q.Get("project") != "crm" || q.Get("taskId") != "prior-task" || q.Get("cursor") != "4" || q.Get("limit") != "1" {
					t.Error("lost scoped history query")
				}
				entry := map[string]any{"sequence": 5, "project": "crm", "task_id": "prior-task", "payload": map[string]any{"summary": "historical record"}}
				events := []any{entry}
				next, more := "5", false
				switch scenario {
				case "empty":
					events = []any{}
					next = "4"
				case "project":
					entry["project"] = "other"
				case "task":
					entry["task_id"] = "other"
				case "payload-project":
					entry["payload"] = map[string]any{"project": "other"}
				case "payload-task":
					entry["payload"] = map[string]any{"taskId": "other"}
				case "order":
					entry["sequence"] = 4
				case "cursor":
					next = "6"
				case "oversize":
					events = append(events, entry)
				case "missing-events":
					events = nil
				case "empty-more":
					events = []any{}
					next = "4"
					more = true
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "events": events, "nextCursor": next, "hasMore": more})
			})
			_, err := c.Events(context.Background(), "crm", "prior-task", "4", 1)
			if (err == nil) != (scenario == "valid" || scenario == "empty") {
				t.Fatalf("history validation: %v", err)
			}
		})
	}
}

func TestEvidenceRequiresExactProjectRevisionAndFullObservation(t *testing.T) {
	const id = "33333333-3333-4333-8333-333333333333"
	revision := strings.Repeat("a", 64)
	for _, scenario := range []string{"valid", "project", "revision", "id", "truncated", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v2/knowledge/evidence/"+id || r.URL.Query().Get("revision") != revision {
					t.Error("immutable evidence query lost")
				}
				record := map[string]any{"id": id, "revision": revision, "project": "crm", "observation": map[string]any{"available": false}}
				switch scenario {
				case "project":
					record["project"] = "other"
				case "revision":
					record["revision"] = strings.Repeat("b", 64)
				case "id":
					record["id"] = workspaceID
				case "truncated":
					record["truncated"] = true
				case "missing":
					delete(record, "observation")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"schemaVersion": "silvermediums.brain.v2", "servedAt": "now", "evidence": record})
			})
			_, err := c.Evidence(context.Background(), "crm", id, revision)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("evidence validation: %v", err)
			}
		})
	}
}

func TestHistoryRejectsUnboundedInputsBeforeNetwork(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid input reached service") })
	for _, cursor := range []string{"-1", "bad", "9223372036854775808"} {
		if _, err := c.Events(context.Background(), "crm", "", cursor, 20); err == nil {
			t.Fatal("invalid cursor accepted")
		}
	}
	for _, limit := range []int{0, 101} {
		if _, err := c.Events(context.Background(), "crm", "", "0", limit); err == nil {
			t.Fatal("invalid limit accepted")
		}
	}
	if _, err := c.Evidence(context.Background(), "crm", projectID, strings.Repeat("z", 64)); err == nil {
		t.Fatal("non-hash revision accepted")
	}
}
