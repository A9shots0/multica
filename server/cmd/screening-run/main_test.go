package main

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestPaidExecutionFailsClosed(t *testing.T) {
	err := paidExecutionDisabled()
	if err == nil {
		t.Fatal("paid execution was enabled before its safety gates existed")
	}
	for _, required := range []string{"admission", "isolation", "provenance"} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("refusal omitted %q: %v", required, err)
		}
	}
}

func TestMinimumSuiteManifestIsAvailableWithoutExecution(t *testing.T) {
	suite, err := agentqualification.CurrentMinimumScreeningSuite()
	if err != nil {
		t.Fatal(err)
	}
	if suite.Status != "calibrated_partially_executed" || suite.ProgressionPolicy != "t12_pair_then_survivors_remaining_cases" {
		t.Fatalf("unexpected suite: %+v", suite)
	}
}

func TestOpenRouterBatchDryRunIsAvailableWithoutExecution(t *testing.T) {
	batch, err := agentqualification.ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := agentqualification.ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 20 || assignments[0].Configuration.RuntimeProvider != agentqualification.OpenCodeCLIRuntimeProvider || batch.Status != "planned_not_launched" {
		t.Fatalf("unexpected OpenRouter dry run: %+v", assignments)
	}
}
