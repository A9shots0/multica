package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatBindCarriesExplicitWorkerWithoutChangingRequest(t *testing.T) {
	const issue = "11111111-1111-1111-1111-111111111111"
	const worker = "22222222-2222-2222-2222-222222222222"
	const words = "Show me the layout first.\nKeep the actual columns."
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/chat/intake" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"state":"continued"}`))
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)
	t.Setenv("MULTICA_TOKEN", "mat_test-token")
	defer chatBindCmd.Flags().Set("request", "")
	defer chatBindCmd.Flags().Set("agent-id", "")
	_ = chatBindCmd.Flags().Set("request", words)
	_ = chatBindCmd.Flags().Set("agent-id", worker)
	if _, err := captureStdout(t, func() error { return chatBindCmd.RunE(chatBindCmd, []string{issue}) }); err != nil {
		t.Fatal(err)
	}
	if received["issue_id"] != issue || received["agent_id"] != worker || received["request"] != words {
		t.Fatalf("bind changed owner input or route: %+v", received)
	}
}
