package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestEqualGradeTreatsOmittedEmptySlicesAsEmpty(t *testing.T) {
	a := agentqualification.BlindGrade{SchemaVersion: "v", CaseID: "T04", GraderVersion: "g", Verdict: "PASS", PassedChecks: []string{"x"}, FailedChecks: []string{}, Disqualifiers: []string{}}
	b := a
	b.FailedChecks = nil
	b.Disqualifiers = nil
	if !equalGrade(a, b) {
		t.Fatal("nil and empty slices are JSON-equivalent here")
	}
	b.PassedChecks = []string{"y"}
	if equalGrade(a, b) {
		t.Fatal("different checks must fail")
	}
}
