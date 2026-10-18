package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// buildChatSearchQuery mirrors buildProjectSearchQuery's shape (search.go's
// doc comment on searchStatementTimeout explains why: LIKE '%pattern%'
// against the pg_bigm GIN indexes added in migration 581). Chat is private
// per creator — unlike issues/projects there is no workspace-wide visibility
// to reason about, so every clause scopes to creator_id as well as
// workspace_id.
//
// A session matches on its own title OR on any message inside it; the
// "matched_messages" CTE picks each matching session's most recent matching
// message as the row's snippet, so a hit deep in a long conversation still
// shows the reader why it matched instead of just the (possibly unrelated)
// title.
func buildChatSearchQuery(phrase string, terms []string, includeArchived bool) (string, []any) {
	phrase = strings.ToLower(phrase)
	for i, t := range terms {
		terms[i] = strings.ToLower(t)
	}

	argIdx := 1
	args := []any{}
	nextArg := func(val any) string {
		args = append(args, val)
		s := fmt.Sprintf("$%d", argIdx)
		argIdx++
		return s
	}

	escapedPhrase := escapeLike(phrase)
	phraseParam := nextArg(escapedPhrase)
	phraseContains := "'%' || " + phraseParam + " || '%'"
	phraseStartsWith := phraseParam + " || '%'"

	wsParam := nextArg(nil)   // workspace_id placeholder
	userParam := nextArg(nil) // creator_id placeholder

	var termParams []string
	if len(terms) > 1 {
		for _, t := range terms {
			et := escapeLike(t)
			termParams = append(termParams, nextArg(et))
		}
	}

	titleMatch := fmt.Sprintf("LOWER(s.title) LIKE %s", phraseContains)
	titleAllWords := ""
	if len(termParams) > 1 {
		var titleTerms []string
		for _, tp := range termParams {
			titleTerms = append(titleTerms, fmt.Sprintf("LOWER(s.title) LIKE '%s' || %s || '%s'", "%", tp, "%"))
		}
		titleAllWords = strings.Join(titleTerms, " AND ")
	}

	statusFilter := ""
	if !includeArchived {
		statusFilter = " AND s.status <> 'archived'"
	}

	rankCases := []string{
		fmt.Sprintf("WHEN LOWER(s.title) = %s THEN 0", phraseParam),
		fmt.Sprintf("WHEN LOWER(s.title) LIKE %s THEN 1", phraseStartsWith),
		fmt.Sprintf("WHEN %s THEN 2", titleMatch),
	}
	if titleAllWords != "" {
		rankCases = append(rankCases, fmt.Sprintf("WHEN (%s) THEN 2", titleAllWords))
	}
	rankExpr := "CASE " + strings.Join(rankCases, " ") + " ELSE 3 END"

	matchSourceExpr := fmt.Sprintf(`CASE WHEN %s THEN 'title'`, titleMatch)
	if titleAllWords != "" {
		matchSourceExpr += fmt.Sprintf(" WHEN (%s) THEN 'title'", titleAllWords)
	}
	matchSourceExpr += " ELSE 'message' END"

	limitParam := nextArg(nil)
	offsetParam := nextArg(nil)

	query := fmt.Sprintf(`WITH matched_messages AS (
		SELECT DISTINCT ON (m.chat_session_id) m.chat_session_id, m.content, m.created_at
		FROM chat_message m
		JOIN chat_session cs ON cs.id = m.chat_session_id
		WHERE cs.workspace_id = %s AND cs.creator_id = %s AND LOWER(m.content) LIKE %s
		ORDER BY m.chat_session_id, m.created_at DESC
	)
	SELECT s.id, s.workspace_id, s.agent_id, s.creator_id, s.title, s.session_id, s.work_dir,
		s.status, s.created_at, s.updated_at, s.unread_since, s.runtime_id, s.last_read_at,
		s.is_agent_intro, s.pinned_at, s.project_id, s.explicitly_created_at,
		%s AS match_source,
		COALESCE(mm.content, '') AS matched_snippet
	FROM chat_session s
	LEFT JOIN matched_messages mm ON mm.chat_session_id = s.id
	WHERE s.workspace_id = %s AND s.creator_id = %s%s
		AND (%s OR mm.chat_session_id IS NOT NULL)
	ORDER BY %s, s.updated_at DESC
	LIMIT %s OFFSET %s`,
		wsParam, userParam, phraseContains,
		matchSourceExpr,
		wsParam, userParam, statusFilter,
		titleMatch,
		rankExpr,
		limitParam, offsetParam,
	)

	return query, args
}

type chatSearchRow struct {
	session        db.ChatSession
	matchSource    string
	matchedSnippet string
}

// SearchChatSessions is the "Chats" group behind the Cmd+K command palette
// (see MULTICA_LLM_* config for the other, unrelated LLM-based chat feature —
// this one is plain full-text, no model call). Scoped to the requester's own
// sessions only: unlike issues/projects, a chat is not workspace-shared, and
// searching someone else's private conversation would be a real privacy leak,
// not just an odd result.
func (h *Handler) SearchChatSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	workspaceID := ctxWorkspaceID(ctx)

	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 50 {
		limit = 50
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	terms := splitSearchTerms(q)

	sqlQuery, args := buildChatSearchQuery(q, terms, includeArchived)
	// nextArg emitted nil placeholders for ws/user/limit/offset in the order
	// buildChatSearchQuery calls it — same convention buildProjectSearchQuery
	// uses, so the fixed indices below match its argument layout exactly.
	args[1] = wsUUID
	args[2] = userUUID
	args[len(args)-2] = limit
	args[len(args)-1] = offset

	var results []chatSearchRow
	err := runSearchQuery(ctx, h.TxStarter, sqlQuery, args, func(rows pgx.Rows) error {
		for rows.Next() {
			var row chatSearchRow
			if err := rows.Scan(
				&row.session.ID,
				&row.session.WorkspaceID,
				&row.session.AgentID,
				&row.session.CreatorID,
				&row.session.Title,
				&row.session.SessionID,
				&row.session.WorkDir,
				&row.session.Status,
				&row.session.CreatedAt,
				&row.session.UpdatedAt,
				&row.session.UnreadSince,
				&row.session.RuntimeID,
				&row.session.LastReadAt,
				&row.session.IsAgentIntro,
				&row.session.PinnedAt,
				&row.session.ProjectID,
				&row.session.ExplicitlyCreatedAt,
				&row.matchSource,
				&row.matchedSnippet,
			); err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			results = append(results, row)
		}
		return rows.Err()
	})
	if err != nil {
		if isSearchStatementTimeout(err) {
			slog.Warn("search chats timed out",
				"workspace_id", workspaceID,
				"user_id", userID,
				"query", q,
				"timeout", searchStatementTimeout)
			writeError(w, http.StatusServiceUnavailable, "search timed out; please refine your query or try again")
			return
		}
		slog.Error("search chats failed", "error", err)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	type chatSearchResult struct {
		ID             string `json:"id"`
		WorkspaceID    string `json:"workspace_id"`
		AgentID        string `json:"agent_id"`
		CreatorID      string `json:"creator_id"`
		Title          string `json:"title"`
		Status         string `json:"status"`
		ProjectID      string `json:"project_id,omitempty"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
		MatchSource    string `json:"match_source"`
		MatchedSnippet string `json:"matched_snippet,omitempty"`
	}
	out := make([]chatSearchResult, 0, len(results))
	for _, row := range results {
		out = append(out, chatSearchResult{
			ID:             uuidToString(row.session.ID),
			WorkspaceID:    uuidToString(row.session.WorkspaceID),
			AgentID:        uuidToString(row.session.AgentID),
			CreatorID:      uuidToString(row.session.CreatorID),
			Title:          row.session.Title,
			Status:         row.session.Status,
			ProjectID:      uuidToString(row.session.ProjectID),
			CreatedAt:      timestampToString(row.session.CreatedAt),
			UpdatedAt:      timestampToString(row.session.UpdatedAt),
			MatchSource:    row.matchSource,
			MatchedSnippet: row.matchedSnippet,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": out})
}
