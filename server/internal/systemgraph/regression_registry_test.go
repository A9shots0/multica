package systemgraph

import (
	"errors"
	"testing"
	"time"
)

func regressionSubmission(t *testing.T, id string, kind RegressionChangeKind, expected uint64, requirement string, now time.Time) (RegressionProposal, RegressionDomainEvidence, RegressionApprovalReceipt) {
	t.Helper()
	status := RegressionActive
	if kind == RegressionRetire {
		status = RegressionRetired
	}
	obligation := RegressionObligation{
		ID:                      "reg-account-assignment",
		Version:                 expected + 1,
		Title:                   "Account assignment remains consistent",
		Requirement:             requirement,
		ProofMethod:             "verify stable CRM id through every affected projection",
		AccountableDomainRoleID: "crm_access_outcome",
		Scope: RegressionScope{
			JourneyIDs: []string{"j_identity_access"},
			NodeIDs:    []string{"crm_identity", "linkhub"},
			EdgeIDs:    []string{"e_identity_linkhub"},
		},
		Status:      status,
		ReviewAfter: now.Add(24 * time.Hour),
	}
	if kind == RegressionSupersede {
		obligation.SupersedesVersion = expected
	}
	proposal := RegressionProposal{
		ID:              id,
		Kind:            kind,
		ExpectedVersion: expected,
		ProposedVersion: expected + 1,
		AuthorRoleID:    "crm_access_outcome",
		AuthorFunction:  RegressionAuthorAccountableDomain,
		CreatedAt:       now.Add(-10 * time.Minute),
		Obligation:      obligation,
	}
	evidence := RegressionDomainEvidence{
		ID:                "evidence-" + id,
		ProposalID:        id,
		AuthorAgentID:     "domain-agent",
		AccountableRoleID: "crm_access_outcome",
		Scope:             obligation.Scope,
		SourceRefs:        []string{"receipt://crm/account-assignment"},
		Source:            "controller_record",
		Immutable:         true,
		ObservedAt:        now.Add(-20 * time.Minute),
		ReviewAfter:       now.Add(time.Hour),
	}
	hash, err := regressionHash(normalizeRegressionProposal(proposal))
	if err != nil {
		t.Fatal(err)
	}
	approval := RegressionApprovalReceipt{
		ID:                  "approval-" + id,
		ProposalID:          id,
		ProposalHash:        hash,
		ApproverAgentID:     "independent-agent",
		ApproverRoleID:      "quality_checker",
		ApproverFunction:    "independent_assurance",
		IndependentFromRole: "crm_access_outcome",
		Source:              "controller_record",
		Approved:            true,
		Immutable:           true,
		IssuedAt:            now.Add(-5 * time.Minute),
		ExpiresAt:           now.Add(time.Hour),
	}
	return proposal, evidence, approval
}

func applyRegression(t *testing.T, registry *RegressionRegistry, proposal RegressionProposal, evidence RegressionDomainEvidence, approval RegressionApprovalReceipt, now time.Time) RegressionChangeRecord {
	t.Helper()
	record, err := registry.Apply(proposal, evidence, approval, now)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func regressionImpact(t *testing.T, receiptID string, journeyIDs ...string) RegressionImpactScope {
	t.Helper()
	graph, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	impact, err := BuildRegressionImpactScope(graph, receiptID, journeyIDs)
	if err != nil {
		t.Fatal(err)
	}
	return impact
}

func TestRegressionRegistryVersionedLifecycleAndImmutableHistory(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	for _, step := range []struct {
		id          string
		kind        RegressionChangeKind
		expected    uint64
		requirement string
	}{
		{"proposal-add", RegressionAdd, 0, "v1 behavior"},
		{"proposal-update", RegressionUpdate, 1, "v2 clarified behavior"},
		{"proposal-supersede", RegressionSupersede, 2, "v3 replacement behavior"},
		{"proposal-retire", RegressionRetire, 3, "v4 retired behavior"},
	} {
		proposal, evidence, approval := regressionSubmission(t, step.id, step.kind, step.expected, step.requirement, now)
		applyRegression(t, registry, proposal, evidence, approval, now)
	}
	history := registry.History()
	if len(history) != 4 || history[3].RegistryVersion != 4 || history[3].Proposal.Obligation.Status != RegressionRetired {
		t.Fatalf("unexpected lifecycle history: %+v", history)
	}
	for i := 1; i < len(history); i++ {
		if history[i].PreviousRecordHash != history[i-1].RecordHash {
			t.Fatalf("record %d does not bind previous hash", i)
		}
	}
	// Mutating a returned snapshot must not alter the append-only registry.
	history[0].Proposal.Obligation.Requirement = "forged"
	if registry.History()[0].Proposal.Obligation.Requirement != "v1 behavior" {
		t.Fatal("history snapshot mutated registry state")
	}
	selection, err := registry.ActiveFor(regressionImpact(t, "impact-lifecycle", "j_identity_access"), RegressionConsumerRelease, now)
	if err != nil || len(selection.Obligations) != 0 {
		t.Fatalf("retired obligation remained active: %+v err=%v", selection, err)
	}
}

func TestRegressionRegistrySelectsOnlyAffectedScopeForReleaseAndQC(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	proposal, evidence, approval := regressionSubmission(t, "proposal-add", RegressionAdd, 0, "stable identity remains consistent", now)
	applyRegression(t, registry, proposal, evidence, approval, now)
	for _, consumer := range []string{RegressionConsumerRelease, RegressionConsumerQualityChecker} {
		selection, err := registry.ActiveFor(regressionImpact(t, "impact-identity-"+consumer, "j_identity_access"), consumer, now)
		if err != nil || len(selection.Obligations) != 1 || selection.Obligations[0].ID != proposal.Obligation.ID {
			t.Fatalf("%s selection = %+v err=%v", consumer, selection, err)
		}
	}
	unrelated, err := registry.ActiveFor(regressionImpact(t, "impact-local-control", "j_local_control"), RegressionConsumerRelease, now)
	if err != nil || len(unrelated.Obligations) != 0 {
		t.Fatalf("unrelated scope leaked obligation: %+v err=%v", unrelated, err)
	}
	if _, err := registry.ActiveFor(regressionImpact(t, "impact-denied", "j_identity_access"), "shared_windows_executor", now); !errors.Is(err, ErrRegressionConsumerDenied) {
		t.Fatalf("executor consumed registry: %v", err)
	}
}

func TestRegressionRegistryRejectsUnboundIncompleteAndInvalidImpactQueries(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	proposal, evidence, approval := regressionSubmission(t, "proposal-add", RegressionAdd, 0, "stable identity remains consistent", now)
	applyRegression(t, registry, proposal, evidence, approval, now)
	valid := regressionImpact(t, "controller-impact-1", "j_identity_access")

	tests := []struct {
		name   string
		mutate func(*RegressionImpactScope)
	}{
		{"not complete", func(value *RegressionImpactScope) { value.Complete = false }},
		{"wrong source", func(value *RegressionImpactScope) { value.Source = "release_guess" }},
		{"wrong graph version", func(value *RegressionImpactScope) { value.GraphVersion = "invented" }},
		{"missing controller receipt", func(value *RegressionImpactScope) { value.ControllerReceiptID = "" }},
		{"binding drift", func(value *RegressionImpactScope) { value.BindingHash = "sha256:forged" }},
		{"under scoped", func(value *RegressionImpactScope) {
			value.Scope.NodeIDs = value.Scope.NodeIDs[1:]
			value.BindingHash, _ = regressionImpactHash(*value)
		}},
		{"out of journey", func(value *RegressionImpactScope) {
			value.Scope.NodeIDs = append(value.Scope.NodeIDs, "phonefleet")
			value.BindingHash, _ = regressionImpactHash(*value)
		}},
		{"unknown journey", func(value *RegressionImpactScope) {
			value.Scope.JourneyIDs = []string{"j_invented"}
			value.BindingHash, _ = regressionImpactHash(*value)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			impact := valid
			impact.Scope = cloneRegressionScope(valid.Scope)
			test.mutate(&impact)
			if _, err := registry.ActiveFor(impact, RegressionConsumerRelease, now); !errors.Is(err, ErrRegressionInvalid) {
				t.Fatalf("invalid impact query was accepted: %v", err)
			}
		})
	}
}

func TestRegressionRegistryRejectsNonDomainAuthorsAndNonIndependentApproval(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	for _, author := range []struct{ role, function string }{
		{"release_runtime_assurance", RegressionConsumerRelease},
		{"quality_checker", RegressionConsumerQualityChecker},
	} {
		registry := NewRegressionRegistry()
		proposal, evidence, approval := regressionSubmission(t, "unauthorized-"+author.role, RegressionAdd, 0, "behavior", now)
		proposal.AuthorRoleID, proposal.AuthorFunction = author.role, author.function
		proposal.Obligation.AccountableDomainRoleID = author.role
		evidence.AccountableRoleID = author.role
		evidence.Scope = proposal.Obligation.Scope
		hash, _ := regressionHash(normalizeRegressionProposal(proposal))
		approval.ProposalHash = hash
		if _, err := registry.Apply(proposal, evidence, approval, now); !errors.Is(err, ErrRegressionUnauthorized) {
			t.Fatalf("author %s was not denied: %v", author.role, err)
		}
		if len(registry.History()) != 0 {
			t.Fatal("unauthorized proposal entered immutable history")
		}
	}

	registry := NewRegressionRegistry()
	proposal, evidence, approval := regressionSubmission(t, "not-independent", RegressionAdd, 0, "behavior", now)
	approval.ApproverAgentID = evidence.AuthorAgentID
	if _, err := registry.Apply(proposal, evidence, approval, now); !errors.Is(err, ErrRegressionUnauthorized) {
		t.Fatalf("non-independent approval was not denied: %v", err)
	}

	registry = NewRegressionRegistry()
	proposal, evidence, approval = regressionSubmission(t, "unknown-approver", RegressionAdd, 0, "behavior", now)
	approval.ApproverRoleID = "made_up_reviewer"
	if _, err := registry.Apply(proposal, evidence, approval, now); !errors.Is(err, ErrRegressionUnauthorized) {
		t.Fatalf("unknown approval role was not denied: %v", err)
	}
}

func TestRegressionRegistryRejectsUnknownAndOutOfJourneyGraphTargets(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*RegressionProposal)
	}{
		{"unknown journey", func(proposal *RegressionProposal) { proposal.Obligation.Scope.JourneyIDs = []string{"j_invented"} }},
		{"unknown node", func(proposal *RegressionProposal) {
			proposal.Obligation.Scope.NodeIDs = append(proposal.Obligation.Scope.NodeIDs, "invented_node")
		}},
		{"unknown edge", func(proposal *RegressionProposal) {
			proposal.Obligation.Scope.EdgeIDs = append(proposal.Obligation.Scope.EdgeIDs, "invented_edge")
		}},
		{"node outside journey", func(proposal *RegressionProposal) {
			proposal.Obligation.Scope.NodeIDs = append(proposal.Obligation.Scope.NodeIDs, "phonefleet")
		}},
		{"edge outside journey", func(proposal *RegressionProposal) {
			proposal.Obligation.Scope.EdgeIDs = append(proposal.Obligation.Scope.EdgeIDs, "e_outreach_provider")
		}},
		{"missing journey", func(proposal *RegressionProposal) { proposal.Obligation.Scope.JourneyIDs = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegressionRegistry()
			proposal, evidence, approval := regressionSubmission(t, "bad-graph-"+test.name, RegressionAdd, 0, "behavior", now)
			test.mutate(&proposal)
			evidence.Scope = proposal.Obligation.Scope
			hash, err := regressionHash(normalizeRegressionProposal(proposal))
			if err != nil {
				t.Fatal(err)
			}
			approval.ProposalHash = hash
			if _, err := registry.Apply(proposal, evidence, approval, now); !errors.Is(err, ErrRegressionInvalid) {
				t.Fatalf("invalid graph scope was accepted: %v", err)
			}
			if len(registry.History()) != 0 {
				t.Fatal("invalid graph scope entered immutable history")
			}
		})
	}
}

func TestRegressionRegistryRejectsOutOfDomainAccountableAuthor(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	proposal, evidence, approval := regressionSubmission(t, "wrong-domain", RegressionAdd, 0, "behavior", now)
	proposal.AuthorRoleID = "linkhub_domains_outcome"
	proposal.Obligation.AccountableDomainRoleID = proposal.AuthorRoleID
	evidence.AccountableRoleID = proposal.AuthorRoleID
	approval.IndependentFromRole = proposal.AuthorRoleID
	hash, err := regressionHash(normalizeRegressionProposal(proposal))
	if err != nil {
		t.Fatal(err)
	}
	approval.ProposalHash = hash
	if _, err := registry.Apply(proposal, evidence, approval, now); !errors.Is(err, ErrRegressionUnauthorized) {
		t.Fatalf("out-of-domain author was accepted: %v", err)
	}
	if len(registry.History()) != 0 {
		t.Fatal("out-of-domain proposal entered immutable history")
	}
}

func TestRegressionRegistryExposesStaleAndConflictingChanges(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	proposal, evidence, approval := regressionSubmission(t, "stale", RegressionAdd, 0, "stale behavior", now)
	evidence.ReviewAfter = now
	if _, err := registry.Apply(proposal, evidence, approval, now); !errors.Is(err, ErrRegressionStale) {
		t.Fatalf("stale evidence was not denied: %v", err)
	}
	proposal, evidence, approval = regressionSubmission(t, "valid", RegressionAdd, 0, "valid behavior", now)
	applyRegression(t, registry, proposal, evidence, approval, now)
	conflict, conflictEvidence, conflictApproval := regressionSubmission(t, "conflict", RegressionUpdate, 0, "conflicting behavior", now)
	if _, err := registry.Apply(conflict, conflictEvidence, conflictApproval, now); !errors.Is(err, ErrRegressionConflict) {
		t.Fatalf("stale version was not denied: %v", err)
	}
	findings := registry.Findings()
	if len(findings) != 2 || findings[0].Kind != "stale" || findings[1].Kind != "conflict" {
		t.Fatalf("stale/conflicting changes are not visible: %+v", findings)
	}
	selection, err := registry.ActiveFor(regressionImpact(t, "impact-findings", "j_identity_access"), RegressionConsumerQualityChecker, now.Add(25*time.Hour))
	if err != nil || selection.Admission != "blocked_stale_obligation" || len(selection.StaleObligationIDs) != 1 || len(selection.Findings) != 2 {
		t.Fatalf("selection did not expose stale/conflict state: %+v err=%v", selection, err)
	}
}

func TestRegressionRegistryReplayIdempotencyAndTamperDetection(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	proposal, evidence, approval := regressionSubmission(t, "proposal-add", RegressionAdd, 0, "behavior", now)
	first := applyRegression(t, registry, proposal, evidence, approval, now)
	second := applyRegression(t, registry, proposal, evidence, approval, now)
	if first.RecordHash != second.RecordHash || len(registry.History()) != 1 {
		t.Fatalf("idempotent replay appended history: first=%+v second=%+v", first, second)
	}

	replayed, err := ReplayRegressionRegistry(registry.History())
	if err != nil || len(replayed.History()) != 1 || replayed.History()[0].RecordHash != first.RecordHash {
		t.Fatalf("history replay failed: history=%+v err=%v", replayed, err)
	}

	tampered := registry.History()
	tampered[0].Proposal.Obligation.Requirement = "tampered"
	if _, err := ReplayRegressionRegistry(tampered); !errors.Is(err, ErrRegressionHistoryCorrupt) {
		t.Fatalf("tampered history was accepted: %v", err)
	}

	changed := proposal
	changed.Obligation.Requirement = "same id, changed bytes"
	changedHash, _ := regressionHash(normalizeRegressionProposal(changed))
	approval.ProposalHash = changedHash
	if _, err := registry.Apply(changed, evidence, approval, now); !errors.Is(err, ErrRegressionConflict) {
		t.Fatalf("proposal id reuse was accepted: %v", err)
	}
}

func TestRegressionRegistryRejectedFindingsSurviveRestartAndRemainIdempotent(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	registry := NewRegressionRegistry()
	staleProposal, staleEvidence, staleApproval := regressionSubmission(t, "stale", RegressionAdd, 0, "stale behavior", now)
	staleEvidence.ReviewAfter = now
	if _, err := registry.Apply(staleProposal, staleEvidence, staleApproval, now); !errors.Is(err, ErrRegressionStale) {
		t.Fatalf("stale evidence was not denied: %v", err)
	}
	validProposal, validEvidence, validApproval := regressionSubmission(t, "valid", RegressionAdd, 0, "valid behavior", now)
	applyRegression(t, registry, validProposal, validEvidence, validApproval, now)
	conflictProposal, conflictEvidence, conflictApproval := regressionSubmission(t, "conflict", RegressionUpdate, 0, "conflicting behavior", now)
	if _, err := registry.Apply(conflictProposal, conflictEvidence, conflictApproval, now); !errors.Is(err, ErrRegressionConflict) {
		t.Fatalf("conflicting evidence was not denied: %v", err)
	}

	audit := registry.RejectedAttempts()
	if len(audit) != 2 || audit[0].RegistryVersion != 0 || audit[1].RegistryVersion != 1 || audit[1].PreviousAttemptHash != audit[0].AttemptHash {
		t.Fatalf("unexpected rejected audit: %+v", audit)
	}
	if audit[1].Finding.PreviousHash != audit[0].Finding.FindingHash {
		t.Fatal("finding history is not hash chained")
	}
	replayed, err := ReplayRegressionRegistry(registry.History(), audit)
	if err != nil {
		t.Fatalf("restart replay failed: %v", err)
	}
	beforeAttempts, beforeFindings := len(replayed.RejectedAttempts()), len(replayed.Findings())
	if _, err := replayed.Apply(staleProposal, staleEvidence, staleApproval, now); !errors.Is(err, ErrRegressionStale) {
		t.Fatalf("exact rejected replay changed outcome: %v", err)
	}
	if len(replayed.RejectedAttempts()) != beforeAttempts || len(replayed.Findings()) != beforeFindings {
		t.Fatal("exact rejected replay duplicated immutable audit")
	}
	if replayed.RejectedAttempts()[1].AttemptHash != audit[1].AttemptHash || replayed.Findings()[1].FindingHash != audit[1].Finding.FindingHash {
		t.Fatal("restart did not preserve immutable rejected evidence")
	}

	tampered := registry.RejectedAttempts()
	tampered[0].ErrorMessage = "forged"
	if _, err := ReplayRegressionRegistry(registry.History(), tampered); !errors.Is(err, ErrRegressionHistoryCorrupt) {
		t.Fatalf("tampered rejected audit was accepted: %v", err)
	}
	tampered = registry.RejectedAttempts()
	tampered[0].Finding.Message = "forged"
	if _, err := ReplayRegressionRegistry(registry.History(), tampered); !errors.Is(err, ErrRegressionHistoryCorrupt) {
		t.Fatalf("tampered finding audit was accepted: %v", err)
	}
}
