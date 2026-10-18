package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	meaningfulActivitySchemaVersion = 1
	meaningfulActivityDefaultLimit  = 20
	meaningfulActivityMaxLimit      = 100
)

type meaningfulActivityCursor struct {
	Version     int       `json:"v"`
	WorkspaceID string    `json:"workspace_id"`
	IssueID     string    `json:"issue_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	RecordType  string    `json:"record_type"`
	RecordID    string    `json:"record_id"`
	Revision    int64     `json:"revision"`
}

type meaningfulActivityResponse struct {
	SchemaVersion          int                                        `json:"schema_version"`
	IssueID                string                                     `json:"issue_id"`
	CurrentPictureRevision int64                                      `json:"current_picture_revision"`
	Items                  []workflowprojection.OwnerMeaningfulChange `json:"items"`
	NextCursor             *string                                    `json:"next_cursor"`
}

func meaningfulActivityItems(issueID string, rows []db.ListIssueWorkflowMeaningfulActivityRow) ([]workflowprojection.OwnerMeaningfulChange, error) {
	items := make([]workflowprojection.OwnerMeaningfulChange, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		id := uuidToString(row.ID)
		identity := fmt.Sprintf("%s/%s/%d", row.RecordType, id, row.Revision)
		if seen[identity] {
			return nil, errors.New("duplicate meaningful activity identity")
		}
		seen[identity] = true
		if !row.CreatedAt.Valid || !row.ID.Valid {
			slog.Warn("skipping malformed meaningful activity row", "issue_id", issueID, "record", identity)
			continue
		}
		item, err := workflowprojection.MeaningfulChangeFromRecord(workflowprojection.Record{
			Type: row.RecordType, ID: id, Revision: row.Revision, CreatedAt: row.CreatedAt.Time, Payload: row.Payload,
		})
		if err != nil {
			slog.Warn("skipping malformed meaningful activity row", "issue_id", issueID, "record", identity, "error", err)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func decodeMeaningfulActivityCursor(raw, workspaceID, issueID string) (*meaningfulActivityCursor, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	var cursor meaningfulActivityCursor
	if err := decoder.Decode(&cursor); err != nil {
		return nil, fmt.Errorf("decode cursor payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("cursor has trailing data")
	}
	if cursor.Version != meaningfulActivitySchemaVersion || cursor.WorkspaceID != workspaceID || cursor.IssueID != issueID ||
		cursor.OccurredAt.IsZero() || strings.TrimSpace(cursor.RecordType) == "" || strings.TrimSpace(cursor.RecordID) == "" || cursor.Revision < 1 {
		return nil, errors.New("cursor identity is invalid")
	}
	recordID, err := util.ParseUUID(cursor.RecordID)
	if err != nil || !recordID.Valid || !meaningfulActivityRecordType(cursor.RecordType) {
		return nil, errors.New("cursor record is invalid")
	}
	return &cursor, nil
}

func meaningfulActivityRecordType(value string) bool {
	switch value {
	case "outcome", "scope", "authority", "execution", "decision", "decision_response", "candidate", "review", "release", "evidence", "incident", "consequence", "regression_change":
		return true
	default:
		return false
	}
}

func encodeMeaningfulActivityCursor(workspaceID, issueID string, row db.ListIssueWorkflowMeaningfulActivityRow) (string, error) {
	if !row.CreatedAt.Valid || !row.ID.Valid || row.Revision < 1 {
		return "", errors.New("cannot cursor malformed activity row")
	}
	payload, err := json.Marshal(meaningfulActivityCursor{
		Version: meaningfulActivitySchemaVersion, WorkspaceID: workspaceID, IssueID: issueID,
		OccurredAt: row.CreatedAt.Time.UTC(), RecordType: row.RecordType,
		RecordID: uuidToString(row.ID), Revision: row.Revision,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func meaningfulActivityLimit(raw string) (int32, error) {
	if raw == "" {
		return meaningfulActivityDefaultLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > meaningfulActivityMaxLimit {
		return 0, errors.New("limit must be an integer between 1 and 100")
	}
	return int32(value), nil
}

// GetIssueMeaningfulActivity returns only normalized, owner-meaningful records.
// A malformed record is isolated from its healthy neighbours and the opaque
// cursor is bound to the authenticated workspace and issue.
func (h *Handler) GetIssueMeaningfulActivity(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	limit, err := meaningfulActivityLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	workspaceID, issueID := uuidToString(issue.WorkspaceID), uuidToString(issue.ID)
	cursor, err := decodeMeaningfulActivityCursor(r.URL.Query().Get("cursor"), workspaceID, issueID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid meaningful activity cursor")
		return
	}

	picture, err := h.Queries.GetIssueCurrentPicture(r.Context(), db.GetIssueCurrentPictureParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "issue current picture is unavailable")
		return
	}
	if err != nil || picture.WorkspaceID != issue.WorkspaceID || picture.IssueID != issue.ID || picture.Revision < 1 {
		slog.Warn("load meaningful activity picture failed", append(logger.RequestAttrs(r), "issue_id", issueID, "error", err)...)
		writeError(w, http.StatusInternalServerError, "issue meaningful activity is unavailable")
		return
	}

	params := db.ListIssueWorkflowMeaningfulActivityParams{
		WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, PageSize: limit + 1,
	}
	if cursor != nil {
		params.CursorTime = pgtype.Timestamptz{Time: cursor.OccurredAt, Valid: true}
		params.CursorType = pgtype.Text{String: cursor.RecordType, Valid: true}
		params.CursorID = parseUUID(cursor.RecordID)
		params.CursorRevision = pgtype.Int8{Int64: cursor.Revision, Valid: true}
	}
	rows, err := h.Queries.ListIssueWorkflowMeaningfulActivity(r.Context(), params)
	if err != nil {
		slog.Warn("load meaningful activity failed", append(logger.RequestAttrs(r), "issue_id", issueID, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to load meaningful activity")
		return
	}
	hasMore := len(rows) > int(limit)
	if hasMore {
		rows = rows[:limit]
	}
	items, mapErr := meaningfulActivityItems(issueID, rows)
	if mapErr != nil {
		writeError(w, http.StatusInternalServerError, "issue meaningful activity is unavailable")
		return
	}
	var nextCursor *string
	if hasMore && len(rows) > 0 {
		encoded, encodeErr := encodeMeaningfulActivityCursor(workspaceID, issueID, rows[len(rows)-1])
		if encodeErr != nil {
			writeError(w, http.StatusInternalServerError, "issue meaningful activity is unavailable")
			return
		}
		nextCursor = &encoded
	}
	writeJSON(w, http.StatusOK, meaningfulActivityResponse{
		SchemaVersion: meaningfulActivitySchemaVersion, IssueID: issueID,
		CurrentPictureRevision: picture.Revision, Items: items, NextCursor: nextCursor,
	})
}
