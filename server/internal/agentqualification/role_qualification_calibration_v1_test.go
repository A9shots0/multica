package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRoleQualificationV1EveryCalibrationFixtureGetsExpectedSemanticGrade(t *testing.T) {
	report, err := CalibrateRoleQualificationV1()
	if err != nil {
		t.Fatal(err)
	}
	if report.Fixtures != 57 || report.Passed != 24 || report.Rejected != 33 {
		t.Fatalf("report=%+v", report)
	}
}

func TestRoleQualificationV1SafeParaphrasesPassAndUnsafeAnswersFail(t *testing.T) {
	var graders roleQualificationHiddenGradersFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &graders); err != nil {
		t.Fatal(err)
	}
	var fixtures roleQualificationFixturesFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_calibration_fixtures_v1.json", &fixtures); err != nil {
		t.Fatal(err)
	}
	byCase := map[string]RoleQualificationHiddenGraderV1{}
	for _, g := range graders.Graders {
		byCase[g.CaseID] = g
	}
	for _, f := range fixtures.Fixtures {
		grade := GradeRoleQualificationAnswerV1(f.Answer, byCase[f.CaseID])
		safe := f.Variant == "strong_pass" || f.Variant == "semantic_paraphrase_pass"
		if safe && !grade.Pass {
			t.Errorf("safe fixture %s failed: %+v", f.FixtureID, grade)
		}
		if !safe && grade.Pass {
			t.Errorf("unsafe fixture %s passed", f.FixtureID)
		}
	}
}

func TestRoleQualificationV1CandidatePacketsExcludeHiddenGraderAndCalibrationData(t *testing.T) {
	packets, err := CurrentRoleQualificationCandidatePacketsV1()
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 12 {
		t.Fatalf("packets=%d", len(packets))
	}
	data, err := json.Marshal(packets)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, hidden := range []string{"pass_requirements", "subtle_failures", "adversarial_failures", "coverage_tags", "required_concepts", "forbidden_concepts", "expected_pass", "calibration_variant", "strong_pass", "adversarial_fail"} {
		if strings.Contains(text, hidden) {
			t.Errorf("candidate packet leaks %q", hidden)
		}
	}
	if !strings.Contains(text, "response_schema") || !strings.Contains(text, "scenario") {
		t.Fatal("candidate packet omitted required visible content")
	}
}

func TestRoleQualificationV1DraftHasFixedMatrixExactAccountingTypedStopsAndNoProviderPath(t *testing.T) {
	if err := ValidateRoleQualificationPrevalidationV1(); err != nil {
		t.Fatal(err)
	}
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		t.Fatal(err)
	}
	if matrix.ProviderCallsAllowed || matrix.RuntimeWiring != "none" || matrix.Accounting.ProviderInvocations != 0 || matrix.Accounting.Retries != 0 {
		t.Fatalf("draft opens provider path: %+v", matrix)
	}
	if len(matrix.ConfigurationClasses) != 3 || len(matrix.RoleBindings) != 4 || len(matrix.Assignments) != 12 || matrix.Accounting.PlannedObservations != 36 {
		t.Fatalf("matrix=%+v", matrix)
	}
}

func TestRoleQualificationV1MatrixCrossesEveryCarryForwardClassWithEveryRoleAndCase(t *testing.T) {
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, assignment := range matrix.Assignments {
		if assignment.AttemptsPerCase != 1 || len(assignment.CaseIDs) != 3 {
			t.Fatalf("bad assignment: %+v", assignment)
		}
		seen[assignment.ConfigurationClassID+":"+assignment.CandidateClass] += len(assignment.CaseIDs)
	}
	for _, configuration := range matrix.ConfigurationClasses {
		for _, binding := range matrix.RoleBindings {
			key := configuration.ClassID + ":" + binding.CandidateClass
			if seen[key] != 3 {
				t.Errorf("%s observations=%d, want 3", key, seen[key])
			}
		}
	}
	for _, forbidden := range []string{"mika.owner_interface.luna.medium.v1", "quality_checker.terra.xhigh.v1", "product_ui.sol.high.v1"} {
		for _, assignment := range matrix.Assignments {
			if assignment.ConfigurationClassID == forbidden {
				t.Errorf("provisional role config used as model evidence: %s", forbidden)
			}
		}
	}
}

func TestRoleQualificationV1ManifestBindsEverySourceInstructionSkillAndExecutableArtifact(t *testing.T) {
	var manifest roleQualificationArtifactManifestFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_artifact_manifest_v1.json", &manifest); err != nil {
		t.Fatal(err)
	}
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactManifestV1(group, manifest); err != nil {
		t.Fatal(err)
	}
	root := roleQualificationRepoRoot(t)
	for _, artifact := range manifest.Artifacts {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.Path)))
		if err != nil {
			t.Fatalf("read %s: %v", artifact.Path, err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != artifact.SHA256 {
			t.Errorf("%s hash drift: got %x want %s", artifact.ID, sum, artifact.SHA256)
		}
	}
}

func TestRoleQualificationV1FailClosedOnMissingConceptForbiddenConceptAndAccountingDrift(t *testing.T) {
	g := RoleQualificationHiddenGraderV1{CaseID: "x", RequiredConcepts: []RoleQualificationConceptV1{{ID: "truth", Alternatives: [][]string{{"truth", "verified"}}}}, ForbiddenConcepts: []RoleQualificationConceptV1{{ID: "unsafe", Alternatives: [][]string{{"phone", "farm", "windows"}}}}}
	if GradeRoleQualificationAnswerV1("truth only", g).Pass {
		t.Fatal("missing required semantic concept passed")
	}
	if GradeRoleQualificationAnswerV1("truth verified; inspect Phone Farm from Windows", g).Pass {
		t.Fatal("forbidden semantic concept passed")
	}
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		t.Fatal(err)
	}
	matrix.Accounting.InputTokens = 1
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRunMatrixV1(group, matrix); err == nil {
		t.Fatal("nonzero draft accounting accepted")
	}
}

func roleQualificationRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
