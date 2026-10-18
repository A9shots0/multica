package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ErrChatReplyTargetChanged is deliberately enumeration-safe. A caller gets
// the same conflict whether the supplied id is missing, belongs to another
// Chat, names a hidden control-plane event, or names a stale sub-target.
var ErrChatReplyTargetChanged = errors.New("reply target is no longer available")

// ChatReplyTarget is the member's explicit selection. EventID is the durable
// chat_message id; optional sub-targets identify a block or review artifact
// inside that exact event.
type ChatReplyTarget struct {
	EventID    string `json:"event_id"`
	BlockID    string `json:"block_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
}

type ChatReplyAttachmentRef struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
}

type ChatReplyArtifactRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// ChatReplyContext is an immutable, server-authored snapshot. It gives both
// the UI and model the same exact reference without granting workflow-action
// authority or depending on later mutable display state.
type ChatReplyContext struct {
	SchemaVersion    int                      `json:"schema_version"`
	EventID          string                   `json:"event_id"`
	BlockID          string                   `json:"block_id,omitempty"`
	ArtifactID       string                   `json:"artifact_id,omitempty"`
	Actor            string                   `json:"actor"`
	MessageKind      string                   `json:"message_kind"`
	Content          string                   `json:"content"`
	ContentSHA256    string                   `json:"content_sha256"`
	ContentTruncated bool                     `json:"content_truncated"`
	CreatedAt        string                   `json:"created_at"`
	WorkIssueID      string                   `json:"work_issue_id,omitempty"`
	RequestID        string                   `json:"request_id,omitempty"`
	Attachments      []ChatReplyAttachmentRef `json:"attachments"`
	Artifacts        []ChatReplyArtifactRef   `json:"artifacts"`
}

// CaptureChatReplyTarget validates and snapshots a member-selected target
// using the same transaction that owns the new turn. Intervening messages do
// not matter: identity is by exact durable id, never by transcript position.
func CaptureChatReplyTarget(ctx context.Context, q *db.Queries, session db.ChatSession, target ChatReplyTarget) (pgtype.UUID, []byte, error) {
	targetID, err := util.ParseUUID(strings.TrimSpace(target.EventID))
	if err != nil {
		return pgtype.UUID{}, nil, ErrChatReplyTargetChanged
	}
	message, err := q.GetVisibleChatMessageForReply(ctx, db.GetVisibleChatMessageForReplyParams{
		MessageID: targetID, ChatSessionID: session.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, nil, ErrChatReplyTargetChanged
	}
	if err != nil {
		return pgtype.UUID{}, nil, err
	}
	if message.Role != "user" && message.Role != "assistant" {
		return pgtype.UUID{}, nil, ErrChatReplyTargetChanged
	}

	contentRunes := []rune(message.Content)
	content := message.Content
	contentTruncated := len(contentRunes) > 8000
	if contentTruncated {
		content = string(contentRunes[:8000]) + "…"
	}
	contentHash := sha256.Sum256([]byte(message.Content))
	snapshot := ChatReplyContext{
		SchemaVersion:    1,
		EventID:          util.UUIDToString(message.ID),
		BlockID:          strings.TrimSpace(target.BlockID),
		ArtifactID:       strings.TrimSpace(target.ArtifactID),
		Actor:            message.Role,
		MessageKind:      message.MessageKind,
		Content:          content,
		ContentSHA256:    hex.EncodeToString(contentHash[:]),
		ContentTruncated: contentTruncated,
		CreatedAt:        util.TimestampToString(message.CreatedAt),
		Attachments:      make([]ChatReplyAttachmentRef, 0),
		Artifacts:        make([]ChatReplyArtifactRef, 0),
	}
	if message.WorkIssueID.Valid {
		snapshot.WorkIssueID = util.UUIDToString(message.WorkIssueID)
	}

	attachments, err := q.ListAttachmentsByChatMessage(ctx, db.ListAttachmentsByChatMessageParams{
		ChatMessageID: message.ID, WorkspaceID: session.WorkspaceID,
	})
	if err != nil {
		return pgtype.UUID{}, nil, err
	}
	for _, attachment := range attachments {
		snapshot.Attachments = append(snapshot.Attachments, ChatReplyAttachmentRef{
			ID: util.UUIDToString(attachment.ID), Filename: attachment.Filename, ContentType: attachment.ContentType,
		})
	}

	var update ChatWorkUpdate
	if len(message.WorkUpdate) > 0 {
		if err := json.Unmarshal(message.WorkUpdate, &update); err != nil {
			return pgtype.UUID{}, nil, ErrChatReplyTargetChanged
		}
		if update.IssueID != "" {
			snapshot.WorkIssueID = update.IssueID
		}
		snapshot.RequestID = update.RequestID
		if update.Review != nil {
			for _, artifact := range update.Review.Artifacts {
				snapshot.Artifacts = append(snapshot.Artifacts, ChatReplyArtifactRef{
					ID: artifact.ID, Label: artifact.Label, Kind: artifact.Kind,
				})
			}
		}
	}

	if snapshot.BlockID != "" && snapshot.BlockID != update.SourceID && snapshot.BlockID != update.RequestID {
		return pgtype.UUID{}, nil, ErrChatReplyTargetChanged
	}
	if snapshot.ArtifactID != "" {
		found := false
		for _, artifact := range snapshot.Artifacts {
			if artifact.ID == snapshot.ArtifactID {
				found = true
				break
			}
		}
		if !found {
			return pgtype.UUID{}, nil, ErrChatReplyTargetChanged
		}
	}

	raw, err := json.Marshal(snapshot)
	if err != nil {
		return pgtype.UUID{}, nil, err
	}
	return targetID, raw, nil
}
