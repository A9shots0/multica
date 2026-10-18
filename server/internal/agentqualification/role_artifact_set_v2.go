package agentqualification

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"
)

const RoleArtifactSetSchemaV2 = "silver-mediums.role-artifact-set.v2"

type RoleArtifactSetV2 struct {
	SchemaVersion            string            `json:"schema_version"`
	Status                   string            `json:"status"`
	IndependentReviewOutcome string            `json:"independent_review_outcome"`
	ConsolidatedCorrections  int               `json:"consolidated_corrections"`
	ConfigurationBinding     string            `json:"configuration_binding"`
	ProviderCallsAllowed     bool              `json:"provider_calls_allowed"`
	RuntimeWiring            string            `json:"runtime_wiring"`
	ModelSelectionAllowed    bool              `json:"model_selection_allowed"`
	CandidateRoles           []string          `json:"candidate_roles"`
	SkillOnly                []string          `json:"skill_only"`
	Artifacts                map[string]string `json:"artifacts"`
}

//go:embed role_artifact_set_v2.json
var roleArtifactSetV2JSON []byte

//go:embed role_artifacts/v2
var roleArtifactSetV2FS embed.FS

func CurrentRoleArtifactSetV2() (RoleArtifactSetV2, error) {
	var set RoleArtifactSetV2
	if err := strictJSON(roleArtifactSetV2JSON, &set); err != nil {
		return set, err
	}
	return set, ValidateRoleArtifactSetV2(set)
}

// LoadBoundRoleArtifactPairV2 returns the exact embedded bytes whose hashes
// are pinned by a previously validated binding. It never reads workspace or
// task-controlled paths.
func LoadBoundRoleArtifactPairV2(binding RoleArtifactBindingV2) ([]byte, []byte, error) {
	validated, err := ValidatedRoleArtifactBindingV2(binding.BindingID)
	if err != nil || validated != binding {
		return nil, nil, fmt.Errorf("role artifact binding is not current")
	}
	instructions, err := roleArtifactSetV2FS.ReadFile(validated.Instruction.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("read bound instructions: %w", err)
	}
	skill, err := roleArtifactSetV2FS.ReadFile(validated.Skill.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("read bound skill: %w", err)
	}
	return append([]byte(nil), instructions...), append([]byte(nil), skill...), nil
}

func ValidateRoleArtifactSetV2(set RoleArtifactSetV2) error {
	if set.SchemaVersion != RoleArtifactSetSchemaV2 || set.Status != "accepted_provider_free" {
		return fmt.Errorf("role artifact set identity or status drifted")
	}
	if set.IndependentReviewOutcome != "revise_p1_no_p0" || set.ConsolidatedCorrections != 1 || set.ConfigurationBinding != "deferred_fail_closed" {
		return fmt.Errorf("role artifact review or configuration-binding boundary drifted")
	}
	if set.ProviderCallsAllowed || set.RuntimeWiring != "none" || set.ModelSelectionAllowed {
		return fmt.Errorf("role artifact set opened execution or selection authority")
	}
	wantRoles := []string{"architecture_assurance", "company_auditor", "data_sync_integrity", "phone_fleet_reliability", "phonefarm_application_specialist", "release_runtime_assurance", "security_privacy_review"}
	if !equalRoleQualificationStringsV1(set.CandidateRoles, wantRoles) || !equalRoleQualificationStringsV1(set.SkillOnly, []string{"mac_alert_analysis"}) || len(set.Artifacts) != 16 {
		return fmt.Errorf("role artifact coverage drifted")
	}
	for path, want := range set.Artifacts {
		if !validSHA256(want) || !strings.HasPrefix(path, "role_artifacts/v2/") {
			return fmt.Errorf("invalid artifact receipt: %s", path)
		}
		raw, err := roleArtifactSetV2FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read role artifact %s: %w", path, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != want {
			return fmt.Errorf("role artifact drifted: %s", path)
		}
	}
	for _, role := range wantRoles {
		instructionPath := "role_artifacts/v2/" + role + ".instructions.md"
		skillPath := "role_artifacts/v2/" + role + ".skill.md"
		instructions, okInstructions := set.Artifacts[instructionPath]
		skill, okSkill := set.Artifacts["role_artifacts/v2/"+role+".skill.md"]
		if !okInstructions || !okSkill || instructions == skill {
			return fmt.Errorf("role pair missing or collapsed: %s", role)
		}
		instructionBytes, _ := roleArtifactSetV2FS.ReadFile(instructionPath)
		skillBytes, _ := roleArtifactSetV2FS.ReadFile(skillPath)
		instructionText := string(instructionBytes)
		for _, heading := range []string{"Status:", "## Purpose", "## Inputs", "## Allowed decisions", "## Forbidden actions", "## Evidence and response schema", "## Stop and escalation rules", "## Non-responsibilities"} {
			if !strings.Contains(instructionText, heading) {
				return fmt.Errorf("role %s instruction missing %s", role, heading)
			}
		}
		if !strings.Contains(strings.ToLower(instructionText), "no authority granted") || !strings.Contains(strings.ToLower(string(skillBytes)), "candidate") {
			return fmt.Errorf("role %s artifact implies authority or omits candidate boundary", role)
		}
	}
	if _, hasSeat := set.Artifacts["role_artifacts/v2/mac_alert_analysis.instructions.md"]; hasSeat {
		return fmt.Errorf("skill-only alert analysis gained a seat")
	}
	seenFiles := map[string]bool{}
	if err := fs.WalkDir(roleArtifactSetV2FS, "role_artifacts/v2", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			seenFiles[path] = true
		}
		return nil
	}); err != nil {
		return fmt.Errorf("enumerate role artifacts: %w", err)
	}
	if len(seenFiles) != len(set.Artifacts) {
		return fmt.Errorf("unmanifested or missing embedded role artifact")
	}
	for path := range seenFiles {
		if _, bound := set.Artifacts[path]; !bound {
			return fmt.Errorf("unmanifested embedded role artifact: %s", path)
		}
	}
	for _, path := range []string{
		"role_artifacts/v2/phonefarm_application_specialist.instructions.md",
		"role_artifacts/v2/phone_fleet_reliability.instructions.md",
		"role_artifacts/v2/release_runtime_assurance.instructions.md",
	} {
		raw, _ := roleArtifactSetV2FS.ReadFile(path)
		lower := strings.ToLower(string(raw))
		if !strings.Contains(lower, "phone farm") || !strings.Contains(lower, "windows") || !strings.Contains(lower, "mac") {
			return fmt.Errorf("artifact %s lost Phone Farm host boundary", path)
		}
	}
	return nil
}
