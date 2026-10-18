package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRoleQualificationFreezeV1BindsEveryArtifactAndAcceptedBoundary(t *testing.T) {
	freeze, err := CurrentRoleQualificationFreezeV1()
	if err != nil {
		t.Fatal(err)
	}
	root := roleQualificationRepoRoot(t)
	for path, want := range freeze.Artifacts {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read frozen artifact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("frozen artifact %s: got %s want %s", path, got, want)
		}
	}
}

func TestRoleQualificationFreezeV1FailsClosedOnStatusCountAuthorityAndArtifactDrift(t *testing.T) {
	base, err := CurrentRoleQualificationFreezeV1()
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(change func(*RoleQualificationFreezeV1)) error {
		copy := base
		copy.Artifacts = map[string]string{}
		for k, v := range base.Artifacts {
			copy.Artifacts[k] = v
		}
		change(&copy)
		return ValidateRoleQualificationFreezeV1(copy)
	}
	for name, change := range map[string]func(*RoleQualificationFreezeV1){
		"status":       func(f *RoleQualificationFreezeV1) { f.Status = "draft" },
		"review":       func(f *RoleQualificationFreezeV1) { f.IndependentReviewCount = 2 },
		"calls":        func(f *RoleQualificationFreezeV1) { f.ProviderCallsAllowed = true },
		"runtime":      func(f *RoleQualificationFreezeV1) { f.RuntimeWiring = "provider" },
		"cases":        func(f *RoleQualificationFreezeV1) { f.CaseCount = 11 },
		"fixtures":     func(f *RoleQualificationFreezeV1) { f.CalibrationFixtureCount = 56 },
		"models":       func(f *RoleQualificationFreezeV1) { f.ConfigurationClassCount = 4 },
		"roles":        func(f *RoleQualificationFreezeV1) { f.CandidateRoleCount = 3 },
		"observations": func(f *RoleQualificationFreezeV1) { f.PlannedObservationCount = 35 },
		"selection":    func(f *RoleQualificationFreezeV1) { f.RoleModelSelectionAllowed = true },
		"artifact": func(f *RoleQualificationFreezeV1) {
			f.Artifacts["server/internal/agentqualification/role_qualification_run_matrix_v1.json"] = "00"
		},
	} {
		if err := mutate(change); err == nil {
			t.Errorf("%s drift was accepted", name)
		}
	}
}

func TestRoleQualificationFreezeV1PreservesExactMatrixAccountingAndStopSemantics(t *testing.T) {
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		t.Fatal(err)
	}
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRunMatrixV1(group, matrix); err != nil {
		t.Fatal(err)
	}
	if matrix.Accounting.PlannedObservations != 36 || len(matrix.ConfigurationClasses) != 3 || len(matrix.RoleBindings) != 4 || len(matrix.Assignments) != 12 {
		t.Fatalf("matrix drifted: %+v", matrix)
	}
	mutations := map[string]func(*roleQualificationRunMatrixFileV1){
		"model": func(m *roleQualificationRunMatrixFileV1) { m.ConfigurationClasses[0].Model = "other" },
		"role":  func(m *roleQualificationRunMatrixFileV1) { m.RoleBindings[0].CandidateClass = "other" },
		"stop": func(m *roleQualificationRunMatrixFileV1) {
			m.StopPolicy.GlobalHalt = m.StopPolicy.GlobalHalt[:len(m.StopPolicy.GlobalHalt)-1]
		},
		"accounting": func(m *roleQualificationRunMatrixFileV1) { m.Accounting.PlannedObservations = 35 },
	}
	for name, mutate := range mutations {
		var changed roleQualificationRunMatrixFileV1
		if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &changed); err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		if err := validateRunMatrixV1(group, changed); err == nil {
			t.Errorf("%s matrix drift was accepted", name)
		}
	}
}
