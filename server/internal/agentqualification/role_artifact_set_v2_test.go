package agentqualification

import "testing"

func TestRoleArtifactSetV2BindsSevenCandidatePairsAndOneSkillOnlyArtifact(t *testing.T) {
	if _, err := CurrentRoleArtifactSetV2(); err != nil {
		t.Fatal(err)
	}
}

func TestRoleArtifactSetV2FailsClosedOnAuthorityCoverageAndHashDrift(t *testing.T) {
	base, err := CurrentRoleArtifactSetV2()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RoleArtifactSetV2){
		"review":     func(s *RoleArtifactSetV2) { s.IndependentReviewOutcome = "go" },
		"correction": func(s *RoleArtifactSetV2) { s.ConsolidatedCorrections = 2 },
		"binding":    func(s *RoleArtifactSetV2) { s.ConfigurationBinding = "live" },
		"provider":   func(s *RoleArtifactSetV2) { s.ProviderCallsAllowed = true },
		"runtime":    func(s *RoleArtifactSetV2) { s.RuntimeWiring = "live" },
		"model":      func(s *RoleArtifactSetV2) { s.ModelSelectionAllowed = true },
		"role":       func(s *RoleArtifactSetV2) { s.CandidateRoles = s.CandidateRoles[:6] },
		"skill":      func(s *RoleArtifactSetV2) { s.SkillOnly = nil },
		"hash": func(s *RoleArtifactSetV2) {
			s.Artifacts["role_artifacts/v2/company_auditor.skill.md"] = "00"
		},
	} {
		copy := base
		copy.CandidateRoles = append([]string(nil), base.CandidateRoles...)
		copy.SkillOnly = append([]string(nil), base.SkillOnly...)
		copy.Artifacts = map[string]string{}
		for path, hash := range base.Artifacts {
			copy.Artifacts[path] = hash
		}
		mutate(&copy)
		if err := ValidateRoleArtifactSetV2(copy); err == nil {
			t.Errorf("%s drift was accepted", name)
		}
	}
}
