package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

const RoleQualificationFreezeSchemaV1 = "silver-mediums.role-qualification-freeze.v1"

type RoleQualificationFreezeV1 struct {
	SchemaVersion              string            `json:"schema_version"`
	FreezeID                   string            `json:"freeze_id"`
	Status                     string            `json:"status"`
	IndependentReviewCount     int               `json:"independent_review_count"`
	IndependentReviewOutcome   string            `json:"independent_review_outcome"`
	ConsolidatedCorrections    int               `json:"consolidated_corrections"`
	MatrixFlawCorrectionScope  string            `json:"matrix_flaw_correction_scope"`
	SecondReviewCycle          bool              `json:"second_review_cycle"`
	LocalProofs                []string          `json:"local_proofs"`
	ProviderCallsAllowed       bool              `json:"provider_calls_allowed"`
	RuntimeWiring              string            `json:"runtime_wiring"`
	CaseCount                  int               `json:"case_count"`
	CalibrationFixtureCount    int               `json:"calibration_fixture_count"`
	ConfigurationClassCount    int               `json:"configuration_class_count"`
	CandidateRoleCount         int               `json:"candidate_role_count"`
	PlannedObservationCount    int64             `json:"planned_observation_count"`
	RoleModelSelectionAllowed  bool              `json:"role_model_selection_allowed"`
	ProductionSelectionAllowed bool              `json:"production_selection_allowed"`
	Artifacts                  map[string]string `json:"artifacts"`
}

//go:embed role_qualification_freeze_v1.json
var roleQualificationFreezeV1JSON []byte

//go:embed role_qualification_case_group_v1.json
var roleQualificationFreezeCaseGroupV1 []byte

//go:embed role_qualification_candidate_packets_v1.json
var roleQualificationFreezeCandidatePacketsV1 []byte

//go:embed role_qualification_hidden_graders_v1.json
var roleQualificationFreezeHiddenGradersV1 []byte

//go:embed role_qualification_calibration_fixtures_v1.json
var roleQualificationFreezeCalibrationFixturesV1 []byte

//go:embed role_qualification_run_matrix_v1.json
var roleQualificationFreezeRunMatrixV1 []byte

//go:embed role_qualification_artifact_manifest_v1.json
var roleQualificationFreezeArtifactManifestV1 []byte

//go:embed role_qualification_calibration_v1.go
var roleQualificationFreezeCalibrationSourceV1 []byte

//go:embed role_qualification_calibration_v1_test.go
var roleQualificationFreezeCalibrationTestsV1 []byte

//go:embed role_qualification_case_group_v1.go
var roleQualificationFreezeCaseLoaderV1 []byte

//go:embed role_qualification_case_group_v1_test.go
var roleQualificationFreezeCaseTestsV1 []byte

//go:embed role_artifacts/v1/mika_owner_interface.instructions.md
var roleQualificationFreezeMikaInstructionsV1 []byte

//go:embed role_artifacts/v1/mika_owner_interface.skill.md
var roleQualificationFreezeMikaSkillV1 []byte

//go:embed role_artifacts/v1/quality_checker.instructions.md
var roleQualificationFreezeQualityInstructionsV1 []byte

//go:embed role_artifacts/v1/quality_checker.skill.md
var roleQualificationFreezeQualitySkillV1 []byte

//go:embed role_artifacts/v1/product_ui.instructions.md
var roleQualificationFreezeProductInstructionsV1 []byte

//go:embed role_artifacts/v1/product_ui.skill.md
var roleQualificationFreezeProductSkillV1 []byte

//go:embed role_artifacts/v1/shared_windows_executor.instructions.md
var roleQualificationFreezeWindowsInstructionsV1 []byte

//go:embed role_artifacts/v1/shared_windows_executor.skill.md
var roleQualificationFreezeWindowsSkillV1 []byte

//go:embed role_qualification_prevalidation_record_v1.md
var roleQualificationFreezePrevalidationRecordV1 []byte

//go:embed role_qualification_freeze_v1.go
var roleQualificationFreezeSourceV1 []byte

//go:embed role_qualification_freeze_v1_test.go
var roleQualificationFreezeTestsV1 []byte

func CurrentRoleQualificationFreezeV1() (RoleQualificationFreezeV1, error) {
	var freeze RoleQualificationFreezeV1
	if err := strictJSON(roleQualificationFreezeV1JSON, &freeze); err != nil {
		return freeze, fmt.Errorf("decode role qualification freeze v1: %w", err)
	}
	if err := ValidateRoleQualificationFreezeV1(freeze); err != nil {
		return RoleQualificationFreezeV1{}, err
	}
	return freeze, nil
}

func ValidateRoleQualificationFreezeV1(freeze RoleQualificationFreezeV1) error {
	if freeze.SchemaVersion != RoleQualificationFreezeSchemaV1 || freeze.FreezeID != "role-qualification-core-freeze-v1" || freeze.Status != "accepted_frozen_provider_locked" {
		return fmt.Errorf("role qualification freeze identity or status drifted")
	}
	if freeze.IndependentReviewCount != 1 || freeze.IndependentReviewOutcome != "not_ready" || freeze.ConsolidatedCorrections != 1 || freeze.MatrixFlawCorrectionScope != "corrected_inside_same_open_consolidated_pass" || freeze.SecondReviewCycle {
		return fmt.Errorf("role qualification freeze review history drifted")
	}
	if !equalRoleQualificationStringsV1(freeze.LocalProofs, []string{"focused_role_qualification_tests_green", "full_agentqualification_package_green", "go_vet_agentqualification_green", "scoped_diff_check_green"}) {
		return fmt.Errorf("role qualification freeze local proof record drifted")
	}
	if freeze.ProviderCallsAllowed || freeze.RuntimeWiring != "none" || freeze.RoleModelSelectionAllowed || freeze.ProductionSelectionAllowed {
		return fmt.Errorf("role qualification freeze opened execution or selection authority")
	}
	if freeze.CaseCount != 12 || freeze.CalibrationFixtureCount != 57 || freeze.ConfigurationClassCount != 3 || freeze.CandidateRoleCount != 4 || freeze.PlannedObservationCount != 36 {
		return fmt.Errorf("role qualification freeze counts drifted")
	}
	if err := ValidateRoleQualificationPrevalidationV1(); err != nil {
		return fmt.Errorf("role qualification frozen semantics invalid: %w", err)
	}
	embedded := roleQualificationFreezeEmbeddedArtifactsV1()
	if len(freeze.Artifacts) != 28 || len(embedded) != 21 {
		return fmt.Errorf("role qualification freeze artifact coverage drifted")
	}
	for path, raw := range embedded {
		sum := sha256.Sum256(raw)
		if freeze.Artifacts[path] != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("role qualification frozen artifact drifted: %s", path)
		}
	}
	for _, path := range roleQualificationFreezeDiskDocumentsV1() {
		if !validSHA256(freeze.Artifacts[path]) {
			return fmt.Errorf("role qualification frozen document hash missing: %s", path)
		}
	}
	return nil
}

func roleQualificationFreezeEmbeddedArtifactsV1() map[string][]byte {
	return map[string][]byte{
		"server/internal/agentqualification/role_qualification_case_group_v1.json":                     roleQualificationFreezeCaseGroupV1,
		"server/internal/agentqualification/role_qualification_candidate_packets_v1.json":              roleQualificationFreezeCandidatePacketsV1,
		"server/internal/agentqualification/role_qualification_hidden_graders_v1.json":                 roleQualificationFreezeHiddenGradersV1,
		"server/internal/agentqualification/role_qualification_calibration_fixtures_v1.json":           roleQualificationFreezeCalibrationFixturesV1,
		"server/internal/agentqualification/role_qualification_run_matrix_v1.json":                     roleQualificationFreezeRunMatrixV1,
		"server/internal/agentqualification/role_qualification_artifact_manifest_v1.json":              roleQualificationFreezeArtifactManifestV1,
		"server/internal/agentqualification/role_qualification_calibration_v1.go":                      roleQualificationFreezeCalibrationSourceV1,
		"server/internal/agentqualification/role_qualification_calibration_v1_test.go":                 roleQualificationFreezeCalibrationTestsV1,
		"server/internal/agentqualification/role_qualification_case_group_v1.go":                       roleQualificationFreezeCaseLoaderV1,
		"server/internal/agentqualification/role_qualification_case_group_v1_test.go":                  roleQualificationFreezeCaseTestsV1,
		"server/internal/agentqualification/role_artifacts/v1/mika_owner_interface.instructions.md":    roleQualificationFreezeMikaInstructionsV1,
		"server/internal/agentqualification/role_artifacts/v1/mika_owner_interface.skill.md":           roleQualificationFreezeMikaSkillV1,
		"server/internal/agentqualification/role_artifacts/v1/quality_checker.instructions.md":         roleQualificationFreezeQualityInstructionsV1,
		"server/internal/agentqualification/role_artifacts/v1/quality_checker.skill.md":                roleQualificationFreezeQualitySkillV1,
		"server/internal/agentqualification/role_artifacts/v1/product_ui.instructions.md":              roleQualificationFreezeProductInstructionsV1,
		"server/internal/agentqualification/role_artifacts/v1/product_ui.skill.md":                     roleQualificationFreezeProductSkillV1,
		"server/internal/agentqualification/role_artifacts/v1/shared_windows_executor.instructions.md": roleQualificationFreezeWindowsInstructionsV1,
		"server/internal/agentqualification/role_artifacts/v1/shared_windows_executor.skill.md":        roleQualificationFreezeWindowsSkillV1,
		"server/internal/agentqualification/role_qualification_prevalidation_record_v1.md":             roleQualificationFreezePrevalidationRecordV1,
		"server/internal/agentqualification/role_qualification_freeze_v1.go":                           roleQualificationFreezeSourceV1,
		"server/internal/agentqualification/role_qualification_freeze_v1_test.go":                      roleQualificationFreezeTestsV1,
	}
}

func roleQualificationFreezeDiskDocumentsV1() []string {
	return []string{
		"docs/silver-mediums/HOSTED-WORKFLOW-INTERACTION-AUDIT-20260910.md",
		"docs/silver-mediums/ACCEPTANCE.md",
		"docs/silver-mediums/ITEM-10-INBOX-CHAT-ACCEPTANCE.md",
		"docs/silver-mediums/ITEM-11-MOBILE-PARITY.md",
		"docs/silver-mediums/ITEM-12-SMS-NOTIFICATIONS.md",
		"docs/silver-mediums/ITEM-14-BENCHMARK-CLOSURE.md",
		"docs/silver-mediums/ITEM-14-ROLE-DESIGN-DECISION-REGISTER.md",
	}
}

func RoleQualificationFreezeV1SHA256() string {
	sum := sha256.Sum256(roleQualificationFreezeV1JSON)
	return hex.EncodeToString(sum[:])
}
