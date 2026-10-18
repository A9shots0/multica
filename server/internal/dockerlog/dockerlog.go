// Package dockerlog reads bounded container stdout/stderr through the local
// Docker Engine API without importing the moby client. The daemon calls this
// over the engine socket (a named pipe on Windows, a unix socket elsewhere);
// an agent sandbox cannot reach either, which is why the broker runs in the
// daemon process.
package dockerlog

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Stream constants from the Engine's multiplexed log protocol.
const (
	StreamStdout byte = 1
	StreamStderr byte = 2
)

// Line is one demultiplexed log record from a container.
type Line struct {
	// Timestamp is the RFC3339Nano timestamp the engine prefixes when
	// Timestamps is requested. Zero when the engine sent none (only possible
	// for very old engines) or the prefix did not parse.
	Timestamp time.Time
	// Source names the stream: "stdout" or "stderr".
	Source string
	// Content is the line payload with the timestamp prefix stripped. It
	// keeps its trailing newline when the engine supplied one.
	Content string
}

// Dialer connects to the Docker Engine socket. It is separately injectable so
// tests can stand in a fake engine on a plain TCP listener.
type Dialer func(ctx context.Context) (net.Conn, error)

// Client reads logs from the Engine API over an operator-supplied dialer.
type Client struct {
	dialer   Dialer
	apiPath  string
	timeout  time.Duration
	maxBytes int64
}

const (
	// engineHost is the HTTP Host header for engine requests. The scheme is
	// meaningless (we dial the socket ourselves); http.NewRequest still wants
	// a parseable URL.
	engineHost = "docker"

	// defaultAPIPath uses a conservative Engine API version supported by
	// every modern engine while staying well above the ancient versions that
	// predate the /logs endpoint shape we call.
	defaultAPIPath = "/v1.41"

	defaultTimeout = 30 * time.Second

	// defaultMaxReadBytes bounds one /logs response so a misbehaving engine
	// cannot balloon the daemon's memory: at ~200 bytes per frame that is
	// roughly 50k lines, far beyond any bounded tail.
	defaultMaxReadBytes = 10 * 1024 * 1024
)

// NewClient returns a Client that dials the engine through dial. Pass
// DefaultDialer() for real engine access; tests inject their own.
func NewClient(dial Dialer) *Client {
	return &Client{
		dialer:   dial,
		apiPath:  defaultAPIPath,
		timeout:  defaultTimeout,
		maxBytes: defaultMaxReadBytes,
	}
}

// maxTail is the engine-side ceiling on /logs tail we ever request. The
// broker clamps below this; the API rejects absurd values.
const maxTail = 10000

// ReadLogs fetches up to tail recent log lines for container, optionally
// starting at since (zero time = no lower bound). It is strictly read-only:
// only the GET /containers/{name}/logs endpoint is used.
func (c *Client) ReadLogs(ctx context.Context, container string, tail int, since time.Time) ([]Line, error) {
	if strings.TrimSpace(container) == "" {
		return nil, errors.New("container name is required")
	}
	if tail <= 0 {
		tail = 1
	}
	if tail > maxTail {
		tail = maxTail
	}

	u := url.URL{
		Scheme: "http",
		Host:   engineHost,
		Path:   c.apiPath + "/containers/" + url.PathEscape(container) + "/logs",
	}
	q := u.Query()
	q.Set("stdout", "1")
	q.Set("stderr", "1")
	q.Set("timestamps", "1")
	q.Set("tail", fmt.Sprintf("%d", tail))
	if !since.IsZero() {
		q.Set("since", fmt.Sprintf("%d", since.Unix()))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build engine request: %w", err)
	}
	req.Host = engineHost

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := c.dialer(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to docker engine: %w", err)
	}
	defer conn.Close()

	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("write engine request: %w", err)
	}

	// Reading a named pipe cannot be interrupted mid-Read on Windows, so the
	// whole exchange runs in a goroutine and the context only aborts it by
	// closing the connection. http.ReadResponse then errors with a closed
	// connection and the caller sees ctx.Err().
	type result struct {
		lines []Line
		err   error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := http.ReadResponse(bufio.NewReader(conn), req)
		if err != nil {
			resCh <- result{err: fmt.Errorf("read engine response: %w", err)}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			msg, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			resCh <- result{err: engineError(resp.StatusCode, string(msg))}
			return
		}
		lines, err := readFrames(io.LimitReader(resp.Body, c.maxBytes))
		resCh <- result{lines: lines, err: err}
	}()
	select {
	case <-ctx.Done():
		conn.Close()
		<-resCh
		return nil, ctx.Err()
	case res := <-resCh:
		if res.err != nil {
			return nil, res.err
		}
		return res.lines, nil
	}
}

// engineError turns an engine non-200 into an actionable error.
func engineError(status int, body string) error {
	body = strings.TrimSpace(body)
	// The engine returns a JSON {"message":"..."} envelope.
	msg := body
	if i := strings.LastIndex(body, `"message":`); i >= 0 {
		msg = strings.TrimSpace(strings.Trim(strings.TrimPrefix(body[i+len(`"message":`):], " "), `"`))
	}
	if msg == "" {
		msg = fmt.Sprintf("engine returned %s", http.StatusText(status))
	}
	return fmt.Errorf("docker engine: %s: %s", http.StatusText(status), msg)
}

// readFrames demultiplexes the Engine's 8-byte-header frame stream. Each
// frame is 8 bytes of header (stream type + big-endian payload size) followed
// by that many payload bytes. A truncated stream is treated as an error: the
// engine only terminates a bounded /logs response cleanly.
func readFrames(r io.Reader) ([]Line, error) {
	var lines []Line
	hdr := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, hdr); err != nil {
			if errors.Is(err, io.EOF) {
				return lines, nil
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return lines, errors.New("docker engine closed mid-frame: truncated log stream")
			}
			return lines, err
		}
		size := binary.BigEndian.Uint32(hdr[4:])
		if size == 0 {
			continue
		}
		if int64(size) > int64(maxPayloadSize) {
			return lines, fmt.Errorf("docker engine frame of %d bytes exceeds the max payload of %d", size, maxPayloadSize)
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			return lines, fmt.Errorf("docker engine truncated a %d-byte frame: %w", size, err)
		}
		lines = append(lines, parseFrame(hdr[0], payload))
	}
}

// maxPayloadSize bounds a single frame; per-line container output is never
// near this. Protects the demuxer from a hostile engine payload size field.
const maxPayloadSize = 4 * 1024 * 1024

func parseFrame(kind byte, payload []byte) Line {
	line := Line{Source: streamName(kind), Content: string(payload)}
	if i := strings.IndexByte(string(payload), ' '); i > 0 {
		if ts, err := time.Parse(time.RFC3339Nano, string(payload[:i])); err == nil {
			line.Timestamp = ts
			line.Content = string(payload[i+1:])
		}
	}
	return line
}

func streamName(kind byte) string {
	switch kind {
	case StreamStdout:
		return "stdout"
	case StreamStderr:
		return "stderr"
	default:
		return "unknown"
	}
}
