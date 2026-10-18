package externalreceipt

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

func receiptForProjection(state string) publicapiv1.OutcomeReceiptV02 {
	return publicapiv1.OutcomeReceiptV02{
		WorkID: "work-1", RunID: "run-1", ReceiptRevision: 1, Provider: "codex",
		Objective: "Restore the employee workflow.", Scope: "One issue", RequestedMode: "diagnose",
		Authority: publicapiv1.OutcomeReceiptAuthority{Reference: "observation-only"}, State: state,
		CurrentMilestone: "Confirm the employee workflow.", ChangeSinceLastReceipt: "The API recovered.",
		Evidence:     []publicapiv1.OutcomeReceiptEvidence{{Type: "runtime", Reference: "authenticated API probe"}},
		Dependencies: []string{"employee workflow verification remains"}, NextAction: "Run the employee workflow.",
		OwnerDecision: publicapiv1.OutcomeReceiptOwnerDecision{Options: []string{}, InvalidatedBy: []string{}}, Risk: "medium",
		ResourceUsage: publicapiv1.OutcomeReceiptResourceUsage{Source: "unavailable"},
		Effect:        publicapiv1.OutcomeReceiptEffect{UserOutcome: "temporarily_restored"},
		Freshness:     publicapiv1.OutcomeReceiptFreshness{ObservedAt: "2026-10-14T21:00:00Z"},
		Acceptance:    publicapiv1.OutcomeReceiptAcceptance{Stage: "local", Status: "partial", CriteriaRemaining: []string{"employee workflow"}},
	}
}

func TestExecutionPayloadsRetainTheCoreReconstructionPacket(t *testing.T) {
	receipt := receiptForProjection("blocked")
	evidenceID := pgtype.UUID{Bytes: uuid.MustParse("d147c431-7ff6-4c85-a975-3ed82fda55e0"), Valid: true}
	checkpointRaw, _, waitRaw := executionPayloads(receipt, evidenceID, "not_required_for_observation", "none")
	var checkpoint map[string]any
	if err := json.Unmarshal(checkpointRaw, &checkpoint); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"reported_state", "objective", "current_milestone", "change_since_last_receipt", "evidence", "dependencies", "next_action", "resource_usage", "effect_report", "acceptance_report"} {
		if _, ok := checkpoint[field]; !ok {
			t.Fatalf("checkpoint lost %q: %s", field, checkpointRaw)
		}
	}
	if checkpoint["reported_state"] != "blocked" {
		t.Fatalf("checkpoint reported_state = %#v", checkpoint["reported_state"])
	}
	var wait map[string]any
	if err := json.Unmarshal(waitRaw, &wait); err != nil {
		t.Fatal(err)
	}
	if wait["reported_state"] != "blocked" {
		t.Fatalf("wait reason did not preserve provider state: %s", waitRaw)
	}
}

func TestProviderCompletionClosesOnlyItsObservedExecution(t *testing.T) {
	receipt := receiptForProjection("complete")
	receipt.Dependencies = []string{}
	receipt.Acceptance.Status = "accepted"
	state, reason := executionState(receipt)
	if state != "completed" || reason != "" {
		t.Fatalf("execution state = %q/%q", state, reason)
	}
	// There is intentionally no outcome, release, health, or decision write in
	// this mapper. Those native ledgers have separate acceptance authority.
}

func TestAuthorityAndDecisionClaimsRemainFailClosed(t *testing.T) {
	receipt := receiptForProjection("waiting_owner")
	category, question, why := "business", "Ship now?", "The provider says it is ready."
	receipt.OwnerDecision = publicapiv1.OutcomeReceiptOwnerDecision{
		Required: true, Category: &category, Question: &question,
		Options: []string{"ship", "wait"}, WhyNow: &why, InvalidatedBy: []string{},
	}
	if got := decisionProposalState(receipt); got != "recorded_non_actionable" {
		t.Fatalf("decision proposal state = %q", got)
	}
	_, limitations, _ := evidenceMetadata(receipt, "unverified", "recorded_non_actionable")
	if string(limitations) == "[]" {
		t.Fatal("unverified authority and decision proposal lost their limitations")
	}
}

func TestActorCredentialPairingIsFailClosed(t *testing.T) {
	id := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if validActorCredential(Actor{Type: "member", ID: id, Credential: publicapiv1.CredentialPluginInstall}) {
		t.Fatal("member accepted a plugin credential")
	}
	if validActorCredential(Actor{Type: "plugin", ID: id, Credential: publicapiv1.CredentialUserOAuth}) {
		t.Fatal("plugin accepted a member credential")
	}
	if !validActorCredential(Actor{Type: "member", ID: id, Credential: publicapiv1.CredentialPersonalAccess}) {
		t.Fatal("member PAT credential was rejected")
	}
	if !validActorCredential(Actor{Type: "member", ID: id, Credential: publicapiv1.CredentialPluginInvocation}) {
		t.Fatal("member acting through a signed plugin invocation was rejected")
	}
}
