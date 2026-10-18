package dockerlog

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakeEngine stands in for the Docker Engine API. It records the request the
// client wrote and serves a canned /logs response.
type fakeEngine struct {
	server *httptest.Server
	// logFrames is the multiplexed frame stream served for /logs.
	logFrames []byte
	// status overrides the success response for /logs.
	status int
	// responseBody replaces the demultiplexed stream (for non-200 paths).
	responseBody string

	gotPath string
	gotRaw  string
}

func newFakeEngine(t *testing.T) *fakeEngine {
	t.Helper()
	e := &fakeEngine{status: http.StatusOK}
	e.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1.41/containers/") || !strings.HasSuffix(r.URL.Path, "/logs") {
			http.NotFound(w, r)
			return
		}
		e.gotPath = r.URL.Path
		e.gotRaw = r.URL.RawQuery
		if e.status != http.StatusOK {
			w.WriteHeader(e.status)
			fmt.Fprint(w, e.responseBody)
			return
		}
		w.Write(e.logFrames)
	}))
	t.Cleanup(e.server.Close)
	return e
}

// dialer returns a Dialer that reaches the fake engine over TCP, the same
// path the unix/pipe dialers use in production.
func (e *fakeEngine) dialer() Dialer {
	addr := e.server.Listener.Addr().String()
	return func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
}

// frame builds one multiplexed Engine log frame.
func frame(kind byte, payload string) []byte {
	out := make([]byte, 8+len(payload))
	out[0] = kind
	binary.BigEndian.PutUint32(out[4:8], uint32(len(payload)))
	copy(out[8:], payload)
	return out
}

func TestReadLogsRequestsBoundedTimestamps(t *testing.T) {
	e := newFakeEngine(t)
	client := NewClient(e.dialer())

	since := time.Unix(1730000000, 0)
	_, err := client.ReadLogs(context.Background(), "multica-backend-1", 137, since)
	if err != nil {
		t.Fatalf("ReadLogs = %v, want nil", err)
	}

	q, err := url.ParseQuery(e.gotRaw)
	if err != nil {
		t.Fatalf("RawQuery %q does not parse: %v", e.gotRaw, err)
	}
	for key, want := range map[string]string{
		"stdout":     "1",
		"stderr":     "1",
		"timestamps": "1",
		"tail":       "137",
		"since":      "1730000000",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if !strings.HasSuffix(e.gotPath, "/containers/multica-backend-1/logs") {
		t.Errorf("path = %q, want the escaped container logs path", e.gotPath)
	}
}

func TestReadLogsDemultiplexesFrames(t *testing.T) {
	line1 := "2026-09-20T02:14:02.848683229Z INF backend ready\n"
	line2 := "2026-09-20T02:14:03.287314017Z WRN something\n"
	e := newFakeEngine(t)
	e.logFrames = append(frame(StreamStdout, line1), frame(StreamStderr, line2)...)
	client := NewClient(e.dialer())

	lines, err := client.ReadLogs(context.Background(), "c", 10, time.Time{})
	if err != nil {
		t.Fatalf("ReadLogs = %v, want nil", err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0].Source != "stdout" || lines[1].Source != "stderr" {
		t.Errorf("sources = %s/%s, want stdout/stderr", lines[0].Source, lines[1].Source)
	}
	ts := lines[0].Timestamp
	if ts.IsZero() || ts.Format(time.RFC3339Nano) != "2026-09-20T02:14:02.848683229Z" {
		t.Errorf("timestamp = %v, want the engine prefix parsed", ts)
	}
	if !strings.HasPrefix(lines[0].Content, "INF backend ready") {
		t.Errorf("content = %q, want the timestamp prefix stripped", lines[0].Content)
	}
}

func TestReadLogsTruncatedFrameIsError(t *testing.T) {
	// One complete frame followed by a torn frame whose body never arrives.
	e := newFakeEngine(t)
	e.logFrames = append(frame(StreamStdout, "ok\n"), make([]byte, 3)...)
	client := NewClient(e.dialer())

	_, err := client.ReadLogs(context.Background(), "c", 10, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("ReadLogs = %v, want a truncated-stream error", err)
	}
}

func TestReadLogsEngine404SurfacesMessage(t *testing.T) {
	e := newFakeEngine(t)
	e.status = http.StatusNotFound
	e.responseBody = `{"message":"No such container: nope"}`
	client := NewClient(e.dialer())

	_, err := client.ReadLogs(context.Background(), "nope", 10, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "No such container") {
		t.Fatalf("ReadLogs = %v, want the engine's message", err)
	}
}

func TestReadLogsContextCancellationClosesConn(t *testing.T) {
	// An engine that writes the response head but then stalls the body.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/logs") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{1}) // 1 of the 8 header bytes, then stall forever
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	addr := server.Listener.Addr().String()

	client := NewClient(func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := client.ReadLogs(ctx, "c", 10, time.Time{})
	if err == nil {
		t.Fatal("ReadLogs = nil, want a deadline error for a stalled engine")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("ReadLogs = %v, want a context error", err)
	}
}

func TestReadLogsClampsTailAndRejectsEmptyContainer(t *testing.T) {
	e := newFakeEngine(t)
	client := NewClient(e.dialer())

	if _, err := client.ReadLogs(context.Background(), "  ", 10, time.Time{}); err == nil {
		t.Error("ReadLogs with an empty container = nil, want an error")
	}
	if _, err := client.ReadLogs(context.Background(), "c", 1<<20, time.Time{}); err != nil {
		t.Fatalf("ReadLogs = %v, want a clamped tail to succeed", err)
	}
	q, _ := url.ParseQuery(e.gotRaw)
	if got := q.Get("tail"); got != "10000" {
		t.Errorf("tail = %q, want clamped to 10000", got)
	}
}

// ioReader resume point guard: ReadFrames must consume no bytes when the
// stream is empty, so an engine that returns an empty body degrades to zero
// lines rather than an error.
func TestReadFramesEmptyStream(t *testing.T) {
	lines, err := readFrames(strings.NewReader(""))
	if err != nil || len(lines) != 0 {
		t.Fatalf("readFrames(empty) = %d lines, %v; want 0, nil", len(lines), err)
	}
}
