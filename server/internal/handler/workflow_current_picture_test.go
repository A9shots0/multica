package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func currentPictureFixtureColumns(workspaceID, issueID string) testutil.Cols {
	return testutil.Cols{
		"workspace_id": workspaceID, "issue_id": issueID,
		"contract_version": 2, "revision": 4,
		"scope_revision": 2, "authority_revision": 3,
		"outcome_state": "future_outcome", "outcome_open": false,
		"execution_state": "future_execution", "release_state": "future_release",
		"health_state": "future_health", "attention_state": "future_attention",
		"current_milestone": "reviewing", "latest_meaningful_change": `{"kind":"review"}`,
		"next_automatic_action": `{"kind":"dispatch"}`, "active_actor": `{"id":"agent-1"}`,
		"wait_reason": `{}`, "candidate": `{}`, "target": `{}`,
		"evidence_summary": `{}`, "authority_summary": `{}`,
		"unresolved_decisions": `[]`, "unresolved_incidents": `[]`,
		"unresolved_dependencies": `[]`, "budget_summary": `[]`,
		"supersession_summary": `{}`, "terminal_reason": "", "next_actor": `{}`,
		"source_revisions": `[]`, "document": `{
          "contract_version":2,
          "owner_summary_schema_version":1,
          "original_business_request":{},
          "outcome":{"state":"future_outcome"},
          "historical_completion":{},
          "scope":{"revision":2},
          "authority":{"revision":3},
          "execution":{"state":"future_execution"},
          "active_branches":[],
          "primary_branch":{},
          "release":{"state":"future_release"},
          "health":"future_health",
          "attention":{"resolution_state":"future_attention"},
          "current_milestone":"reviewing",
          "latest_meaningful_change":{"kind":"review"},
          "next_automatic_action":{"kind":"dispatch"},
          "active_actor":{"id":"agent-1"},
          "wait_reason":{},
          "candidate":{},
          "target":{},
          "evidence_summary":{},
          "authority_summary":{},
          "unresolved_decisions":[],
          "unresolved_incidents":[],
          "unresolved_dependencies":[],
          "budgets":[],
          "supersession":{},
          "terminal_reason":"",
          "next_actor":{}
        }`,
		"generated_at": testutil.Raw("now()"),
	}
}

func TestCurrentPictureCompactFailsClosedAndKeepsFutureStates(t *testing.T) {
	row := db.IssueCurrentPicture{
		ContractVersion: 2, Revision: 1, OutcomeState: "future_outcome", OutcomeOpen: false,
		ExecutionState: "future_execution", ReleaseState: "future_release",
		HealthState: "future_health", AttentionState: "future_attention",
		CurrentMilestone:       "reviewing",
		LatestMeaningfulChange: []byte(`{}`), NextAutomaticAction: []byte(`{}`),
		ActiveActor: []byte(`{}`), WaitReason: []byte(`{}`), Candidate: []byte(`{}`),
		Target: []byte(`{}`), EvidenceSummary: []byte(`{}`), AuthoritySummary: []byte(`{}`),
		UnresolvedDecisions: []byte(`[]`), UnresolvedIncidents: []byte(`[]`),
		UnresolvedDependencies: []byte(`[]`), BudgetSummary: []byte(`[]`),
		SupersessionSummary: []byte(`{}`), NextActor: []byte(`{}`), SourceRevisions: []byte(`[]`),
		Document: []byte(`{
          "contract_version":2,"owner_summary_schema_version":1,"original_business_request":{},
          "outcome":{"state":"future_outcome"},"historical_completion":{},
          "scope":{},"authority":{},"execution":{"state":"future_execution"},
          "active_branches":[],"primary_branch":{},"release":{"state":"future_release"},
          "health":"future_health","attention":{"resolution_state":"future_attention"},
          "current_milestone":"reviewing","latest_meaningful_change":{},
          "next_automatic_action":{},"active_actor":{},"wait_reason":{},
          "candidate":{},"target":{},"evidence_summary":{},"authority_summary":{},
          "unresolved_decisions":[],"unresolved_incidents":[],"unresolved_dependencies":[],
          "budgets":[],"supersession":{},"terminal_reason":"","next_actor":{}
        }`),
		GeneratedAt: pgtype.Timestamptz{Valid: true},
	}
	compact, ok := currentPictureCompact(row)
	if !ok || compact.OutcomeState != "future_outcome" || compact.ExecutionState != "future_execution" {
		t.Fatalf("future state strings were not preserved: %#v, ok=%v", compact, ok)
	}
	row.ContractVersion = 3
	if _, ok := currentPictureCompact(row); ok {
		t.Fatal("future contract version was accepted")
	}
	row.ContractVersion = 2
	row.Document = []byte(`{"contract_version":1}`)
	semantic, ok := currentPictureSemanticFields(row)
	if !ok {
		t.Fatal("valid compact semantics rejected")
	}
	if validCurrentPictureDetailJSON(row, semantic) {
		t.Fatal("corrupt document contract was accepted")
	}
}

func TestCurrentPictureRejectsSemanticShapeAndAgreementFailures(t *testing.T) {
	issueID := dbfx.Issue(t, "Malformed Current Picture semantics")
	cases := []struct {
		name   string
		column string
		value  any
	}{
		{name: "decisions object", column: "unresolved_decisions", value: `{}`},
		{name: "evidence array", column: "evidence_summary", value: `[]`},
		{name: "source revisions null", column: "source_revisions", value: `null`},
		{name: "minimal document", column: "document", value: `{"contract_version":2}`},
		{name: "contradictory outcome", column: "outcome_state", value: "completed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			columns := currentPictureFixtureColumns(testWorkspaceID, issueID)
			columns[test.column] = test.value
			dbfx.InsertNoID(t, "issue_current_picture", columns,
				"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)
			testutil.Call(t, testHandler.GetIssueCurrentPicture,
				withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/current-picture", nil), "id", issueID)).
				Want(http.StatusInternalServerError)
			dbfx.Exec(t, "DELETE FROM issue_current_picture WHERE workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)
		})
	}
}

func TestGetIssueCurrentPictureDetailAndMissingAreTenantSafe(t *testing.T) {
	issueID := dbfx.Issue(t, "Current Picture detail")
	dbfx.InsertNoID(t, "issue_current_picture", currentPictureFixtureColumns(testWorkspaceID, issueID),
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)

	var detail map[string]any
	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/current-picture", nil), "id", issueID)).
		Want(http.StatusOK).JSON(&detail)
	if detail["contract_version"] != float64(2) || detail["outcome_state"] != "future_outcome" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if _, isString := detail["document"].(string); isString {
		t.Fatal("document was base64/string encoded instead of embedded JSON")
	}
	ownerSummary, ok := detail["owner_summary"].(map[string]any)
	if !ok || ownerSummary["stage"] != "status_unavailable" || ownerSummary["current_picture_revision"] != float64(4) {
		t.Fatalf("owner summary = %#v, want revision-bound unavailable", detail["owner_summary"])
	}

	missingID := dbfx.Issue(t, "No projection")
	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+missingID+"/current-picture", nil), "id", missingID)).
		Want(http.StatusNotFound)

	foreignWorkspaceID := dbfx.Workspace(t, "Foreign Current Picture", "foreign-current-picture")
	foreignIssueID := dbfx.Issue(t, "Foreign issue", testutil.Cols{"workspace_id": foreignWorkspaceID})
	dbfx.InsertNoID(t, "issue_current_picture", currentPictureFixtureColumns(foreignWorkspaceID, foreignIssueID),
		"workspace_id = $1 AND issue_id = $2", foreignWorkspaceID, foreignIssueID)
	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+foreignIssueID+"/current-picture", nil), "id", foreignIssueID)).
		Want(http.StatusNotFound)
}

func TestGetIssueEmbedsCurrentPictureForIssueNow(t *testing.T) {
	issueID := dbfx.Issue(t, "Issue detail Current Picture")
	dbfx.InsertNoID(t, "issue_current_picture", currentPictureFixtureColumns(testWorkspaceID, issueID),
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)

	var detail IssueResponse
	testutil.Call(t, testHandler.GetIssue,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID, nil), "id", issueID)).
		Want(http.StatusOK).JSON(&detail)
	if detail.CurrentPicture == nil || detail.CurrentPicture.Revision != 4 ||
		detail.CurrentPicture.OwnerSummary.CurrentPictureRevision != 4 {
		t.Fatalf("embedded Current Picture = %#v, want revision-aligned issue detail", detail.CurrentPicture)
	}
}

func TestCurrentPictureOwnerSummaryFailsClosedWithoutDroppingValidV2(t *testing.T) {
	issueID := dbfx.Issue(t, "Unavailable owner summary")
	columns := currentPictureFixtureColumns(testWorkspaceID, issueID)
	var document map[string]any
	if err := json.Unmarshal([]byte(columns["document"].(string)), &document); err != nil {
		t.Fatal(err)
	}
	document["owner_summary_schema_version"] = 99
	raw, _ := json.Marshal(document)
	columns["document"] = string(raw)
	dbfx.InsertNoID(t, "issue_current_picture", columns,
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)

	var detail struct {
		OwnerSummary workflowprojection.OwnerIssueSummary `json:"owner_summary"`
	}
	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/current-picture", nil), "id", issueID)).
		Want(http.StatusOK).JSON(&detail)
	if detail.OwnerSummary.Stage != workflowprojection.OwnerStageStatusUnavailable {
		t.Fatalf("owner summary = %#v", detail.OwnerSummary)
	}
}

func TestCurrentPictureV3RequiresStrictReleaseStatus(t *testing.T) {
	issueID := dbfx.Issue(t, "Malformed v3 release status")
	columns := currentPictureFixtureColumns(testWorkspaceID, issueID)
	columns["contract_version"] = 3
	var document map[string]any
	if err := json.Unmarshal([]byte(columns["document"].(string)), &document); err != nil {
		t.Fatal(err)
	}
	document["contract_version"] = 3
	document["release_status"] = map[string]any{"schema_version": 1, "phase": "verified_live"}
	raw, _ := json.Marshal(document)
	columns["document"] = string(raw)
	dbfx.InsertNoID(t, "issue_current_picture", columns,
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, issueID)

	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+issueID+"/current-picture", nil), "id", issueID)).
		Want(http.StatusInternalServerError)
}

func TestListIssuesCurrentPictureIsOptionalAndBatched(t *testing.T) {
	projectID := dbfx.Project(t, "Current Picture collection project")
	validIssueID := dbfx.Issue(t, "Valid compact picture", testutil.Cols{"project_id": projectID})
	invalidIssueID := dbfx.Issue(t, "Invalid compact picture", testutil.Cols{"project_id": projectID})
	dbfx.InsertNoID(t, "issue_current_picture", currentPictureFixtureColumns(testWorkspaceID, validIssueID),
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, validIssueID)
	invalid := currentPictureFixtureColumns(testWorkspaceID, invalidIssueID)
	invalid["evidence_summary"] = `[]`
	dbfx.InsertNoID(t, "issue_current_picture", invalid,
		"workspace_id = $1 AND issue_id = $2", testWorkspaceID, invalidIssueID)

	var response struct {
		Issues []struct {
			ID             string          `json:"id"`
			CurrentPicture json.RawMessage `json:"current_picture"`
		} `json:"issues"`
	}
	path := "/api/issues?ids=" + validIssueID + "," + invalidIssueID
	testutil.Call(t, testHandler.ListIssues, newRequest(http.MethodGet, path, nil)).Want(http.StatusOK).JSON(&response)
	byID := make(map[string]json.RawMessage)
	for _, issue := range response.Issues {
		byID[issue.ID] = issue.CurrentPicture
	}
	if len(byID[validIssueID]) == 0 {
		t.Fatal("valid compact Current Picture was not included")
	}
	var compact issueCurrentPictureCompactResponse
	if err := json.Unmarshal(byID[validIssueID], &compact); err != nil {
		t.Fatal(err)
	}
	if compact.OwnerSummary.Stage != workflowprojection.OwnerStageStatusUnavailable || compact.OwnerSummary.CurrentPictureRevision != compact.Revision {
		t.Fatalf("compact owner summary = %#v", compact.OwnerSummary)
	}
	if len(byID[invalidIssueID]) != 0 {
		t.Fatal("invalid optional Current Picture was not dropped")
	}

	var openResponse struct {
		Issues []struct {
			ID             string          `json:"id"`
			CurrentPicture json.RawMessage `json:"current_picture"`
		} `json:"issues"`
	}
	testutil.Call(t, testHandler.ListIssues,
		newRequest(http.MethodGet, "/api/issues?open_only=true&project_id="+projectID, nil)).
		Want(http.StatusOK).JSON(&openResponse)
	openByID := make(map[string]json.RawMessage)
	for _, issue := range openResponse.Issues {
		openByID[issue.ID] = issue.CurrentPicture
	}
	if len(openByID[validIssueID]) == 0 {
		t.Fatal("open overview omitted a valid compact Current Picture")
	}
	if len(openByID[invalidIssueID]) != 0 {
		t.Fatal("open overview exposed a malformed compact Current Picture")
	}

	var grouped GroupedIssuesResponse
	testutil.Call(t, testHandler.ListGroupedIssues,
		newRequest(http.MethodGet, "/api/issues/grouped?group_by=assignee&project_id="+projectID+"&limit=10", nil)).
		Want(http.StatusOK).JSON(&grouped)
	if len(grouped.Groups) != 1 || len(grouped.Groups[0].Issues) != 2 {
		t.Fatalf("unexpected grouped Current Picture window: %#v", grouped)
	}
	groupedByID := make(map[string]*issueCurrentPictureCompactResponse)
	for _, issue := range grouped.Groups[0].Issues {
		groupedByID[issue.ID] = issue.CurrentPicture
	}
	if groupedByID[validIssueID] == nil || groupedByID[invalidIssueID] != nil {
		t.Fatalf("grouped optional enrichment mismatch: %#v", groupedByID)
	}

	var table issueTableRowsResponse
	testutil.Call(t, testHandler.ListIssueTableRows,
		newRequest(http.MethodPost, "/api/issues/table/rows", issueTableRowsRequest{
			Query: issueTableQuerySpec{
				Scope: issueTableScope{Kind: "project", ProjectID: projectID},
				Sort:  issueTableSortRequest{Field: "position", Direction: "asc"},
			},
			Group:     issueTableGroupSpec{Kind: "none"},
			Hierarchy: issueTableHierarchyRequest{Enabled: false},
			Page:      issueTablePageRequest{Limit: 10},
		})).Want(http.StatusOK).JSON(&table)
	if len(table.Rows) != 2 {
		t.Fatalf("unexpected Table Current Picture window: %#v", table.Rows)
	}
	tableByID := make(map[string]*issueCurrentPictureCompactResponse)
	for _, row := range table.Rows {
		tableByID[row.Issue.ID] = row.Issue.CurrentPicture
	}
	if tableByID[validIssueID] == nil || tableByID[invalidIssueID] != nil {
		t.Fatalf("Table optional enrichment mismatch: %#v", tableByID)
	}

	testutil.Call(t, testHandler.GetIssueCurrentPicture,
		withURLParam(newRequest(http.MethodGet, "/api/issues/"+invalidIssueID+"/current-picture", nil), "id", invalidIssueID)).
		Want(http.StatusInternalServerError)
}
