// Package shellbridge gives the signed-in desktop a revocable, short-lived
// read capability. It does not reuse a user token as a background credential.
package shellbridge

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const Prefix = "msr_"
const Lifetime = 5 * time.Minute
const maxSessions = 2048

// Querier deliberately rechecks parent revocation and membership on every read;
// the ordinary PAT and membership caches cannot delay revocation here.
type Querier interface {
	GetPersonalAccessTokenByHash(context.Context, string) (db.PersonalAccessToken, error)
	GetMemberByUserAndWorkspace(context.Context, db.GetMemberByUserAndWorkspaceParams) (db.Member, error)
	GetWorkspace(context.Context, pgtype.UUID) (db.Workspace, error)
	GetUser(context.Context, pgtype.UUID) (db.User, error)
}

type grant struct {
	userID, workspaceID pgtype.UUID
	parentHash          string
	parentPAT           bool
	expiresAt           time.Time
}

type Sessions struct {
	queries Querier
	now     func() time.Time
	mu      sync.Mutex
	grants  map[string]grant
}

func New(queries Querier) *Sessions {
	return &Sessions{queries: queries, now: time.Now, grants: make(map[string]grant)}
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func deny(w http.ResponseWriter, status int) {
	respond(w, status, map[string]string{"error": http.StatusText(status)})
}

func lookupStatus(err error) int {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusUnauthorized
	}
	return http.StatusServiceUnavailable
}

// Exchange is mounted behind normal Auth. The desktop's signed session JWT
// and existing PAT login can delegate; task, plugin, cloud and reader tokens
// cannot. JWT logout revokes this grant, not the parent JWT (Multica's existing
// JWT logout clears cookies rather than maintaining a JWT revocation ledger).
func (s *Sessions) Exchange(w http.ResponseWriter, r *http.Request) {
	raw := middleware.BearerToken(r)
	if r.Method != http.MethodPost {
		deny(w, http.StatusForbidden)
		return
	}
	userUUID, err := util.ParseUUID(r.Header.Get("X-User-ID"))
	if err != nil {
		deny(w, http.StatusForbidden)
		return
	}
	userID := util.UUIDToString(userUUID)
	expiresAt := s.now().Add(Lifetime)
	parentPAT := strings.HasPrefix(raw, "mul_")
	if parentPAT {
		pat, err := s.queries.GetPersonalAccessTokenByHash(r.Context(), auth.HashToken(raw))
		if err != nil {
			deny(w, lookupStatus(err))
			return
		}
		if pat.UserID != userUUID || pat.Revoked {
			deny(w, http.StatusUnauthorized)
			return
		}
		if pat.ExpiresAt.Valid && pat.ExpiresAt.Time.Before(expiresAt) {
			expiresAt = pat.ExpiresAt.Time
		}
	} else {
		claims := jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(raw, &claims, func(_ *jwt.Token) (any, error) { return auth.JWTSecret(), nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithTimeFunc(s.now))
		if err != nil || !token.Valid || claims.Subject != userID {
			deny(w, http.StatusForbidden)
			return
		}
		if claims.ExpiresAt.Time.Before(expiresAt) {
			expiresAt = claims.ExpiresAt.Time
		}
	}
	if !expiresAt.After(s.now()) {
		deny(w, http.StatusUnauthorized)
		return
	}
	account, err := s.queries.GetUser(r.Context(), userUUID)
	if err != nil {
		deny(w, lookupStatus(err))
		return
	}
	if auth.IsTemporarilyDisabledUser(userID, account.Email) {
		deny(w, http.StatusForbidden)
		return
	}
	workspaceID, err := util.ParseUUID(r.Header.Get("X-Workspace-ID"))
	if err != nil || r.Header.Get("X-Workspace-Slug") != "" || r.URL.RawQuery != "" {
		deny(w, http.StatusBadRequest)
		return
	}
	_, err = s.queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{UserID: userUUID, WorkspaceID: workspaceID})
	if err != nil {
		deny(w, lookupStatus(err))
		return
	}
	workspace, err := s.queries.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		deny(w, lookupStatus(err))
		return
	}
	entropy := make([]byte, 32)
	if _, err := rand.Read(entropy); err != nil {
		deny(w, http.StatusServiceUnavailable)
		return
	}
	token := Prefix + base64.RawURLEncoding.EncodeToString(entropy)
	s.mu.Lock()
	now := s.now()
	for key, g := range s.grants {
		if !g.expiresAt.After(now) {
			delete(s.grants, key)
		}
	}
	activeForParent := 0
	for _, g := range s.grants {
		if g.parentHash == auth.HashToken(raw) {
			activeForParent++
		}
	}
	if len(s.grants) >= maxSessions || activeForParent >= 8 {
		s.mu.Unlock()
		deny(w, http.StatusTooManyRequests)
		return
	}
	s.grants[auth.HashToken(token)] = grant{userID: userUUID, workspaceID: workspaceID, parentHash: auth.HashToken(raw), parentPAT: parentPAT, expiresAt: expiresAt}
	s.mu.Unlock()
	respond(w, http.StatusCreated, map[string]any{
		"token": token, "audience": "bluefin-work", "scopes": []string{"issues:read", "agents:read"},
		"user_id": userID, "workspace_id": util.UUIDToString(workspaceID), "workspace_slug": workspace.Slug,
		"expires_at": expiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func permitted(r *http.Request) bool {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return false
	}
	switch {
	case r.Method == http.MethodDelete && r.URL.Path == "/api/shell/session":
		return len(q) == 0
	case r.Method == http.MethodGet && r.URL.Path == "/api/shell/issues":
		for key, values := range q {
			if len(values) != 1 || (key != "limit" && key != "offset") {
				return false
			}
			n, err := strconv.Atoi(values[0])
			if err != nil || n < 0 || (key == "limit" && (n < 1 || n > 100)) || (key == "offset" && n > 1000) {
				return false
			}
		}
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/shell/working-agents":
		return len(q) == 1 && len(q["type"]) == 1 && q.Get("type") == "issue"
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/shell/issues/"):
		suffix := strings.TrimPrefix(r.URL.Path, "/api/shell/issues/")
		if len(q) != 0 {
			return false
		}
		if strings.HasSuffix(suffix, "/preview-publication") {
			return !strings.Contains(strings.TrimSuffix(suffix, "/preview-publication"), "/") &&
				strings.TrimSuffix(suffix, "/preview-publication") != ""
		}
		fallthrough
	case r.Method == http.MethodHead && strings.HasPrefix(r.URL.Path, "/api/shell/issues/"):
		if len(q) != 0 {
			return false
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/shell/issues/"), "/")
		return len(parts) >= 5 && parts[0] != "" && parts[1] == "preview" &&
			parts[2] != "" && parts[3] == "1"
	default:
		return false
	}
}

// Authorize is used only on explicitly registered shell routes. A scoped token
// is opaque, so none of Multica's JWT/PAT/daemon/plugin routes accepts it.
func (s *Sessions) Authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		raw := middleware.BearerToken(r)
		if !strings.HasPrefix(raw, Prefix) {
			deny(w, http.StatusUnauthorized)
			return
		}
		hash := auth.HashToken(raw)
		s.mu.Lock()
		g, ok := s.grants[hash]
		if ok && !g.expiresAt.After(s.now()) {
			delete(s.grants, hash)
			ok = false
		}
		s.mu.Unlock()
		if !ok {
			deny(w, http.StatusUnauthorized)
			return
		}
		if !permitted(r) {
			deny(w, http.StatusForbidden)
			return
		}
		workspaceID := util.UUIDToString(g.workspaceID)
		if r.Header.Get("X-Workspace-Slug") != "" || (r.Header.Get("X-Workspace-ID") != "" && r.Header.Get("X-Workspace-ID") != workspaceID) {
			deny(w, http.StatusForbidden)
			return
		}
		// Revocation works even while the parent or membership is unavailable.
		if r.Method == http.MethodDelete {
			s.mu.Lock()
			delete(s.grants, hash)
			s.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if g.parentPAT {
			pat, err := s.queries.GetPersonalAccessTokenByHash(r.Context(), g.parentHash)
			if err != nil {
				deny(w, lookupStatus(err))
				return
			}
			if pat.UserID != g.userID || pat.Revoked || (pat.ExpiresAt.Valid && !pat.ExpiresAt.Time.After(s.now())) {
				deny(w, http.StatusUnauthorized)
				return
			}
		}
		userID := util.UUIDToString(g.userID)
		account, err := s.queries.GetUser(r.Context(), g.userID)
		if err != nil {
			deny(w, lookupStatus(err))
			return
		}
		if auth.IsTemporarilyDisabledUser(userID, account.Email) {
			deny(w, http.StatusUnauthorized)
			return
		}
		member, err := s.queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{UserID: g.userID, WorkspaceID: g.workspaceID})
		if err != nil {
			deny(w, lookupStatus(err))
			return
		}
		for _, name := range []string{"X-User-ID", "X-User-Email", "X-Agent-ID", "X-Task-ID", "X-Actor-Source"} {
			r.Header.Del(name)
		}
		r.Header.Set("X-User-ID", userID)
		r.Header.Set("X-Workspace-ID", workspaceID)
		next.ServeHTTP(w, r.WithContext(middleware.SetMemberContext(r.Context(), workspaceID, member)))
	})
}
