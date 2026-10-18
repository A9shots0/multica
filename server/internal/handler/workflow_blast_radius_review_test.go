package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// validWorkflowBlastRadiusReview builds a genuine-shaped blast_radius_review.v1
// packet: real-looking related areas the reviewer says it actually opened,
// and reasoning long enough and varied enough to not be a rubber stamp.
func validWorkflowBlastRadiusReview() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"contract":       "blast_radius_review.v1",
		"related_areas_checked": []string{
			"server/internal/service/client_metrics.go",
			"server/internal/handler/client_handler.go",
		},
		"reasoning": "Checked how client records are represented elsewhere in the CRM before approving. This change adds a subscription_tier column on the existing metrics row rather than creating a second client record per tier, so it stays consistent with how every other part of the system keys off one client row. It does not duplicate or multiply client data, and no other route, table, or downstream app reads client records in a way this would break.",
	}
}

// workflowBlastRadiusReviewPassedRequest builds a raw review_passed request so
// tests in this file can vary blast_radius_review independently of test_evidence,
// which is always held valid here since these tests are about the blast-radius
// gate specifically.
func workflowBlastRadiusReviewPassedRequest(fx workflowControlFixture, taskID string, blastRadiusReview any, omitField bool) *http.Request {
	body := map[string]any{"event": workflowstate.ReviewPassed, "reason": "scenario evidence", "test_evidence": validWorkflowTestEvidence()}
	if !omitField {
		body["blast_radius_review"] = blastRadiusReview
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	req.Header.Set("X-Agent-ID", fx.ReviewerID)
	req.Header.Set("X-Task-ID", taskID)
	return req
}

// TestWorkflowControlReviewPassedRequiresBlastRadiusReview proves the gap the
// owner's correction closes: today (after tonight's test-evidence fix) a
// review_passed carrying perfect build/test evidence but zero reasoning about
// whether the change makes sense against the rest of the system is accepted.
// That is exactly how a technically-passing-but-bad design decision (e.g.
// duplicating a CLIENT record per subscription metric instead of keeping one
// client with two metrics) could ship: the evidence gate has nothing to say
// about it. This must now be refused.
func TestWorkflowControlReviewPassedRequiresBlastRadiusReview(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)

	result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowBlastRadiusReviewPassedRequest(fx, reviewerTaskID, nil, true))
	if result.Code != http.StatusBadRequest {
		t.Fatalf("review_passed with no blast_radius_review = %d %s, want 400 (reasoning must be required)", result.Code, result.Body.String())
	}
	if !strings.Contains(result.Body.String(), "blast_radius_review") {
		t.Fatalf("rejection did not explain the missing blast-radius reasoning: %s", result.Body.String())
	}

	plan, err := testHandler.Queries.GetIssueWorkflowPlan(context.Background(), db.GetIssueWorkflowPlanParams{
		IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	if plan.CurrentState != string(workflowstate.TechnicalReview) {
		t.Fatalf("plan advanced past review without blast-radius reasoning: %q", plan.CurrentState)
	}
}

// TestWorkflowControlReviewPassedRejectsTrivialBlastRadiusReview covers the
// shapes a rubber-stamping reviewer could otherwise submit to clear the gate
// without actually reasoning about design or blast radius: a blank/near-empty
// field, a bare "looks fine"-style phrase, one phrase padded by repetition to
// clear a length check, and a missing related-areas list (reasoning present
// but nothing named as actually having been checked). All must be refused.
func TestWorkflowControlReviewPassedRejectsTrivialBlastRadiusReview(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	cases := []struct {
		name    string
		mutate  func(packet map[string]any)
		wantErr string
	}{
		{
			name: "blank reasoning",
			mutate: func(packet map[string]any) {
				packet["reasoning"] = ""
			},
			wantErr: "reasoning",
		},
		{
			name: "bare rubber-stamp phrase",
			mutate: func(packet map[string]any) {
				packet["reasoning"] = "looks fine"
			},
			wantErr: "rubber-stamp",
		},
		{
			name: "rubber-stamp phrase padded by repetition past the length minimum",
			mutate: func(packet map[string]any) {
				packet["reasoning"] = strings.Repeat("looks fine ", 20)
			},
			wantErr: "distinct words",
		},
		{
			name: "under the minimum length",
			mutate: func(packet map[string]any) {
				packet["reasoning"] = "Checked the client table, seems okay to me."
			},
			wantErr: "characters",
		},
		{
			name: "no related areas actually named",
			mutate: func(packet map[string]any) {
				packet["related_areas_checked"] = []string{}
			},
			wantErr: "related area",
		},
		{
			name: "placeholder related area",
			mutate: func(packet map[string]any) {
				packet["related_areas_checked"] = []string{"n/a"}
			},
			wantErr: "placeholder",
		},
		{
			name: "wrong contract",
			mutate: func(packet map[string]any) {
				packet["contract"] = "some_other.v1"
			},
			wantErr: "blast_radius_review.v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx, reviewerTaskID := prepareNonLiveReviewFixture(t)
			packet := validWorkflowBlastRadiusReview()
			tc.mutate(packet)

			result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowBlastRadiusReviewPassedRequest(fx, reviewerTaskID, packet, false))
			if result.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d %s, want 400", tc.name, result.Code, result.Body.String())
			}
			if !strings.Contains(strings.ToLower(result.Body.String()), strings.ToLower(tc.wantErr)) {
				t.Fatalf("%s: rejection %q does not mention %q", tc.name, result.Body.String(), tc.wantErr)
			}
		})
	}
}

// TestWorkflowControlReviewPassedAcceptsGenuineBlastRadiusReview is the green
// side: real, substantive blast-radius reasoning alongside valid test
// evidence is accepted and the plan advances exactly as before, and both
// packets land together on the durable workflow event record so the owner
// could pull them later to spot-check reviewer quality.
func TestWorkflowControlReviewPassedAcceptsGenuineBlastRadiusReview(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)

	result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowBlastRadiusReviewPassedRequest(fx, reviewerTaskID, validWorkflowBlastRadiusReview(), false))
	if result.Code != http.StatusCreated {
		t.Fatalf("genuine blast-radius reasoning was rejected: %d %s", result.Code, result.Body.String())
	}
	var response issueWorkflowEventResponse
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Decision.Next != workflowstate.Done {
		t.Fatalf("plan did not advance to done: %+v", response)
	}

	var payload []byte
	if err := testPool.QueryRow(context.Background(),
		`SELECT payload FROM issue_workflow_event WHERE id = $1`, response.EventID,
	).Scan(&payload); err != nil {
		t.Fatalf("load stored event payload: %v", err)
	}
	var stored struct {
		TestEvidence       map[string]any `json:"test_evidence"`
		BlastRadiusReview  map[string]any `json:"blast_radius_review"`
	}
	if err := json.Unmarshal(payload, &stored); err != nil {
		t.Fatalf("decode stored payload: %v", err)
	}
	if stored.TestEvidence == nil {
		t.Fatalf("stored event payload dropped test_evidence: %s", payload)
	}
	if stored.BlastRadiusReview == nil || stored.BlastRadiusReview["reasoning"] == "" {
		t.Fatalf("stored event payload did not record reviewer's blast-radius reasoning for later spot-checking: %s", payload)
	}
}
