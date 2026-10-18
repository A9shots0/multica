package publicapiv1

import (
	"strings"
	"testing"
)

func validOutcomeReceipt() OutcomeReceiptV02 {
	return OutcomeReceiptV02{
		WorkID: "work-1", RunID: "run-1", ReceiptRevision: 1, Provider: "codex",
		Objective: "Deliver the bounded result.", Scope: "Only the named issue.", RequestedMode: "change",
		Authority: OutcomeReceiptAuthority{Reference: "native-authority-1", AllowedEffects: []string{"source write"}},
		State:     "working", CurrentMilestone: "Run the focused verification.",
		ChangeSinceLastReceipt: "The bounded implementation is ready for verification.",
		Evidence:               []OutcomeReceiptEvidence{{Type: "test", Reference: "focused test result"}},
		Dependencies:           []string{}, NextAction: "Run the focused verification.",
		OwnerDecision: OutcomeReceiptOwnerDecision{Options: []string{}, InvalidatedBy: []string{}}, Risk: "low",
		ResourceUsage: OutcomeReceiptResourceUsage{Source: "unavailable"},
		Effect:        OutcomeReceiptEffect{UserOutcome: "unchanged"},
		Freshness:     OutcomeReceiptFreshness{ObservedAt: "2026-10-14T21:00:00Z"},
		Acceptance:    OutcomeReceiptAcceptance{Stage: "local", Status: "partial", CriteriaRemaining: []string{"focused verification"}},
	}
}

func TestOutcomeReceiptV02ValidationGuards(t *testing.T) {
	valid := validOutcomeReceipt()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*OutcomeReceiptV02)
	}{
		{"false completion", func(r *OutcomeReceiptV02) { r.State = "complete" }},
		{"skipped predecessor", func(r *OutcomeReceiptV02) {
			r.ReceiptRevision = 3
			prior := int64(1)
			r.SupersedesReceiptRevision = &prior
		}},
		{"owner wait without proposal", func(r *OutcomeReceiptV02) { r.State = "waiting_owner" }},
		{"negative usage", func(r *OutcomeReceiptV02) { n := int64(-1); r.ResourceUsage.Attempts = &n }},
		{"active usage exceeds elapsed time", func(r *OutcomeReceiptV02) {
			active, elapsed := int64(2), int64(1)
			r.ResourceUsage.ActiveComputeSeconds, r.ResourceUsage.WallClockSeconds = &active, &elapsed
		}},
		{"unknown provider", func(r *OutcomeReceiptV02) { r.Provider = "untrusted-new-provider" }},
		{"effectful without explicit effects", func(r *OutcomeReceiptV02) { r.Authority.AllowedEffects = []string{} }},
		{"oversized identifier", func(r *OutcomeReceiptV02) { r.WorkID = strings.Repeat("x", maxExternalIdentifierLength+1) }},
		{"blank dependency", func(r *OutcomeReceiptV02) { r.Dependencies = []string{" "} }},
		{"missing evidence", func(r *OutcomeReceiptV02) { r.Evidence = []OutcomeReceiptEvidence{} }},
		{"blank expiry", func(r *OutcomeReceiptV02) { blank := " "; r.Freshness.ExpiresAt = &blank }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := validOutcomeReceipt()
			tt.mutate(&receipt)
			if err := receipt.Validate(); err == nil {
				t.Fatal("invalid receipt was accepted")
			}
		})
	}
}

func TestOutcomeReceiptV02AcceptsExactNextRevisionAndBoundedDecisionProposal(t *testing.T) {
	receipt := validOutcomeReceipt()
	receipt.ReceiptRevision = 2
	prior := int64(1)
	receipt.SupersedesReceiptRevision = &prior
	receipt.State = "waiting_owner"
	category, question, why := "business", "Which bounded option should proceed?", "The two safe options have different business outcomes."
	receipt.OwnerDecision = OutcomeReceiptOwnerDecision{
		Required: true, Category: &category, Question: &question,
		Options: []string{"A", "B"}, WhyNow: &why, InvalidatedBy: []string{"scope changes"},
	}
	if err := receipt.Validate(); err != nil {
		t.Fatalf("bounded decision proposal rejected: %v", err)
	}
}

func TestContextV02ContractsRejectAmbiguousOrUntraceablePackets(t *testing.T) {
	reference := "repo@commit"
	request := ContextRequestV02{
		RequestID: "ctx-1", WorkID: "work-1", RequestType: "source", Need: "Exact source revision",
		Reason: "Bind verification to source", Scope: "one repository", FreshnessRequired: "immutable revision",
		MaximumPayload: "compact", ResponseContract: ContextResponseContract{RequiredFields: []string{"revision"}, EvidenceLevel: "authoritative"},
		AuthorizationBoundary: "read_only", CachePolicy: "until source revision changes",
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid context request rejected: %v", err)
	}
	response := ContextResponseV02{
		RequestID: "ctx-1", WorkID: "work-1", Status: "found",
		Source: ContextResponseSource{System: "repository", Reference: &reference}, EvidenceLevel: "authoritative",
		ObservedAt: "2026-10-14T21:00:00Z", Payload: map[string]any{"revision": "abc"}, Limitations: []string{},
	}
	if err := response.Validate(); err != nil {
		t.Fatalf("valid context response rejected: %v", err)
	}

	request.AuthorizationBoundary = "do_anything"
	if err := request.Validate(); err == nil {
		t.Fatal("unsupported authorization boundary was accepted")
	}
	response.Source.Reference = nil
	if err := response.Validate(); err == nil {
		t.Fatal("found response without a traceable source was accepted")
	}
}

func TestContextRequestV02BoundsFieldsAndText(t *testing.T) {
	request := ContextRequestV02{
		RequestID: "ctx-1", WorkID: "work-1", RequestType: "source", Need: "One exact fact",
		Reason: "Needed now", Scope: "one issue", FreshnessRequired: "current", MaximumPayload: "compact",
		ResponseContract:      ContextResponseContract{RequiredFields: []string{"references"}, EvidenceLevel: "observed"},
		AuthorizationBoundary: "read_only", CachePolicy: "never",
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	request.ResponseContract.RequiredFields = []string{"references", "references"}
	if err := request.Validate(); err == nil {
		t.Fatal("duplicate response fields were accepted")
	}
	request.ResponseContract.RequiredFields = []string{"references"}
	request.Need = strings.Repeat("x", maxExternalTextLength+1)
	if err := request.Validate(); err == nil {
		t.Fatal("oversized context need was accepted")
	}
}
