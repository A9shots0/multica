package developmentmemory

import (
	"strings"
	"testing"
)

func TestParseProposalAcceptsStrictBoundDraft(t *testing.T) {
	binding := Binding{SchemaVersion: 1, WorkstreamID: "w", Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/smcrm"}}}
	draft := ParseProposal(`Done.
<development-memory>{"summary":"Kept the contract.","next_action":"Run acceptance.","remaining_uncertainty":["Preview not accepted."],"memory_items":[{"repository_identity":"a9shots0/smcrm","kind":"decision","statement":"CRM owns the UI.","evidence_refs":["commit:abc"]}],"repository_evidence":[{"canonical_identity":"a9shots0/smcrm","revision":null,"branch":null,"dirty":null,"evidence_refs":[]}]}</development-memory>`, "", binding)
	if draft.Summary != "Kept the contract." || len(draft.MemoryItems) != 1 {
		t.Fatalf("draft = %#v", draft)
	}
	if draft.MemoryItems[0].Scope != "workstream" {
		t.Fatalf("default scope = %q", draft.MemoryItems[0].Scope)
	}
}

func TestValidateProposalAcceptsExplicitProjectScope(t *testing.T) {
	binding := Binding{SchemaVersion: 1, WorkstreamID: "w", Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/smcrm"}}}
	draft, err := ValidateProposal(ProposalDraft{
		Summary: "Keep every CRM-facing control consistent.", NextAction: "Review future UI against this rule.",
		MemoryItems: []MemoryItemDraft{{Scope: "project", Kind: "decision", Statement: "CRM owns operator-facing UI."}},
	}, binding)
	if err != nil {
		t.Fatal(err)
	}
	if draft.MemoryItems[0].Scope != "project" {
		t.Fatalf("scope = %q", draft.MemoryItems[0].Scope)
	}
}

func TestValidateProposalRejectsUnknownScope(t *testing.T) {
	binding := Binding{SchemaVersion: 1, WorkstreamID: "w", Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/smcrm"}}}
	_, err := ValidateProposal(ProposalDraft{
		Summary: "Bad scope.", NextAction: "Reject it.",
		MemoryItems: []MemoryItemDraft{{Scope: "company", Kind: "decision", Statement: "Unbounded."}},
	}, binding)
	if err == nil {
		t.Fatal("unknown scope was accepted")
	}
}

func TestParseProposalDoesNotTrustInvalidRepository(t *testing.T) {
	binding := Binding{SchemaVersion: 1, WorkstreamID: "w", Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/smcrm"}}}
	draft := ParseProposal(`<development-memory>{"summary":"Bad","next_action":"Ship","remaining_uncertainty":[],"memory_items":[],"repository_evidence":[{"canonical_identity":"attacker/repo","revision":"abc","branch":"main","dirty":false,"evidence_refs":[]}]}</development-memory>`, "", binding)
	if draft.Summary == "Bad" {
		t.Fatal("invalid repository proposal was trusted")
	}
	if len(draft.RepositoryEvidence) != 1 || draft.RepositoryEvidence[0].CanonicalIdentity != "a9shots0/smcrm" {
		t.Fatalf("fallback = %#v", draft)
	}
}

func TestFailedTaskFallbackPreservesTheFailureAsReviewableMemory(t *testing.T) {
	binding := Binding{SchemaVersion: 1, WorkstreamID: "w", Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/smcrm"}}}
	draft := parseProposal("provider exited before the patch was verified", "codex/fix", "failed", binding)
	if draft.Summary != "provider exited before the patch was verified" {
		t.Fatalf("summary = %q", draft.Summary)
	}
	if draft.NextAction != "Review the failure, preserve the verified blocker, and record the next recovery action." {
		t.Fatalf("next action = %q", draft.NextAction)
	}
}

func TestProposalRedactsCredentialShapedTextBeforePersistence(t *testing.T) {
	binding := Binding{SchemaVersion: 1, WorkstreamID: "w", Repositories: []BoundRepository{{CanonicalIdentity: "a9shots0/smcrm"}}}
	draft := ParseProposal(`<development-memory>{"summary":"Found Bearer secret-token-value","next_action":"Remove TOKEN=secret-value","remaining_uncertainty":[],"memory_items":[],"repository_evidence":[]}</development-memory>`, "", binding)
	if strings.Contains(draft.Summary, "secret-token-value") || strings.Contains(draft.NextAction, "secret-value") {
		t.Fatalf("credential-shaped text was retained: %#v", draft)
	}
}

func TestReviewableProposalKeepsOnlyDurableOwnerContext(t *testing.T) {
	draft := ProposalDraft{
		Summary: "Technical run report", NextAction: "Check the build", RemainingUncertainty: []string{"runtime unknown"},
		MemoryItems: []MemoryItemDraft{
			{Kind: "finding", Statement: "Repository clean at HEAD e2f340bb1e2b5ea63ce9fa03fd7c529e7fa644ef."},
			{Kind: "decision", Statement: "Chat and Inbox are the owner's primary Multica surfaces."},
			{Kind: "decision", Statement: "Chat and Inbox are the owner's primary Multica surfaces."},
			{Kind: "fact", Statement: "Acceptance test canary passed."},
		},
	}
	reviewable, ok := ReviewableProposal(draft)
	if !ok || len(reviewable.MemoryItems) != 1 {
		t.Fatalf("reviewable = %#v ok=%v", reviewable, ok)
	}
	if reviewable.MemoryItems[0].Statement != "Chat and Inbox are the owner's primary Multica surfaces." {
		t.Fatalf("statement = %q", reviewable.MemoryItems[0].Statement)
	}
	if reviewable.Summary != reviewable.MemoryItems[0].Statement || len(reviewable.RemainingUncertainty) != 0 {
		t.Fatalf("sanitized review = %#v", reviewable)
	}
}

func TestReviewableProposalRejectsRunMetadataOnly(t *testing.T) {
	_, ok := ReviewableProposal(ProposalDraft{MemoryItems: []MemoryItemDraft{
		{Kind: "acceptance", Statement: "Final write-back schema acceptance passed."},
		{Kind: "finding", Statement: "Tests passed at commit abcdef1234567."},
	}})
	if ok {
		t.Fatal("run metadata became durable memory")
	}
}
