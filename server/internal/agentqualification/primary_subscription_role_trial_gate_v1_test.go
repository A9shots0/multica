package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestPrimarySubscriptionRoleTrialGateV1RequiresOwnerWithoutSelecting(t *testing.T) {
	gate, err := historicalContracts(t).CurrentPrimarySubscriptionRoleTrialGateV1()
	if err != nil {
		t.Fatal(err)
	}
	if gate.ProviderCallsAllowed || gate.CredentialReadsAllowed || gate.ToolExecutionAllowed || gate.FixtureRepositoryAccessAllowed ||
		gate.ProductionActionsAllowed || gate.PermanentModelOrSeatSelectionAllowed {
		t.Fatalf("owner gate opened authority: %+v", gate)
	}
	if gate.ProposedTrial.MaximumCLISessions != 12 || len(gate.ProposedTrial.InvocationPlan) != 8 || len(gate.OwnerDecisionsRequired) != 2 {
		t.Fatalf("owner gate is incomplete: %+v", gate)
	}
}

func TestPrimarySubscriptionRoleTrialGateV1RejectsMutations(t *testing.T) {
	base, err := historicalContracts(t).CurrentPrimarySubscriptionRoleTrialGateV1()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*PrimarySubscriptionRoleTrialGateV1){
		"provider":      func(v *PrimarySubscriptionRoleTrialGateV1) { v.ProviderCallsAllowed = true },
		"credential":    func(v *PrimarySubscriptionRoleTrialGateV1) { v.CredentialReadsAllowed = true },
		"tools":         func(v *PrimarySubscriptionRoleTrialGateV1) { v.ToolExecutionAllowed = true },
		"fixture":       func(v *PrimarySubscriptionRoleTrialGateV1) { v.FixtureRepositoryAccessAllowed = true },
		"selection":     func(v *PrimarySubscriptionRoleTrialGateV1) { v.PermanentModelOrSeatSelectionAllowed = true },
		"extra session": func(v *PrimarySubscriptionRoleTrialGateV1) { v.ProposedTrial.MaximumCLISessions = 13 },
		"retry":         func(v *PrimarySubscriptionRoleTrialGateV1) { v.ProposedTrial.Retries = 1 },
		"native tools":  func(v *PrimarySubscriptionRoleTrialGateV1) { v.ProposedTrial.ProviderNativeToolsAllowed = true },
		"participant": func(v *PrimarySubscriptionRoleTrialGateV1) {
			v.ProposedTrial.InvocationPlan[0].Participants[0] = "product_ui"
		},
		"browser": func(v *PrimarySubscriptionRoleTrialGateV1) { v.ProposedTrial.R02BrowserBoundary = "any_browser" },
		"Claude subset": func(v *PrimarySubscriptionRoleTrialGateV1) {
			v.ProposedTrial.ClaudeExclusionRule = "skip nothing"
		},
		"real repo": func(v *PrimarySubscriptionRoleTrialGateV1) {
			v.ProposedTrial.AlwaysForbidden = v.ProposedTrial.AlwaysForbidden[1:]
		},
		"Claude bypass": func(v *PrimarySubscriptionRoleTrialGateV1) {
			v.KnownTransportConstraints = v.KnownTransportConstraints[1:]
		},
		"decision bypass": func(v *PrimarySubscriptionRoleTrialGateV1) { v.OwnerDecisionsRequired = v.OwnerDecisionsRequired[:1] },
		"source drift":    func(v *PrimarySubscriptionRoleTrialGateV1) { v.SourceHashes.RoleSquadRehearsalResult = "00" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := clonePrimarySubscriptionRoleTrialGateV1(t, base)
			mutate(&changed)
			if err := historicalContracts(t).ValidatePrimarySubscriptionRoleTrialGateV1(changed); err == nil {
				t.Fatal("mutated primary subscription owner gate was accepted")
			}
		})
	}
}

func clonePrimarySubscriptionRoleTrialGateV1(t *testing.T, value PrimarySubscriptionRoleTrialGateV1) PrimarySubscriptionRoleTrialGateV1 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone PrimarySubscriptionRoleTrialGateV1
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
