package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/previewpublication"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type previewPublicationStoreStub struct {
	publication previewpublication.Publication
	replay      bool
	registerErr error
	loadErr     error
}

func (s previewPublicationStoreStub) Register(context.Context, pgtype.UUID, string, previewpublication.ProducerClaim) (previewpublication.Publication, bool, error) {
	return s.publication, s.replay, s.registerErr
}

func (s previewPublicationStoreStub) Load(context.Context, pgtype.UUID, pgtype.UUID) (previewpublication.Publication, error) {
	return s.publication, s.loadErr
}

func previewRequest(t *testing.T, method, path, param, value, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	route := chi.NewRouteContext()
	route.URLParams.Add(param, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
}

func TestPublishTaskPreviewRejectsNonDaemonAndMalformedClaims(t *testing.T) {
	const taskID = "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name string
		ctx  bool
		body string
		want int
	}{
		{"non daemon", false, `{}`, http.StatusForbidden},
		{"unknown binding", true, `{"binding":{},"preview_url":"http://127.0.0.1:1/"}`, http.StatusBadRequest},
		{"non UTC expiry", true, `{"preview_url":"http://127.0.0.1:1/","identity_url":"http://127.0.0.1:1/.well-known/multica-preview-identity","data_policy":{"id":"synthetic-or-redacted-read-only-v1","data_class":"synthetic_or_redacted","mutations":"denied","external_network":"denied","credentials":"denied"},"expires_at":"2026-09-21T01:00:00-04:00"}`, http.StatusBadRequest},
		{"trailing value", true, `{} {}`, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/preview-publication", strings.NewReader(tc.body))
			route := chi.NewRouteContext()
			route.URLParams.Add("taskId", taskID)
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, route)
			if tc.ctx {
				ctx = middleware.WithDaemonContext(ctx, "22222222-2222-4222-8222-222222222222", "daemon")
			}
			w := httptest.NewRecorder()
			(&Handler{}).PublishTaskPreview(w, req.WithContext(ctx))
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status/cache = %d/%q, want %d/no-store", w.Code, w.Header().Get("Cache-Control"), tc.want)
			}
		})
	}
}

func TestPublishTaskPreviewStatusContract(t *testing.T) {
	const taskID = "11111111-1111-4111-8111-111111111111"
	body := `{"preview_url":"http://127.0.0.1:4173/","identity_url":"http://127.0.0.1:4173/.well-known/multica-preview-identity","data_policy":{"id":"synthetic-or-redacted-read-only-v1","data_class":"synthetic_or_redacted","mutations":"denied","external_network":"denied","credentials":"denied"},"expires_at":"2026-09-21T01:05:00Z"}`
	for _, tc := range []struct {
		name   string
		replay bool
		err    error
		want   int
	}{
		{"created", false, nil, http.StatusCreated},
		{"identical replay", true, nil, http.StatusOK},
		{"authority unavailable", false, previewpublication.ErrAuthorityUnavailable, http.StatusConflict},
		{"immutable conflict", false, previewpublication.ErrPublicationConflict, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := previewRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/preview-publication", "taskId", taskID, body)
			req = req.WithContext(middleware.WithDaemonContext(req.Context(), "22222222-2222-4222-8222-222222222222", "daemon"))
			h := &Handler{PreviewPublications: previewPublicationStoreStub{replay: tc.replay, registerErr: tc.err}}
			w := httptest.NewRecorder()
			h.PublishTaskPreview(w, req)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status/cache = %d/%q, want %d/no-store", w.Code, w.Header().Get("Cache-Control"), tc.want)
			}
		})
	}
}

func TestGetIssuePreviewPublicationStatusContract(t *testing.T) {
	const issueID = "22222222-2222-4222-8222-222222222222"
	publication := previewpublication.Publication{
		SchemaVersion: previewpublication.SchemaVersion,
		PublishedAt:   time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC),
		ExpiresAt:     time.Date(2026, 9, 21, 1, 5, 0, 0, time.UTC),
	}

	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"active", nil, http.StatusOK},
		{"absent", previewpublication.ErrPublicationUnavailable, http.StatusNotFound},
		{"expired", previewpublication.ErrPublicationExpired, http.StatusGone},
		{"storage failure", errors.New("database unavailable"), http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := previewRequest(t, http.MethodGet, "/api/shell/issues/"+issueID+"/preview-publication", "issueId", issueID, "")
			req.Header.Set("X-Workspace-ID", "11111111-1111-4111-8111-111111111111")
			h := &Handler{PreviewPublications: previewPublicationStoreStub{publication: publication, loadErr: tc.err}}
			w := httptest.NewRecorder()
			h.GetIssuePreviewPublication(w, req)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status/cache = %d/%q, want %d/no-store", w.Code, w.Header().Get("Cache-Control"), tc.want)
			}
		})
	}
}

func TestRelayIssuePreviewForwardsBoundedDaemonResponse(t *testing.T) {
	const (
		workspaceID = "11111111-1111-4111-8111-111111111111"
		issueID     = "22222222-2222-4222-8222-222222222222"
		candidateID = "33333333-3333-4333-8333-333333333333"
		runtimeID   = "44444444-4444-4444-8444-444444444444"
		taskID      = "55555555-5555-4555-8555-555555555555"
	)
	hub := daemonws.NewHub()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r, daemonws.ClientIdentity{
			DaemonID: "daemon-1", WorkspaceIDs: []string{workspaceID}, RuntimeIDs: []string{runtimeID},
		})
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	go func() {
		_, raw, readErr := conn.ReadMessage()
		if readErr != nil {
			return
		}
		var message protocol.Message
		if json.Unmarshal(raw, &message) != nil {
			return
		}
		var request protocol.RPCRequestPayload
		if json.Unmarshal(message.Payload, &request) != nil {
			return
		}
		body, _ := json.Marshal(protocol.PreviewFetchResponse{
			Status: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "text/html; charset=utf-8", "Content-Length": "4",
				"Content-Security-Policy": "default-src 'none'", "Set-Cookie": "secret=bad",
			},
			Body: []byte("safe"),
		})
		payload, _ := json.Marshal(protocol.RPCResponsePayload{
			RequestID: request.RequestID, Status: http.StatusOK, Body: body,
		})
		frame, _ := json.Marshal(protocol.Message{
			Type: protocol.EventDaemonRPCResponse, Payload: payload,
		})
		_ = conn.WriteMessage(websocket.TextMessage, frame)
	}()
	publication := previewpublication.Publication{
		SchemaVersion: previewpublication.SchemaVersion,
		Binding: previewpublication.Binding{
			WorkspaceID: workspaceID, IssueID: issueID,
			AgentID: "66666666-6666-4666-8666-666666666666", AgentRunID: taskID,
			CandidateID: candidateID, CandidateRevision: 1, SourceRevision: "source-1",
			Worktree: previewpublication.WorktreeProvenance{
				DaemonID: "daemon-1", RuntimeID: runtimeID, Path: "/tmp/worktree", Branch: "preview",
			},
		},
	}
	request := httptest.NewRequest(http.MethodGet,
		"/api/shell/issues/"+issueID+"/preview/"+candidateID+"/1/", nil)
	request.Header.Set("X-Workspace-ID", workspaceID)
	route := chi.NewRouteContext()
	route.URLParams.Add("issueId", issueID)
	route.URLParams.Add("candidateId", candidateID)
	route.URLParams.Add("candidateRevision", "1")
	route.URLParams.Add("*", "")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
	recorder := httptest.NewRecorder()
	(&Handler{DaemonHub: hub, PreviewPublications: previewPublicationStoreStub{publication: publication}}).
		RelayIssuePreview(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "safe" ||
		recorder.Header().Get("Cache-Control") != "no-store" ||
		recorder.Header().Get("Content-Security-Policy") != "default-src 'none'" ||
		recorder.Header().Get("Set-Cookie") != "" {
		t.Fatalf("relay response status=%d headers=%v body=%q", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}
