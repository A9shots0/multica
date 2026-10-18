package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func inboxRequest(method, path, workspaceID string) *http.Request {
	return testutil.WithHeaders(
		testutil.JSONRequest(method, path, nil),
		"X-User-ID", testUserID,
		"X-Workspace-ID", workspaceID,
	)
}

func inboxWorkspaceHandler(handler http.HandlerFunc) http.HandlerFunc {
	return middleware.RequireWorkspaceMember(testHandler.Queries)(handler).ServeHTTP
}

func TestListInboxProjectsCurrentIssueStatusAndPriority(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Inbox filter projections", "inbox-filter-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	issueID := dbfx.Issue(t, "Filtered issue", testutil.Cols{
		"workspace_id": workspaceID,
		"status":       "in_review",
		"priority":     "high",
	})
	dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id":   workspaceID,
		"recipient_type": "member",
		"recipient_id":   testUserID,
		"type":           "status_changed",
		"severity":       "info",
		"issue_id":       issueID,
		"title":          "Projected issue",
		"details":        testutil.Raw(`'{"owner_relevant":"true"}'::jsonb`),
	})

	var items []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox),
		inboxRequest(http.MethodGet, "/api/inbox", workspaceID)).
		Want(http.StatusOK).
		JSON(&items)

	if len(items) != 1 {
		t.Fatalf("inbox items = %d, want 1: %+v", len(items), items)
	}
	if items[0].IssueStatus == nil || *items[0].IssueStatus != "in_review" {
		t.Errorf("issue_status = %v, want in_review", items[0].IssueStatus)
	}
	if items[0].IssuePriority == nil || *items[0].IssuePriority != "high" {
		t.Errorf("issue_priority = %v, want high", items[0].IssuePriority)
	}
}

func TestListInboxIncludesActionRequiredWithoutStructuredMetadata(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Inbox action required fallback", "inbox-action-required-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	issueID := dbfx.Issue(t, "Assignment requires attention", testutil.Cols{"workspace_id": workspaceID})
	dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id":   workspaceID,
		"recipient_type": "member",
		"recipient_id":   testUserID,
		"type":           "issue_assigned",
		"severity":       "action_required",
		"issue_id":       issueID,
		"title":          "Assignment",
		"details":        testutil.Raw(`'{}'::jsonb`),
	})

	var items []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox),
		inboxRequest(http.MethodGet, "/api/inbox", workspaceID)).
		Want(http.StatusOK).
		JSON(&items)

	if len(items) != 1 || items[0].Type != "issue_assigned" {
		t.Fatalf("action-required item = %+v, want one visible assignment", items)
	}
	var details map[string]any
	if err := json.Unmarshal(items[0].Details, &details); err != nil {
		t.Fatalf("decode normalized details: %v", err)
	}
	if details["owner_relevant"] != "true" || details["required_action"] != "review_assignment" {
		t.Fatalf("normalized details = %#v, want owner action metadata", details)
	}
}

// Owner-control records are independent (archiving one never hides a sibling),
// so the archived list is the newest 200 owner-relevant records, not the newest
// 200 issue groups. Records that are not owner-relevant never appear.
func TestListArchivedInboxReturnsNewestOwnerRelevantRecords(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Archived inbox records", "archived-records-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	noisyIssueID := dbfx.Issue(t, "Noisy archived issue", testutil.Cols{"workspace_id": workspaceID})
	olderIssueID := dbfx.Issue(t, "Older archived issue", testutil.Cols{"workspace_id": workspaceID})

	base := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 200; i++ {
		dbfx.Insert(t, "inbox_item", testutil.Cols{
			"workspace_id":   workspaceID,
			"recipient_type": "member",
			"recipient_id":   testUserID,
			"type":           "status_changed",
			"severity":       "info",
			"issue_id":       noisyIssueID,
			"title":          fmt.Sprintf("noisy-%03d", i),
			"archived":       true,
			"created_at":     base.Add(-time.Duration(i) * time.Millisecond),
			"details":        testutil.Raw(`'{"owner_relevant":"true"}'::jsonb`),
		})
	}
	dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id":   workspaceID,
		"recipient_type": "member",
		"recipient_id":   testUserID,
		"type":           "new_comment",
		"severity":       "info",
		"issue_id":       olderIssueID,
		"title":          "older-record",
		"archived":       true,
		"created_at":     base.Add(-time.Hour),
		"details":        testutil.Raw(`'{"owner_relevant":"true"}'::jsonb`),
	})
	dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id":   workspaceID,
		"recipient_type": "member",
		"recipient_id":   testUserID,
		"type":           "new_comment",
		"severity":       "info",
		"issue_id":       olderIssueID,
		"title":          "not-owner-relevant",
		"archived":       true,
		"created_at":     base.Add(time.Second),
	})

	var items []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListArchivedInbox),
		inboxRequest(http.MethodGet, "/api/inbox/archived", workspaceID)).
		Want(http.StatusOK).
		JSON(&items)

	if len(items) != 200 {
		t.Fatalf("archived records = %d, want the newest 200", len(items))
	}
	if items[0].Title != "noisy-000" {
		t.Errorf("first archived record = %q, want the newest owner-relevant record noisy-000", items[0].Title)
	}
	for _, item := range items {
		if item.Title == "older-record" || item.Title == "not-owner-relevant" {
			t.Errorf("archived list includes %q", item.Title)
		}
	}
}

func TestArchiveAllReadInboxUsesNewestIssueRow(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Archive read groups", "archive-read-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	readIssueID := dbfx.Issue(t, "Newest row is read", testutil.Cols{"workspace_id": workspaceID})
	unreadIssueID := dbfx.Issue(t, "Newest row is unread", testutil.Cols{"workspace_id": workspaceID})

	insert := func(issueID, title string, read bool, createdAt testutil.Raw) {
		t.Helper()
		dbfx.Insert(t, "inbox_item", testutil.Cols{
			"workspace_id":   workspaceID,
			"recipient_type": "member",
			"recipient_id":   testUserID,
			"type":           "task_failed",
			"severity":       "info",
			"issue_id":       issueID,
			"title":          title,
			"read":           read,
			"archived":       false,
			"created_at":     createdAt,
		})
	}
	insert(readIssueID, "older unread", false, "now() - interval '2 minutes'")
	insert(readIssueID, "newest read", true, "now() - interval '1 minute'")
	insert(unreadIssueID, "older read", true, "now() - interval '2 minutes'")
	insert(unreadIssueID, "newest unread", false, "now() - interval '1 minute'")

	testutil.Call(t, inboxWorkspaceHandler(testHandler.ArchiveAllReadInbox),
		inboxRequest(http.MethodPost, "/api/inbox/archive-all-read", workspaceID)).
		Want(http.StatusOK)

	if got := dbfx.Count(t,
		"SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND archived = true", readIssueID); got != 2 {
		t.Fatalf("archived rows in read issue = %d, want the whole two-row group", got)
	}
	if got := dbfx.Count(t,
		"SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND archived = true", unreadIssueID); got != 0 {
		t.Fatalf("archived rows in unread issue = %d, want the whole group untouched", got)
	}
}

func TestArchiveCompletedInboxExpandsCustomTerminalStatuses(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Archive custom completed", "archive-custom-completed-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	dbfx.Insert(t, "issue_status", testutil.Cols{
		"workspace_id": workspaceID,
		"key":          "verified_complete",
		"name":         "Verified complete",
		"category":     "done",
		"color":        "#22c55e",
		"is_system":    false,
		"position":     1,
	})
	completedIssueID := dbfx.Issue(t, "Custom completed issue", testutil.Cols{
		"workspace_id": workspaceID,
		"status":       "verified_complete",
	})
	openIssueID := dbfx.Issue(t, "Open issue", testutil.Cols{
		"workspace_id": workspaceID,
		"status":       "todo",
	})
	for _, issueID := range []string{completedIssueID, openIssueID} {
		dbfx.Insert(t, "inbox_item", testutil.Cols{
			"workspace_id":   workspaceID,
			"recipient_type": "member",
			"recipient_id":   testUserID,
			"type":           "status_changed",
			"severity":       "info",
			"issue_id":       issueID,
			"title":          "Status changed",
			"archived":       false,
		})
	}

	testutil.Call(t, inboxWorkspaceHandler(testHandler.ArchiveCompletedInbox),
		inboxRequest(http.MethodPost, "/api/inbox/archive-completed", workspaceID)).
		Want(http.StatusOK)

	if got := dbfx.Count(t,
		"SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND archived = true", completedIssueID); got != 1 {
		t.Fatalf("archived rows for custom completed issue = %d, want 1", got)
	}
	if got := dbfx.Count(t,
		"SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND archived = true", openIssueID); got != 0 {
		t.Fatalf("archived rows for open issue = %d, want 0", got)
	}
}
