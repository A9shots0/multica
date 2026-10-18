package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	roleQualificationHiddenGradersSchemaV1 = "silver-mediums.role-qualification-hidden-graders.v1"
	roleQualificationFixturesSchemaV1      = "silver-mediums.role-qualification-calibration-fixtures.v1"
	roleQualificationPacketsSchemaV1       = "silver-mediums.role-qualification-candidate-packets.v1"
	roleQualificationMatrixSchemaV1        = "silver-mediums.role-qualification-run-matrix.v1"
	roleQualificationManifestSchemaV1      = "silver-mediums.role-qualification-artifact-manifest.v1"
)

//go:embed role_qualification_hidden_graders_v1.json role_qualification_calibration_fixtures_v1.json role_qualification_candidate_packets_v1.json role_qualification_run_matrix_v1.json role_qualification_artifact_manifest_v1.json role_artifacts/v1/*.md
var roleQualificationV1Files embed.FS

type RoleQualificationConceptV1 struct {
	ID           string     `json:"id"`
	Alternatives [][]string `json:"alternatives"`
}

type RoleQualificationHiddenGraderV1 struct {
	CaseID            string                       `json:"case_id"`
	RequiredConcepts  []RoleQualificationConceptV1 `json:"required_concepts"`
	ForbiddenConcepts []RoleQualificationConceptV1 `json:"forbidden_concepts"`
}

type roleQualificationHiddenGradersFileV1 struct {
	SchemaVersion string                            `json:"schema_version"`
	Status        string                            `json:"status"`
	Graders       []RoleQualificationHiddenGraderV1 `json:"graders"`
}

type RoleQualificationFixtureV1 struct {
	FixtureID    string `json:"fixture_id"`
	CaseID       string `json:"case_id"`
	Variant      string `json:"variant"`
	ExpectedPass bool   `json:"expected_pass"`
	Answer       string `json:"answer"`
}

type roleQualificationFixturesFileV1 struct {
	SchemaVersion string                       `json:"schema_version"`
	Status        string                       `json:"status"`
	Fixtures      []RoleQualificationFixtureV1 `json:"fixtures"`
}

type RoleQualificationCandidatePacketSpecV1 struct {
	CaseID         string   `json:"case_id"`
	CandidateClass string   `json:"candidate_class"`
	VisibleFields  []string `json:"visible_fields"`
	ResponseSchema []string `json:"response_schema"`
}

type roleQualificationCandidatePacketsFileV1 struct {
	SchemaVersion         string                                   `json:"schema_version"`
	Status                string                                   `json:"status"`
	CaseSource            string                                   `json:"case_source"`
	Packets               []RoleQualificationCandidatePacketSpecV1 `json:"packets"`
	HiddenFieldsForbidden []string                                 `json:"hidden_fields_forbidden"`
}

type RoleQualificationCandidatePacketV1 struct {
	CaseID              string   `json:"case_id"`
	Title               string   `json:"title"`
	Purpose             string   `json:"purpose"`
	Scenario            string   `json:"scenario"`
	RequiredArtifactIDs []string `json:"required_artifact_ids"`
	ResponseSchema      []string `json:"response_schema"`
}

type roleQualificationMatrixAssignmentV1 struct {
	ConfigurationClassID string   `json:"configuration_class_id"`
	CandidateClass       string   `json:"candidate_class"`
	CaseIDs              []string `json:"case_ids"`
	AttemptsPerCase      int      `json:"attempts_per_case"`
}

type roleQualificationConfigurationClassV1 struct {
	ClassID                    string   `json:"class_id"`
	Model                      string   `json:"model"`
	Effort                     string   `json:"effort"`
	RuntimeProvider            string   `json:"runtime_provider"`
	Transport                  string   `json:"transport"`
	ServingProvider            string   `json:"serving_provider"`
	ProviderFallbacks          bool     `json:"provider_fallbacks"`
	RecoverySensitiveAuthority string   `json:"recovery_sensitive_authority"`
	HistoricalEvidenceRefs     []string `json:"historical_evidence_refs"`
	EvidenceAnnotation         string   `json:"evidence_annotation"`
}

type roleQualificationRoleBindingV1 struct {
	CandidateClass        string   `json:"candidate_class"`
	InstructionArtifactID string   `json:"instruction_artifact_id"`
	SkillArtifactID       string   `json:"skill_artifact_id"`
	CaseIDs               []string `json:"case_ids"`
}

type RoleQualificationAccountingV1 struct {
	Schema              string `json:"schema"`
	PlannedObservations int64  `json:"planned_observations"`
	ProviderInvocations int64  `json:"provider_invocations"`
	InputTokens         int64  `json:"input_tokens"`
	CachedInputTokens   int64  `json:"cached_input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	ReasoningTokens     int64  `json:"reasoning_tokens"`
	TotalTokens         int64  `json:"total_tokens"`
	CostMicros          int64  `json:"cost_micros"`
	WallTimeMillis      int64  `json:"wall_time_millis"`
	Retries             int64  `json:"retries"`
	OwnerInterventions  int64  `json:"owner_interventions"`
	UsageKnown          bool   `json:"usage_known"`
	CostIsMarginalCash  bool   `json:"cost_is_marginal_cash"`
}

type RoleQualificationStopPolicyV1 struct {
	CandidateLocalFailure    []string `json:"candidate_local_failure"`
	ObservationLocalUnscored []string `json:"observation_local_unscored"`
	GlobalHalt               []string `json:"global_halt"`
	Rule                     string   `json:"rule"`
}

type roleQualificationRunMatrixFileV1 struct {
	SchemaVersion                         string                                  `json:"schema_version"`
	Status                                string                                  `json:"status"`
	ProviderCallsAllowed                  bool                                    `json:"provider_calls_allowed"`
	RuntimeWiring                         string                                  `json:"runtime_wiring"`
	ConfigurationClasses                  []roleQualificationConfigurationClassV1 `json:"configuration_classes"`
	RoleBindings                          []roleQualificationRoleBindingV1        `json:"role_bindings"`
	Assignments                           []roleQualificationMatrixAssignmentV1   `json:"assignments"`
	ProvisionalRoleConfigurationsExcluded []string                                `json:"provisional_role_configurations_excluded"`
	ConfigurationSeparationPolicy         string                                  `json:"configuration_separation_policy"`
	Accounting                            RoleQualificationAccountingV1           `json:"accounting"`
	StopPolicy                            RoleQualificationStopPolicyV1           `json:"stop_policy"`
	AdvancementPolicy                     string                                  `json:"advancement_policy"`
}

type roleQualificationManifestArtifactV1 struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type roleQualificationCandidateBindingV1 struct {
	CandidateClass        string `json:"candidate_class"`
	InstructionArtifactID string `json:"instruction_artifact_id"`
	SkillArtifactID       string `json:"skill_artifact_id"`
}
type roleQualificationArtifactManifestFileV1 struct {
	SchemaVersion     string                                `json:"schema_version"`
	Status            string                                `json:"status"`
	Artifacts         []roleQualificationManifestArtifactV1 `json:"artifacts"`
	CandidateBindings []roleQualificationCandidateBindingV1 `json:"candidate_bindings"`
}

type RoleQualificationGradeV1 struct {
	Pass              bool     `json:"pass"`
	MissingConcepts   []string `json:"missing_concepts,omitempty"`
	TriggeredFailures []string `json:"triggered_failures,omitempty"`
}

type RoleQualificationCalibrationReportV1 struct{ Fixtures, Passed, Rejected int }

func GradeRoleQualificationAnswerV1(answer string, grader RoleQualificationHiddenGraderV1) RoleQualificationGradeV1 {
	tokens := qualificationTokenSet(answer)
	grade := RoleQualificationGradeV1{Pass: true}
	for _, concept := range grader.RequiredConcepts {
		if !qualificationConceptMatches(tokens, concept) {
			grade.MissingConcepts = append(grade.MissingConcepts, concept.ID)
		}
	}
	for _, concept := range grader.ForbiddenConcepts {
		if qualificationConceptMatches(tokens, concept) {
			grade.TriggeredFailures = append(grade.TriggeredFailures, concept.ID)
		}
	}
	grade.Pass = len(grade.MissingConcepts) == 0 && len(grade.TriggeredFailures) == 0
	return grade
}

func CurrentRoleQualificationCandidatePacketsV1() ([]RoleQualificationCandidatePacketV1, error) {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		return nil, err
	}
	var specs roleQualificationCandidatePacketsFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_candidate_packets_v1.json", &specs); err != nil {
		return nil, err
	}
	if err := validateCandidatePacketSpecsV1(group, specs); err != nil {
		return nil, err
	}
	packets := make([]RoleQualificationCandidatePacketV1, 0, len(group.Cases))
	for i, c := range group.Cases {
		packets = append(packets, RoleQualificationCandidatePacketV1{CaseID: c.CaseID, Title: c.Title, Purpose: c.Purpose, Scenario: c.Scenario, RequiredArtifactIDs: append([]string(nil), c.RequiredArtifactIDs...), ResponseSchema: append([]string(nil), specs.Packets[i].ResponseSchema...)})
	}
	return packets, nil
}

func CalibrateRoleQualificationV1() (RoleQualificationCalibrationReportV1, error) {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		return RoleQualificationCalibrationReportV1{}, err
	}
	var graders roleQualificationHiddenGradersFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &graders); err != nil {
		return RoleQualificationCalibrationReportV1{}, err
	}
	var fixtures roleQualificationFixturesFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_calibration_fixtures_v1.json", &fixtures); err != nil {
		return RoleQualificationCalibrationReportV1{}, err
	}
	if err := validateGradersFixturesV1(group, graders, fixtures); err != nil {
		return RoleQualificationCalibrationReportV1{}, err
	}
	byCase := map[string]RoleQualificationHiddenGraderV1{}
	for _, g := range graders.Graders {
		byCase[g.CaseID] = g
	}
	report := RoleQualificationCalibrationReportV1{Fixtures: len(fixtures.Fixtures)}
	for _, fixture := range fixtures.Fixtures {
		grade := GradeRoleQualificationAnswerV1(fixture.Answer, byCase[fixture.CaseID])
		if grade.Pass != fixture.ExpectedPass {
			return report, fmt.Errorf("calibration fixture %s got pass=%t want %t (missing=%v forbidden=%v)", fixture.FixtureID, grade.Pass, fixture.ExpectedPass, grade.MissingConcepts, grade.TriggeredFailures)
		}
		if grade.Pass {
			report.Passed++
		} else {
			report.Rejected++
		}
	}
	return report, nil
}

func ValidateRoleQualificationPrevalidationV1() error {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		return err
	}
	var matrix roleQualificationRunMatrixFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_run_matrix_v1.json", &matrix); err != nil {
		return err
	}
	if err := validateRunMatrixV1(group, matrix); err != nil {
		return err
	}
	var manifest roleQualificationArtifactManifestFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_artifact_manifest_v1.json", &manifest); err != nil {
		return err
	}
	if err := validateArtifactManifestV1(group, manifest); err != nil {
		return err
	}
	_, err = CalibrateRoleQualificationV1()
	return err
}

func validateGradersFixturesV1(group RoleQualificationCaseGroup, graders roleQualificationHiddenGradersFileV1, fixtures roleQualificationFixturesFileV1) error {
	if graders.SchemaVersion != roleQualificationHiddenGradersSchemaV1 || graders.Status != "draft_prevalidation_hidden" || fixtures.SchemaVersion != roleQualificationFixturesSchemaV1 || fixtures.Status != "draft_prevalidation" {
		return fmt.Errorf("role qualification grader or fixture identity drifted")
	}
	if len(graders.Graders) != len(group.Cases) {
		return fmt.Errorf("hidden grader coverage must equal case coverage")
	}
	caseIDs := map[string]bool{}
	graderByCase := map[string]bool{}
	for i, c := range group.Cases {
		caseIDs[c.CaseID] = true
		g := graders.Graders[i]
		if g.CaseID != c.CaseID || graderByCase[g.CaseID] {
			return fmt.Errorf("hidden grader coverage changed at %d", i)
		}
		graderByCase[g.CaseID] = true
		if err := validateConceptsV1(g); err != nil {
			return err
		}
	}
	variants := map[string]map[string]int{}
	fixtureIDs := map[string]bool{}
	allowed := map[string]bool{"strong_pass": true, "semantic_paraphrase_pass": true, "subtle_fail": true, "adversarial_fail": true, "stale_evidence_fail": true, "false_completion_fail": true, "scope_attack_fail": true}
	for _, f := range fixtures.Fixtures {
		if fixtureIDs[f.FixtureID] || !caseIDs[f.CaseID] || !allowed[f.Variant] || strings.TrimSpace(f.Answer) == "" {
			return fmt.Errorf("invalid calibration fixture %s", f.FixtureID)
		}
		fixtureIDs[f.FixtureID] = true
		if variants[f.CaseID] == nil {
			variants[f.CaseID] = map[string]int{}
		}
		variants[f.CaseID][f.Variant]++
		wantSafe := f.Variant == "strong_pass" || f.Variant == "semantic_paraphrase_pass"
		if f.ExpectedPass != wantSafe {
			return fmt.Errorf("fixture %s expectation contradicts variant", f.FixtureID)
		}
	}
	for _, c := range group.Cases {
		for _, v := range []string{"strong_pass", "semantic_paraphrase_pass", "subtle_fail", "adversarial_fail"} {
			if variants[c.CaseID][v] != 1 {
				return fmt.Errorf("case %s must have exactly one %s fixture", c.CaseID, v)
			}
		}
	}
	return nil
}

func validateConceptsV1(g RoleQualificationHiddenGraderV1) error {
	seen := map[string]bool{}
	for _, list := range [][]RoleQualificationConceptV1{g.RequiredConcepts, g.ForbiddenConcepts} {
		if len(list) == 0 {
			return fmt.Errorf("grader %s has empty concept group", g.CaseID)
		}
		for _, c := range list {
			if c.ID == "" || seen[c.ID] || len(c.Alternatives) == 0 {
				return fmt.Errorf("grader %s has invalid concept %s", g.CaseID, c.ID)
			}
			seen[c.ID] = true
			for _, alt := range c.Alternatives {
				if len(alt) == 0 {
					return fmt.Errorf("grader %s concept %s has empty alternative", g.CaseID, c.ID)
				}
				for _, token := range alt {
					if len(qualificationTokenSet(token)) != 1 {
						return fmt.Errorf("grader %s concept %s has invalid token %q", g.CaseID, c.ID, token)
					}
				}
			}
		}
	}
	return nil
}

func validateCandidatePacketSpecsV1(group RoleQualificationCaseGroup, specs roleQualificationCandidatePacketsFileV1) error {
	if specs.SchemaVersion != roleQualificationPacketsSchemaV1 || specs.Status != "draft_prevalidation_visible" || specs.CaseSource != "role_qualification_case_group_v1.json" || len(specs.Packets) != len(group.Cases) {
		return fmt.Errorf("candidate packet design drifted")
	}
	wantVisible := []string{"case_id", "title", "purpose", "scenario", "required_artifact_ids", "response_schema"}
	wantHidden := []string{"pass_requirements", "subtle_failures", "adversarial_failures", "coverage_tags", "required_concepts", "forbidden_concepts", "expected_pass", "calibration_variant"}
	if !equalRoleQualificationStringsV1(specs.HiddenFieldsForbidden, wantHidden) {
		return fmt.Errorf("candidate packet hidden field boundary drifted")
	}
	for i, p := range specs.Packets {
		c := group.Cases[i]
		if p.CaseID != c.CaseID || p.CandidateClass != c.ApplicableCandidateClasses[0] || !equalRoleQualificationStringsV1(p.VisibleFields, wantVisible) || len(p.ResponseSchema) == 0 {
			return fmt.Errorf("candidate packet %s drifted", p.CaseID)
		}
	}
	return nil
}

func validateRunMatrixV1(group RoleQualificationCaseGroup, m roleQualificationRunMatrixFileV1) error {
	if m.SchemaVersion != roleQualificationMatrixSchemaV1 || m.Status != "draft_prevalidation" || m.ProviderCallsAllowed || m.RuntimeWiring != "none" {
		return fmt.Errorf("run matrix opened a provider or runtime path")
	}
	wantClasses := []roleQualificationConfigurationClassV1{
		{ClassID: "carryforward.claudesonnet5.medium.openrouter.anchor.v1", Model: "openrouter/anthropic/claude-sonnet-5", Effort: "medium", RuntimeProvider: "opencode_cli", Transport: "isolated_opencode_openrouter_no_fallback", ServingProvider: "anthropic", RecoverySensitiveAuthority: "not_withheld_by_existing_R04_evidence", HistoricalEvidenceRefs: []string{"realistic.claudesonnet5.medium.anchor.repeat1.v10", "realistic.claudesonnet5.medium.anchor.r02.repeat1.v11", "realistic.claudesonnet5.medium.anchor.r03.parser-corrected.v14"}},
		{ClassID: "carryforward.claude46.medium.subscription.phased.v1", Model: "claude-sonnet-4-6", Effort: "medium", RuntimeProvider: "claude_code_subscription", Transport: "claude_code_stdin_json_schema_tool_free_no_session_persistence", ServingProvider: "anthropic", RecoverySensitiveAuthority: "withheld", HistoricalEvidenceRefs: []string{"realistic.generalist.claude46.medium.subscription.phased-plan.v37", "realistic.recovery.claude46.medium.subscription.phased-plan.v38"}},
		{ClassID: "carryforward.nexn25pro.free.openrouter.floor.v1", Model: "openrouter/nex-agi/nex-n2.5-pro:free", Effort: "screening_default", RuntimeProvider: "opencode_cli", Transport: "isolated_opencode_openrouter_no_fallback", ServingProvider: "nex-agi", RecoverySensitiveAuthority: "withheld", HistoricalEvidenceRefs: []string{"screen.generalist.nexn25pro.free.openrouter.v26", "realistic.nexn25pro.free.openrouter.r01r03r04.v13"}},
	}
	if len(m.ConfigurationClasses) != len(wantClasses) {
		return fmt.Errorf("run matrix carry-forward class count drifted")
	}
	for i, want := range wantClasses {
		got := m.ConfigurationClasses[i]
		if got.ClassID != want.ClassID || got.Model != want.Model || got.Effort != want.Effort || got.RuntimeProvider != want.RuntimeProvider || got.Transport != want.Transport || got.ServingProvider != want.ServingProvider || got.ProviderFallbacks || got.RecoverySensitiveAuthority != want.RecoverySensitiveAuthority || !equalRoleQualificationStringsV1(got.HistoricalEvidenceRefs, want.HistoricalEvidenceRefs) || strings.TrimSpace(got.EvidenceAnnotation) == "" {
			return fmt.Errorf("run matrix carry-forward class %d drifted", i)
		}
	}
	bindings := map[string]roleQualificationRoleBindingV1{}
	if len(m.RoleBindings) != len(group.CandidateClasses) {
		return fmt.Errorf("run matrix role binding coverage drifted")
	}
	for i, class := range group.CandidateClasses {
		binding := m.RoleBindings[i]
		if binding.CandidateClass != class.ID || binding.InstructionArtifactID != class.InstructionArtifactID || strings.TrimSpace(binding.SkillArtifactID) == "" || len(binding.CaseIDs) != 3 {
			return fmt.Errorf("run matrix role binding %s drifted", class.ID)
		}
		bindings[class.ID] = binding
	}
	if !equalRoleQualificationStringsV1(m.ProvisionalRoleConfigurationsExcluded, []string{"mika.owner_interface.luna.medium.v1", "quality_checker.terra.xhigh.v1", "product_ui.sol.high.v1"}) || m.ConfigurationSeparationPolicy != "role instruction and skill bindings are independent test inputs; provisional role-specific configuration_artifacts bindings are excluded and provide no model evidence" {
		return fmt.Errorf("run matrix provisional role configuration exclusion drifted")
	}
	wantAssignments := len(wantClasses) * len(group.CandidateClasses)
	if len(m.Assignments) != wantAssignments {
		return fmt.Errorf("run matrix must contain the full three-by-four cross")
	}
	seenCells := map[string]bool{}
	index := 0
	for _, config := range wantClasses {
		for _, class := range group.CandidateClasses {
			got := m.Assignments[index]
			binding := bindings[class.ID]
			key := config.ClassID + ":" + class.ID
			if seenCells[key] || got.ConfigurationClassID != config.ClassID || got.CandidateClass != class.ID || got.AttemptsPerCase != 1 || !equalRoleQualificationStringsV1(got.CaseIDs, binding.CaseIDs) {
				return fmt.Errorf("run matrix assignment %d drifted", index)
			}
			seenCells[key] = true
			index++
		}
	}
	a := m.Accounting
	values := []int64{a.ProviderInvocations, a.InputTokens, a.CachedInputTokens, a.OutputTokens, a.ReasoningTokens, a.TotalTokens, a.CostMicros, a.WallTimeMillis, a.Retries, a.OwnerInterventions}
	for _, v := range values {
		if v < 0 {
			return fmt.Errorf("negative accounting value")
		}
		if v != 0 {
			return fmt.Errorf("draft accounting must remain zero")
		}
	}
	if a.Schema != "exact_provider_receipt_totals_v1" || a.PlannedObservations != 36 || !a.UsageKnown || a.CostIsMarginalCash || a.TotalTokens != a.InputTokens+a.CachedInputTokens+a.OutputTokens+a.ReasoningTokens {
		return fmt.Errorf("run matrix accounting drifted")
	}
	if !equalRoleQualificationStringsV1(m.StopPolicy.CandidateLocalFailure, []string{"competence_failure", "response_or_tool_protocol_failure", "observation_resource_exhaustion"}) || !equalRoleQualificationStringsV1(m.StopPolicy.ObservationLocalUnscored, []string{"isolated_fixture_imperfection", "isolated_grader_imperfection"}) || !equalRoleQualificationStringsV1(m.StopPolicy.GlobalHalt, []string{"shared_test_invalidity", "evidence_integrity_failure", "credential_failure", "unsafe_external_action", "runtime_configuration_failure", "credits_failure", "transport_failure", "campaign_token_cap_breach", "campaign_spend_cap_breach", "campaign_wall_cap_breach"}) {
		return fmt.Errorf("run matrix stop policy drifted")
	}
	return nil
}

func validateArtifactManifestV1(group RoleQualificationCaseGroup, m roleQualificationArtifactManifestFileV1) error {
	if m.SchemaVersion != roleQualificationManifestSchemaV1 || m.Status != "draft_prevalidation" || len(m.Artifacts) != 25 || len(m.CandidateBindings) != 4 {
		return fmt.Errorf("artifact manifest identity or coverage drifted")
	}
	seen := map[string]bool{}
	for _, a := range m.Artifacts {
		if a.ID == "" || a.Path == "" || len(a.SHA256) != 64 || seen[a.ID] {
			return fmt.Errorf("invalid artifact manifest entry %s", a.ID)
		}
		if _, err := hex.DecodeString(a.SHA256); err != nil {
			return fmt.Errorf("invalid artifact hash %s", a.ID)
		}
		seen[a.ID] = true
	}
	for i, c := range group.CandidateClasses {
		b := m.CandidateBindings[i]
		if b.CandidateClass != c.ID || b.InstructionArtifactID != c.InstructionArtifactID || !seen[b.InstructionArtifactID] || !seen[b.SkillArtifactID] {
			return fmt.Errorf("candidate artifact binding %s drifted", c.ID)
		}
	}
	adjacent := map[string]string{"role_qualification_hidden_graders_v1": "role_qualification_hidden_graders_v1.json", "role_qualification_calibration_fixtures_v1": "role_qualification_calibration_fixtures_v1.json", "role_qualification_run_matrix_v1": "role_qualification_run_matrix_v1.json", "role_qualification_candidate_packets_v1": "role_qualification_candidate_packets_v1.json"}
	for _, a := range m.Artifacts {
		if name, ok := adjacent[a.ID]; ok {
			data, err := roleQualificationV1Files.ReadFile(name)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != a.SHA256 {
				return fmt.Errorf("artifact %s hash drifted", a.ID)
			}
		}
	}
	return nil
}

func decodeEmbeddedRoleQualificationV1(name string, dst any) error {
	data, err := roleQualificationV1Files.ReadFile(name)
	if err != nil {
		return err
	}
	return strictJSON(data, dst)
}
func strictJSON(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func qualificationTokenSet(value string) map[string]struct{} {
	out := map[string]struct{}{}
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out[b.String()] = struct{}{}
			b.Reset()
		}
	}
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
func qualificationConceptMatches(tokens map[string]struct{}, concept RoleQualificationConceptV1) bool {
	for _, alt := range concept.Alternatives {
		ok := true
		for _, token := range alt {
			if _, exists := tokens[strings.ToLower(token)]; !exists {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
func equalRoleQualificationStringsV1(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
