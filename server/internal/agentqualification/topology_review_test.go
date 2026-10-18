package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func topologyReviewFixture(t *testing.T) TopologyReviewRequest {
	t.Helper()
	ids := []string{"regression_mechanism", "simulated_rollback_receipt", "baseline_restored", "requested_outcome_still_open_until_successor_verified", "release_contract_test_pass", "new_journey_pass", "historical_journey_pass", "current_truth_receipt"}
	items := make([]TopologyEvidenceItem, 0, len(ids))
	for _, id := range ids {
		content := id + " inspectable proof"
		items = append(items, TopologyEvidenceItem{EvidenceID: id, Kind: "harness_receipt", Content: content, SHA256: hashText(content)})
	}
	request, err := BuildTopologyReviewRequest("obs-1", "review-1", "R03", strings.Repeat("a", 64), "Local repair complete; requested outcome remains open.", []string{"EVIDENCE.md", "release/receipt.json"}, ids, items)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestTopologyReviewerAcceptsEvidenceBoundPassAndBoundedFailure(t *testing.T) {
	request := topologyReviewFixture(t)
	pass := TopologyReviewVerdict{Verdict: "PASS", Evidence: append([]string(nil), request.RequiredEvidence...)}
	if err := ValidateTopologyReviewVerdict(request, pass); err != nil {
		t.Fatalf("strong pass rejected: %v", err)
	}
	fail := TopologyReviewVerdict{Verdict: "FAIL", FailedCriteria: []string{"historical_journey_pass"}, Evidence: []string{"EVIDENCE.md"}, RequiredCorrection: "Add the missing historical journey result to EVIDENCE.md."}
	if err := ValidateTopologyReviewVerdict(request, fail); err != nil {
		t.Fatalf("bounded failure rejected: %v", err)
	}
	packet, err := BuildTopologyCorrectionPacket(request, fail)
	if err != nil {
		t.Fatal(err)
	}
	task, err := TopologyCorrectionTask(packet)
	if err != nil || strings.Contains(string(task), request.ExecutorSummary) || !strings.Contains(string(task), "historical_journey_pass") {
		t.Fatalf("correction packet leaked history or omitted the bounded failure: task=%s err=%v", task, err)
	}
}

func TestTopologyReviewerRejectsOpaqueOrIncompletePassEvidence(t *testing.T) {
	request := topologyReviewFixture(t)
	request.EvidenceItems = nil
	if _, err := BuildTopologyReviewRequest(request.ObservationID, request.ReviewRunID, request.CaseID, request.WorkspaceTreeHash, request.ExecutorSummary, request.ArtifactManifest, request.EvidenceIDs, nil); err == nil {
		t.Fatal("opaque evidence-only request was accepted")
	}
	request = topologyReviewFixture(t)
	if err := ValidateTopologyReviewVerdict(request, TopologyReviewVerdict{Verdict: "PASS", Evidence: []string{"baseline_restored"}}); err == nil {
		t.Fatal("pass without complete required evidence coverage was accepted")
	}
}

func hashText(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func TestTopologyReviewerRejectsFabricationScopeExpansionAndOwnerRelay(t *testing.T) {
	request := topologyReviewFixture(t)
	bad := []TopologyReviewVerdict{
		{Verdict: "PASS"},
		{Verdict: "FAIL", FailedCriteria: []string{"invented"}, Evidence: []string{"EVIDENCE.md"}, RequiredCorrection: "Fix locally."},
		{Verdict: "FAIL", FailedCriteria: []string{"historical_journey_pass"}, Evidence: []string{"made-up.json"}, RequiredCorrection: "Fix locally."},
		{Verdict: "FAIL", FailedCriteria: []string{"historical_journey_pass"}, Evidence: []string{"EVIDENCE.md"}, RequiredCorrection: "Deploy to production."},
		{Verdict: "FAIL", FailedCriteria: []string{"historical_journey_pass"}, Evidence: []string{"EVIDENCE.md"}, RequiredCorrection: "Ask the owner for proof."},
		{Verdict: "PASS", Evidence: []string{"baseline_restored"}, OwnerDecisionRequired: true},
	}
	for index, verdict := range bad {
		if ValidateTopologyReviewVerdict(request, verdict) == nil {
			t.Fatalf("adversarial reviewer output %d passed", index)
		}
	}
}

func TestTopologyReviewParserRejectsEssaysUnknownFieldsAndTrailingJSON(t *testing.T) {
	valid := []byte(`{"verdict":"PASS","failed_criteria":[],"evidence":["baseline_restored"],"required_correction":"","owner_decision_required":false}`)
	if _, err := ParseTopologyReviewVerdict(valid); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{
		[]byte("Looks good."),
		[]byte(`{"verdict":"PASS","failed_criteria":[],"evidence":["baseline_restored"],"required_correction":"","owner_decision_required":false,"confidence":"high"}`),
		[]byte(string(valid) + ` {"verdict":"PASS"}`),
	} {
		if _, err := ParseTopologyReviewVerdict(raw); err == nil {
			t.Fatalf("invalid reviewer response parsed: %s", raw)
		}
	}
}

func TestTopologyReviewPlanIsToolFreeAndProviderPinned(t *testing.T) {
	definition, _ := CurrentTopologyExperiment()
	plan, err := BuildOpenCodeTopologyReviewPlan(definition.Reviewer, topologyReviewFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Transport != OpenCodeTopologyReviewTransport || plan.ProviderFallbacks == nil || *plan.ProviderFallbacks || plan.ServingProvider != "anthropic" || !strings.Contains(string(plan.InlineConfig), `"tools":{"*":false}`) || !strings.Contains(string(plan.InlineConfig), `"permission":"deny"`) {
		t.Fatalf("review plan escaped frozen isolation: %+v config=%s", plan, plan.InlineConfig)
	}
	if !strings.Contains(plan.Prompt, "Do not edit, execute tools") || !strings.Contains(plan.Prompt, "Use only listed evidence identifiers") {
		t.Fatal("review prompt omitted independence or evidence constraints")
	}
}
