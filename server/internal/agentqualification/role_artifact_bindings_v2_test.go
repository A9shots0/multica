package agentqualification

import (
	"bytes"
	"strings"
	"testing"
)

func TestRoleArtifactBindingsV2ValidateExactModelNeutralCoverage(t *testing.T) {
	bindings, err := EmbeddedRoleArtifactBindingsV2()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRoleArtifactBindingsV2(bindings); err != nil {
		t.Fatal(err)
	}
	if len(bindings.Bindings) != 7 {
		t.Fatalf("bindings = %d, want 7", len(bindings.Bindings))
	}
	for _, binding := range bindings.Bindings {
		resolved, err := ValidatedRoleArtifactBindingV2(binding.BindingID)
		if err != nil {
			t.Fatalf("resolve %s: %v", binding.BindingID, err)
		}
		if resolved.RoleID != binding.RoleID {
			t.Fatalf("resolve %s role = %s, want %s", binding.BindingID, resolved.RoleID, binding.RoleID)
		}
	}
}

func TestRoleArtifactBindingsV2RejectsHistoricalOrUnknownConfigurationIDs(t *testing.T) {
	for _, id := range []string{
		"data_sync.sol.high.v1",
		"architecture_assurance.sol.high.v1",
		"security_privacy.sol.high.v1",
		"missing",
	} {
		if _, err := ValidatedRoleArtifactBindingV2(id); err == nil {
			t.Errorf("historical or unknown ID %q resolved", id)
		}
	}
}

func TestRoleArtifactBindingsV2SchemaRejectsSelectionAndRuntimeFields(t *testing.T) {
	raw, err := roleArtifactBindingsV2FS.ReadFile("role_artifact_bindings_v2.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"model", "provider", "effort", "runtime_wiring"} {
		injected := bytes.Replace(raw, []byte(`"binding_scope": "candidate_artifacts_only",`), []byte(`"binding_scope": "candidate_artifacts_only", "`+field+`": "forbidden",`), 1)
		if _, err := decodeRoleArtifactBindingsV2(injected); err == nil {
			t.Errorf("unknown selection/runtime field %q was accepted", field)
		}
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{`"model"`, `"provider"`, `"effort"`, `"runtime_wiring"`} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("embedded bindings contain forbidden field %s", forbidden)
		}
	}
}

func TestRoleArtifactBindingsV2FailsClosedOnDrift(t *testing.T) {
	base, err := EmbeddedRoleArtifactBindingsV2()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RoleArtifactBindingsV2){
		"schema": func(v *RoleArtifactBindingsV2) { v.SchemaVersion = "v3" },
		"scope":  func(v *RoleArtifactBindingsV2) { v.BindingScope = "execution" },
		"set":    func(v *RoleArtifactBindingsV2) { v.RoleArtifactSetSHA256 = strings.Repeat("0", 64) },
		"missing": func(v *RoleArtifactBindingsV2) {
			v.Bindings = v.Bindings[:len(v.Bindings)-1]
		},
		"id":   func(v *RoleArtifactBindingsV2) { v.Bindings[0].BindingID = "architecture_assurance.sol.high.v1" },
		"role": func(v *RoleArtifactBindingsV2) { v.Bindings[0].RoleID = "missing_role" },
		"instruction_path": func(v *RoleArtifactBindingsV2) {
			v.Bindings[0].Instruction.Path = "role_artifacts/v2/company_auditor.instructions.md"
		},
		"instruction_hash": func(v *RoleArtifactBindingsV2) { v.Bindings[0].Instruction.SHA256 = strings.Repeat("0", 64) },
		"skill_version":    func(v *RoleArtifactBindingsV2) { v.Bindings[0].Skill.Version = "latest" },
		"skill_hash":       func(v *RoleArtifactBindingsV2) { v.Bindings[0].Skill.SHA256 = strings.Repeat("0", 64) },
		"duplicate": func(v *RoleArtifactBindingsV2) {
			v.Bindings[1] = v.Bindings[0]
		},
	} {
		t.Run(name, func(t *testing.T) {
			copy := cloneRoleArtifactBindingsV2(base)
			mutate(&copy)
			if err := ValidateRoleArtifactBindingsV2(copy); err == nil {
				t.Fatal("drift was accepted")
			}
		})
	}
}

func cloneRoleArtifactBindingsV2(source RoleArtifactBindingsV2) RoleArtifactBindingsV2 {
	copy := source
	copy.Bindings = append([]RoleArtifactBindingV2(nil), source.Bindings...)
	return copy
}
