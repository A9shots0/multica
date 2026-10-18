package agentqualification

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

func TestCasesCoverAllNineteenArchitectureScenarios(t *testing.T) {
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 19 {
		t.Fatalf("cases = %d, want 19", len(cases))
	}
}

func TestAutomaticDisqualifierCannotBeOutrankedByCheapTokens(t *testing.T) {
	c := Case{ID: "T", RequiredEvidence: []string{"proof"}, AutomaticDisqualifiers: []string{"unsafe"}, Mandatory: true}
	observation := validObservation(t)
	observation.CaseID = c.ID
	observation.RunReceipts[0].EvaluationBinding = observation.ObservationID + ":" + c.ID
	trial := Trial{ConfigurationID: observation.Configurations[0].ID, CaseID: c.ID, Attempt: 1, OutcomePass: true, SafetyPass: true, EvidencePass: true, IndependencePass: true, ObservedEvidence: []string{"proof"}, Violations: []string{"unsafe"}, Observation: observation}
	verdict := EvaluateTrial(c, trial)
	if verdict.Qualified {
		t.Fatalf("unsafe cheap trial qualified: %+v", verdict)
	}
}

func TestTrialCannotQualifyWithoutValidatedObservation(t *testing.T) {
	c := Case{ID: "T", RequiredEvidence: []string{"proof"}, AutomaticDisqualifiers: []string{"unsafe"}, Mandatory: true}
	trial := Trial{ConfigurationID: "candidate", CaseID: c.ID, Attempt: 1, OutcomePass: true, SafetyPass: true, EvidencePass: true, IndependencePass: true, ObservedEvidence: []string{"proof"}}
	verdict := EvaluateTrial(c, trial)
	if verdict.Qualified || !hasReasonPrefix(verdict.Reasons, "invalid_observation:") {
		t.Fatalf("unmeasured trial qualified: %+v", verdict)
	}
}

func TestBlindGradeFailureCannotBeOverriddenByManualPassBooleans(t *testing.T) {
	c := Case{ID: "T", RequiredEvidence: []string{"proof"}, AutomaticDisqualifiers: []string{"unsafe"}, Mandatory: true}
	observation := validObservation(t)
	observation.CaseID = c.ID
	observation.TrialStatus = "failed"
	observation.FailureClass = "model"
	observation.Severity = "p2"
	observation.BlindGradeFailedChecks = []string{"root_cause"}
	observation.RunReceipts[0].EvaluationBinding = observation.ObservationID + ":" + c.ID
	sealObservation(t, &observation)
	trial := Trial{ConfigurationID: observation.Configurations[0].ID, CaseID: c.ID, Attempt: 1, OutcomePass: true, SafetyPass: true, EvidencePass: true, IndependencePass: true, ObservedEvidence: []string{"proof"}, Observation: observation}
	verdict := EvaluateTrial(c, trial)
	if verdict.Qualified || !hasReasonPrefix(verdict.Reasons, "blind_grade_failed") || !hasReasonPrefix(verdict.Reasons, "blind_grade_failed_check:") {
		t.Fatalf("manual pass booleans overrode blind failure: %+v", verdict)
	}
}

func TestObservationDisqualifierIsAlwaysAQualificationHardGate(t *testing.T) {
	c := Case{ID: "T", RequiredEvidence: []string{"proof"}, AutomaticDisqualifiers: []string{"legacy_name"}, Mandatory: true}
	observation := validObservation(t)
	observation.CaseID = c.ID
	observation.Disqualifiers = []string{"new_blind_grader_name"}
	observation.RunReceipts[0].EvaluationBinding = observation.ObservationID + ":" + c.ID
	sealObservation(t, &observation)
	trial := Trial{ConfigurationID: observation.Configurations[0].ID, CaseID: c.ID, Attempt: 1, OutcomePass: true, SafetyPass: true, EvidencePass: true, IndependencePass: true, ObservedEvidence: []string{"proof"}, Observation: observation}
	verdict := EvaluateTrial(c, trial)
	if verdict.Qualified || !hasReasonPrefix(verdict.Reasons, "automatic_disqualifier:new_blind_grader_name") {
		t.Fatalf("blind disqualifier vocabulary drift bypassed hard gate: %+v", verdict)
	}
}

func TestConfigurationSnapshotDriftCannotQualify(t *testing.T) {
	record := validObservation(t)
	configured := record.Configurations[0]
	drifted := configured
	drifted.ReasoningEffort = "high"
	trial := Trial{ConfigurationID: configured.ID, CaseID: record.CaseID, Attempt: 1, OutcomePass: true, SafetyPass: true, EvidencePass: true, IndependencePass: true, Observation: record}
	result := EvaluateConfiguration(drifted, []Trial{trial}, 1)
	if result.Qualified || !hasReasonPrefix(result.Reasons, record.CaseID+":configuration_snapshot_mismatch") {
		t.Fatalf("drifted configuration qualified: %+v", result)
	}
}

func TestDuplicateObservationCannotCountAsARepeat(t *testing.T) {
	record := validObservation(t)
	configuration := record.Configurations[0]
	trial := Trial{ConfigurationID: configuration.ID, CaseID: record.CaseID, Attempt: 1, Observation: record}
	result := EvaluateConfiguration(configuration, []Trial{trial, trial}, 2)
	if !hasReasonPrefix(result.Reasons, record.CaseID+":duplicate_observation:") {
		t.Fatalf("duplicate observation was accepted as a repeat: %+v", result)
	}
	if result.Runs != 1 || result.TotalTokens != 130 {
		t.Fatalf("duplicate observation was charged twice: %+v", result)
	}
}

func TestDuplicateAttemptCannotCountAsARepeat(t *testing.T) {
	first := validObservation(t)
	second := validObservation(t)
	second.ObservationID = "obs-002"
	second.RunReceipts[0].RunID = "run-2"
	second.RunReceipts[0].FamilyRootID = second.ObservationID
	second.RunReceipts[0].EvaluationBinding = second.ObservationID + ":" + second.CaseID
	sealObservation(t, &second)
	configuration := first.Configurations[0]
	trials := []Trial{
		{ConfigurationID: configuration.ID, CaseID: first.CaseID, Attempt: 1, Observation: first},
		{ConfigurationID: configuration.ID, CaseID: second.CaseID, Attempt: 1, Observation: second},
	}
	result := EvaluateConfiguration(configuration, trials, 2)
	if !hasReasonPrefix(result.Reasons, first.CaseID+":duplicate_attempt:") {
		t.Fatalf("duplicate attempt was accepted as a repeat: %+v", result)
	}
	if result.Runs != 2 || result.TotalTokens != 260 {
		t.Fatalf("paid duplicate attempt was omitted from cost: %+v", result)
	}
}

func hasReasonPrefix(reasons []string, prefix string) bool {
	for _, reason := range reasons {
		if len(reason) >= len(prefix) && reason[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func TestConfigurationRequiresCompleteVersionedAgentNotJustModel(t *testing.T) {
	if err := ValidateConfiguration(Configuration{ID: "luna", RoleID: "mika", Model: "gpt-5.6-luna"}); err == nil {
		t.Fatal("model-only candidate should be invalid")
	}
}

func TestRankingOnlyConsidersQualifiedConfigurations(t *testing.T) {
	results := []CandidateResult{
		{Configuration: Configuration{ID: "cheap-bad"}, Qualified: false, TotalTokens: 1},
		{Configuration: Configuration{ID: "strong"}, Qualified: true, TotalTokens: 100, OwnerInterruptions: 0},
		{Configuration: Configuration{ID: "efficient"}, Qualified: true, TotalTokens: 50, OwnerInterruptions: 0},
	}
	ranked := RankQualified(results)
	if len(ranked) != 2 || ranked[0].Configuration.ID != "efficient" {
		t.Fatalf("ranking = %+v", ranked)
	}
}

func TestConfigurationCannotChooseAnEasyApplicableSubset(t *testing.T) {
	configuration := Configuration{ID: "builder", RoleID: "shared_windows_executor", Model: "gpt-test", ReasoningEffort: "high", InstructionVersion: "v1", SkillVersions: map[string]string{"builder": "v1"}, PermissionVersion: "v1", RuntimeProvider: "test"}
	result := EvaluateConfiguration(configuration, nil, 1)
	if result.CasesRequired < 10 || result.Qualified {
		t.Fatalf("locked applicability was bypassed: %+v", result)
	}
}

func TestDomainOutcomeOwnerQualificationUsesLockedCapabilityCases(t *testing.T) {
	configuration := Configuration{ID: "media-owner", RoleID: "media_pipeline_outcome", Model: "gpt-test", ReasoningEffort: "high", InstructionVersion: "v1", SkillVersions: map[string]string{"media": "v1"}, PermissionVersion: "v1", RuntimeProvider: "test"}
	result := EvaluateConfiguration(configuration, nil, 1)
	if result.CasesRequired == 0 || result.Qualified {
		t.Fatalf("domain owner escaped locked capability cases: %+v", result)
	}
}

func TestAcceptedConfigurationCatalogIsStrictAndCoversEveryAccountableBinding(t *testing.T) {
	decisions, err := AcceptedConfigurations()
	if err != nil {
		t.Fatal(err)
	}
	roles, err := systemgraph.DefaultRoles()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range roles.AccountableActorBindings {
		decision, ok := decisions.Decision(binding.ConfigurationID)
		if !ok {
			t.Errorf("binding %s references configuration without a decision: %s", binding.Capability, binding.ConfigurationID)
			continue
		}
		if decision.Configuration.RoleID != binding.RoleID {
			t.Errorf("binding %s role %s does not match configuration role %s", binding.Capability, binding.RoleID, decision.Configuration.RoleID)
		}
	}
}

func TestAcceptedConfigurationsFailClosedForProductionAndExternalPhoneFarm(t *testing.T) {
	decisions, err := AcceptedConfigurations()
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range decisions.Decisions {
		if decision.AllowsExecution(true) {
			t.Errorf("Item 14 configuration %s unexpectedly allows production", decision.Configuration.ID)
		}
		if decision.Configuration.RoleID == "phonefarm_application_specialist" && decision.AllowsExecution(false) {
			t.Errorf("Windows catalog unexpectedly qualified Phone Farm execution: %s", decision.Configuration.ID)
		}
	}
}

func TestExecutionPlanNamesEveryConfigurationAndDeniesUnqualifiedAssignments(t *testing.T) {
	decisions, _ := AcceptedConfigurations()
	roles, _ := systemgraph.DefaultRoles()
	graph, _ := systemgraph.Default()
	route, err := graph.RouteIssue(systemgraph.RouteRequest{ObservedNodeIDs: []string{"phonefarm_app", "phonefleet"}, RiskFlags: []string{"production"}})
	if err != nil {
		t.Fatal(err)
	}
	plan := decisions.BuildExecutionPlan(roles, route)
	if plan.Admission != "denied_until_all_configurations_qualified" || len(plan.Unfilled) == 0 {
		t.Fatalf("unqualified plan admitted: %+v", plan)
	}
	for _, assignment := range plan.Assignments {
		if assignment.ConfigurationID == "" || assignment.QualificationStatus == "" {
			t.Errorf("assignment lacks exact configuration decision: %+v", assignment)
		}
	}
}
