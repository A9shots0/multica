package agentqualification

import (
	"strings"
	"testing"
	"time"
)

func TestProvisionalRoleConfigurationV2ExactScopeAndRehearsal(t *testing.T) {
	configuration, err := CurrentProvisionalRoleConfigurationV2()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.TransportLane != "claude_code_subscription_composite" || configuration.ExactSingleModelClaimAllowed || configuration.ConfigurationAuthorizesProviderCall || configuration.AutomaticDispatchAllowed || configuration.PermanentRoleOrSeatSelectionAllowed {
		t.Fatalf("provisional operating default expanded: %+v", configuration)
	}
	result, receipts, err := RunProvisionalRoleConfigurationRehearsalV2()
	if err != nil {
		t.Fatal(err)
	}
	if result.AllowedPrepare != 2 || result.Denied != 48 || len(receipts) != 50 || !result.Valid {
		t.Fatalf("unexpected provisional v2 rehearsal: %+v", result)
	}
	if recorded, err := CurrentProvisionalRoleConfigurationRehearsalV2(); err != nil || recorded != result {
		t.Fatalf("recorded provisional v2 rehearsal mismatch: %+v %v", recorded, err)
	}
}

func TestProvisionalRoleConfigurationV2RejectsConfigurationMutations(t *testing.T) {
	configuration, err := CurrentProvisionalRoleConfigurationV2()
	if err != nil {
		t.Fatal(err)
	}
	mutations := []func(*ProvisionalRoleConfigurationV2){
		func(v *ProvisionalRoleConfigurationV2) {
			v.SourceHashes.IntegrationReplayResult = strings.Repeat("0", 64)
		},
		func(v *ProvisionalRoleConfigurationV2) { v.SupersedesConfigurationSHA256 = strings.Repeat("0", 64) },
		func(v *ProvisionalRoleConfigurationV2) { v.CaseFamily = "R02" },
		func(v *ProvisionalRoleConfigurationV2) { v.RouteStages[1] = "product_ui" },
		func(v *ProvisionalRoleConfigurationV2) { v.AllowedObservedModels = []string{"claude-sonnet-4-6"} },
		func(v *ProvisionalRoleConfigurationV2) { v.ExactSingleModelClaimAllowed = true },
		func(v *ProvisionalRoleConfigurationV2) { v.ConfigurationAuthorizesProviderCall = true },
		func(v *ProvisionalRoleConfigurationV2) { v.AutomaticDispatchAllowed = true },
		func(v *ProvisionalRoleConfigurationV2) { v.MaximumCorrections = 1 },
		func(v *ProvisionalRoleConfigurationV2) { v.PermanentRoleOrSeatSelectionAllowed = true },
		func(v *ProvisionalRoleConfigurationV2) { v.ValidUntil = "2026-10-15T13:15:00Z" },
	}
	for index, mutate := range mutations {
		copy := configuration
		copy.RouteStages = append([]string(nil), configuration.RouteStages...)
		copy.AllowedObservedModels = append([]string(nil), configuration.AllowedObservedModels...)
		mutate(&copy)
		if err := ValidateProvisionalRoleConfigurationV2(copy); err == nil {
			t.Fatalf("provisional v2 configuration mutation %d passed", index)
		}
	}
}

func TestProvisionalRoleConfigurationV2LifecycleFailsClosed(t *testing.T) {
	configuration, err := CurrentProvisionalRoleConfigurationV2()
	if err != nil {
		t.Fatal(err)
	}
	exact := exactProvisionalRoleConfigurationRequestV2(configuration)
	lifecycle, err := CurrentProvisionalRoleConfigurationLifecycleV2()
	if err != nil {
		t.Fatal(err)
	}
	validFrom, _ := time.Parse(time.RFC3339, configuration.ValidFrom)
	validUntil, _ := time.Parse(time.RFC3339, configuration.ValidUntil)
	if decision, reason := EvaluateProvisionalRoleConfigurationRequestV2At(exact, lifecycle, validFrom); decision != "ALLOW_PREPARE" || reason != "exact_reversible_local_r01_operating_default" {
		t.Fatalf("exact request denied: %s %s", decision, reason)
	}
	checks := []struct {
		name   string
		at     time.Time
		mutate func(*ProvisionalRoleConfigurationLifecycleV2, *ProvisionalRoleConfigurationRequestV2)
		reason string
	}{
		{name: "before-valid", at: validFrom.Add(-time.Second), reason: "configuration_not_yet_valid"},
		{name: "exact-expiry", at: validUntil, reason: "configuration_expired"},
		{name: "after-expiry", at: validUntil.Add(time.Second), reason: "configuration_expired"},
		{name: "revoked", at: validFrom, mutate: func(state *ProvisionalRoleConfigurationLifecycleV2, _ *ProvisionalRoleConfigurationRequestV2) {
			state.Revoked = []ProvisionalRoleConfigurationRevocationV2{{ConfigurationID: state.Active.ConfigurationID, ConfigurationSHA256: state.Active.ConfigurationSHA256, RevokedAt: validFrom.Format(time.RFC3339), Reason: "test"}}
		}, reason: "configuration_revoked"},
		{name: "retired", at: validFrom, mutate: func(state *ProvisionalRoleConfigurationLifecycleV2, _ *ProvisionalRoleConfigurationRequestV2) {
			state.Active.Status = "retired"
		}, reason: "configuration_retired"},
		{name: "provider-call", at: validFrom, mutate: func(_ *ProvisionalRoleConfigurationLifecycleV2, request *ProvisionalRoleConfigurationRequestV2) {
			request.ProviderCalls = 1
		}, reason: "configuration_does_not_authorize_provider_call"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			request := exact
			request.RouteStages = append([]string(nil), exact.RouteStages...)
			request.AllowedObservedModels = append([]string(nil), exact.AllowedObservedModels...)
			state := cloneProvisionalRoleConfigurationLifecycleV2(lifecycle)
			if check.mutate != nil {
				check.mutate(&state, &request)
			}
			decision, reason := EvaluateProvisionalRoleConfigurationRequestV2At(request, state, check.at)
			if decision != "DENY" || reason != check.reason {
				t.Fatalf("got %s %s", decision, reason)
			}
		})
	}
}

func TestProvisionalRoleConfigurationV1IsRetiredByControllerLifecycle(t *testing.T) {
	configuration, err := CurrentProvisionalRoleConfigurationV1()
	if err != nil {
		t.Fatal(err)
	}
	request := ProvisionalRoleConfigurationRequestV1{CaseID: "R01", Participant: configuration.Participant, ParticipantKind: configuration.ParticipantKind, InlineSkillPacket: configuration.InlineSkillPacket, ExecutionMode: configuration.ExecutionMode, ProviderLane: "unselected", Model: "unselected", Squad: "unselected"}
	if decision, reason := EvaluateProvisionalRoleConfigurationRequestV1(request); decision != "DENY" || reason != "superseded_configuration" {
		t.Fatalf("retired v1 was not denied: %s %s", decision, reason)
	}
	if _, _, err := RunProvisionalRoleConfigurationRehearsalV1(); err != nil {
		t.Fatalf("historical v1 rehearsal no longer verifies: %v", err)
	}
}

func TestProvisionalRoleConfigurationRehearsalV2RejectsReceiptReplay(t *testing.T) {
	result, receipts, err := RunProvisionalRoleConfigurationRehearsalV2()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*ProvisionalRoleConfigurationRehearsalResultV2, *[]ProvisionalRoleConfigurationRehearsalReceiptV2)
	}{
		{name: "renamed", mutate: func(_ *ProvisionalRoleConfigurationRehearsalResultV2, rows *[]ProvisionalRoleConfigurationRehearsalReceiptV2) {
			(*rows)[1].ScenarioID = "renamed-replay"
		}},
		{name: "altered-request", mutate: func(_ *ProvisionalRoleConfigurationRehearsalResultV2, rows *[]ProvisionalRoleConfigurationRehearsalReceiptV2) {
			(*rows)[1].RequestSHA256 = strings.Repeat("f", 64)
		}},
		{name: "missing", mutate: func(_ *ProvisionalRoleConfigurationRehearsalResultV2, rows *[]ProvisionalRoleConfigurationRehearsalReceiptV2) {
			*rows = (*rows)[:len(*rows)-1]
		}},
		{name: "reordered", mutate: func(_ *ProvisionalRoleConfigurationRehearsalResultV2, rows *[]ProvisionalRoleConfigurationRehearsalReceiptV2) {
			(*rows)[1], (*rows)[2] = (*rows)[2], (*rows)[1]
		}},
		{name: "aggregate", mutate: func(value *ProvisionalRoleConfigurationRehearsalResultV2, _ *[]ProvisionalRoleConfigurationRehearsalReceiptV2) {
			value.AllowedPrepare++
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copyResult := result
			copyReceipts := append([]ProvisionalRoleConfigurationRehearsalReceiptV2(nil), receipts...)
			test.mutate(&copyResult, &copyReceipts)
			if err := ValidateProvisionalRoleConfigurationRehearsalV2(copyResult, copyReceipts); err == nil {
				t.Fatal("altered or replayed rehearsal receipt was accepted")
			}
		})
	}
}

func TestProvisionalRoleConfigurationV2RejectsUnknownFields(t *testing.T) {
	injected := strings.Replace(string(provisionalRoleConfigurationV2JSON), `"status":`, `"unexpected":true,"status":`, 1)
	var configuration ProvisionalRoleConfigurationV2
	if err := strictJSON([]byte(injected), &configuration); err == nil {
		t.Fatal("unknown field was accepted")
	}
}
