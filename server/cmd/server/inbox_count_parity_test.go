package main

import (
	"context"
	"testing"
)

func TestInboxCountsMatchFailureGroupsAndIndependentOwnerDecisions(t *testing.T) {
	ctx := context.Background()
	count := func() int64 {
		resp := authRequest(t, "GET", "/api/inbox/unread-count", nil)
		if resp.StatusCode != 200 {
			t.Fatalf("count status %d", resp.StatusCode)
		}
		var result struct {
			Count int64 `json:"count"`
		}
		readJSON(t, resp, &result)
		return result.Count
	}
	baseline := count()
	var issueID string
	if err := testPool.QueryRow(ctx, `INSERT INTO issue(workspace_id,title,creator_type,creator_id) VALUES($1,'CI inbox parity','member',$2) RETURNING id`, testWorkspaceID, testUserID).Scan(&issueID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := testPool.Exec(ctx, `DELETE FROM inbox_item WHERE issue_id=$1`, issueID); err != nil {
			t.Error(err)
		}
		if _, err := testPool.Exec(ctx, `DELETE FROM issue WHERE id=$1`, issueID); err != nil {
			t.Error(err)
		}
	})
	if _, err := testPool.Exec(ctx, `INSERT INTO inbox_item(workspace_id,recipient_type,recipient_id,type,severity,title,issue_id,read,created_at) VALUES
 ($1,'member',$2,'task_failed','info','old failure',$3,false,now()-interval '1 hour'),
 ($1,'member',$2,'task_failed','attention','latest failure',$3,false,now())`, testWorkspaceID, testUserID, issueID); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != baseline+1 {
		t.Fatalf("failed attempts count=%d want=%d", got, baseline+1)
	}
	if _, err := testPool.Exec(ctx, `UPDATE inbox_item SET read=true WHERE issue_id=$1 AND title='latest failure'`, issueID); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != baseline {
		t.Fatalf("newest read/older unread count=%d want=%d", got, baseline)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO inbox_item(workspace_id,recipient_type,recipient_id,type,severity,title,issue_id,details) VALUES
 ($1,'member',$2,'workflow_owner_decision','action_required','owner question A',$3,'{}'),
 ($1,'member',$2,'workflow_owner_decision','action_required','owner question B',$3,'{}'),
 ($1,'member',$2,'task_completed','info','routine hidden update',$3,'{}')`, testWorkspaceID, testUserID, issueID); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != baseline+2 {
		t.Fatalf("independent decisions count=%d want=%d", got, baseline+2)
	}
	resp := authRequest(t, "GET", "/api/inbox", nil)
	var rows []struct {
		IssueID *string `json:"issue_id"`
		Title   string  `json:"title"`
	}
	readJSON(t, resp, &rows)
	visible := map[string]bool{}
	for _, row := range rows {
		if row.IssueID != nil && *row.IssueID == issueID {
			visible[row.Title] = true
		}
	}
	if len(visible) != 4 || visible["routine hidden update"] {
		t.Fatalf("visible rows: %v", visible)
	}
	if _, err := testPool.Exec(ctx, `UPDATE inbox_item SET archived=true WHERE issue_id=$1 AND title='owner question A'`, issueID); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != baseline+1 {
		t.Fatalf("archived decision count=%d want=%d", got, baseline+1)
	}
	resp = authRequest(t, "GET", "/api/inbox/archived", nil)
	readJSON(t, resp, &rows)
	found := false
	for _, row := range rows {
		if row.IssueID != nil && *row.IssueID == issueID && row.Title == "owner question A" {
			found = true
		}
	}
	if !found {
		t.Fatal("independent archived decision hidden by active sibling on same issue")
	}
}
