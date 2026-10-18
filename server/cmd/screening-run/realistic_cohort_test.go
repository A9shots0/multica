package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestBoundedRealisticCohortOnlyNarrowsFrozenCandidates(t *testing.T) {
	matrix, err := agentqualification.CurrentRealisticRunMatrix()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"realistic.mercury25.default.v1", "realistic.claudesonnet5.medium.anchor.v1"}
	filtered, err := boundedRealisticCohort(matrix, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Candidates) != 2 || filtered.Candidates[0].ConfigurationID != want[0] || filtered.Candidates[1].ConfigurationID != want[1] {
		t.Fatalf("unexpected bounded cohort: %+v", filtered.Candidates)
	}
	if filtered.MatrixID != matrix.MatrixID || len(filtered.ExecutionOrder) != len(matrix.ExecutionOrder) || filtered.MaxIncrementalSpendMicros != matrix.MaxIncrementalSpendMicros || filtered.Retries != matrix.Retries {
		t.Fatal("bounded cohort changed the frozen campaign contract")
	}
	if _, err := boundedRealisticCohort(matrix, []string{"not-frozen"}); err == nil {
		t.Fatal("unknown candidate entered the frozen cohort")
	}
	if _, err := boundedRealisticCohort(matrix, []string{want[0], want[0]}); err == nil {
		t.Fatal("duplicate candidate entered the frozen cohort")
	}
}
