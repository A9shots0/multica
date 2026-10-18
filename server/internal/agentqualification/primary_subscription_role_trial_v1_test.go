package agentqualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrimarySubscriptionRoleTrialV1MatchesAuthorizedGate(t *testing.T) {
	definition, err := historicalContracts(t).CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	if definition.MaximumCLISessions != 12 || len(definition.Invocations) != 12 || definition.Invocations[11].Ordinal != 12 {
		t.Fatalf("unexpected frozen trial: %+v", definition)
	}
	if PrimarySubscriptionRoleTrialV1SHA256() == definition.GateSHA256 {
		t.Fatal("execution definition must be distinct from its authority gate")
	}
}

func TestPrimarySubscriptionRoleTrialRejectsMutation(t *testing.T) {
	definition, err := historicalContracts(t).CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	mutations := []func(*PrimarySubscriptionRoleTrialV1){
		func(v *PrimarySubscriptionRoleTrialV1) { v.MaximumCLISessions++ },
		func(v *PrimarySubscriptionRoleTrialV1) { v.Retries = 1 },
		func(v *PrimarySubscriptionRoleTrialV1) { v.ProviderNativeToolsAllowed = true },
		func(v *PrimarySubscriptionRoleTrialV1) { v.Invocations[0].Participant = "other" },
		func(v *PrimarySubscriptionRoleTrialV1) {
			v.Lanes[0].AllowedObservedModels = append(v.Lanes[0].AllowedObservedModels, "other")
		},
	}
	for index, mutate := range mutations {
		copy := definition
		copy.Invocations = append([]PrimarySubscriptionTrialInvocationV1(nil), definition.Invocations...)
		copy.Lanes = append([]PrimarySubscriptionTrialLaneV1(nil), definition.Lanes...)
		copy.Lanes[0].AllowedObservedModels = append([]string(nil), definition.Lanes[0].AllowedObservedModels...)
		mutate(&copy)
		if err := historicalContracts(t).ValidatePrimarySubscriptionRoleTrialV1(copy); err == nil {
			t.Fatalf("mutation %d passed", index)
		}
	}
}

func TestPrimarySubscriptionReliabilityTrialV2IsFreshAndExactlyCapped(t *testing.T) {
	definition, err := CurrentPrimarySubscriptionReliabilityTrialV2()
	if err != nil {
		t.Fatal(err)
	}
	if definition.MaximumCLISessions != 12 || definition.RepetitionsPerCell != 3 || len(definition.Invocations) != 12 || !definition.ProvisionalLocalSelectionAfterReview || definition.ProductionActivationAllowed {
		t.Fatalf("unexpected reliability v2 definition: %+v", definition)
	}
	if definition.PriorTrialResultSHA256 != PrimarySubscriptionRoleTrialResultV1SHA256() || PrimarySubscriptionReliabilityTrialV2SHA256() == PrimarySubscriptionRoleTrialV1SHA256() {
		t.Fatal("reliability v2 did not bind and separate the prior terminal result")
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, invocation := range definition.Invocations {
		key := invocation.Lane + ":" + invocation.CaseID
		counts[key]++
		if seen[invocation.ObservationID] || invocation.Participant != "shared_windows_executor" || invocation.ParticipantKind != "executor" || invocation.Arm != "shared_executor_with_exact_skill_packet" {
			t.Fatalf("reliability invocation drifted: %+v", invocation)
		}
		seen[invocation.ObservationID] = true
	}
	for _, key := range []string{"claude_code_subscription:R01", "claude_code_subscription:R02", "codex_subscription:R01", "codex_subscription:R02"} {
		if counts[key] != 3 {
			t.Fatalf("cell %s has %d repetitions", key, counts[key])
		}
	}
}

func TestPrimarySubscriptionReliabilityTrialV2RejectsMutations(t *testing.T) {
	definition, _ := CurrentPrimarySubscriptionReliabilityTrialV2()
	mutations := []func(*PrimarySubscriptionRoleTrialV1){
		func(v *PrimarySubscriptionRoleTrialV1) { v.MaximumCLISessions++ },
		func(v *PrimarySubscriptionRoleTrialV1) { v.RepetitionsPerCell-- },
		func(v *PrimarySubscriptionRoleTrialV1) {
			v.PriorTrialResultSHA256 = PrimarySubscriptionRoleTrialV1SHA256()
		},
		func(v *PrimarySubscriptionRoleTrialV1) { v.ProvisionalLocalSelectionAfterReview = false },
		func(v *PrimarySubscriptionRoleTrialV1) { v.ProductionActivationAllowed = true },
		func(v *PrimarySubscriptionRoleTrialV1) { v.Lanes[0].RequestedModel = "other" },
		func(v *PrimarySubscriptionRoleTrialV1) {
			v.Invocations[0].ObservationID = v.Invocations[1].ObservationID
		},
		func(v *PrimarySubscriptionRoleTrialV1) { v.AlwaysForbidden = v.AlwaysForbidden[1:] },
	}
	for index, mutate := range mutations {
		copy := definition
		copy.Invocations = append([]PrimarySubscriptionTrialInvocationV1(nil), definition.Invocations...)
		copy.Lanes = append([]PrimarySubscriptionTrialLaneV1(nil), definition.Lanes...)
		copy.Lanes[0].AllowedObservedModels = append([]string(nil), definition.Lanes[0].AllowedObservedModels...)
		copy.AlwaysForbidden = append([]string(nil), definition.AlwaysForbidden...)
		mutate(&copy)
		if err := ValidatePrimarySubscriptionReliabilityTrialV2(copy); err == nil {
			t.Fatalf("reliability v2 mutation %d passed", index)
		}
	}
}

func TestPrimarySubscriptionReliabilityTrialV2PromptIsSelfContained(t *testing.T) {
	definition, _ := CurrentPrimarySubscriptionReliabilityTrialV2()
	invocation := definition.Invocations[0]
	workspace, err := PrepareRealisticCaseWorkspaceForRun(t.TempDir(), invocation.CaseID, invocation.ObservationID)
	if err != nil {
		t.Fatal(err)
	}
	prompt, schema, err := BuildPrimarySubscriptionReliabilityTrialPromptV2(invocation, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, definition.TrialID) || strings.Contains(prompt, "item-14-primary-subscription-role-trial-20260914") || len(schema) == 0 {
		t.Fatal("reliability v2 prompt did not isolate its definition")
	}
}

func TestPrimarySubscriptionTrialActionPlanFailsClosed(t *testing.T) {
	parent := t.TempDir()
	workspace, err := PrepareRealisticCaseWorkspaceForRun(parent, "R01", "test-run")
	if err != nil {
		t.Fatal(err)
	}
	invocation, _ := trialInvocationForTest(t, "claude-r01-inline", "executor")
	valid := PrimarySubscriptionTrialParticipantOutputV1{Actions: []RealisticCandidateAction{
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "edit", Path: "consumer_faulty/normalize.py", Contents: "def normalize_role(role: str) -> str:\n    return role\n"},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "local-only pre/post contract proof; authority and Phone Farm fixture untouched\n"},
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "finish", Summary: "bounded local repair completed"},
	}}
	raw, _ := json.Marshal(valid)
	if _, err := historicalContracts(t).ParsePrimarySubscriptionTrialParticipantOutputV1(raw, invocation, workspace); err != nil {
		t.Fatal(err)
	}
	for _, altered := range [][]byte{
		[]byte(`{"actions":[{"kind":"edit","path":"../escape","contents":"x"},{"kind":"finish","summary":"x"}]}`),
		[]byte(`{"actions":[{"kind":"run_check","check":"powershell secret.ps1"},{"kind":"finish","summary":"x"}]}`),
		[]byte(`{"actions":[{"kind":"edit","path":"consumer_faulty/normalize.py","contents":"x"},{"kind":"run_check","check":"python -m unittest discover -s tests -v"},{"kind":"finish","summary":"x"}]}`),
	} {
		if _, err := historicalContracts(t).ParsePrimarySubscriptionTrialParticipantOutputV1(altered, invocation, workspace); err == nil {
			t.Fatalf("unsafe altered action plan passed: %s", altered)
		}
	}
}

func TestPrimarySubscriptionTrialFakeR01ExecutesAndGrades(t *testing.T) {
	workspace, err := PrepareRealisticCaseWorkspaceForRun(t.TempDir(), "R01", "fake-run")
	if err != nil {
		t.Fatal(err)
	}
	invocation, _ := trialInvocationForTest(t, "claude-r01-inline", "executor")
	out := PrimarySubscriptionTrialParticipantOutputV1{Actions: []RealisticCandidateAction{
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "edit", Path: "consumer_faulty/normalize.py", Contents: "def normalize_role(role: str) -> str:\n    return role\n"},
		{Kind: "edit", Path: "EVIDENCE.md", Contents: "Local-only proof: failing test before, passing test after. CRM authority, correct consumer, Phone Farm contract, repositories, production, and external systems were untouched.\n"},
		{Kind: "run_check", Check: "python -m unittest discover -s tests -v"},
		{Kind: "finish", Summary: "repaired only the synthetic faulty consumer"},
	}}
	raw, _ := json.Marshal(out)
	parsed, err := historicalContracts(t).ParsePrimarySubscriptionTrialParticipantOutputV1(raw, invocation, workspace)
	if err != nil {
		t.Fatal(err)
	}
	state, err := ExecutePrimarySubscriptionTrialActionsV1(workspace, "fake-run", parsed.Actions, PrimarySubscriptionTrialExecutionStateV1{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	grade, err := GradePrimarySubscriptionTrialWorkspaceV1(workspace, "fake", state.Controller)
	if err != nil || grade.Verdict != "PASS" {
		t.Fatalf("grade=%+v err=%v", grade, err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Root, "EVIDENCE.md")); err != nil {
		t.Fatal(err)
	}
}

func trialInvocationForTest(t *testing.T, observation, kind string) (PrimarySubscriptionTrialInvocationV1, bool) {
	definition, err := historicalContracts(t).CurrentPrimarySubscriptionRoleTrialV1()
	if err != nil {
		t.Fatal(err)
	}
	for _, invocation := range definition.Invocations {
		if invocation.ObservationID == observation && invocation.ParticipantKind == kind {
			return invocation, true
		}
	}
	return PrimarySubscriptionTrialInvocationV1{}, false
}
