package agentqualification

import (
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedConfigurationArtifactManifestValidates(t *testing.T) {
	manifest, err := EmbeddedConfigurationArtifactManifest()
	if err != nil {
		t.Fatal(err)
	}
	decisions, err := AcceptedConfigurations()
	if err != nil {
		t.Fatal(err)
	}
	roles, err := historicalConfigurationRoles()
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Validate(decisions, roles, historicalConfigurationRolesSHA256(), configurationArtifactFiles); err != nil {
		t.Fatal(err)
	}
	for _, configurationID := range []string{
		"mika.owner_interface.luna.medium.v1",
		"quality_checker.terra.xhigh.v1",
		"product_ui.sol.high.v1",
	} {
		binding, err := historicalContracts(t).ValidatedConfigurationArtifacts(configurationID)
		if err != nil {
			t.Errorf("validate %s: %v", configurationID, err)
			continue
		}
		if binding.Status != StatusProvisionalShadow {
			t.Errorf("binding %s status = %s, want %s", configurationID, binding.Status, StatusProvisionalShadow)
		}
	}
}

func TestValidatedConfigurationArtifactsFailsClosedWhenUnbound(t *testing.T) {
	_, err := historicalContracts(t).ValidatedConfigurationArtifacts("windows_builder.crm_product.v1")
	if err == nil || !strings.Contains(err.Error(), "no immutable artifact binding") {
		t.Fatalf("unbound configuration error = %v", err)
	}
}

func TestConfigurationArtifactManifestRejectsMissingAndMismatchedFiles(t *testing.T) {
	manifest, decisions, files := artifactManifestFixture(t)
	delete(files, manifest.Bindings[0].Instruction.Path)
	if err := validateArtifactManifest(manifest, decisions, files); err == nil || !strings.Contains(err.Error(), "read artifact") {
		t.Fatalf("missing artifact error = %v", err)
	}

	manifest, decisions, files = artifactManifestFixture(t)
	files[manifest.Bindings[0].Instruction.Path].Data = []byte("tampered")
	if err := validateArtifactManifest(manifest, decisions, files); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("mismatched artifact error = %v", err)
	}
}

func TestConfigurationArtifactManifestRejectsMutableSkillVersion(t *testing.T) {
	manifest, decisions, files := artifactManifestFixture(t)
	manifest.Bindings[0].Skills[0].Version = "project-current"
	decisions.Decisions[0].Configuration.SkillVersions["owner_interface"] = "project-current"
	if err := validateArtifactManifest(manifest, decisions, files); err == nil || !strings.Contains(err.Error(), "uses mutable version") {
		t.Fatalf("mutable skill version error = %v", err)
	}
}

func TestConfigurationArtifactManifestRejectsDecisionDrift(t *testing.T) {
	manifest, decisions, files := artifactManifestFixture(t)
	decisions.Decisions[0].Configuration.PermissionVersion = "different.v1"
	if err := validateArtifactManifest(manifest, decisions, files); err == nil || !strings.Contains(err.Error(), "does not match its decision") {
		t.Fatalf("decision drift error = %v", err)
	}
}

func TestConfigurationArtifactManifestRejectsRoleCatalogDrift(t *testing.T) {
	manifest, decisions, files := artifactManifestFixture(t)
	roles, err := historicalConfigurationRoles()
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Validate(decisions, roles, strings.Repeat("0", 64), files); err == nil || !strings.Contains(err.Error(), "role catalog SHA-256 mismatch") {
		t.Fatalf("role catalog drift error = %v", err)
	}
}

func artifactManifestFixture(t *testing.T) (ConfigurationArtifactManifest, DecisionCatalog, fstest.MapFS) {
	t.Helper()
	manifest, err := EmbeddedConfigurationArtifactManifest()
	if err != nil {
		t.Fatal(err)
	}
	allDecisions, err := AcceptedConfigurations()
	if err != nil {
		t.Fatal(err)
	}
	decisions := DecisionCatalog{Version: allDecisions.Version}
	files := fstest.MapFS{}
	var paths []string
	for _, binding := range manifest.Bindings {
		decision, ok := allDecisions.Decision(binding.ConfigurationID)
		if !ok {
			t.Fatalf("fixture configuration decision %s missing", binding.ConfigurationID)
		}
		decision.Configuration.SkillVersions = cloneArtifactStringMap(decision.Configuration.SkillVersions)
		decisions.Decisions = append(decisions.Decisions, decision)
		paths = append(paths, binding.Instruction.Path)
		for _, skill := range binding.Skills {
			paths = append(paths, skill.Path)
		}
	}
	for _, artifactPath := range paths {
		data, err := configurationArtifactFiles.ReadFile(artifactPath)
		if err != nil {
			t.Fatal(err)
		}
		files[artifactPath] = &fstest.MapFile{Data: append([]byte(nil), data...)}
	}
	return manifest, decisions, files
}

func cloneArtifactStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func validateArtifactManifest(manifest ConfigurationArtifactManifest, decisions DecisionCatalog, files fstest.MapFS) error {
	roles, err := historicalConfigurationRoles()
	if err != nil {
		return err
	}
	return manifest.Validate(decisions, roles, historicalConfigurationRolesSHA256(), files)
}

func historicalConfigurationRoles() (systemgraph.RoleCatalog, error) {
	c, err := HistoricalQualificationContracts()
	if err != nil {
		return systemgraph.RoleCatalog{}, err
	}
	return c.roles()
}
func historicalConfigurationRolesSHA256() string {
	c, err := HistoricalQualificationContracts()
	if err != nil {
		return ""
	}
	return c.rolesSHA256()
}
func TestCurrentConfigurationArtifactBindingRejectsHistoricalRoleCatalog(t *testing.T) {
	if _, err := ValidatedConfigurationArtifacts("mika.owner_interface.luna.medium.v1"); err == nil {
		t.Fatal("current source drift reused historical configuration authority")
	}
}
