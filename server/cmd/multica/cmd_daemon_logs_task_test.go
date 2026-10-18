package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// daemonLogsTaskCmdFor builds a cobra command carrying the full flag set
// runDaemonLogs reads (human + task source flags), wired to buffers so the
// command's stdout and stderr are observable.
func daemonLogsTaskCmdFor(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := daemonStatusCmdFor(t, "", "")
	cmd.Flags().BoolP("follow", "f", false, "")
	cmd.Flags().IntP("lines", "n", 50, "")
	cmd.Flags().String("source", "daemon", "")
	cmd.Flags().String("container", "", "")
	cmd.Flags().String("since", "", "")
	cmd.Flags().Bool("include-verification-code", false, "")
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	return cmd, &out, &errOut
}

func setDaemonTaskEnv(t *testing.T, port string) {
	t.Helper()
	t.Setenv("MULTICA_AGENT_ID", "agent_logs_test")
	t.Setenv("MULTICA_TASK_ID", "task_logs_test")
	t.Setenv("MULTICA_WORKSPACE_ID", "ws_logs_test")
	t.Setenv("MULTICA_DAEMON_PORT", port)
	t.Setenv("MULTICA_TOKEN", "task-token-logs-test")
}

// tcpPort extracts the port of a loopback httptest server for injection as
// MULTICA_DAEMON_PORT.
func tcpPort(t *testing.T, server *httptest.Server) string {
	t.Helper()
	port := server.Listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("%d", port)
}

func TestDaemonLogsTaskModeReadsThroughBroker(t *testing.T) {
	var gotAuth, gotTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTarget = r.URL.RequestURI()
		q := r.URL.Query()
		if q.Get("workspace_id") != "ws_logs_test" || q.Get("task_id") != "task_logs_test" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"error":"context mismatch"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"source":       "container",
			"container":    "multica-backend-1",
			"workspace_id": "ws_logs_test",
			"task_id":      "task_logs_test",
			"server_time":  "2026-09-19T22:00:00Z",
			"truncated":    false,
			"lines":        []string{"INF backend ready", "WRN late sync"},
			"verification_code": map[string]any{
				"email":              "bob@example.com",
				"line":               "[DEV] Verification code for bob@example.com: 987654",
				"age_seconds":        120,
				"expires_in_seconds": 480,
				"expired":            false,
			},
		})
	}))
	t.Cleanup(server.Close)
	setDaemonTaskEnv(t, tcpPort(t, server))

	cmd, _, errOut := daemonLogsTaskCmdFor(t)
	if err := cmd.Flags().Set("source", "container"); err != nil {
		t.Fatalf("set source: %v", err)
	}
	if err := cmd.Flags().Set("container", "multica-backend-1"); err != nil {
		t.Fatalf("set container: %v", err)
	}
	if err := cmd.Flags().Set("lines", "25"); err != nil {
		t.Fatalf("set lines: %v", err)
	}
	if err := cmd.Flags().Set("include-verification-code", "true"); err != nil {
		t.Fatalf("set include-verification-code: %v", err)
	}

	gotStdout, err := captureStdout(t, func() error { return runDaemonLogsInTask(cmd) })
	if err != nil {
		t.Fatalf("runDaemonLogsInTask = %v, want nil", err)
	}

	if gotAuth != "Bearer task-token-logs-test" {
		t.Errorf("Authorization = %q, want the task token", gotAuth)
	}
	if !strings.Contains(gotTarget, "/log/read") {
		t.Errorf("target = %q, want /log/read on the daemon port", gotTarget)
	}
	if !strings.Contains(gotTarget, "lines=25") {
		t.Errorf("target = %q, want lines=25 forwarded", gotTarget)
	}
	for _, line := range []string{"INF backend ready", "WRN late sync"} {
		if !strings.Contains(gotStdout, line) {
			t.Errorf("stdout missing %q:\n%s", line, gotStdout)
		}
	}
	if strings.Contains(gotStdout, "987654") {
		t.Errorf("stdout should not echo the raw code line (it goes to stderr):\n%s", gotStdout)
	}
	if !strings.Contains(errOut.String(), "expires in 480s") {
		t.Errorf("stderr missing the code-expiry line:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Reading container log") {
		t.Errorf("stderr missing the source announcement:\n%s", errOut.String())
	}
}

func TestDaemonLogsTaskModeSendContextSoBrokerCanReject(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"log read task context does not match the active task"}`)
	}))
	t.Cleanup(server.Close)
	setDaemonTaskEnv(t, tcpPort(t, server))

	cmd, _, _ := daemonLogsTaskCmdFor(t)
	err := runDaemonLogsInTask(cmd)
	if err == nil {
		t.Fatal("runDaemonLogsInTask = nil, want the broker rejection surfaced")
	}
	if !strings.Contains(err.Error(), "does not match the active task") {
		t.Errorf("error = %v, want the daemon's message", err)
	}
	if gotAuth != "Bearer task-token-logs-test" {
		t.Errorf("Authorization = %q, want the task token even on failure", gotAuth)
	}
}

func TestDaemonLogsTaskModeRejectsUnavailableFlags(t *testing.T) {
	t.Run("profile", func(t *testing.T) {
		setDaemonTaskEnv(t, "1234")
		cmd, _, _ := daemonLogsTaskCmdFor(t)
		if err := cmd.Flags().Set("profile", "prod"); err != nil {
			t.Fatalf("set profile: %v", err)
		}
		if err := runDaemonLogsInTask(cmd); err == nil || !strings.Contains(err.Error(), "--profile") {
			t.Fatalf("runDaemonLogsInTask = %v, want a --profile rejection", err)
		}
	})
	t.Run("follow", func(t *testing.T) {
		setDaemonTaskEnv(t, "1234")
		cmd, _, _ := daemonLogsTaskCmdFor(t)
		if err := cmd.Flags().Set("follow", "true"); err != nil {
			t.Fatalf("set follow: %v", err)
		}
		if err := runDaemonLogsInTask(cmd); err == nil || !strings.Contains(err.Error(), "--follow") {
			t.Fatalf("runDaemonLogsInTask = %v, want a --follow rejection", err)
		}
	})
}

func TestDaemonLogsTaskModeFailsClosedWithoutEnv(t *testing.T) {
	clearDaemonTaskEnv(t)
	t.Setenv("MULTICA_AGENT_ID", "agent_logs_test")
	t.Setenv("MULTICA_TASK_ID", "task_logs_test")

	cmd, _, _ := daemonLogsTaskCmdFor(t)
	if err := runDaemonLogsInTask(cmd); err == nil || !strings.Contains(err.Error(), "MULTICA_DAEMON_PORT") {
		t.Fatalf("runDaemonLogsInTask = %v, want a MULTICA_DAEMON_PORT failure", err)
	}

	t.Setenv("MULTICA_DAEMON_PORT", "1234")
	if err := runDaemonLogsInTask(cmd); err == nil || !strings.Contains(err.Error(), "MULTICA_TOKEN") {
		t.Fatalf("runDaemonLogsInTask = %v, want a MULTICA_TOKEN failure", err)
	}
}

// The source-specific flags exist for the in-task broker path. Outside a task
// they must be rejected before profile/file resolution, so a human who types
// them gets the why instead of an unrelated missing-profile error. The test
// chdirs out of the daemon task's own repo tree: that workdir carries a daemon
// task-context marker, which would wrongly put the command in task mode.
func TestDaemonLogsHumanModeRejectsTaskOnlyFlags(t *testing.T) {
	t.Chdir(t.TempDir())
	clearDaemonTaskEnv(t)
	cases := []struct {
		name  string
		setup func(*cobra.Command, *bytes.Buffer, *bytes.Buffer) error
	}{
		{"source container", func(c *cobra.Command, _, _ *bytes.Buffer) error { return c.Flags().Set("source", "container") }},
		{"container", func(c *cobra.Command, _, _ *bytes.Buffer) error {
			return c.Flags().Set("container", "multica-backend-1")
		}},
		{"since", func(c *cobra.Command, _, _ *bytes.Buffer) error {
			return c.Flags().Set("since", "2026-09-19T12:00:00Z")
		}},
		{"include-verification-code", func(c *cobra.Command, _, _ *bytes.Buffer) error {
			return c.Flags().Set("include-verification-code", "true")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearDaemonTaskEnv(t)
			cmd, _, _ := daemonLogsTaskCmdFor(t)
			if err := tc.setup(cmd, nil, nil); err != nil {
				t.Fatalf("set flag: %v", err)
			}
			err := runDaemonLogs(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), "only available inside a daemon-managed task") {
				t.Fatalf("runDaemonLogs = %v, want the task-only flag rejection", err)
			}
		})
	}
}
