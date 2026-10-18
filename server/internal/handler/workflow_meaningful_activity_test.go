package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestMeaningfulActivityLimit(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    int32
		invalid bool
	}{
		{"", 20, false}, {"1", 1, false}, {"100", 100, false},
		{"0", 0, true}, {"101", 0, true}, {"no", 0, true},
	} {
		got, err := meaningfulActivityLimit(tc.raw)
		if (err != nil) != tc.invalid || got != tc.want {
			t.Fatalf("limit %q = %d, %v", tc.raw, got, err)
		}
	}
}

func TestGetIssueMeaningfulActivityPagesNormalizedRecords(t *testing.T) {
	issueID := dbfx.Issue(t, "Meaningful activity")
	dbfx.InsertNoID(t, "issue_current_picture", currentPictureFixtureColumns(testWorkspaceID, issueID),
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)
	for index, state := range []string{"received", "satisfied"} {
		outcomeID := uuid.NewString()
		dbfx.InsertNoID(t, "issue_workflow_outcome", testutil.Cols{
			"id": outcomeID, "workspace_id": testWorkspaceID, "issue_id": issueID,
			"revision": int64(index + 1), "state": state, "acceptance_version": int64(1),
			"created_at": time.Date(2026, 9, 10, 12, 0, index, 0, time.UTC),
		}, "id = $1", outcomeID)
	}
	var first meaningfulActivityResponse
	testutil.Call(t, testHandler.GetIssueMeaningfulActivity,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/meaningful-activity?limit=1", nil), "id", issueID)).
		Want(http.StatusOK).JSON(&first)
	if len(first.Items) != 1 || first.NextCursor == nil || first.CurrentPictureRevision != 4 || first.Items[0].Code != "outcome_changed" {
		t.Fatalf("first page = %#v", first)
	}
	var second meaningfulActivityResponse
	path := "/api/issues/" + issueID + "/meaningful-activity?limit=1&cursor=" + url.QueryEscape(*first.NextCursor)
	testutil.Call(t, testHandler.GetIssueMeaningfulActivity,
		withURLParam(newRequest(http.MethodGet, path, nil), "id", issueID)).
		Want(http.StatusOK).JSON(&second)
	if len(second.Items) != 1 || second.NextCursor != nil || second.Items[0].OccurredAt >= first.Items[0].OccurredAt {
		t.Fatalf("second page = %#v after %#v", second, first)
	}

	foreign := dbfx.Workspace(t, "Meaningful activity foreign", "meaningful-foreign")
	foreignIssue := dbfx.Issue(t, "Meaningful foreign issue", testutil.Cols{"workspace_id": foreign})
	testutil.Call(t, testHandler.GetIssueMeaningfulActivity,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+foreignIssue+"/meaningful-activity", nil), "id", foreignIssue)).
		Want(http.StatusNotFound)
}

func TestMeaningfulActivityCursorIsOpaqueAndIdentityBound(t *testing.T) {
	id := parseUUID("11111111-1111-1111-1111-111111111111")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	raw, err := encodeMeaningfulActivityCursor("ws-1", "issue-1", db.ListIssueWorkflowMeaningfulActivityRow{
		RecordType: "review", ID: id, Revision: 3, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(raw); err != nil {
		t.Fatalf("cursor is not base64url: %v", err)
	}
	cursor, err := decodeMeaningfulActivityCursor(raw, "ws-1", "issue-1")
	if err != nil || cursor.RecordID != uuidToString(id) || cursor.Revision != 3 {
		t.Fatalf("decoded cursor = %#v, %v", cursor, err)
	}
	if _, err := decodeMeaningfulActivityCursor(raw, "ws-1", "issue-2"); err == nil {
		t.Fatal("cross-issue cursor accepted")
	}
	if _, err := decodeMeaningfulActivityCursor(raw, "ws-2", "issue-1"); err == nil {
		t.Fatal("cross-workspace cursor accepted")
	}
	if _, err := decodeMeaningfulActivityCursor(raw+"garbage", "ws-1", "issue-1"); err == nil {
		t.Fatal("corrupt cursor accepted")
	}
}

func TestGetIssueMeaningfulActivityPagesImmutableRecords(t *testing.T) {
	issueID := dbfx.Issue(t, "Immutable meaningful activity")
	dbfx.InsertNoID(t, "issue_current_picture", currentPictureFixtureColumns(testWorkspaceID, issueID),
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)
	// Equal times exercise the record-type/id cursor tie-breakers. These three
	// immutable tables have no revision column; their record revision is one.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	expected := make(map[string]string)
	for _, record := range []struct {
		table string
		code  string
		cols  testutil.Cols
	}{
		{"issue_workflow_decision_response", "decision_answered", testutil.Cols{
			"decision_id": uuid.NewString(), "decision_revision": 7, "response_type": "approve",
			"actor_id": testUserID, "idempotency_key": uuid.NewString(), "request_hash": "fixture",
		}},
		{"issue_workflow_review", "review_recorded", testutil.Cols{
			"candidate_id": uuid.NewString(), "candidate_revision": 3,
			"scope_id": uuid.NewString(), "scope_revision": 2,
			"reviewer_id": testUserID, "outcome": "passed",
		}},
		{"issue_workflow_evidence", "evidence_recorded", testutil.Cols{
			"concern_type": "candidate", "evidence_type": "local_test", "level": "local_code",
			"source": "local-integration-test", "producer_type": "member",
			"observed_at": now, "payload": `{}`,
		}},
	} {
		id := uuid.NewString()
		record.cols["id"] = id
		record.cols["workspace_id"] = testWorkspaceID
		record.cols["issue_id"] = issueID
		record.cols["created_at"] = now
		dbfx.InsertNoID(t, record.table, record.cols, "id = $1", id)
		expected[id] = record.code
	}

	path := "/api/issues/" + issueID + "/meaningful-activity?limit=1"
	for page := 0; page < 3; page++ {
		var response meaningfulActivityResponse
		testutil.Call(t, testHandler.GetIssueMeaningfulActivity,
			withURLParam(newRequest(http.MethodGet, path, nil), "id", issueID)).Want(http.StatusOK).JSON(&response)
		if len(response.Items) != 1 {
			t.Fatalf("page %d must retain its immutable record: %#v", page, response)
		}
		item := response.Items[0]
		if code, ok := expected[item.Reference.ID]; !ok || item.Code != code || item.Reference.Revision != 1 {
			t.Fatalf("page %d returned an unexpected, duplicate, or mis-versioned record: %#v", page, item)
		}
		delete(expected, item.Reference.ID)
		if page == 2 {
			if response.NextCursor != nil || len(expected) != 0 {
				t.Fatalf("immutable activity did not end exactly once: %#v, remaining=%v", response, expected)
			}
		} else {
			if response.NextCursor == nil {
				t.Fatalf("page %d lost remaining immutable records", page)
			}
			path = "/api/issues/" + issueID + "/meaningful-activity?limit=1&cursor=" + url.QueryEscape(*response.NextCursor)
		}
	}
}

func TestMeaningfulActivityCursorRejectsMalformedRecordID(t *testing.T) {
	raw, err := json.Marshal(meaningfulActivityCursor{
		Version: 1, WorkspaceID: "ws-1", IssueID: "issue-1",
		OccurredAt: time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
		RecordType: "review", RecordID: "invalid-uuid", Revision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeMeaningfulActivityCursor(base64.RawURLEncoding.EncodeToString(raw), "ws-1", "issue-1"); err == nil {
		t.Fatal("malformed record identity must return a validation error")
	}
}

func TestMeaningfulActivityItemsIsolatesPoisonAndRejectsDuplicates(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	valid := db.ListIssueWorkflowMeaningfulActivityRow{
		RecordType: "review", ID: parseUUID("11111111-1111-1111-1111-111111111111"), Revision: 1,
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		Payload:   []byte(`{"id":"11111111-1111-1111-1111-111111111111","created_at":"2026-09-10T12:00:00Z","candidate_id":"candidate-1","candidate_revision":1,"scope_id":"scope-1","scope_revision":1,"reviewer_id":"reviewer-1","outcome":"passed","criteria":[],"coverage":{},"evidence_refs":["evidence-1"],"gaps":[],"correction_packet":{},"correction_count":0,"max_corrections":1}`),
	}
	poison := valid
	poison.ID = parseUUID("22222222-2222-2222-2222-222222222222")
	poison.Payload = []byte(`{`)
	items, err := meaningfulActivityItems("issue-1", []db.ListIssueWorkflowMeaningfulActivityRow{poison, valid})
	if err != nil || len(items) != 1 || items[0].Reference.ID != uuidToString(valid.ID) {
		t.Fatalf("items = %#v, %v", items, err)
	}
	if _, err := meaningfulActivityItems("issue-1", []db.ListIssueWorkflowMeaningfulActivityRow{valid, valid}); err == nil {
		t.Fatal("duplicate identity accepted")
	}
	items, err = meaningfulActivityItems("issue-1", nil)
	if err != nil || len(items) != 0 {
		t.Fatalf("empty items = %#v, %v", items, err)
	}
}
