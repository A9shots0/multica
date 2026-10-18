package main

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestReliabilityMatrixFreezeStatusControlsProviderReadiness(t *testing.T) {
	root := t.TempDir()
	plan, err := realisticPlanForMatrix(root, "realistic-foundation-screen-v10", nil)
	if err != nil {
		t.Fatal(err)
	}
	assignments, ok := plan["next_assignments"].([]agentqualification.RealisticAssignment)
	if !ok || len(assignments) != 1 || assignments[0].CaseID != "R01" {
		t.Fatalf("pending reliability plan drifted: %#v", plan["next_assignments"])
	}
	if plan["freeze_ready"] != true {
		t.Fatalf("reviewed reliability matrix was not presented as provider-ready: %#v", plan)
	}
	matrix, err := agentqualification.CurrentRealisticRunMatrixByID("realistic-foundation-screen-v10")
	if err != nil {
		t.Fatal(err)
	}
	matrix.Status = "pending_independent_review"
	err = requireReviewedRealisticMatrix(matrix)
	if err == nil || !strings.Contains(err.Error(), "independently reviewed and frozen") {
		t.Fatalf("pending matrix execution was not locked: %v", err)
	}
}
