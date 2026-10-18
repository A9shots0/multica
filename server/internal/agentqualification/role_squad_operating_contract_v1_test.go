package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestRoleSquadOperatingContractV1CoversTheWholeOrganizationWithoutSelection(t *testing.T) {
	result, err := historicalContracts(t).CurrentRoleSquadOperatingValidationV1()
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != RoleSquadOperatingValidationV1Schema ||
		result.ExecutionMode != "deterministic_provider_free" ||
		result.ReviewOutcome != "self_adversarial_revise_p1_no_p0" || result.ConsolidatedCorrections != 1 ||
		!result.Valid || result.RolesCovered != 21 || result.OutcomeIdentitiesCovered != 10 ||
		result.SquadsCovered != 3 || result.SystemJourneysCovered != 13 ||
		result.LockedCasesCovered != 19 || result.UIRoutingRulesCovered != 3 ||
		result.AdversarialDenialsCovered != 8 {
		t.Fatalf("incomplete role/squad validation: %+v", result)
	}
	if result.ProviderRequests != 0 || result.CredentialReads != 0 || result.ProductionActions != 0 ||
		result.PermanentSeatSelections != 0 || result.ModelSelections != 0 {
		t.Fatalf("provider-free role design expanded authority: %+v", result)
	}
}

func TestRoleSquadOperatingContractV1UIRoutesUseMinimumParticipants(t *testing.T) {
	contract, err := historicalContracts(t).CurrentRoleSquadOperatingContractV1()
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.UIRoutingRules[0].Participants) != 2 || len(contract.UIRoutingRules[1].Participants) != 3 ||
		len(contract.UIRoutingRules[2].Participants) != 1 {
		t.Fatalf("UI routes are not minimal: %+v", contract.UIRoutingRules)
	}
}

func TestRoleSquadOperatingContractV1RejectsAuthorityAndRoutingMutations(t *testing.T) {
	base, err := historicalContracts(t).CurrentRoleSquadOperatingContractV1()
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*RoleSquadOperatingContractV1){
		"provider":           func(v *RoleSquadOperatingContractV1) { v.ProviderCallsAllowed = true },
		"credential":         func(v *RoleSquadOperatingContractV1) { v.CredentialReadsAllowed = true },
		"production":         func(v *RoleSquadOperatingContractV1) { v.ProductionActionsAllowed = true },
		"seat selection":     func(v *RoleSquadOperatingContractV1) { v.PermanentSeatSelectionAllowed = true },
		"model selection":    func(v *RoleSquadOperatingContractV1) { v.ModelSelectionAllowed = true },
		"outcome executes":   func(v *RoleSquadOperatingContractV1) { v.CaseRoutes[12].Participants[0] = "crm_product_outcome" },
		"self review":        func(v *RoleSquadOperatingContractV1) { v.CaseRoutes[12].Participants[1] = "shared_windows_executor" },
		"windows Phone Farm": func(v *RoleSquadOperatingContractV1) { v.CaseRoutes[4].Host = "alienware_windows" },
		"extra UI role": func(v *RoleSquadOperatingContractV1) {
			v.UIRoutingRules[0].Participants = append(v.UIRoutingRules[0].Participants, "product_ui")
		},
		"second correction":  func(v *RoleSquadOperatingContractV1) { v.CollaborationRules.MaxAutomaticCorrections = 2 },
		"worker concurrency": func(v *RoleSquadOperatingContractV1) { v.CollaborationRules.MaxConcurrentWorkers = 3 },
		"mika relay":         func(v *RoleSquadOperatingContractV1) { v.CollaborationRules.MikaRelayForbidden = false },
		"unjustified specialist": func(v *RoleSquadOperatingContractV1) {
			v.CollaborationRules.SpecialistFanoutRequiresUniqueNamedEvidence = false
		},
		"unordered participants": func(v *RoleSquadOperatingContractV1) {
			v.CollaborationRules.ParticipantsAreOrderedStages = false
		},
		"historical label creates seat": func(v *RoleSquadOperatingContractV1) {
			v.CollaborationRules.HistoricalConfigurationLabelsDoNotCreateSeats = false
		},
		"shared skill claims independence": func(v *RoleSquadOperatingContractV1) {
			v.CollaborationRules.SharedSkillPacketIsNotIndependentReview = false
		},
		"shared skill without executor": func(v *RoleSquadOperatingContractV1) {
			v.CaseRoutes[7].SharedExecutorSkills = []string{"architecture_assurance"}
		},
		"squad drift": func(v *RoleSquadOperatingContractV1) { v.CaseRoutes[8].SquadID = "cross_boundary_change" },
		"source drift": func(v *RoleSquadOperatingContractV1) {
			v.SourceHashes.SystemGraph = "0" + v.SourceHashes.SystemGraph[1:]
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changed := cloneRoleSquadOperatingContractV1(t, base)
			mutate(&changed)
			if err := historicalContracts(t).ValidateRoleSquadOperatingContractV1(changed); err == nil {
				t.Fatal("mutated role/squad contract was accepted")
			}
		})
	}
}

func cloneRoleSquadOperatingContractV1(t *testing.T, value RoleSquadOperatingContractV1) RoleSquadOperatingContractV1 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone RoleSquadOperatingContractV1
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
