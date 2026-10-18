package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const testResolverSlug = "middleware-resolver-test"

// openPool returns a connected pgxpool, or skips the test if the database is
// unreachable. Mirrors the handler package's fixture approach so tests don't
// require a DB in environments where one isn't available.
func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("skipping: could not connect to database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("skipping: database not reachable: %v", err)
	}
	return pool
}

// setupResolverFixture inserts a workspace with a known slug and returns its
// UUID. The caller is responsible for calling the returned cleanup func.
func setupResolverFixture(t *testing.T, pool *pgxpool.Pool) (workspaceID string, cleanup func()) {
	t.Helper()
	ctx := context.Background()
	// Pre-cleanup in case a previous run didn't finish.
	_, _ = pool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, testResolverSlug)

	if err := pool.QueryRow(ctx,
		`INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ($1, $2, '', 'MRT') RETURNING id`,
		"Middleware Resolver Test", testResolverSlug,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	return workspaceID, func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, testResolverSlug)
	}
}

// TestWorkspaceSlugResolution verifies the priority order and error behaviour of
// resolveWorkspaceUUID using an injected slug lookup — no database required.
//
// The critical case is that the X-Workspace-Slug header is checked before the
// ?workspace_slug query parameter, matching ResolveWorkspaceIDFromRequest and the
// function's own documented priority. The original implementation had these two
// reversed, which this test encodes as a hard failure.
func TestWorkspaceSlugResolution(t *testing.T) {
	const (
		knownSlug  = "known-workspace"
		knownUUID  = "11111111-1111-1111-1111-111111111111"
		boundUUID  = "22222222-2222-2222-2222-222222222222"
		directUUID = "33333333-3333-3333-3333-333333333333"
	)

	fakeWS := db.Workspace{
		ID:   util.MustParseUUID(knownUUID),
		Slug: knownSlug,
	}

	fakeLookup := slugLookupFn(func(_ context.Context, slug string) (db.Workspace, error) {
		if slug == knownSlug {
			return fakeWS, nil
		}
		return db.Workspace{}, errors.New("not found")
	})

	resolver := resolveWorkspaceUUID(fakeLookup)

	cases := []struct {
		name    string
		setup   func(r *http.Request)
		wantID  string
		wantErr error
	}{
		{
			name: "header slug resolves to UUID",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", knownSlug)
			},
			wantID: knownUUID,
		},
		{
			name: "query param slug resolves to UUID",
			setup: func(r *http.Request) {
				q := r.URL.Query()
				q.Set("workspace_slug", knownSlug)
				r.URL.RawQuery = q.Encode()
			},
			wantID: knownUUID,
		},
		{
			// The header must be checked before the query param (matching
			// ResolveWorkspaceIDFromRequest and the docstring). Before this fix,
			// resolveWorkspaceUUID checked ?workspace_slug first, so setting the
			// header to a valid slug and the query to an unknown slug would
			// return errWorkspaceNotFound instead of the header's UUID.
			name: "slug header wins over slug query param when both are present",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", knownSlug)
				q := r.URL.Query()
				q.Set("workspace_slug", "does-not-exist")
				r.URL.RawQuery = q.Encode()
			},
			wantID: knownUUID,
		},
		{
			name: "unknown header slug returns errWorkspaceNotFound",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", "does-not-exist")
			},
			wantErr: errWorkspaceNotFound,
		},
		{
			name: "unknown query slug returns errWorkspaceNotFound",
			setup: func(r *http.Request) {
				q := r.URL.Query()
				q.Set("workspace_slug", "does-not-exist")
				r.URL.RawQuery = q.Encode()
			},
			wantErr: errWorkspaceNotFound,
		},
		{
			name: "task_token binding ignores all slug params and returns bound workspace",
			setup: func(r *http.Request) {
				r.Header.Set("X-Actor-Source", "task_token")
				r.Header.Set("X-Workspace-ID", boundUUID)
				r.Header.Set("X-Workspace-Slug", knownSlug)
				q := r.URL.Query()
				q.Set("workspace_slug", knownSlug)
				r.URL.RawQuery = q.Encode()
			},
			wantID: boundUUID,
		},
		{
			name: "task_token with no workspace ID returns errWorkspaceNotFound",
			setup: func(r *http.Request) {
				r.Header.Set("X-Actor-Source", "task_token")
			},
			wantErr: errWorkspaceNotFound,
		},
		{
			name: "falls back to X-Workspace-ID header when no slug given",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-ID", directUUID)
			},
			wantID: directUUID,
		},
		{
			name: "falls back to workspace_id query param as last resort",
			setup: func(r *http.Request) {
				q := r.URL.Query()
				q.Set("workspace_id", directUUID)
				r.URL.RawQuery = q.Encode()
			},
			wantID: directUUID,
		},
		{
			name:   "no identifier returns empty string with no error",
			setup:  func(r *http.Request) {},
			wantID: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			tc.setup(req)

			gotID, gotErr := resolver(req)

			if tc.wantErr != nil {
				if gotErr != tc.wantErr {
					t.Fatalf("want error %v, got error %v (id=%q)", tc.wantErr, gotErr, gotID)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("unexpected error: %v", gotErr)
			}
			if gotID != tc.wantID {
				t.Fatalf("want workspace ID %q, got %q", tc.wantID, gotID)
			}
		})
	}
}

// TestResolveWorkspaceIDFromRequest pins down the priority order of the
// shared resolver. Every handler-level lookup of workspace identity — whether
// a route sits inside or outside the workspace middleware — must produce
// identical results, in the same priority, across all five supported
// mechanisms. Breaking any row here is a behavioral regression.
func TestResolveWorkspaceIDFromRequest(t *testing.T) {
	pool := openPool(t)
	defer pool.Close()
	queries := db.New(pool)

	workspaceID, cleanup := setupResolverFixture(t, pool)
	defer cleanup()

	const (
		uuidA = "00000000-0000-0000-0000-000000000001"
		uuidB = "00000000-0000-0000-0000-000000000002"
	)

	cases := []struct {
		name      string
		setup     func(r *http.Request)
		want      string
		wantEmpty bool
	}{
		{
			name: "context UUID wins over everything else",
			setup: func(r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKeyWorkspaceID, uuidA)
				*r = *r.WithContext(ctx)
				r.Header.Set("X-Workspace-Slug", testResolverSlug)
				r.Header.Set("X-Workspace-ID", uuidB)
			},
			want: uuidA,
		},
		{
			name: "X-Workspace-Slug header resolves to UUID via DB lookup",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", testResolverSlug)
			},
			want: workspaceID,
		},
		{
			name: "X-Workspace-Slug wins over X-Workspace-ID (post-refactor priority)",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", testResolverSlug)
				r.Header.Set("X-Workspace-ID", uuidB)
			},
			want: workspaceID,
		},
		{
			name: "unknown X-Workspace-Slug falls through to UUID header",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", "does-not-exist")
				r.Header.Set("X-Workspace-ID", uuidB)
			},
			want: uuidB,
		},
		{
			name: "?workspace_slug query resolves to UUID via DB lookup",
			setup: func(r *http.Request) {
				q := r.URL.Query()
				q.Set("workspace_slug", testResolverSlug)
				r.URL.RawQuery = q.Encode()
			},
			want: workspaceID,
		},
		{
			name: "X-Workspace-ID header is returned when no slug provided",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-ID", uuidA)
			},
			want: uuidA,
		},
		{
			name: "?workspace_id query is the last-resort fallback",
			setup: func(r *http.Request) {
				q := r.URL.Query()
				q.Set("workspace_id", uuidA)
				r.URL.RawQuery = q.Encode()
			},
			want: uuidA,
		},
		{
			name:      "no identifier at all returns empty",
			setup:     func(r *http.Request) {},
			wantEmpty: true,
		},
		{
			name: "unknown slug with no UUID fallback returns empty",
			setup: func(r *http.Request) {
				r.Header.Set("X-Workspace-Slug", "does-not-exist")
			},
			wantEmpty: true,
		},
		{
			// MUL-2600: a mat_ task token authenticates the request and
			// the auth middleware writes the token-bound workspace into
			// X-Workspace-ID along with X-Actor-Source=task_token. Any
			// other workspace identifier the agent puts on the wire — a
			// slug pointing at a sibling workspace, a different
			// workspace_id — must be ignored. Otherwise an agent could
			// route owner-token traffic at any workspace its host is
			// also a member of.
			name: "task_token actor: client-supplied slug/id cannot override token-bound workspace",
			setup: func(r *http.Request) {
				r.Header.Set("X-Actor-Source", "task_token")
				r.Header.Set("X-Workspace-ID", uuidA)
				// All of these should be ignored under task_token.
				r.Header.Set("X-Workspace-Slug", testResolverSlug)
				q := r.URL.Query()
				q.Set("workspace_slug", testResolverSlug)
				q.Set("workspace_id", uuidB)
				r.URL.RawQuery = q.Encode()
			},
			want: uuidA,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/anything", nil)
			tc.setup(req)

			got := ResolveWorkspaceIDFromRequest(req, queries)

			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty, got %q", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
