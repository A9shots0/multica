package agentqualification

import (
	"github.com/multica-ai/multica/server/internal/testutil"
	"testing"
)

func TestIntegrationReplayResultV1VerifiesFrozenEvidence(t *testing.T) {
	if err := VerifyIntegrationReplayResultV1(testutil.QualificationArchiveFS(t, "integration-replay-v1")); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationReplayResultV1RejectsAuthorityExpansion(t *testing.T) {
	result, err := CurrentIntegrationReplayResultV1()
	if err != nil {
		t.Fatal(err)
	}
	mutations := []func(*IntegrationReplayResultV1){
		func(v *IntegrationReplayResultV1) { v.Transport.ExactSingleModelProven = true },
		func(v *IntegrationReplayResultV1) { v.Transport.AutomaticRetries = 1 },
		func(v *IntegrationReplayResultV1) { v.Transport.Tools = 1 },
		func(v *IntegrationReplayResultV1) { v.GuardRoleInterpretation = "permanent_role_qualification" },
		func(v *IntegrationReplayResultV1) { v.PermanentSelections = 1 },
		func(v *IntegrationReplayResultV1) { v.Cleanup.ActiveReceipts = 1 },
	}
	for index, mutate := range mutations {
		copy := result
		copy.Tasks = append([]IntegrationReplayTaskV1(nil), result.Tasks...)
		copy.WorkflowEvents = append([]string(nil), result.WorkflowEvents...)
		copy.Transport.ObservedModels = append([]string(nil), result.Transport.ObservedModels...)
		copy.Report.Artifacts = append([]IntegrationReplayArtifactV1(nil), result.Report.Artifacts...)
		mutate(&copy)
		if err := ValidateIntegrationReplayResultV1(copy); err == nil {
			t.Fatalf("integration replay mutation %d passed", index)
		}
	}
}
