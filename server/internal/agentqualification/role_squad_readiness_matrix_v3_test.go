package agentqualification

import "testing"

func TestRoleSquadReadinessMatrixV3KeepsOneReversibleLocalDefault(t *testing.T) {
	matrix, err := historicalContracts(t).CurrentRoleSquadReadinessMatrixV3()
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix.RolePackets) != 11 || len(matrix.TaskScopedFunctions) != 2 || len(matrix.Squads) != 3 || matrix.ProvisionalLocalOperatingDefaults != 1 || matrix.ActivatedPermanentSquads != 0 || matrix.ExactSingleModelQualification || matrix.ConfigurationAuthorizesProviderCall {
		t.Fatalf("unexpected role readiness matrix v3: %+v", matrix)
	}
}

func TestRoleSquadReadinessMatrixV3RejectsQualificationInflation(t *testing.T) {
	matrix, err := historicalContracts(t).CurrentRoleSquadReadinessMatrixV3()
	if err != nil {
		t.Fatal(err)
	}
	mutations := []func(*RoleSquadReadinessMatrixV3){
		func(v *RoleSquadReadinessMatrixV3) { v.RolePackets[2].RuntimeStatus = "qualified_runtime_role" },
		func(v *RoleSquadReadinessMatrixV3) { v.RolePackets[6].RuntimeStatus = "qualified_product_ui" },
		func(v *RoleSquadReadinessMatrixV3) { v.TaskScopedFunctions[0].Binding = "permanent_seat" },
		func(v *RoleSquadReadinessMatrixV3) { v.Squads[0].Status = "active" },
		func(v *RoleSquadReadinessMatrixV3) { v.ActivatedPermanentSquads = 1 },
		func(v *RoleSquadReadinessMatrixV3) { v.ExactSingleModelQualification = true },
		func(v *RoleSquadReadinessMatrixV3) { v.ConfigurationAuthorizesProviderCall = true },
		func(v *RoleSquadReadinessMatrixV3) { v.PermanentRoleSelection = "shared_windows_executor" },
		func(v *RoleSquadReadinessMatrixV3) { v.ProductionActivation = true },
	}
	for index, mutate := range mutations {
		copy := matrix
		copy.RolePackets = append([]RolePacketReadinessV2(nil), matrix.RolePackets...)
		copy.TaskScopedFunctions = append([]TaskScopedFunctionReadinessV3(nil), matrix.TaskScopedFunctions...)
		copy.Squads = append([]SquadReadinessV2(nil), matrix.Squads...)
		mutate(&copy)
		if err := historicalContracts(t).ValidateRoleSquadReadinessMatrixV3(copy); err == nil {
			t.Fatalf("readiness v3 inflation mutation %d passed", index)
		}
	}
}
