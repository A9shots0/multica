package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestBrainMCPUsesTaskBrokerAndExcludesGuardedRuns(t *testing.T) {
	upstreamCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path != "/api/daemon/tasks/task-fixture/brain" || r.Header.Get("Authorization") != "Bearer daemon-fixture" {
			t.Error("task/daemon authority changed")
		}
		var req struct {
			Operation string         `json:"operation"`
			Input     map[string]any `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Operation {
		case "context":
			if req.Input["q"] != "identity changes" {
				t.Error("query lost")
			}
		case "events":
			if req.Input["taskId"] != "prior-task" || req.Input["cursor"] != "12" {
				t.Error("history filter lost")
			}
		case "evidence":
			if req.Input["id"] != "evidence-id" || req.Input["revision"] != "immutable-revision" {
				t.Error("evidence reference lost")
			}
		default:
			t.Error("unexpected operation")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"project": map[string]string{"id": "crm"}, "assured": false})
	}))
	t.Cleanup(backend.Close)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	task := Task{ID: "task-fixture", RemoteMCPDaemonToken: "daemon-fixture", BrainContext: json.RawMessage(`{"lifecycle":"durable_source_bound"}`)}
	config, server, err := startTaskBrainMCP(ctx, task, NewClient(backend.URL), nil)
	if err != nil || server == nil {
		t.Fatalf("broker start: %v", err)
	}
	defer server.Close()
	var parsed struct {
		Servers map[string]struct {
			URL string `json:"url"`
		} `json:"mcpServers"`
	}
	_ = json.Unmarshal(config, &parsed)
	url := parsed.Servers["silver-mediums-brain"].URL
	if url == "" || strings.Contains(string(config), "daemon-fixture") {
		t.Fatal("missing broker or exposed daemon credential")
	}
	list, err := http.Post(url, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Result struct {
			Tools []struct {
				Name        string                `json:"name"`
				Annotations *localToolAnnotations `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeErr := json.NewDecoder(list.Body).Decode(&catalog)
	list.Body.Close()
	if decodeErr != nil || len(catalog.Result.Tools) != 5 {
		t.Fatal("missing scoped tool catalog")
	}
	for _, tool := range catalog.Result.Tools {
		if tool.Annotations == nil || tool.Annotations.ReadOnly != (tool.Name != "brain_record_event") || tool.Annotations.Destructive || tool.Annotations.OpenWorld || !tool.Annotations.Idempotent {
			t.Fatalf("incorrect tool semantics: %s", tool.Name)
		}
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"brain_context","arguments":{"q":"identity changes"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if json.NewDecoder(resp.Body).Decode(&result) != nil || upstreamCalls != 1 {
		t.Fatal("MCP did not reach scoped task broker")
	}
	if _, ok := result["result"]; !ok {
		t.Fatal("missing MCP result")
	}
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"brain_events","arguments":{"taskId":"prior-task","cursor":"12","limit":10}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"brain_evidence","arguments":{"id":"evidence-id","revision":"immutable-revision"}}}`,
	} {
		reply, err := http.Post(url, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var packet struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		err = json.NewDecoder(reply.Body).Decode(&packet)
		reply.Body.Close()
		if err != nil || packet.Result.IsError {
			t.Fatal("scoped history tool failed")
		}
	}
	if upstreamCalls != 3 {
		t.Fatal("history tools did not reach task broker")
	}
	task.GuardedExecution = &agentqualification.GuardedExecutionPackageV1{}
	if config, server, err = startTaskBrainMCP(ctx, task, NewClient(backend.URL), nil); err != nil || server != nil || len(config) > 0 {
		t.Fatal("guarded task gained ambient brain tools")
	}
}
