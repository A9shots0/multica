package handler

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatWorkHistoryCursorAndUnlink(t *testing.T) {
	_, sessionID, _, _ := setupDirectChatSession(t, context.Background(), "Paged work")
	issueID := dbfx.Issue(t, "Long running review")
	linkDeliveryIssue(t, sessionID, issueID)
	stamp := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	for i := 0; i < 60; i++ {
		dbfx.Insert(t, "issue_workflow_event", testutil.Cols{"id": uuid.NewString(), "workspace_id": testWorkspaceID, "issue_id": issueID, "source_task_id": uuid.NewString(), "actor_id": testUserID, "event_type": "review", "reason": "Recorded review", "decision": `{}`, "created_at": stamp})
	}
	read := func(query string) service.ChatWork {
		var work service.ChatWork
		r := withURLParam(withChatTestWorkspaceCtx(t, newRequest("GET", "/work"+query, nil)), "sessionId", sessionID)
		testutil.Call(t, testHandler.GetChatWork, r).Want(200).JSON(&work)
		return work
	}
	first := read("")
	if len(first.Activity) != 50 || !first.HasOlder {
		t.Fatalf("first page: %d older=%v", len(first.Activity), first.HasOlder)
	}
	last := first.Activity[49]
	cursor := "?before_time=" + url.QueryEscape(last.CreatedAt.Time.Format(time.RFC3339Nano)) + "&before_id=" + uuidToString(last.ID)
	second := read(cursor)
	if len(second.Activity) != 10 || second.HasOlder || len(second.Issues) != 1 {
		t.Fatalf("second page: %d older=%v issues=%d", len(second.Activity), second.HasOlder, len(second.Issues))
	}
	seen := map[string]bool{}
	for _, event := range append(first.Activity, second.Activity...) {
		id := uuidToString(event.ID)
		if seen[id] {
			t.Fatal("overlapping history pages")
		}
		seen[id] = true
	}
	dbfx.Exec(t, `DELETE FROM chat_issue_link WHERE chat_session_id=$1 AND issue_id=$2`, sessionID, issueID)
	if unlinked := read(cursor); len(unlinked.Activity) != 0 || len(unlinked.Issues) != 0 {
		t.Fatal("unlinked issue still visible through saved cursor")
	}
}
