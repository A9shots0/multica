package agentqualification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRehearsalArtifactResolutionIsBoundedAndReceiptDriftStillFails(t *testing.T) {
	calls := map[string]int{}
	underlying := func(roleID string) (ShadowRoleArtifactRefV1, error) {
		calls[roleID]++
		return roleSquadArtifactRefV1(roleID)
	}
	resolve := memoizedRoleSquadArtifactsV1(underlying)
	result, receipts, err := historicalContracts(t).runRoleSquadRehearsalV1(resolve)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRoleSquadRehearsalWithResolverV1(result, receipts, resolve); err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 69 || len(calls) != 11 {
		t.Fatalf("coverage changed: %d receipts, %d roles", len(receipts), len(calls))
	}
	for role, count := range calls {
		if count != 1 {
			t.Fatalf("role %s validated %d times in one call", role, count)
		}
	}

	// A valid recomputed trace cannot hide a changed artifact from the cached
	// independently validated references used by the receipt checker.
	changed := append([]RoleSquadRehearsalReceiptV1(nil), receipts...)
	changed[0].InstructionSHA256 = strings.Repeat("0", 64)
	raw, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedResult := result
	changedResult.TraceSHA256 = localShadowSHA256V1(raw)
	if err := validateRoleSquadRehearsalWithResolverV1(changedResult, changed, resolve); err == nil || !strings.Contains(err.Error(), "artifact or contribution drifted") {
		t.Fatalf("artifact mutation not rejected by identity check: %v", err)
	}

	fresh := memoizedRoleSquadArtifactsV1(underlying)
	if err := validateRoleSquadRehearsalWithResolverV1(result, receipts, fresh); err != nil {
		t.Fatal(err)
	}
	for role, count := range calls {
		if count != 2 {
			t.Fatalf("role %s did not validate again in a fresh call: %d", role, count)
		}
	}
}

func TestRoleSquadRehearsalV1ResolvesEveryExecutableArtifactAndRoute(t *testing.T) {
	result, receipts, err := historicalContracts(t).RunRoleSquadRehearsalV1()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRoleSquadRehearsalV1(result, receipts); err != nil {
		t.Fatal(err)
	}
	recorded, err := historicalContracts(t).CurrentRoleSquadRehearsalResultV1()
	if err != nil {
		t.Fatal(err)
	}
	if recorded != result {
		t.Fatalf("recorded rehearsal result differs: got %+v want %+v", recorded, result)
	}
}

func TestRoleSquadRehearsalV1RejectsTraceMutations(t *testing.T) {
	result, receipts, err := historicalContracts(t).RunRoleSquadRehearsalV1()
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*RoleSquadRehearsalResultV1, []RoleSquadRehearsalReceiptV1){
		"trace hash": func(r *RoleSquadRehearsalResultV1, _ []RoleSquadRehearsalReceiptV1) { r.TraceSHA256 = "00" },
		"provider":   func(r *RoleSquadRehearsalResultV1, _ []RoleSquadRehearsalReceiptV1) { r.ProviderRequests = 1 },
		"concurrency": func(r *RoleSquadRehearsalResultV1, _ []RoleSquadRehearsalReceiptV1) {
			r.MaxObservedConcurrentWorkers = 3
		},
		"unordered": func(_ *RoleSquadRehearsalResultV1, v []RoleSquadRehearsalReceiptV1) { v[3].StageSequence = 1 },
		"self review": func(_ *RoleSquadRehearsalResultV1, v []RoleSquadRehearsalReceiptV1) {
			v[4].RoleID = "shared_windows_executor"
		},
		"skill as worker": func(_ *RoleSquadRehearsalResultV1, v []RoleSquadRehearsalReceiptV1) { v[1].SeparateWorker = true },
		"duplicate evidence": func(_ *RoleSquadRehearsalResultV1, v []RoleSquadRehearsalReceiptV1) {
			v[1].Contribution = v[0].Contribution
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changedResult := result
			changedReceipts := append([]RoleSquadRehearsalReceiptV1(nil), receipts...)
			mutate(&changedResult, changedReceipts)
			if err := validateRoleSquadRehearsalV1(changedResult, changedReceipts); err == nil {
				t.Fatal("mutated role/squad rehearsal was accepted")
			}
		})
	}
}
