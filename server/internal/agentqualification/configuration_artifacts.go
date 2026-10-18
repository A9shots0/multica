package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

type ConfigurationArtifactManifest struct {
	Version            string                         `json:"version"`
	RoleCatalogVersion string                         `json:"role_catalog_version"`
	RoleCatalogSHA256  string                         `json:"role_catalog_sha256"`
	Bindings           []ConfigurationArtifactBinding `json:"bindings"`
}

type ConfigurationArtifactBinding struct {
	ConfigurationID    string                   `json:"configuration_id"`
	RoleID             string                   `json:"role_id"`
	InstructionVersion string                   `json:"instruction_version"`
	Instruction        ArtifactReference        `json:"instruction"`
	Skills             []SkillArtifactReference `json:"skills"`
	PermissionVersion  string                   `json:"permission_version"`
	Status             string                   `json:"status"`
}

type ArtifactReference struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type SkillArtifactReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

//go:embed configuration_artifacts.json role_artifacts/v1/*
var configurationArtifactFiles embed.FS

func EmbeddedConfigurationArtifactManifest() (ConfigurationArtifactManifest, error) {
	data, err := configurationArtifactFiles.ReadFile("configuration_artifacts.json")
	if err != nil {
		return ConfigurationArtifactManifest{}, fmt.Errorf("read embedded configuration artifact manifest: %w", err)
	}
	var manifest ConfigurationArtifactManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ConfigurationArtifactManifest{}, fmt.Errorf("decode configuration artifact manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ConfigurationArtifactManifest{}, fmt.Errorf("decode configuration artifact manifest: trailing JSON")
	}
	return manifest, nil
}

// ValidatedConfigurationArtifacts resolves one exact configuration. Unbound,
// mutable, missing, or hash-mismatched artifacts are denied rather than
// falling back to labels in accepted_configurations.json.
func (contracts QualificationContracts) ValidatedConfigurationArtifacts(configurationID string) (ConfigurationArtifactBinding, error) {
	manifest, err := EmbeddedConfigurationArtifactManifest()
	if err != nil {
		return ConfigurationArtifactBinding{}, err
	}
	decisions, err := AcceptedConfigurations()
	if err != nil {
		return ConfigurationArtifactBinding{}, err
	}
	roles, err := contracts.roles()
	if err != nil {
		return ConfigurationArtifactBinding{}, err
	}
	if err := manifest.Validate(decisions, roles, contracts.rolesSHA256(), configurationArtifactFiles); err != nil {
		return ConfigurationArtifactBinding{}, err
	}
	for _, binding := range manifest.Bindings {
		if binding.ConfigurationID == configurationID {
			return binding, nil
		}
	}
	return ConfigurationArtifactBinding{}, fmt.Errorf("configuration %s has no immutable artifact binding", configurationID)
}

func (m ConfigurationArtifactManifest) Validate(decisions DecisionCatalog, roles systemgraph.RoleCatalog, roleCatalogSHA256 string, artifactFS fs.FS) error {
	if m.Version == "" {
		return fmt.Errorf("configuration artifact manifest version is required")
	}
	if m.RoleCatalogVersion == "" || m.RoleCatalogVersion != roles.Version {
		return fmt.Errorf("configuration artifact manifest role catalog version mismatch")
	}
	if !artifactValidSHA256(m.RoleCatalogSHA256) || m.RoleCatalogSHA256 != roleCatalogSHA256 {
		return fmt.Errorf("configuration artifact manifest role catalog SHA-256 mismatch")
	}
	if len(m.Bindings) == 0 {
		return fmt.Errorf("configuration artifact manifest has no bindings")
	}
	seen := make(map[string]struct{}, len(m.Bindings))
	for _, binding := range m.Bindings {
		if binding.ConfigurationID == "" || binding.RoleID == "" || binding.InstructionVersion == "" || binding.PermissionVersion == "" || binding.Status == "" {
			return fmt.Errorf("configuration artifact binding is incomplete")
		}
		if _, duplicate := seen[binding.ConfigurationID]; duplicate {
			return fmt.Errorf("duplicate configuration artifact binding %s", binding.ConfigurationID)
		}
		seen[binding.ConfigurationID] = struct{}{}
		decision, ok := decisions.Decision(binding.ConfigurationID)
		if !ok {
			return fmt.Errorf("configuration artifact binding %s has no configuration decision", binding.ConfigurationID)
		}
		configuration := decision.Configuration
		if binding.RoleID != configuration.RoleID || binding.InstructionVersion != configuration.InstructionVersion || binding.PermissionVersion != configuration.PermissionVersion || binding.Status != decision.Status {
			return fmt.Errorf("configuration artifact binding %s does not match its decision", binding.ConfigurationID)
		}
		if _, ok := roles.Role(binding.RoleID); !ok {
			return fmt.Errorf("configuration artifact binding %s references missing role %s", binding.ConfigurationID, binding.RoleID)
		}
		if mutableArtifactVersion(binding.InstructionVersion) || mutableArtifactVersion(binding.PermissionVersion) {
			return fmt.Errorf("configuration artifact binding %s uses a mutable version", binding.ConfigurationID)
		}
		if err := validateArtifactReference(artifactFS, binding.Instruction); err != nil {
			return fmt.Errorf("configuration %s instruction: %w", binding.ConfigurationID, err)
		}
		if len(binding.Skills) == 0 || len(binding.Skills) != len(configuration.SkillVersions) {
			return fmt.Errorf("configuration %s skill artifact coverage is incomplete", binding.ConfigurationID)
		}
		seenSkills := make(map[string]struct{}, len(binding.Skills))
		for _, skill := range binding.Skills {
			if skill.Name == "" || skill.Version == "" {
				return fmt.Errorf("configuration %s has incomplete skill artifact", binding.ConfigurationID)
			}
			if _, duplicate := seenSkills[skill.Name]; duplicate {
				return fmt.Errorf("configuration %s has duplicate skill artifact %s", binding.ConfigurationID, skill.Name)
			}
			seenSkills[skill.Name] = struct{}{}
			decisionVersion, ok := configuration.SkillVersions[skill.Name]
			if !ok || decisionVersion != skill.Version {
				return fmt.Errorf("configuration %s skill %s does not match its decision", binding.ConfigurationID, skill.Name)
			}
			if mutableArtifactVersion(skill.Version) {
				return fmt.Errorf("configuration %s skill %s uses mutable version %s", binding.ConfigurationID, skill.Name, skill.Version)
			}
			if err := validateArtifactReference(artifactFS, ArtifactReference{Path: skill.Path, SHA256: skill.SHA256}); err != nil {
				return fmt.Errorf("configuration %s skill %s: %w", binding.ConfigurationID, skill.Name, err)
			}
		}
	}
	return nil
}

func validateArtifactReference(artifactFS fs.FS, reference ArtifactReference) error {
	cleaned := path.Clean(reference.Path)
	if reference.Path == "" || cleaned != reference.Path || cleaned == "." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return fmt.Errorf("artifact path is missing or unsafe")
	}
	expected, err := hex.DecodeString(reference.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("artifact SHA-256 is invalid")
	}
	data, err := fs.ReadFile(artifactFS, cleaned)
	if err != nil {
		return fmt.Errorf("read artifact %s: %w", cleaned, err)
	}
	actual := sha256.Sum256(data)
	if !bytes.Equal(actual[:], expected) {
		return fmt.Errorf("artifact %s SHA-256 mismatch", cleaned)
	}
	return nil
}

func artifactValidSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func mutableArtifactVersion(version string) bool {
	normalized := strings.ToLower(strings.TrimSpace(version))
	return normalized == "current" || normalized == "latest" || normalized == "head" || strings.HasSuffix(normalized, "-current")
}

func ValidatedConfigurationArtifacts(configurationID string) (ConfigurationArtifactBinding, error) {
	return (QualificationContracts{}).ValidatedConfigurationArtifacts(configurationID)
}
