package agentqualification

import "testing"

func TestRoleQualificationExecutionFreezeV4ExactAndZeroCall(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV4()
	if err != nil {
		t.Fatal(err)
	}
	if f.ProviderCallsMade != 0 || f.CellsPerBatch != 12 || f.ImplicitAllModels || len(f.Artifacts) != 10 {
		t.Fatalf("freeze=%+v", f)
	}
}
func TestRoleQualificationExecutionFreezeV4RejectsCommandDrift(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV4()
	if err != nil {
		t.Fatal(err)
	}
	f.Artifacts["server/cmd/role-qualification-run-v4/provider.go"] = "0000000000000000000000000000000000000000000000000000000000000000"
	if ValidateRoleQualificationExecutionFreezeV4(f) == nil {
		t.Fatal("provider source drift accepted")
	}
}
