package agentqualification

import (
	"errors"
	"strings"
	"testing"
)

func TestGuardedClaimExposureRequiresExactArtifactsAndNoAmbientSurfaces(t *testing.T) {
	pkg, _, _ := guardedExecutionPackageFixture(t)
	instructions, err := roleArtifactSetV2FS.ReadFile(pkg.Envelope.ArtifactBinding.Instruction.Path)
	if err != nil {
		t.Fatal(err)
	}
	valid := GuardedClaimExposureFactsV1{
		RuntimeProvider: pkg.Envelope.RuntimeProvider, Model: pkg.Envelope.Model,
		ReasoningEffort: pkg.Envelope.ReasoningEffort, Instructions: string(instructions),
		SkillSHA256: []string{pkg.Envelope.ArtifactBinding.Skill.SHA256},
	}
	if err := ValidateGuardedClaimExposureV1(&pkg, valid); err != nil {
		t.Fatalf("exact guarded exposure rejected: %v", err)
	}

	cases := map[string]func(*GuardedClaimExposureFactsV1){
		"provider drift":    func(v *GuardedClaimExposureFactsV1) { v.RuntimeProvider = "other" },
		"service tier":      func(v *GuardedClaimExposureFactsV1) { v.ServiceTier = "priority" },
		"instruction drift": func(v *GuardedClaimExposureFactsV1) { v.Instructions += " changed" },
		"extra skill":       func(v *GuardedClaimExposureFactsV1) { v.SkillSHA256 = append(v.SkillSHA256, v.SkillSHA256[0]) },
		"custom env":        func(v *GuardedClaimExposureFactsV1) { v.CustomEnvCount = 1 },
		"custom args":       func(v *GuardedClaimExposureFactsV1) { v.CustomArgsCount = 1 },
		"lazy skill":        func(v *GuardedClaimExposureFactsV1) { v.LazySkills = true },
		"disabled skill":    func(v *GuardedClaimExposureFactsV1) { v.DisabledSkills = true },
		"direct mcp":        func(v *GuardedClaimExposureFactsV1) { v.DirectMCP = true },
		"runtime config":    func(v *GuardedClaimExposureFactsV1) { v.RuntimeConfig = true },
		"connected app":     func(v *GuardedClaimExposureFactsV1) { v.ConnectedApps = true },
		"remote mcp":        func(v *GuardedClaimExposureFactsV1) { v.RemoteMCP = true },
		"plugin hook":       func(v *GuardedClaimExposureFactsV1) { v.PluginHooks = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.SkillSHA256 = append([]string(nil), valid.SkillSHA256...)
			mutate(&candidate)
			if err := ValidateGuardedClaimExposureV1(&pkg, candidate); !errors.Is(err, ErrGuardedClaimExposureDenied) {
				t.Fatalf("error = %v, want guarded exposure denial", err)
			}
		})
	}
}

func TestGuardedHostExecutionRequiresExactBoundBoundary(t *testing.T) {
	pkg, expected, now := guardedExecutionPackageFixture(t)
	boundary := pkg.RuntimeBinding.ChildExecution
	actual := GuardedHostExecutionFactsV1{
		HostOS: boundary.HostOS, Adapter: boundary.Adapter, EnvironmentPolicy: boundary.EnvironmentPolicy,
		EnvironmentAllowlistSHA256: boundary.EnvironmentAllowlistSHA256, ToolPolicy: boundary.ToolPolicy,
		HostAttestationCustody: boundary.HostAttestationCustody, ExecutableSHA256: boundary.ExecutableSHA256,
	}
	if err := ValidateGuardedHostExecutionV1(&pkg, expected, actual, now); err != nil {
		t.Fatalf("exact guarded child boundary rejected: %v", err)
	}
	expected.HostAttestation = "other-host"
	if err := ValidateGuardedHostExecutionV1(&pkg, expected, actual, now); err == nil || errors.Is(err, ErrGuardedEnvironmentNotIsolated) {
		t.Fatalf("host substitution reached environment boundary: %v", err)
	}
	expected.HostAttestation = "alienware_windows"
	mutations := map[string]func(*GuardedHostExecutionFactsV1){
		"host os":      func(v *GuardedHostExecutionFactsV1) { v.HostOS = "linux" },
		"adapter":      func(v *GuardedHostExecutionFactsV1) { v.Adapter = GuardedOpenCodeAdapterV1 },
		"environment":  func(v *GuardedHostExecutionFactsV1) { v.EnvironmentAllowlistSHA256 = strings.Repeat("b", 64) },
		"tool policy":  func(v *GuardedHostExecutionFactsV1) { v.ToolPolicy = "tools_allowed" },
		"host custody": func(v *GuardedHostExecutionFactsV1) { v.HostAttestationCustody = "child_env" },
		"executable":   func(v *GuardedHostExecutionFactsV1) { v.ExecutableSHA256 = strings.Repeat("b", 64) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := actual
			mutate(&candidate)
			if err := ValidateGuardedHostExecutionV1(&pkg, expected, candidate, now); !errors.Is(err, ErrGuardedEnvironmentNotIsolated) {
				t.Fatalf("boundary substitution error = %v", err)
			}
		})
	}
	if err := ValidateGuardedHostExecutionV1(nil, GuardedExecutionExpectedV1{}, GuardedHostExecutionFactsV1{}, now); err != nil {
		t.Fatalf("legacy execution changed: %v", err)
	}
}
