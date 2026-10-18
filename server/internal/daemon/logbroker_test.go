package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/dockerlog"
)

const (
	logBrokerTestWorkspace = "ws_log_broker_test"
	logBrokerTestTask      = "task_log_broker_test"
	logBrokerTestToken     = "log-broker-test-token"
)

// stubContainerLogs is a containerLogReader the broker tests inject in place
// of a real Docker Engine client.
type stubContainerLogs struct {
	lines        []dockerlog.Line
	err          error
	gotContainer string
	gotTail      int
	gotSince     time.Time
}

func (s *stubContainerLogs) ReadLogs(ctx context.Context, container string, tail int, since time.Time) ([]dockerlog.Line, error) {
	s.gotContainer = container
	s.gotTail = tail
	s.gotSince = since
	if s.err != nil {
		return nil, s.err
	}
	return s.lines, nil
}

func newLogBrokerTestDaemon(t *testing.T, clogs containerLogReader) *Daemon {
	t.Helper()
	d := &Daemon{
		cfg: Config{
			CLIVersion: "v0.4.35-test",
		},
		logger:        slog.Default(),
		containerLogs: clogs,
	}
	d.registerActiveTaskCredential(logBrokerTestToken, activeTaskCredential{
		WorkspaceID: logBrokerTestWorkspace,
		TaskID:      logBrokerTestTask,
		AgentID:     "agent_test",
		AgentName:   "Builder",
		WorkDir:     t.TempDir(),
	})
	t.Cleanup(func() { d.clearActiveTaskCredential(logBrokerTestToken) })
	return d
}

func serveLogRead(t *testing.T, d *Daemon, query string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/log/read"
	if query != "" {
		target += "?" + query
	}
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.Header.Set("Authorization", "Bearer "+logBrokerTestToken)
	w := httptest.NewRecorder()
	d.logReadHandler()(w, r)
	return w
}

func decodeLogRead(t *testing.T, w *httptest.ResponseRecorder) logReadResponse {
	t.Helper()
	var resp logReadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", w.Body.String(), err)
	}
	return resp
}

func TestLogReadRequiresCredential(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil)
	r := httptest.NewRequest(http.MethodGet, "/log/read?workspace_id=w&task_id=t", nil)
	w := httptest.NewRecorder()
	d.logReadHandler()(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "credential") {
		t.Errorf("body = %q, want a credential explanation", w.Body.String())
	}
	if strings.Contains(w.Body.String(), logBrokerTestToken) {
		t.Errorf("body leaked the token: %q", w.Body.String())
	}
}

func TestLogReadRejectsUnknownCredential(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil)
	r := httptest.NewRequest(http.MethodGet, "/log/read?workspace_id=w&task_id=t", nil)
	r.Header.Set("Authorization", "Bearer not-registered-token")
	w := httptest.NewRecorder()
	d.logReadHandler()(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not bound to a task") {
		t.Errorf("body = %q, want the unknown-credential explanation", w.Body.String())
	}
}

func TestLogReadRejectsTaskContextMismatch(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil)
	w := serveLogRead(t, d, "workspace_id=someone_else&task_id="+logBrokerTestTask)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestLogReadDaemonSourceScopesAndRedacts(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "daemon.log")
	matchingTask := "task=" + logBrokerTestTask
	matchingWS := "workspace_id=" + logBrokerTestWorkspace
	secret := "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefgh"
	content := []string{
		"2026-09-19T12:00:01Z INF begin " + matchingTask,
		"2026-09-19T12:00:02Z INF sync " + matchingWS,
		"2026-09-19T12:00:03Z INF auth " + matchingTask + " password=supersecretk3y",
		"2026-09-19T12:00:04Z INF creds " + matchingTask + " " + secret,
		"2026-09-19T12:00:05Z DEV task_id=" + logBrokerTestTask + " [DEV] Verification code for alice@example.com: 123456",
		"2026-09-19T12:00:06Z INF unrelated workspace_id=ws_another_workspace ignored",
		"2026-09-19T12:00:07Z INF no marker at all",
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(content, "\n")), 0o600); err != nil {
		t.Fatalf("write daemon.log: %v", err)
	}

	d := newLogBrokerTestDaemon(t, nil)
	d.cfg.LogPath = logPath
	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&lines=50")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeLogRead(t, w)

	if len(resp.Lines) != 5 {
		t.Fatalf("got %d lines, want the 5 matching lines:\n%v", len(resp.Lines), resp.Lines)
	}
	if resp.Truncated {
		t.Error("Truncated = true, want false")
	}
	if resp.WorkspaceID != logBrokerTestWorkspace || resp.TaskID != logBrokerTestTask {
		t.Errorf("identity = %s/%s, want the authoritative task", resp.WorkspaceID, resp.TaskID)
	}
	joined := strings.Join(resp.Lines, "\n")
	if strings.Contains(joined, secret) {
		t.Errorf("lines leaked the token: %v", resp.Lines)
	}
	if strings.Contains(joined, "supersecretk3y") {
		t.Errorf("lines leaked the password: %v", resp.Lines)
	}
	if strings.Contains(joined, "123456") {
		t.Errorf("lines leaked the verification code: %v", resp.Lines)
	}
	if !strings.Contains(joined, "[REDACTED VERIFICATION CODE]") {
		t.Errorf("verification-code line not redacted visibly: %v", resp.Lines)
	}
	if !strings.Contains(joined, "alice@example.com") {
		t.Errorf("the email that proves a code was issued disappeared: %v", resp.Lines)
	}
	if strings.Contains(joined, "ws_another_workspace") {
		t.Errorf("another workspace's line leaked out: %v", resp.Lines)
	}
	if resp.VerificationCode != nil {
		t.Errorf("verification_code present without include flag: %+v", resp.VerificationCode)
	}
}

func TestLogReadDaemonSourceTruncation(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	var content []string
	for i := 0; i < 60; i++ {
		content = append(content, "INF line task="+logBrokerTestTask+" seq="+strings.Repeat("a", i%3))
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(content, "\n")), 0o600); err != nil {
		t.Fatalf("write daemon.log: %v", err)
	}
	d := newLogBrokerTestDaemon(t, nil)
	d.cfg.LogPath = logPath

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&lines=10")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeLogRead(t, w)
	if !resp.Truncated {
		t.Error("Truncated = false, want true")
	}
	if len(resp.Lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(resp.Lines))
	}
	if !strings.HasSuffix(resp.Lines[9], "seq=aa") {
		t.Errorf("newest line not last: %v", resp.Lines[9])
	}
}

func TestLogReadDaemonSourceUnavailable(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil) // LogPath empty by default
	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestLogReadContainerSource(t *testing.T) {
	secret := "AKIAIOSFODNN7EXAMPLE"
	stub := &stubContainerLogs{lines: []dockerlog.Line{
		{Timestamp: time.Now().Add(-time.Minute), Source: "stdout", Content: "INF backend ready\n"},
		{Timestamp: time.Now().Add(-30 * time.Second), Source: "stderr", Content: "WRN key " + secret + "\n"},
	}}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"multica-backend-1"}

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=multica-backend-1&lines=50")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeLogRead(t, w)

	if resp.Container != "multica-backend-1" {
		t.Errorf("container = %q, want the request echoed", resp.Container)
	}
	if stub.gotContainer != "multica-backend-1" || stub.gotTail != 50 {
		t.Errorf("reader got container %q tail %d, want multica-backend-1 / 50", stub.gotContainer, stub.gotTail)
	}
	joined := strings.Join(resp.Lines, "\n")
	if strings.Contains(joined, secret) {
		t.Errorf("lines leaked the secret: %v", resp.Lines)
	}
	if len(resp.Lines) != 2 {
		t.Errorf("got %d lines, want 2", len(resp.Lines))
	}
}

func TestLogReadContainerForwardsSince(t *testing.T) {
	since := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	stub := &stubContainerLogs{}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"backend-1"}

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=backend-1&since=2026-09-19T12:00:00Z")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !stub.gotSince.Equal(since) {
		t.Errorf("reader since = %v, want %v", stub.gotSince, since)
	}
}

func TestLogReadContainerDisabledAndNotAllowed(t *testing.T) {
	stub := &stubContainerLogs{}
	d := newLogBrokerTestDaemon(t, stub) // allow-list empty
	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=multica-backend-1")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled: status = %d, want 503", w.Code)
	}

	d2 := newLogBrokerTestDaemon(t, stub)
	d2.cfg.LogBrokerContainers = []string{"multica-backend"}
	w = serveLogRead(t, d2, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=crm-shared")
	if w.Code != http.StatusForbidden {
		t.Fatalf("not allowed: status = %d, want 403", w.Code)
	}

	w = serveLogRead(t, d2, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=other-multica-backend")
	if w.Code != http.StatusForbidden {
		t.Fatalf("substring match: status = %d, want 403", w.Code)
	}

	w = serveLogRead(t, d2, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=Multica-Backend")
	if w.Code != http.StatusForbidden {
		t.Fatalf("case-folded match: status = %d, want 403", w.Code)
	}
}

func TestLogReadContainerSurfacesEngineErrors(t *testing.T) {
	stub := &stubContainerLogs{err: errors.New("docker engine: Not Found: No such container: nope")}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"nope"}

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=nope")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No such container") {
		t.Errorf("body = %q, want the engine message surfaced", w.Body.String())
	}
}

func TestLogReadIncludeVerificationCode(t *testing.T) {
	stub := &stubContainerLogs{lines: []dockerlog.Line{
		{Timestamp: time.Now().Add(-10 * time.Minute), Source: "stdout", Content: "[DEV] Verification code for old@example.com: 111222\n"},
		{Timestamp: time.Now().Add(-2 * time.Minute), Source: "stdout", Content: "token=super-secret [DEV] Verification code for bob@example.com: 987654\n"},
	}}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"backend-1"}

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=backend-1&include_verification_code=1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeLogRead(t, w)

	if resp.VerificationCode == nil {
		t.Fatal("verification_code = nil, want the newest code")
	}
	vc := resp.VerificationCode
	if vc.Email != "bob@example.com" {
		t.Errorf("email = %q, want the newest line's", vc.Email)
	}
	if !strings.Contains(vc.Line, "987654") {
		t.Errorf("code line = %q, want the digits", vc.Line)
	}
	if strings.Contains(vc.Line, "super-secret") || !strings.Contains(vc.Line, "[REDACTED CREDENTIAL]") {
		t.Errorf("code line = %q, want unrelated credentials redacted", vc.Line)
	}
	if vc.Expired {
		t.Error("Expired = true, want false for a fresh code")
	}
	if vc.ExpiresInSeconds <= 7*60 || vc.ExpiresInSeconds > 8*60 {
		t.Errorf("ExpiresInSeconds = %d, want ~8 minutes", vc.ExpiresInSeconds)
	}

	// The general line stream stays deny-first even with the carve-out asked.
	for _, l := range resp.Lines {
		if strings.Contains(l, "987654") || strings.Contains(l, "111222") {
			t.Errorf("line stream still leaked a code: %q", l)
		}
	}
}

func TestLogReadVerificationCodeExpired(t *testing.T) {
	stub := &stubContainerLogs{lines: []dockerlog.Line{
		{Timestamp: time.Now().Add(-11 * time.Minute), Source: "stdout", Content: "[DEV] Verification code for late@example.com: 555444\n"},
	}}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"backend-1"}

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=backend-1&include_verification_code=1")
	resp := decodeLogRead(t, w)
	if resp.VerificationCode == nil {
		t.Fatal("verification_code = nil, want an expired marker")
	}
	if !resp.VerificationCode.Expired {
		t.Error("Expired = false, want true for a stale code")
	}
	if resp.VerificationCode.ExpiresInSeconds != 0 {
		t.Errorf("ExpiresInSeconds = %d, want 0", resp.VerificationCode.ExpiresInSeconds)
	}
}

func TestLogReadVerificationCodeSearchUsesBoundedMaxWindow(t *testing.T) {
	lines := []dockerlog.Line{{
		Timestamp: time.Now().Add(-2 * time.Minute),
		Source:    "stdout",
		Content:   "[DEV] Verification code for busy@example.com: 246810\n",
	}}
	for i := 0; i < 60; i++ {
		lines = append(lines, dockerlog.Line{Timestamp: time.Now(), Source: "stdout", Content: "INF busy backend line\n"})
	}
	stub := &stubContainerLogs{lines: lines}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"backend-1"}

	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=backend-1&lines=2&include_verification_code=1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeLogRead(t, w)
	if stub.gotTail != logReadMaxLines {
		t.Errorf("reader tail = %d, want bounded verification-code search window %d", stub.gotTail, logReadMaxLines)
	}
	if len(resp.Lines) != 2 || !resp.Truncated {
		t.Errorf("general lines = %d truncated=%v, want requested tail of 2", len(resp.Lines), resp.Truncated)
	}
	if resp.VerificationCode == nil || !strings.Contains(resp.VerificationCode.Line, "246810") {
		t.Errorf("verification code = %+v, want the current code before the busy tail", resp.VerificationCode)
	}
}

// The verification-code carve-out exists only where lines carry authoritative
// engine timestamps. On the daemon source nothing can prove the code is still
// valid, so the broker refuses to answer even when asked.
func TestLogReadIncludeVerificationCodeDeniedOnDaemonSource(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil)
	d.cfg.LogPath = filepath.Join(t.TempDir(), "daemon.log")
	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=daemon&include_verification_code=1")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestLogReadSinceOnlyForContainerSource(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil)
	d.cfg.LogPath = filepath.Join(t.TempDir(), "daemon.log")
	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=daemon&since=2026-09-19T12:00:00Z")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("daemon+since: status = %d, want 400", w.Code)
	}
	w = serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=c&since=not-a-time")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad since: status = %d, want 400", w.Code)
	}
}

func TestLogReadValidation(t *testing.T) {
	d := newLogBrokerTestDaemon(t, nil)
	d.cfg.LogPath = filepath.Join(t.TempDir(), "daemon.log")
	cases := []struct {
		name  string
		query string
	}{
		{"missing workspace", "task_id=" + logBrokerTestTask},
		{"missing task", "workspace_id=" + logBrokerTestWorkspace},
		{"bad source", "workspace_id=" + logBrokerTestWorkspace + "&task_id=" + logBrokerTestTask + "&source=weird"},
		{"lines zero", "workspace_id=" + logBrokerTestWorkspace + "&task_id=" + logBrokerTestTask + "&lines=0"},
		{"lines non-numeric", "workspace_id=" + logBrokerTestWorkspace + "&task_id=" + logBrokerTestTask + "&lines=abc"},
		{"container without name", "workspace_id=" + logBrokerTestWorkspace + "&task_id=" + logBrokerTestTask + "&source=container"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := serveLogRead(t, d, tc.query); w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, body = %s; want 400", w.Code, w.Body.String())
			}
		})
	}
}

func TestLogReadClampsLinesToMax(t *testing.T) {
	stub := &stubContainerLogs{}
	d := newLogBrokerTestDaemon(t, stub)
	d.cfg.LogBrokerContainers = []string{"backend-1"}
	w := serveLogRead(t, d, "workspace_id="+logBrokerTestWorkspace+"&task_id="+logBrokerTestTask+"&source=container&container=backend-1&lines=999999")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if stub.gotTail != logReadMaxLines {
		t.Errorf("reader tail = %d, want clamped to %d", stub.gotTail, logReadMaxLines)
	}
}
