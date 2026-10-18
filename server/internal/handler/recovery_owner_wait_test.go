package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
)

func TestRecoveryOwnerWaitFallbackRequiresDurableOwnerTruth(t *testing.T) {
	base := IssueResponse{
		ID: "00000000-0000-4000-8000-000000000053", CreatorID: "00000000-0000-4000-8000-000000000001",
		Status: "blocked", StatusCategory: "blocked", Revision: 7,
		UpdatedAt: "2026-09-20T03:35:08Z",
	}
	owner := base
	owner.Metadata = map[string]any{
		"recovery.wait_kind":   "owner_ui_approval",
		"recovery.wait_reason": "Approve or revise the populated Board preview before implementation continues.",
		"recovery.next_action": "Approval resumes the preserved implementation candidate.",
	}
	applyRecoveryOwnerWaitFallback(&owner)
	if owner.CurrentPicture == nil || owner.CurrentPicture.OwnerSummary.Stage != workflowprojection.OwnerStageNeedsOwner {
		t.Fatalf("owner wait picture = %#v", owner.CurrentPicture)
	}
	var waitReason map[string]any
	if err := json.Unmarshal(owner.CurrentPicture.WaitReason, &waitReason); err != nil {
		t.Fatal(err)
	}
	if waitReason["question"] != owner.Metadata["recovery.wait_reason"] || waitReason["consequence"] != owner.Metadata["recovery.next_action"] {
		t.Fatalf("wait reason lost exact owner text: %#v", waitReason)
	}

	provider := base
	provider.Metadata = map[string]any{
		"recovery.wait_kind":   "provider_capacity",
		"recovery.wait_reason": "Provider capacity is exhausted.",
	}
	applyRecoveryOwnerWaitFallback(&provider)
	if provider.CurrentPicture != nil {
		t.Fatalf("provider wait was projected into Needs You: %#v", provider.CurrentPicture)
	}

	resolved := owner
	resolved.CurrentPicture = nil
	resolved.Status = "done"
	resolved.StatusCategory = "done"
	applyRecoveryOwnerWaitFallback(&resolved)
	if resolved.CurrentPicture != nil {
		t.Fatalf("closed owner wait remained in Needs You: %#v", resolved.CurrentPicture)
	}
}

func TestRecoveryOwnerWaitAgreesAcrossBoardInboxAndIssueDetail(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	question := "Approve or revise the nested Board preview before implementation continues."
	consequence := "Approval resumes the same preserved issue; revision keeps it paused."
	issueID := dbfx.Issue(t, "Recovery owner wait surface agreement", testutil.Cols{
		"status":   "blocked",
		"metadata": `{"recovery.wait_kind":"owner_ui_approval","recovery.wait_reason":"` + question + `","recovery.next_action":"` + consequence + `"}`,
	})
	issueRow, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	if err := testHandler.projectRecoveryOwnerWaitInbox(t.Context(), issueRow); err != nil {
		t.Fatal(err)
	}

	var board struct {
		Issues []IssueResponse `json:"issues"`
	}
	testutil.Call(t, testHandler.ListIssues,
		newRequest(http.MethodGet, "/api/issues?ids="+issueID, nil)).
		Want(http.StatusOK).JSON(&board)
	if len(board.Issues) != 1 {
		t.Fatalf("board issues = %#v", board.Issues)
	}

	var detail IssueResponse
	testutil.Call(t, testHandler.GetIssue,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID, nil), "id", issueID)).
		Want(http.StatusOK).JSON(&detail)

	var inbox []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox),
		newRequest(http.MethodGet, "/api/inbox", nil)).
		Want(http.StatusOK).JSON(&inbox)
	var ownerItem *InboxItemResponse
	for i := range inbox {
		if inbox[i].IssueID != nil && *inbox[i].IssueID == issueID && inbox[i].Type == "workflow_owner_decision" {
			ownerItem = &inbox[i]
			break
		}
	}
	if ownerItem == nil {
		t.Fatalf("recovery owner wait missing from inbox: %#v", inbox)
	}
	for surface, issue := range map[string]IssueResponse{"board": board.Issues[0], "detail": detail} {
		if issue.CurrentPicture == nil || issue.CurrentPicture.OwnerSummary.Stage != workflowprojection.OwnerStageNeedsOwner ||
			issue.CurrentPicture.OwnerSummary.NextAction == nil || issue.CurrentPicture.OwnerSummary.NextAction.Kind != "answer_decision" {
			t.Fatalf("%s owner projection = %#v", surface, issue.CurrentPicture)
		}
		var wait map[string]any
		if err := json.Unmarshal(issue.CurrentPicture.WaitReason, &wait); err != nil {
			t.Fatal(err)
		}
		if wait["question"] != question || wait["consequence"] != consequence {
			t.Fatalf("%s wait = %#v", surface, wait)
		}
	}
	var details map[string]string
	if err := json.Unmarshal(ownerItem.Details, &details); err != nil {
		t.Fatal(err)
	}
	if details["owner_category"] != "needs_you" || details["required_action"] != "respond" ||
		details["question"] != question || details["consequence"] != consequence {
		t.Fatalf("inbox owner wait details = %#v", details)
	}

	// Closing the durable recovery wait removes the compatibility projection;
	// the issue row and all of its work remain intact.
	testutil.Call(t, testHandler.DeleteIssueMetadataKey,
		withURLParams(newRequest(http.MethodDelete, "/api/issues/"+issueID+"/metadata/recovery.wait_kind", nil),
			"id", issueID, "key", "recovery.wait_kind")).Want(http.StatusOK)

	var after IssueResponse
	testutil.Call(t, testHandler.GetIssue,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID, nil), "id", issueID)).
		Want(http.StatusOK).JSON(&after)
	if after.ID != issueID || after.Title != "Recovery owner wait surface agreement" {
		t.Fatalf("closing wait lost issue work: %#v", after)
	}
	if after.CurrentPicture != nil {
		t.Fatalf("closed wait still has Needs You projection: %#v", after.CurrentPicture)
	}
	var afterInbox []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox), newRequest(http.MethodGet, "/api/inbox", nil)).
		Want(http.StatusOK).JSON(&afterInbox)
	for _, item := range afterInbox {
		if item.IssueID != nil && *item.IssueID == issueID && item.Type == "workflow_owner_decision" {
			t.Fatalf("closed wait remained in Needs You inbox: %#v", item)
		}
	}
}

func TestRecoveryOwnerWaitCreatedByAgentRoutesToAgentOwner(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	agentID := dbfx.Agent(t, "Recovery issue creator", "")
	question := "Choose the least-privilege access before work continues."
	consequence := "The same issue resumes after the access choice is recorded."
	issueID := dbfx.Issue(t, "Agent-created owner wait", testutil.Cols{
		"status":       "blocked",
		"creator_type": "agent",
		"creator_id":   agentID,
		"metadata":     `{"recovery.wait_kind":"owner_access_decision","recovery.wait_reason":"` + question + `","recovery.next_action":"` + consequence + `"}`,
	})
	issueRow, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	if err := testHandler.projectRecoveryOwnerWaitInbox(t.Context(), issueRow); err != nil {
		t.Fatal(err)
	}
	var recipientID string
	dbfx.QueryRow(t, `
		SELECT recipient_id::text
		FROM inbox_item
		WHERE issue_id = $1 AND type = 'workflow_owner_decision'
	`, issueID).Scan(&recipientID)
	if recipientID != dbfx.UserID {
		t.Fatalf("agent-created owner wait recipient = %s, want agent owner %s", recipientID, dbfx.UserID)
	}
	assertRecoveryOwnerWaitSurfaceAgreement(t, issueID, question, consequence)
}

func TestRecoveryOwnerWaitProjectionIsMetadataStatusOrderIndependent(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	question := "Confirm the durable owner choice before work resumes."
	consequence := "The existing issue resumes without creating replacement work."

	setMetadata := func(t *testing.T, issueID, key, value string) {
		t.Helper()
		req := withURLParams(newRequest(http.MethodPut, "/api/issues/"+issueID+"/metadata/"+key, map[string]any{
			"value": value,
		}), "id", issueID, "key", key)
		testutil.Call(t, testHandler.SetIssueMetadataKey, req).Want(http.StatusOK)
	}
	setBlocked := func(t *testing.T, issueID string) {
		t.Helper()
		req := withURLParam(newRequest(http.MethodPut, "/api/issues/"+issueID, map[string]any{
			"status": "blocked",
		}), "id", issueID)
		testutil.Call(t, testHandler.UpdateIssue, req).Want(http.StatusOK)
	}
	assertProjected := func(t *testing.T, issueID string) {
		t.Helper()
		var details []byte
		dbfx.QueryRow(t, `
			SELECT details
			FROM inbox_item
			WHERE issue_id = $1 AND type = 'workflow_owner_decision'
		`, issueID).Scan(&details)
		var decoded map[string]string
		if err := json.Unmarshal(details, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["owner_category"] != "needs_you" || decoded["required_action"] != "respond" ||
			decoded["question"] != question || decoded["consequence"] != consequence {
			t.Fatalf("projected owner wait details = %#v", decoded)
		}
	}

	for _, tc := range []struct {
		name          string
		metadataFirst bool
	}{
		{name: "metadata_then_status", metadataFirst: true},
		{name: "status_then_metadata", metadataFirst: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issueID := dbfx.Issue(t, "Owner wait ordering "+tc.name, testutil.Cols{"status": "todo"})
			writeMetadata := func() {
				setMetadata(t, issueID, "recovery.wait_kind", "owner")
				setMetadata(t, issueID, "recovery.wait_reason", question)
				setMetadata(t, issueID, "recovery.next_action", consequence)
			}
			if tc.metadataFirst {
				writeMetadata()
				if got := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_owner_decision'`, issueID); got != 0 {
					t.Fatalf("active issue projected %d owner wait item(s) before blocked status", got)
				}
				setBlocked(t, issueID)
			} else {
				setBlocked(t, issueID)
				if got := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type = 'workflow_owner_decision'`, issueID); got != 0 {
					t.Fatalf("blocked issue projected %d owner wait item(s) before durable metadata", got)
				}
				writeMetadata()
			}
			assertProjected(t, issueID)
		})
	}
}

func assertRecoveryOwnerWaitSurfaceAgreement(t *testing.T, issueID, question, consequence string) {
	t.Helper()
	var board struct {
		Issues []IssueResponse `json:"issues"`
	}
	testutil.Call(t, testHandler.ListIssues,
		newRequest(http.MethodGet, "/api/issues?ids="+issueID, nil)).
		Want(http.StatusOK).JSON(&board)
	if len(board.Issues) != 1 {
		t.Fatalf("board issues = %#v", board.Issues)
	}

	var detail IssueResponse
	testutil.Call(t, testHandler.GetIssue,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID, nil), "id", issueID)).
		Want(http.StatusOK).JSON(&detail)

	for surface, issue := range map[string]IssueResponse{"board": board.Issues[0], "detail": detail} {
		if issue.CurrentPicture == nil || issue.CurrentPicture.OwnerSummary.Stage != workflowprojection.OwnerStageNeedsOwner ||
			issue.CurrentPicture.OwnerSummary.NextAction == nil || issue.CurrentPicture.OwnerSummary.NextAction.Kind != "answer_decision" {
			t.Fatalf("%s owner projection = %#v", surface, issue.CurrentPicture)
		}
		var wait map[string]any
		if err := json.Unmarshal(issue.CurrentPicture.WaitReason, &wait); err != nil {
			t.Fatal(err)
		}
		if wait["question"] != question || wait["consequence"] != consequence {
			t.Fatalf("%s wait = %#v", surface, wait)
		}
	}

	var inbox []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox),
		newRequest(http.MethodGet, "/api/inbox", nil)).
		Want(http.StatusOK).JSON(&inbox)
	for _, item := range inbox {
		if item.IssueID == nil || *item.IssueID != issueID || item.Type != "workflow_owner_decision" {
			continue
		}
		var details map[string]string
		if err := json.Unmarshal(item.Details, &details); err != nil {
			t.Fatal(err)
		}
		if details["owner_category"] != "needs_you" || details["required_action"] != "respond" ||
			details["question"] != question || details["consequence"] != consequence {
			t.Fatalf("inbox owner wait details = %#v", details)
		}
		return
	}
	t.Fatalf("recovery owner wait %s missing from inbox: %#v", issueID, inbox)
}
