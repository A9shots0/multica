package workflowcommand

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNotifyProjectionRequiresCommittedPositiveRevision(t *testing.T) {
	workspaceID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	issueID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	calls := 0
	service := Service{OnProjection: func(_ context.Context, gotWorkspace, gotIssue pgtype.UUID, revision int64) {
		calls++
		if gotWorkspace != workspaceID || gotIssue != issueID || revision != 4 {
			t.Fatalf("notification = %#v/%#v/%d", gotWorkspace, gotIssue, revision)
		}
	}}
	service.NotifyProjection(t.Context(), workspaceID, issueID, Receipt{})
	service.NotifyProjection(t.Context(), workspaceID, issueID, Receipt{CurrentPictureRevision: 4})
	if calls != 1 {
		t.Fatalf("notifications = %d, want 1", calls)
	}
}
