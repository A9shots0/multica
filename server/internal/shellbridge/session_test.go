package shellbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const user = "11111111-1111-4111-8111-111111111111"
const workspace = "22222222-2222-4222-8222-222222222222"
const parent = "mul_test-parent-never-production"

func uid(value string) pgtype.UUID { var id pgtype.UUID; _ = id.Scan(value); return id }

type fixture struct {
	parentErr, memberErr error
	pat                  db.PersonalAccessToken
}

func (f *fixture) GetPersonalAccessTokenByHash(_ context.Context, hash string) (db.PersonalAccessToken, error) {
	if hash != auth.HashToken(parent) {
		return db.PersonalAccessToken{}, pgx.ErrNoRows
	}
	return f.pat, f.parentErr
}
func (f *fixture) GetMemberByUserAndWorkspace(_ context.Context, p db.GetMemberByUserAndWorkspaceParams) (db.Member, error) {
	if p.UserID != uid(user) || p.WorkspaceID != uid(workspace) {
		return db.Member{}, pgx.ErrNoRows
	}
	return db.Member{UserID: p.UserID, WorkspaceID: p.WorkspaceID, Role: "owner"}, f.memberErr
}
func (f *fixture) GetWorkspace(_ context.Context, id pgtype.UUID) (db.Workspace, error) {
	return db.Workspace{ID: id, Slug: "test-workspace"}, nil
}
func setup(t *testing.T) (*Sessions, *fixture, string, *time.Time) {
	t.Helper()
	f := &fixture{pat: db.PersonalAccessToken{UserID: uid(user)}}
	s := New(f)
	now := time.Now()
	s.now = func() time.Time { return now }
	req := httptest.NewRequest("POST", "/api/shell/session", nil)
	req.Header.Set("Authorization", "Bearer "+parent)
	req.Header.Set("X-User-ID", user)
	req.Header.Set("X-Workspace-ID", workspace)
	w := httptest.NewRecorder()
	s.Exchange(w, req)
	if w.Code != 201 {
		t.Fatalf("exchange: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Token, Audience string
		Scopes          []string
		Expires         string `json:"expires_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Audience != "bluefin-work" || len(body.Scopes) != 2 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unbounded grant")
	}
	return s, f, body.Token, &now
}
func call(s *Sessions, token, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Authorize(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-ID") != user || middleware.WorkspaceIDFromContext(r.Context()) != workspace || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Actor-Source") != "" {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	})).ServeHTTP(w, req)
	return w
}
func TestSessionBoundary(t *testing.T) {
	s, _, token, _ := setup(t)
	for _, path := range []string{"/api/shell/issues?limit=100&offset=0", "/api/shell/working-agents?type=issue", "/api/shell/issues/33333333-3333-4333-8333-333333333333/preview-publication"} {
		if w := call(s, token, "GET", path, map[string]string{"X-Agent-ID": "forged", "X-Actor-Source": "task_token", "X-User-ID": "forged"}); w.Code != 200 {
			t.Fatalf("read %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/issues", "/api/shell/issues?workspace_id=other", "/api/shell/issues?limit=101", "/api/shell/issues?limit=1&limit=2", "/api/shell/issues?offset=1001", "/api/shell/issues?x=%XX", "/api/shell/working-agents?type=chat", "/api/shell/working-agents", "/api/shell/issues/id/preview-publication/extra", "/api/shell/issues/id/preview-publication?x=1", "/api/shell/session"} {
		if w := call(s, token, "GET", path, nil); w.Code != 403 {
			t.Fatalf("unexpected access %s: %d", path, w.Code)
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "HEAD"} {
		if w := call(s, token, method, "/api/shell/issues", nil); w.Code != 403 {
			t.Fatalf("write allowed %s: %d", method, w.Code)
		}
	}
	for _, headers := range []map[string]string{{"X-Workspace-ID": "other"}, {"X-Workspace-Slug": "other"}} {
		if w := call(s, token, "GET", "/api/shell/issues", headers); w.Code != 403 {
			t.Fatal("workspace escaped")
		}
	}
	if w := call(s, parent, "GET", "/api/shell/issues", nil); w.Code != 401 {
		t.Fatal("broad credential accepted")
	}
	// A valid scoped token cannot be used on the ordinary API, even as a cookie.
	for _, cookie := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/api/issues", nil)
		if cookie {
			req.AddCookie(&http.Cookie{Name: auth.AuthCookieName, Value: token})
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		middleware.Auth(nil, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("scope escaped normal auth") })).ServeHTTP(w, req)
		if w.Code < 400 {
			t.Fatal("scope escaped")
		}
	}
}
func TestExpiryRevocationAndRestart(t *testing.T) {
	for _, reason := range []string{"expiry", "revoke", "parent", "membership", "restart", "parent-expiry"} {
		t.Run(reason, func(t *testing.T) {
			s, f, token, now := setup(t)
			switch reason {
			case "expiry":
				*now = now.Add(Lifetime)
			case "revoke":
				if w := call(s, token, "DELETE", "/api/shell/session", nil); w.Code != 204 {
					t.Fatal(w.Code)
				}
			case "parent":
				f.parentErr = pgx.ErrNoRows
			case "membership":
				f.memberErr = pgx.ErrNoRows
			case "restart":
				s = New(f)
			case "parent-expiry":
				f.pat.ExpiresAt = pgtype.Timestamptz{Time: *now, Valid: true}
			}
			if w := call(s, token, "GET", "/api/shell/issues", nil); w.Code != 401 {
				t.Fatal(w.Code)
			}
		})
	}
}
func TestDatabaseOutageIsNotEmptyOrAuthLoss(t *testing.T) {
	s, f, token, _ := setup(t)
	f.parentErr = errors.New("database offline")
	if w := call(s, token, "GET", "/api/shell/issues", nil); w.Code != 503 {
		t.Fatal(w.Code)
	}
	f.parentErr = nil
	f.memberErr = errors.New("database offline")
	if w := call(s, token, "GET", "/api/shell/issues", nil); w.Code != 503 {
		t.Fatal(w.Code)
	}
	if w := call(s, token, "DELETE", "/api/shell/session", nil); w.Code != 204 {
		t.Fatal("could not revoke during outage")
	}
}
func TestExchangeDoesNotAcceptOtherCredentialsOrWorkspace(t *testing.T) {
	s, _, token, _ := setup(t)
	for _, credential := range []string{token, "mat_task", "mpi_plugin", "mcn_cloud", "JWT"} {
		req := httptest.NewRequest("POST", "/api/shell/session", nil)
		req.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		s.Exchange(w, req)
		if w.Code != 403 {
			t.Fatal("non-human exchange", w.Code)
		}
	}
	for _, ws := range []string{"invalid", "33333333-3333-4333-8333-333333333333"} {
		req := httptest.NewRequest("POST", "/api/shell/session", nil)
		req.Header.Set("Authorization", "Bearer "+parent)
		req.Header.Set("X-User-ID", user)
		req.Header.Set("X-Workspace-ID", ws)
		w := httptest.NewRecorder()
		s.Exchange(w, req)
		if w.Code < 400 {
			t.Fatal("unauthorized workspace granted")
		}
	}
}

func TestParentExpiryAndSessionLimit(t *testing.T) {
	s, f, _, now := setup(t)
	f.pat.ExpiresAt = pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true}
	for index := 1; index <= 8; index++ {
		req := httptest.NewRequest("POST", "/api/shell/session", nil)
		req.Header.Set("Authorization", "Bearer "+parent)
		req.Header.Set("X-User-ID", user)
		req.Header.Set("X-Workspace-ID", workspace)
		w := httptest.NewRecorder()
		s.Exchange(w, req)
		if index == 8 {
			if w.Code != 429 {
				t.Fatal("session cap not enforced", w.Code)
			}
			continue
		}
		if w.Code != 201 {
			t.Fatal(w.Code)
		}
		var body struct {
			Expires string `json:"expires_at"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		expiry, err := time.Parse(time.RFC3339Nano, body.Expires)
		if err != nil || !expiry.Equal(f.pat.ExpiresAt.Time) {
			t.Fatal("grant outlives parent")
		}
	}
}

func (f *fixture) GetUser(_ context.Context, id pgtype.UUID) (db.User, error) {
	return db.User{ID: id, Email: "fixture@example.invalid"}, nil
}
func TestDesktopJWTDelegation(t *testing.T) {
	s, _, _, now := setup(t)
	for _, mode := range []string{"valid", "missing-expiry", "expired", "wrong-user", "wrong-signature"} {
		claims := jwt.RegisteredClaims{Subject: user, ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}
		secret := auth.JWTSecret()
		switch mode {
		case "missing-expiry":
			claims.ExpiresAt = nil
		case "expired":
			claims.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
		case "wrong-user":
			claims.Subject = "other"
		case "wrong-signature":
			secret = []byte("not-the-server-key")
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
		req := httptest.NewRequest("POST", "/api/shell/session", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-User-ID", user)
		req.Header.Set("X-Workspace-ID", workspace)
		w := httptest.NewRecorder()
		s.Exchange(w, req)
		if mode != "valid" {
			if w.Code < 400 {
				t.Fatal(mode)
			}
			continue
		}
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var body struct {
			Token   string
			Expires string `json:"expires_at"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		expiry, _ := time.Parse(time.RFC3339Nano, body.Expires)
		if !expiry.Equal(claims.ExpiresAt.Time) {
			t.Fatal("grant outlives login")
		}
		if w := call(s, body.Token, "GET", "/api/shell/issues", nil); w.Code != 200 {
			t.Fatal(w.Code)
		}
		if w := call(s, body.Token, "DELETE", "/api/shell/session", nil); w.Code != 204 {
			t.Fatal(w.Code)
		}
		if w := call(s, body.Token, "GET", "/api/shell/issues", nil); w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
}
