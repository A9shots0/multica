package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowstate"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// validWorkflowTestEvidence builds a genuine-shaped build_test_run.v1 packet:
// a passing `go build` run and a passing `go test` run, both with plausible
// exit codes, output digests and an observed_at inside the review window.
func validWorkflowTestEvidence() map[string]any {
	buildOutput := "go: downloading nothing new\n"
	testOutput := "ok  \tgithub.com/multica-ai/multica/server/internal/handler\t1.372s\n"
	buildSum := sha256.Sum256([]byte(buildOutput))
	testSum := sha256.Sum256([]byte(testOutput))
	return map[string]any{
		"schema_version": 1,
		"contract":       "build_test_run.v1",
		"runs": []map[string]any{
			{
				"command":       "go build ./...",
				"exit_code":     0,
				"observed_at":   time.Now().Add(-30 * time.Second).Format(time.RFC3339Nano),
				"output_sha256": hex.EncodeToString(buildSum[:]),
				"output_tail":   buildOutput,
			},
			{
				"command":       "go test ./...",
				"exit_code":     0,
				"observed_at":   time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano),
				"output_sha256": hex.EncodeToString(testSum[:]),
				"output_tail":   testOutput,
			},
		},
	}
}

// workflowReviewPassedRequest builds a raw review_passed request body so tests
// in this file can add or omit test_evidence explicitly, independent of the
// shared workflowEventRequestFor helper. It always carries a valid, default
// blast_radius_review packet — this file's tests are about test_evidence
// specifically, so the blast-radius requirement (covered in its own test
// file) is held constant and valid here rather than re-tested incidentally.
func workflowReviewPassedRequest(fx workflowControlFixture, taskID string, testEvidence any) *http.Request {
	body := map[string]any{"event": workflowstate.ReviewPassed, "reason": "scenario evidence", "blast_radius_review": validWorkflowBlastRadiusReview()}
	if testEvidence != nil {
		body["test_evidence"] = testEvidence
	}
	req := withURLParam(newRequest(http.MethodPost, "/api/issues/"+fx.IssueID+"/workflow-events", body), "id", fx.IssueID)
	req.Header.Set("X-Agent-ID", fx.ReviewerID)
	req.Header.Set("X-Task-ID", taskID)
	return req
}

// TestWorkflowControlReviewPassedRequiresTestEvidence proves the gap this
// change closes: today, review_passed carrying zero build/test evidence is
// accepted purely on the reviewer's word, so a review that never actually
// built or tested the candidate is indistinguishable from one that did.
func TestWorkflowControlReviewPassedRequiresTestEvidence(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)

	result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowReviewPassedRequest(fx, reviewerTaskID, nil))
	if result.Code != http.StatusBadRequest {
		t.Fatalf("review_passed with no test_evidence = %d %s, want 400 (evidence must be required)", result.Code, result.Body.String())
	}
	if !strings.Contains(result.Body.String(), "test_evidence") {
		t.Fatalf("rejection did not explain the missing evidence: %s", result.Body.String())
	}

	// The plan must still be sitting in technical_review; nothing advanced.
	plan, err := testHandler.Queries.GetIssueWorkflowPlan(context.Background(), db.GetIssueWorkflowPlanParams{
		IssueID: parseUUID(fx.IssueID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	if plan.CurrentState != string(workflowstate.TechnicalReview) {
		t.Fatalf("plan advanced past review without evidence: %q", plan.CurrentState)
	}
}

// TestWorkflowControlReviewPassedRejectsFabricatedOrFailingTestEvidence covers
// the shapes an unreliable or dishonest reviewer could otherwise submit: a
// missing required command, and a run that admits it failed. Both must be
// refused, not silently accepted the way the bare "reason" string is today.
func TestWorkflowControlReviewPassedRejectsFabricatedOrFailingTestEvidence(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	cases := []struct {
		name    string
		mutate  func(packet map[string]any)
		wantErr string
	}{
		{
			name: "missing test run, only build",
			mutate: func(packet map[string]any) {
				runs := packet["runs"].([]map[string]any)
				packet["runs"] = runs[:1]
			},
			wantErr: "test",
		},
		{
			name: "declared test run failed",
			mutate: func(packet map[string]any) {
				runs := packet["runs"].([]map[string]any)
				runs[1]["exit_code"] = 1
			},
			wantErr: "exit zero",
		},
		{
			name: "unrecognized command cannot stand in for the real test suite",
			mutate: func(packet map[string]any) {
				runs := packet["runs"].([]map[string]any)
				runs[1]["command"] = "echo it passed, trust me"
			},
			wantErr: "test",
		},
		{
			name: "malformed output digest",
			mutate: func(packet map[string]any) {
				runs := packet["runs"].([]map[string]any)
				runs[1]["output_sha256"] = "not-a-hash"
			},
			wantErr: "SHA-256",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx, reviewerTaskID := prepareNonLiveReviewFixture(t)
			packet := validWorkflowTestEvidence()
			tc.mutate(packet)

			result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowReviewPassedRequest(fx, reviewerTaskID, packet))
			if result.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d %s, want 400", tc.name, result.Code, result.Body.String())
			}
			if !strings.Contains(strings.ToLower(result.Body.String()), strings.ToLower(tc.wantErr)) {
				t.Fatalf("%s: rejection %q does not mention %q", tc.name, result.Body.String(), tc.wantErr)
			}
		})
	}
}

// TestWorkflowControlReviewPassedAcceptsGenuinePassingTestEvidence is the green
// side: a structurally genuine, exit-zero build and test run for the required
// commands is accepted and the plan advances exactly as before.
func TestWorkflowControlReviewPassedAcceptsGenuinePassingTestEvidence(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	fx, reviewerTaskID := prepareNonLiveReviewFixture(t)

	result := testutil.Call(t, testHandler.PostIssueWorkflowEvent, workflowReviewPassedRequest(fx, reviewerTaskID, validWorkflowTestEvidence()))
	if result.Code != http.StatusCreated {
		t.Fatalf("genuine passing evidence was rejected: %d %s", result.Code, result.Body.String())
	}
	var response issueWorkflowEventResponse
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Decision.Next != workflowstate.Done {
		t.Fatalf("plan did not advance to done: %+v", response)
	}
}
