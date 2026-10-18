package workflowguard

import (
	"encoding/json"
	"testing"
)

func validLegacyAssignment() LegacyAssignmentV1 {
	return LegacyAssignmentV1{
		ContractVersion: 1, TaskKind: "review", IssueID: "issue", IssueTitle: "title",
		AcceptanceCriteria: json.RawMessage(`[]`), SourceTaskID: "source", WorkflowEventID: "event",
		WorkflowEventType: "implementation_ready", WorkflowEventReason: "ready",
		WorkflowEventPayload: json.RawMessage(`{}`), TargetAgentID: "agent", SquadID: "squad",
		RuntimeID: "runtime", RuntimeProvider: "opencode", Model: "fixture/model", ReasoningEffort: "medium",
		ExecutionPolicy: ExecutionPolicy{EnvironmentPolicy: LegacyEnvironmentPolicy, ToolPolicy: LegacyToolPolicy, FreshTaskHome: true},
		RequiredSkills:  []RequiredSkill{{ID: "builtin:multica-platform", Source: "builtin", SHA256: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SizeBytes: 1}},
	}
}

func TestLegacyAssignmentMatchesEveryAuthoritativePromptField(t *testing.T) {
	want := validLegacyAssignment()
	if !MatchesAuthoritativeSnapshot(want, want) {
		t.Fatal("exact authoritative snapshot did not match")
	}
	mutations := []func(*LegacyAssignmentV1){
		func(value *LegacyAssignmentV1) { value.TaskKind = "correction" },
		func(value *LegacyAssignmentV1) { value.IssueDescription = "altered" },
		func(value *LegacyAssignmentV1) { value.AcceptanceCriteria = json.RawMessage(`["altered"]`) },
		func(value *LegacyAssignmentV1) { value.WorkflowEventReason = "altered" },
		func(value *LegacyAssignmentV1) { value.WorkflowEventPayload = json.RawMessage(`{"altered":true}`) },
		func(value *LegacyAssignmentV1) { value.RuntimeID = "altered" },
		func(value *LegacyAssignmentV1) { value.RuntimeProvider = "altered" },
		func(value *LegacyAssignmentV1) { value.Model = "altered" },
		func(value *LegacyAssignmentV1) { value.ReasoningEffort = "altered" },
	}
	for _, mutate := range mutations {
		got := want
		mutate(&got)
		if MatchesAuthoritativeSnapshot(got, want) {
			t.Fatalf("altered authoritative assignment matched: %#v", got)
		}
	}
}

func TestLegacyAssignmentRoundTripAndTamperFailure(t *testing.T) {
	raw, err := WrapLegacyAssignment(validLegacyAssignment())
	if err != nil {
		t.Fatal(err)
	}
	packet, got, present, err := ExtractLegacyAssignment(raw)
	if err != nil || !present || got.TaskKind != "review" || len(packet) == 0 {
		t.Fatalf("round trip = present %v assignment %#v packet %s err %v", present, got, packet, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope[LegacyAssignmentKey].(map[string]any)["context_policy"] = map[string]any{"load_issue": true}
	tampered, _ := json.Marshal(envelope)
	if _, _, present, err := ExtractLegacyAssignment(tampered); !present || err == nil {
		t.Fatalf("tampered assignment present/error = %v/%v, want true/non-nil", present, err)
	}
}

func TestMatchRequiredSkillsRequiresExactSnapshot(t *testing.T) {
	want := validLegacyAssignment().RequiredSkills
	if !MatchRequiredSkills(want, append([]RequiredSkill(nil), want...)) {
		t.Fatal("exact skill snapshot did not match")
	}
	altered := append([]RequiredSkill(nil), want...)
	altered[0].SHA256 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if MatchRequiredSkills(want, altered) || MatchRequiredSkills(want, append(altered, want[0])) {
		t.Fatal("altered or expanded skill snapshot matched")
	}
}

func TestLegacyExecutionPolicyForAcceptanceNeverGrantsBrokerAuthority(t *testing.T) {
	hash := "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	policy := LegacyExecutionPolicyForAcceptance(json.RawMessage(`["ordinary criterion","bounded-workflow-broker-v1 ` + hash + `"]`))
	if policy.ToolPolicy != LegacyToolPolicy || policy.BrokerExecutableSHA256 != "" || !policy.FreshTaskHome {
		t.Fatalf("editable acceptance data granted broker policy = %#v", policy)
	}
	for _, raw := range []string{
		`["bounded-workflow-broker-v1 sha256:not-a-hash"]`,
		`["prefix bounded-workflow-broker-v1 ` + hash + `"]`,
		`["bounded-workflow-broker-v1 ` + hash + `","bounded-workflow-broker-v1 ` + hash + `"]`,
		`{"criterion":"bounded-workflow-broker-v1 ` + hash + `"}`,
	} {
		got := LegacyExecutionPolicyForAcceptance(json.RawMessage(raw))
		if got.ToolPolicy != LegacyToolPolicy || got.BrokerExecutableSHA256 != "" {
			t.Fatalf("invalid authority %s granted broker policy: %#v", raw, got)
		}
	}

	assignment := validLegacyAssignment()
	assignment.RuntimeProvider = "claude"
	assignment.ExecutionPolicy = ExecutionPolicy{EnvironmentPolicy: LegacyEnvironmentPolicy, ToolPolicy: LegacyBrokerToolPolicy, FreshTaskHome: true, BrokerExecutableSHA256: hash}
	if err := assignment.Validate(); err == nil {
		t.Fatal("retired legacy broker assignment was accepted")
	}
}
