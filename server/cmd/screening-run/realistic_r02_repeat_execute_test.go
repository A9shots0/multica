package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestR02RepeatPaidPathUsesOnlyActiveGenerationAdmission(t *testing.T) {
	admission, _, err := agentqualification.CurrentR02RepeatAdmission()
	if err != nil {
		t.Fatal(err)
	}
	if admission.Status != "frozen_reviewed" || !admission.ProviderCallsAllowed {
		t.Fatal("active paid path admission unexpectedly locked")
	}
	matrix, err := agentqualification.R02RepeatBoundedMatrix(admission)
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix.Candidates) != 1 || len(matrix.ExecutionOrder) != 1 || matrix.ExecutionOrder[0] != "R02" || matrix.MaxIncrementalSpendMicros != 350_000 {
		t.Fatalf("dedicated paid envelope drifted: %+v", matrix)
	}
}
