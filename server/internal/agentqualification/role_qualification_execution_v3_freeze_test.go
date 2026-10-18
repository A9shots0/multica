package agentqualification

import "testing"

func TestRoleQualificationExecutionFreezeV3ExactBytesAndAuthority(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV3()
	if err != nil {
		t.Fatal(err)
	}
	if f.ProviderCallsAllowed || f.RuntimeWiring != "none" || f.Cells != 36 || len(f.Artifacts) != 10 || len(f.TransportAntecedents) != 4 {
		t.Fatalf("freeze=%+v", f)
	}
}
func TestRoleQualificationExecutionFreezeV3RejectsDrift(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV3()
	if err != nil {
		t.Fatal(err)
	}
	f.Artifacts["server/cmd/role-qualification-run-v3/main.go"] = "0000000000000000000000000000000000000000000000000000000000000000"
	if err := ValidateRoleQualificationExecutionFreezeV3(f); err == nil {
		t.Fatal("command byte drift accepted")
	}
}
