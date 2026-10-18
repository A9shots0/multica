package ownernotification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// AdmitProjectionInWrite derives and stores every eligible notification in the
// exact transaction that stored the authoritative Current Picture. A crash
// cannot commit one without the other.
func AdmitProjectionInWrite(ctx context.Context, write *workflowstore.IssueWrite, workspaceID, issueID pgtype.UUID, picture db.IssueCurrentPicture, appBaseURL string) ([]db.OwnerNotification, error) {
	if write == nil || !write.MatchesIssue(workspaceID, issueID) || picture.WorkspaceID != workspaceID || picture.IssueID != issueID {
		return nil, workflowstore.ErrInvalidIssueWriteCapability
	}
	binding, err := write.GetOwnerNotificationBinding(ctx, workspaceID, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get owner notification binding: %w", err)
	}
	issue, err := write.GetIssueForNotification(ctx, workspaceID, issueID)
	if err != nil {
		return nil, fmt.Errorf("get notification issue: %w", err)
	}
	workspace, err := write.GetWorkspaceForNotification(ctx, workspaceID, issueID)
	if err != nil {
		return nil, fmt.Errorf("get notification workspace: %w", err)
	}
	summary := workflowprojection.OwnerIssueSummaryFromDocument(picture.Document, picture.Revision, picture.GeneratedAt.Time)
	intents := Evaluate(PolicyInput{
		Summary: summary, IssueID: googleuuid.UUID(issueID.Bytes).String(), IssueTitle: issue.Title,
		WorkspaceSlug: workspace.Slug, AppBaseURL: appBaseURL,
	})
	admitted := make([]db.OwnerNotification, 0, len(intents))
	for _, intent := range intents {
		payload := map[string]any{
			"schema_version": 1, "current_picture_revision": picture.Revision,
			"signal": intent.Payload,
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode owner notification payload: %w", err)
		}
		stable := googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte(fmt.Sprintf("multica-owner-notification:%s:%s:%s", googleuuid.UUID(workspaceID.Bytes), binding.DeliveryMode, intent.EventKey)))
		row, err := write.StoreOwnerNotificationIntent(ctx, db.AdmitOwnerNotificationParams{
			ID: pgtype.UUID{Bytes: stable, Valid: true}, WorkspaceID: workspaceID, IssueID: issueID,
			RecipientID: binding.RecipientID, DeliveryMode: binding.DeliveryMode, EventType: string(intent.EventType),
			EventKey: intent.EventKey, SourceType: intent.SourceType, SourceID: intent.SourceID,
			SourceRevision: intent.SourceRevision, DestinationKey: binding.DestinationKey,
			Message: intent.Message, ActionUrl: intent.ActionURL, Payload: encoded,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("admit owner notification: %w", err)
		}
		admitted = append(admitted, row)
	}
	return admitted, nil
}
