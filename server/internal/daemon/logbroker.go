package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/dockerlog"
	"github.com/multica-ai/multica/server/pkg/redact"
)

// containerLogReader abstracts the Docker Engine log read so tests can stand
// in a fake engine without touching a socket.
type containerLogReader interface {
	ReadLogs(ctx context.Context, container string, tail int, since time.Time) ([]dockerlog.Line, error)
}

const (
	// logReadDefaultLines / logReadMaxLines bound a single /log/read response.
	// The CLI defaults to the same 50 the human `daemon logs` command uses;
	// the ceiling exists so a task can never ask the broker to chew through
	// the whole raw stream in one response.
	logReadDefaultLines = 50
	logReadMaxLines     = 500

	// verificationCodeTTL mirrors the backend's one-time-code expiry
	// (server/internal/handler/auth.go stores ExpiresAt = now + 10m and the
	// sent email says "expires in 10 minutes"). The broker echoes a code only
	// while its log timestamp is younger than this bound.
	verificationCodeTTL = 10 * time.Minute
)

// verificationCodeRe matches the backend's DEV-mode stdout line:
//
//	[DEV] Verification code for alice@example.com: 481516
//
// The digits are the one-time secret; the email stays visible so the diagnosing
// task can tell which auth flow the code belonged to. `redact.Text` never sees
// this shape (no API_KEY/TOKEN/PASSWORD= key), so the broker handles it
// explicitly: by default the digits are replaced, and only an explicit
// owner-authorized request may retrieve the newest unexpired line.
var verificationCodeRe = regexp.MustCompile(`\[\s*DEV\s*\] Verification code for ([^:]+):\s*(\d{6})\b`)

// redactVerificationCodeRe drives the default (deny) path: the digits are
// replaced while the rest of the line — including the email — still proves to
// the reader that a code was issued for that flow.
var redactVerificationCodeRe = regexp.MustCompile(`(\[\s*DEV\s*\] Verification code for [^:]+:\s*)\d{6}\b`)

// logReadRequest carries the CLI's query parameters. Identity fields
// (workspace_id / task_id) are validated against the token-bound active task
// and then overwritten with the authoritative record, exactly like
// /repo/checkout — the request never decides who the caller is.
type logReadRequest struct {
	Source                  string
	Lines                   int
	Since                   time.Time
	Container               string
	IncludeVerificationCode bool
	WorkspaceID             string
	TaskID                  string
}

// logReadResponse is returned by GET /log/read.
type logReadResponse struct {
	Source           string                `json:"source"`
	Container        string                `json:"container,omitempty"`
	WorkspaceID      string                `json:"workspace_id"`
	TaskID           string                `json:"task_id"`
	ServerTime       time.Time             `json:"server_time"`
	Truncated        bool                  `json:"truncated"`
	Lines            []string              `json:"lines"`
	VerificationCode *verificationCodeInfo `json:"verification_code,omitempty"`
}

// verificationCodeInfo is the owner-authorized verification-code carve-out.
// Age and expiry are only meaningful for the container source, whose lines
// carry authoritative engine timestamps.
type verificationCodeInfo struct {
	Line             string `json:"line"`
	Email            string `json:"email"`
	AgeSeconds       int64  `json:"age_seconds"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
	Expired          bool   `json:"expired"`
}

// logSourceLine is one raw log line before redaction, optionally carrying the
// authoritative timestamp the source attached to it.
type logSourceLine struct {
	raw string
	ts  time.Time
}

// logReadFailure is a broker error with the HTTP status it should be served
// under. Keeping the two together means every failure path decides its own
// status instead of falling through to a generic 500.
type logReadFailure struct {
	status  int
	message string
}

func (e *logReadFailure) Error() string { return e.message }

// logReadMinCLIVersion is the release that first let `multica daemon logs` run
// inside a managed task and send the task credential this endpoint requires.
// There is no earlier CLI that spoke /log/read at all, so unlike the repo
// endpoint this value is informational only.
const logReadMinCLIVersion = "v0.4.35"

// writeLogReadAuthError rejects an unauthenticated or mis-scoped /log/read and
// leaves a trace in daemon.log — the #7520 lesson: a silent 401 is invisible
// to everyone but the agent that saw it, and this path exists for diagnosis.
// The token is never logged: it is a live task credential.
func (d *Daemon) writeLogReadAuthError(w http.ResponseWriter, result taskCredentialAuthResult) {
	reason := "no_credential"
	if result == taskCredentialAuthUnknownCredential {
		reason = "unknown_credential"
	}
	d.logger.Warn("log read rejected",
		"reason", reason,
		"min_cli_version", logReadMinCLIVersion,
		"daemon_version", d.cfg.CLIVersion,
	)
	message := "log read requires an active task credential and this request carried none"
	if result == taskCredentialAuthUnknownCredential {
		message = "log read credential is not bound to a task running in this daemon: " +
			"the task it belongs to has already finished, or the daemon restarted after this agent started. " +
			"In-task log reads are only available while the task that owns this run is still running."
	}
	writeLogReadError(w, http.StatusUnauthorized, message)
}

func writeLogReadError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// logReadHandler serves GET /log/read on the daemon's localhost health mux.
func (d *Daemon) logReadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		active, authResult := d.activeTaskCredential(r)
		if authResult != taskCredentialAuthOK {
			d.writeLogReadAuthError(w, authResult)
			return
		}

		req, err := parseLogReadRequest(r)
		if err != nil {
			writeLogReadError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.WorkspaceID != active.WorkspaceID || req.TaskID != active.TaskID {
			d.logger.Warn("log read rejected",
				"reason", "context_mismatch",
				"workspace_id", req.WorkspaceID,
				"task_id", req.TaskID,
			)
			writeLogReadError(w, http.StatusForbidden, "log read task context does not match the active task")
			return
		}
		req.WorkspaceID = active.WorkspaceID
		req.TaskID = active.TaskID

		serverTime := time.Now()
		readReq := req
		if req.IncludeVerificationCode {
			// Search the full bounded broker window for the newest code even
			// when the caller only wants a small general-log tail. Busy backend
			// output must not push a still-current code out of the default
			// 50-line response window.
			readReq.Lines = logReadMaxLines
		}
		rawLines, truncated, err := d.readLogSource(r.Context(), readReq)
		if err != nil {
			var failure *logReadFailure
			if errors.As(err, &failure) {
				writeLogReadError(w, failure.status, failure.message)
				return
			}
			d.logger.Error("log read failed", "workspace_id", req.WorkspaceID, "task_id", req.TaskID, "source", req.Source, "error", err)
			writeLogReadError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := logReadResponse{
			Source:      req.Source,
			WorkspaceID: req.WorkspaceID,
			TaskID:      req.TaskID,
			ServerTime:  serverTime,
			Truncated:   truncated,
		}
		if req.Source == "container" {
			resp.Container = req.Container
		}
		responseLines := rawLines
		if len(responseLines) > req.Lines {
			responseLines = responseLines[len(responseLines)-req.Lines:]
			resp.Truncated = true
		}
		resp.Lines = make([]string, len(responseLines))
		for i, l := range responseLines {
			resp.Lines[i] = redactLogLine(l.raw)
		}
		if req.IncludeVerificationCode {
			resp.VerificationCode = findVerificationCode(rawLines, serverTime)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// parseLogReadRequest decodes and validates the query string.
func parseLogReadRequest(r *http.Request) (logReadRequest, error) {
	q := r.URL.Query()
	req := logReadRequest{
		Source:      strings.ToLower(strings.TrimSpace(q.Get("source"))),
		Container:   strings.TrimSpace(q.Get("container")),
		WorkspaceID: strings.TrimSpace(q.Get("workspace_id")),
		TaskID:      strings.TrimSpace(q.Get("task_id")),
	}
	if req.Source == "" {
		req.Source = "daemon"
	}
	switch req.Source {
	case "daemon", "container":
	default:
		return req, errors.New(`source must be "daemon" or "container"`)
	}
	if req.WorkspaceID == "" || req.TaskID == "" {
		return req, errors.New("workspace_id and task_id are required")
	}

	req.Lines = logReadDefaultLines
	if raw := strings.TrimSpace(q.Get("lines")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return req, errors.New("lines must be a positive integer")
		}
		if n > logReadMaxLines {
			n = logReadMaxLines
		}
		req.Lines = n
	}

	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return req, errors.New("since must be an RFC 3339 timestamp (e.g. 2026-09-19T12:00:00Z)")
		}
		req.Since = ts
	}
	if !req.Since.IsZero() && req.Source != "container" {
		return req, errors.New("since is only honored for --source container (daemon.log lines carry times, not dates)")
	}

	if raw := strings.TrimSpace(q.Get("include_verification_code")); raw == "1" || raw == "true" {
		req.IncludeVerificationCode = true
	}
	if req.IncludeVerificationCode && req.Source != "container" {
		return req, errors.New("include_verification_code is only available for --source container, whose lines carry authoritative engine timestamps")
	}
	if req.Source == "container" && req.Container == "" {
		return req, errors.New("container is required for --source container")
	}
	return req, nil
}

// readLogSource fetches the raw (unredacted) lines for one source, bounded per
// the request. Redaction happens once, in the handler, against this raw view.
func (d *Daemon) readLogSource(ctx context.Context, req logReadRequest) ([]logSourceLine, bool, error) {
	switch req.Source {
	case "daemon":
		return d.readDaemonLogForTask(req.WorkspaceID, req.TaskID, req.Lines)
	case "container":
		return d.readContainerLog(ctx, req)
	default:
		return nil, false, errors.New("unknown log source")
	}
}

// readDaemonLogForTask returns the last `limit` daemon.log lines attributable
// to the requesting workspace or task. On a host serving several workspaces
// this filtering is the scoping boundary: a line that names neither this
// workspace nor this task is another run's business and never leaves the file.
func (d *Daemon) readDaemonLogForTask(workspaceID, taskID string, limit int) ([]logSourceLine, bool, error) {
	logPath := strings.TrimSpace(d.cfg.LogPath)
	if logPath == "" || !filepath.IsAbs(logPath) {
		return nil, false, &logReadFailure{http.StatusServiceUnavailable,
			"the daemon log source is not configured on this daemon"}
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, &logReadFailure{http.StatusServiceUnavailable,
				"no daemon.log at " + logPath}
		}
		return nil, false, fmt.Errorf("read daemon log: %w", err)
	}
	taskToken := "task=" + taskID
	taskIDToken := "task_id=" + taskID
	wsToken := "workspace_id=" + workspaceID
	var matched []logSourceLine
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, taskToken) || strings.Contains(line, taskIDToken) || strings.Contains(line, wsToken) {
			matched = append(matched, logSourceLine{raw: line})
		}
	}
	truncated := len(matched) > limit
	if truncated {
		matched = matched[len(matched)-limit:]
	}
	return matched, truncated, nil
}

// readContainerLog streams a bounded window from the operator-allow-listed
// container through the Docker Engine API and hands it over in chronology
// order (newest last).
func (d *Daemon) readContainerLog(ctx context.Context, req logReadRequest) ([]logSourceLine, bool, error) {
	if len(d.cfg.LogBrokerContainers) == 0 {
		return nil, false, &logReadFailure{http.StatusServiceUnavailable,
			"the container log source is disabled on this daemon — the operator must set MULTICA_DAEMON_LOG_CONTAINERS"}
	}
	if !d.logContainerAllowed(req.Container) {
		return nil, false, &logReadFailure{http.StatusForbidden,
			"container " + req.Container + " is not on this daemon's readable container list (MULTICA_DAEMON_LOG_CONTAINERS)"}
	}
	if d.containerLogs == nil {
		return nil, false, &logReadFailure{http.StatusServiceUnavailable,
			"the container log reader is not initialized on this daemon"}
	}
	pts, err := d.containerLogs.ReadLogs(ctx, req.Container, req.Lines, req.Since)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, &logReadFailure{http.StatusServiceUnavailable,
				"docker engine read timed out: " + ctx.Err().Error()}
		}
		return nil, false, &logReadFailure{http.StatusBadGateway, err.Error()}
	}
	lines := make([]logSourceLine, len(pts))
	for i, p := range pts {
		lines[i] = logSourceLine{raw: p.Content, ts: p.Timestamp}
	}
	// The engine already bounds the response to the requested tail, so the
	// response carries everything the engine sent.
	return lines, false, nil
}

// logContainerAllowed matches a requested container against the operator's
// exact-name allow-list. Substring matching would let an allow-listed
// "multica-backend-1" authorize a similarly named container owned by another
// workspace. Matching is byte-for-byte after trimming surrounding whitespace.
func (d *Daemon) logContainerAllowed(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, p := range d.cfg.LogBrokerContainers {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if name == p {
			return true
		}
	}
	return false
}

// redactLogLine applies the shared secret patterns and then the
// verification-code digit replacement. The carve-out (returning digits) only
// happens through VerificationCode on an explicit request; the general line
// stream is always deny-first.
func redactLogLine(line string) string {
	line = redact.Text(line)
	return redactVerificationCodeRe.ReplaceAllString(line, "$1[REDACTED VERIFICATION CODE]")
}

// findVerificationCode returns the newest requested verification-code line in
// the served window, or nil when none is present. When the line carries an
// authoritative timestamp it also answers age and remaining validity against
// the backend's 10-minute code TTL.
func findVerificationCode(lines []logSourceLine, serverTime time.Time) *verificationCodeInfo {
	for i := len(lines) - 1; i >= 0; i-- {
		match := verificationCodeRe.FindStringSubmatchIndex(lines[i].raw)
		if match == nil {
			continue
		}
		email := lines[i].raw[match[2]:match[3]]
		info := &verificationCodeInfo{
			Line:  revealVerificationCode(lines[i].raw, match),
			Email: email,
		}
		if lines[i].ts.IsZero() {
			info.AgeSeconds = -1
			info.ExpiresInSeconds = -1
			return info
		}
		age := int64(serverTime.Sub(lines[i].ts).Seconds())
		if age < 0 {
			age = 0
		}
		info.AgeSeconds = age
		expiresIn := int64(verificationCodeTTL.Seconds()) - age
		if expiresIn > 0 {
			info.ExpiresInSeconds = expiresIn
			return info
		}
		info.ExpiresInSeconds = 0
		info.Expired = true
		return info
	}
	return nil
}

// revealVerificationCode keeps the explicitly requested six-digit code while
// applying the normal credential redactor to every other part of the matching
// log line. A backend log line can contain structured fields beside the
// message; returning the raw line here would let an unrelated token on that
// same line bypass the broker's deny-first policy.
func revealVerificationCode(line string, match []int) string {
	const marker = "__MULTICA_REQUESTED_VERIFICATION_CODE__"
	codeStart, codeEnd := match[4], match[5]
	code := line[codeStart:codeEnd]
	withMarker := line[:codeStart] + marker + line[codeEnd:]
	return strings.Replace(redact.Text(withMarker), marker, code, 1)
}
