package handler

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func workflowBriefingTestUUID(last byte) pgtype.UUID {
	var bytes [16]byte
	bytes[15] = last
	return pgtype.UUID{Bytes: bytes, Valid: true}
}

func TestBuildIssueWorkflowBriefingIsRoleSpecificAndDoesNotRouteThroughLeader(t *testing.T) {
	plan := db.IssueWorkflowPlan{
		ImplementerID:  workflowBriefingTestUUID(1),
		ReviewerID:     workflowBriefingTestUUID(2),
		ReleaseAgentID: workflowBriefingTestUUID(3),
		VerifierID:     workflowBriefingTestUUID(4),
	}

	tests := []struct {
		name      string
		agentID   string
		wantEvent string
	}{
		{"implementer", uuidToString(plan.ImplementerID), "implementation_ready"},
		{"reviewer", uuidToString(plan.ReviewerID), "review_passed"},
		{"release", uuidToString(plan.ReleaseAgentID), "deployment_succeeded"},
		{"verifier", uuidToString(plan.VerifierID), "production_verified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildIssueWorkflowBriefing(plan, tt.agentID, "00000000-0000-0000-0000-000000000099")
			if !strings.Contains(got, tt.wantEvent) {
				t.Fatalf("briefing %q does not contain %q", got, tt.wantEvent)
			}
			if !strings.Contains(got, "Do not mention Mika") {
				t.Fatalf("briefing must suppress leader relay: %q", got)
			}
		})
	}

	if got := buildIssueWorkflowBriefing(plan, "00000000-0000-0000-0000-000000000099", "issue"); got != "" {
		t.Fatalf("unplanned agent received workflow instructions: %q", got)
	}
}

func TestBuildIssueWorkflowBriefingIncludesOnlyBoundedCorrectionPacketForWorker(t *testing.T) {
	plan := db.IssueWorkflowPlan{
		ImplementerID: workflowBriefingTestUUID(1),
		ReviewerID:    workflowBriefingTestUUID(2),
	}
	packet := &workflowCorrectionPacket{
		FailedCriteria:     []string{"exact output must pass"},
		Evidence:           "received DRAFT",
		RequiredCorrection: "return APPROVED",
		Candidate:          "candidate-1",
	}
	got := buildIssueWorkflowBriefing(plan, uuidToString(plan.ImplementerID), "SILV-7", packet)
	for _, want := range []string{"Bounded Correction Packet", "exact output must pass", "received DRAFT", "return APPROVED", "candidate-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("correction briefing %q does not contain %q", got, want)
		}
	}
	if reviewer := buildIssueWorkflowBriefing(plan, uuidToString(plan.ReviewerID), "SILV-7", packet); strings.Contains(reviewer, "Bounded Correction Packet") {
		t.Fatalf("reviewer received worker correction packet: %q", reviewer)
	}
}
