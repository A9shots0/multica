package agentqualification

import "testing"

func TestRoleSquadReadinessMatrixV2CoversAllRolesSquadsInstructionsAndSkills(t *testing.T) {
	matrix, err := historicalContracts(t).CurrentRoleSquadReadinessMatrixV2()
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix.RolePackets) != 11 || len(matrix.Squads) != 3 || matrix.StructuralInstructionSkillPairs != 11 || matrix.ProvisionalRuntimeRolePackets != 1 || matrix.ProvisionalInlineSkills != 1 || matrix.ActivatedSquads != 0 {
		t.Fatalf("unexpected role readiness matrix: %+v", matrix)
	}
}

func TestRoleSquadReadinessMatrixV2RejectsQualificationInflation(t *testing.T) {
	matrix, _ := historicalContracts(t).CurrentRoleSquadReadinessMatrixV2()
	mutations := []func(*RoleSquadReadinessMatrixV2){
		func(v *RoleSquadReadinessMatrixV2) { v.RolePackets[6].RuntimeStatus = "qualified" },
		func(v *RoleSquadReadinessMatrixV2) { v.Squads[0].Status = "active" },
		func(v *RoleSquadReadinessMatrixV2) { v.ActivatedSquads = 1 },
		func(v *RoleSquadReadinessMatrixV2) { v.ModelSelection = "claude-sonnet-4-6" },
		func(v *RoleSquadReadinessMatrixV2) { v.ProductionActivation = true },
	}
	for index, mutate := range mutations {
		copy := matrix
		copy.RolePackets = append([]RolePacketReadinessV2(nil), matrix.RolePackets...)
		copy.Squads = append([]SquadReadinessV2(nil), matrix.Squads...)
		mutate(&copy)
		if err := historicalContracts(t).ValidateRoleSquadReadinessMatrixV2(copy); err == nil {
			t.Fatalf("readiness inflation mutation %d passed", index)
		}
	}
}
