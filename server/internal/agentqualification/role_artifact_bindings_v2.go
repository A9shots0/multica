package agentqualification

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

const (
	RoleArtifactBindingsSchemaV2 = "silver-mediums.role-artifact-bindings.v2"
	roleArtifactBindingScopeV2   = "candidate_artifacts_only"
	roleArtifactVersionV2        = "v2"
)

type RoleArtifactBindingsV2 struct {
	SchemaVersion         string                  `json:"schema_version"`
	BindingScope          string                  `json:"binding_scope"`
	RoleArtifactSetSchema string                  `json:"role_artifact_set_schema"`
	RoleArtifactSetSHA256 string                  `json:"role_artifact_set_sha256"`
	Bindings              []RoleArtifactBindingV2 `json:"bindings"`
}

type RoleArtifactBindingV2 struct {
	BindingID   string                  `json:"binding_id"`
	RoleID      string                  `json:"role_id"`
	Instruction RoleArtifactReferenceV2 `json:"instruction"`
	Skill       RoleArtifactReferenceV2 `json:"skill"`
}

type RoleArtifactReferenceV2 struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

//go:embed role_artifact_bindings_v2.json
var roleArtifactBindingsV2FS embed.FS

func EmbeddedRoleArtifactBindingsV2() (RoleArtifactBindingsV2, error) {
	raw, err := roleArtifactBindingsV2FS.ReadFile("role_artifact_bindings_v2.json")
	if err != nil {
		return RoleArtifactBindingsV2{}, fmt.Errorf("read embedded role artifact bindings v2: %w", err)
	}
	return decodeRoleArtifactBindingsV2(raw)
}

func decodeRoleArtifactBindingsV2(raw []byte) (RoleArtifactBindingsV2, error) {
	var bindings RoleArtifactBindingsV2
	if err := strictJSON(raw, &bindings); err != nil {
		return bindings, fmt.Errorf("decode role artifact bindings v2: %w", err)
	}
	return bindings, nil
}

// ValidatedRoleArtifactBindingV2 resolves only immutable candidate artifacts.
// It deliberately does not resolve a model, provider, effort, runtime, or any
// execution authority. An unknown ID is denied rather than falling back to the
// historical configuration catalog.
func ValidatedRoleArtifactBindingV2(bindingID string) (RoleArtifactBindingV2, error) {
	bindings, err := EmbeddedRoleArtifactBindingsV2()
	if err != nil {
		return RoleArtifactBindingV2{}, err
	}
	if err := ValidateRoleArtifactBindingsV2(bindings); err != nil {
		return RoleArtifactBindingV2{}, err
	}
	for _, binding := range bindings.Bindings {
		if binding.BindingID == bindingID {
			return binding, nil
		}
	}
	return RoleArtifactBindingV2{}, fmt.Errorf("role artifact binding %s is not bound", bindingID)
}

func ValidateRoleArtifactBindingsV2(bindings RoleArtifactBindingsV2) error {
	if bindings.SchemaVersion != RoleArtifactBindingsSchemaV2 || bindings.BindingScope != roleArtifactBindingScopeV2 {
		return fmt.Errorf("role artifact bindings v2 identity or scope drifted")
	}
	if bindings.RoleArtifactSetSchema != RoleArtifactSetSchemaV2 || !validSHA256(bindings.RoleArtifactSetSHA256) {
		return fmt.Errorf("role artifact bindings v2 set identity is invalid")
	}
	setSum := sha256.Sum256(roleArtifactSetV2JSON)
	if hex.EncodeToString(setSum[:]) != bindings.RoleArtifactSetSHA256 {
		return fmt.Errorf("role artifact bindings v2 set SHA-256 mismatch")
	}
	set, err := CurrentRoleArtifactSetV2()
	if err != nil {
		return fmt.Errorf("validate role artifact set v2: %w", err)
	}
	roles, err := systemgraph.DefaultRoles()
	if err != nil {
		return fmt.Errorf("load role catalog: %w", err)
	}
	if len(bindings.Bindings) != len(set.CandidateRoles) {
		return fmt.Errorf("role artifact bindings v2 coverage is incomplete")
	}
	seenIDs := make(map[string]struct{}, len(bindings.Bindings))
	seenRoles := make(map[string]struct{}, len(bindings.Bindings))
	boundRoles := make([]string, 0, len(bindings.Bindings))
	for _, binding := range bindings.Bindings {
		wantBindingID := "silver-mediums.role-artifact.v2." + binding.RoleID
		if binding.BindingID != wantBindingID {
			return fmt.Errorf("role %s has non-canonical artifact binding ID", binding.RoleID)
		}
		if _, duplicate := seenIDs[binding.BindingID]; duplicate {
			return fmt.Errorf("duplicate role artifact binding ID %s", binding.BindingID)
		}
		seenIDs[binding.BindingID] = struct{}{}
		if _, duplicate := seenRoles[binding.RoleID]; duplicate {
			return fmt.Errorf("duplicate role artifact binding for %s", binding.RoleID)
		}
		seenRoles[binding.RoleID] = struct{}{}
		if _, ok := roles.Role(binding.RoleID); !ok {
			return fmt.Errorf("role artifact binding references unknown role %s", binding.RoleID)
		}
		if !containsRoleQualificationStringV1(set.CandidateRoles, binding.RoleID) {
			return fmt.Errorf("role artifact binding references unaccepted candidate role %s", binding.RoleID)
		}
		if err := validateRoleArtifactReferenceV2(set, binding.RoleID, "instructions.md", binding.Instruction); err != nil {
			return err
		}
		if err := validateRoleArtifactReferenceV2(set, binding.RoleID, "skill.md", binding.Skill); err != nil {
			return err
		}
		boundRoles = append(boundRoles, binding.RoleID)
	}
	if !equalRoleQualificationStringsV1(boundRoles, set.CandidateRoles) {
		return fmt.Errorf("role artifact bindings v2 role order or coverage drifted")
	}
	return nil
}

func validateRoleArtifactReferenceV2(set RoleArtifactSetV2, roleID, suffix string, ref RoleArtifactReferenceV2) error {
	wantPath := "role_artifacts/v2/" + roleID + "." + suffix
	wantHash, ok := set.Artifacts[wantPath]
	if ref.Version != roleArtifactVersionV2 || ref.Path != wantPath || !ok || ref.SHA256 != wantHash || !validSHA256(ref.SHA256) {
		return fmt.Errorf("role %s %s artifact binding drifted", roleID, suffix)
	}
	return nil
}

func containsRoleQualificationStringV1(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
