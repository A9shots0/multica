package agentqualification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoleQualificationExecutionV3PlanTypedSchemasAndCoverage(t *testing.T) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	if plan.ProviderCallsAllowed || plan.RuntimeWiring != "none" || len(plan.Cells) != 36 {
		t.Fatalf("plan boundary: %+v", plan)
	}
	coverage := map[string]int{}
	for _, cell := range plan.Cells {
		coverage[cell.ConfigurationClass.ClassID+":"+cell.CandidateClass]++
		if cell.Request.Retries != 0 || cell.Request.Fallbacks || cell.RequestSHA256 == "" {
			t.Fatalf("cell drift: %+v", cell)
		}
		raw, _ := json.Marshal(cell.Request.CandidatePacket)
		for _, hidden := range []string{"required_concepts", "forbidden_concepts", "pass_requirements", "subtle_failures", "adversarial_failures"} {
			if strings.Contains(string(raw), hidden) {
				t.Fatalf("candidate packet leaked %s", hidden)
			}
		}
		for field, spec := range cell.Request.CandidatePacket.ResponseSchema.Properties {
			if spec.Type == "array" && (spec.Items == nil || spec.Items.Type != "string") {
				t.Fatalf("%s array item type", field)
			}
		}
	}
	if len(coverage) != 12 {
		t.Fatalf("pair coverage=%d", len(coverage))
	}
	for pair, count := range coverage {
		if count != 3 {
			t.Errorf("%s=%d", pair, count)
		}
	}
}

func TestRoleQualificationExecutionV3FieldStructuredCalibration(t *testing.T) {
	plan, err := BuildRoleQualificationExecutionPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, cell := range plan.Cells {
		if seen[cell.CaseID] {
			continue
		}
		seen[cell.CaseID] = true
		for _, paraphrase := range []bool{false, true} {
			answer := structuredPassingAnswerV3(t, cell, paraphrase)
			raw, _ := json.Marshal(answer)
			parsed, canonical, err := ParseRoleQualificationTypedOutputV3(cell.Request.CandidatePacket.ResponseSchema, raw)
			if err != nil {
				t.Fatalf("%s paraphrase=%v parse: %v answer=%v required=%v", cell.CaseID, paraphrase, err, answer, cell.Request.CandidatePacket.ResponseSchema.Required)
			}
			grade, err := GradeRoleQualificationTypedOutputV3(cell.CaseID, parsed)
			if err != nil || !grade.Pass || !json.Valid(canonical) {
				t.Fatalf("%s paraphrase=%v grade=%+v err=%v", cell.CaseID, paraphrase, grade, err)
			}
		}
	}
	if len(seen) != 12 {
		t.Fatalf("case coverage=%d", len(seen))
	}
}

func TestRoleQualificationExecutionV3RejectsWrongFieldWrongTypeAndAdversarial(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	seenWrongField := map[string]bool{}
	for _, cell := range plan.Cells {
		if seenWrongField[cell.CaseID] {
			continue
		}
		seenWrongField[cell.CaseID] = true
		answer := structuredPassingAnswerV3(t, cell, false)
		var hidden roleQualificationHiddenGradersFileV1
		if err := decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &hidden); err != nil {
			t.Fatal(err)
		}
		var g RoleQualificationHiddenGraderV1
		for _, candidate := range hidden.Graders {
			if candidate.CaseID == cell.CaseID {
				g = candidate
			}
		}
		concept := g.RequiredConcepts[0]
		correct := roleQualificationConceptFieldV3(cell.CaseID, concept.ID)
		wrong := ""
		for f, spec := range cell.Request.CandidatePacket.ResponseSchema.Properties {
			if f != correct && spec.Type == cell.Request.CandidatePacket.ResponseSchema.Properties[correct].Type {
				wrong = f
				break
			}
		}
		if wrong != "" {
			answer[correct] = neutralValueV3(cell.Request.CandidatePacket.ResponseSchema.Properties[correct].Type, correct)
			answer[wrong] = conceptAlternativeValueV3(answer[wrong], concept.Alternatives[0])
			grade, _ := GradeRoleQualificationTypedOutputV3(cell.CaseID, answer)
			if grade.Pass {
				t.Fatalf("%s accepted required concept in wrong field", cell.CaseID)
			}
		}
	}
	seenType := map[string]bool{}
	for _, cell := range plan.Cells {
		if seenType[cell.CandidateClass] {
			continue
		}
		seenType[cell.CandidateClass] = true
		answer := structuredPassingAnswerV3(t, cell, false)
		for f, spec := range cell.Request.CandidatePacket.ResponseSchema.Properties {
			bad := cloneTypedAnswerV3(answer)
			switch spec.Type {
			case "string":
				bad[f] = []string{"wrong"}
			case "array":
				bad[f] = "wrong"
			case "boolean":
				bad[f] = "false"
			}
			raw, _ := json.Marshal(bad)
			if _, _, err := ParseRoleQualificationTypedOutputV3(cell.Request.CandidatePacket.ResponseSchema, raw); err == nil {
				t.Fatalf("%s accepted wrong type for %s", cell.CaseID, f)
			}
			break
		}
	}
	seenAdversarial := map[string]bool{}
	for _, cell := range plan.Cells {
		if seenAdversarial[cell.CaseID] {
			continue
		}
		seenAdversarial[cell.CaseID] = true
		answer := structuredPassingAnswerV3(t, cell, false)
		var hidden roleQualificationHiddenGradersFileV1
		_ = decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &hidden)
		for _, g := range hidden.Graders {
			if g.CaseID == cell.CaseID && len(g.ForbiddenConcepts) > 0 {
				bad := cloneTypedAnswerV3(answer)
				c := g.ForbiddenConcepts[0]
				field := roleQualificationConceptFieldV3(cell.CaseID, c.ID)
				bad[field] = conceptAlternativeValueV3(bad[field], c.Alternatives[0])
				grade, _ := GradeRoleQualificationTypedOutputV3(cell.CaseID, bad)
				if grade.Pass {
					t.Fatalf("%s accepted adversarial concept", cell.CaseID)
				}
			}
		}
	}
}

func TestRoleQualificationExecutionV3SubtleFailuresAndNoFillerAcrossAllCases(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	seen := map[string]bool{}
	for _, cell := range plan.Cells {
		if seen[cell.CaseID] {
			continue
		}
		seen[cell.CaseID] = true
		strong := structuredPassingAnswerV3(t, cell, false)
		raw, _ := json.Marshal(strong)
		if strings.Contains(strings.ToLower(string(raw)), "not applicable") {
			t.Fatalf("%s contains filler", cell.CaseID)
		}
		var hidden roleQualificationHiddenGradersFileV1
		_ = decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &hidden)
		for _, g := range hidden.Graders {
			if g.CaseID == cell.CaseID {
				c := g.RequiredConcepts[0]
				field := roleQualificationConceptFieldV3(cell.CaseID, c.ID)
				subtle := cloneTypedAnswerV3(strong)
				subtle[field] = neutralValueV3(cell.Request.CandidatePacket.ResponseSchema.Properties[field].Type, field)
				grade, err := GradeRoleQualificationTypedOutputV3(cell.CaseID, subtle)
				if err != nil {
					t.Fatal(err)
				}
				if grade.Pass {
					t.Fatalf("%s subtle omission passed", cell.CaseID)
				}
			}
		}
	}
	if len(seen) != 12 {
		t.Fatalf("coverage=%d", len(seen))
	}
}

func structuredPassingAnswerV3(t *testing.T, cell RoleQualificationExecutionCellV3, paraphrase bool) RoleQualificationTypedOutputV3 {
	t.Helper()
	out := RoleQualificationTypedOutputV3{}
	for f, spec := range cell.Request.CandidatePacket.ResponseSchema.Properties {
		out[f] = neutralValueV3(spec.Type, f)
	}
	var hidden roleQualificationHiddenGradersFileV1
	if err := decodeEmbeddedRoleQualificationV1("role_qualification_hidden_graders_v1.json", &hidden); err != nil {
		t.Fatal(err)
	}
	for _, g := range hidden.Graders {
		if g.CaseID == cell.CaseID {
			for _, c := range g.RequiredConcepts {
				field := roleQualificationConceptFieldV3(cell.CaseID, c.ID)
				idx := 0
				if paraphrase && len(c.Alternatives) > 1 {
					idx = 1
				}
				out[field] = conceptAlternativeValueV3(out[field], c.Alternatives[idx])
			}
		}
	}
	for f, want := range roleQualificationBooleanExpectationsV3(cell.CaseID) {
		out[f] = want
	}
	return out
}
func neutralValueV3(typ, field string) any {
	switch typ {
	case "array":
		return []any{"Evidence for " + strings.ReplaceAll(field, "_", " ") + " remains scoped and explicit."}
	case "boolean":
		return false
	default:
		return "The response addresses " + strings.ReplaceAll(field, "_", " ") + " directly."
	}
}
func conceptAlternativeValueV3(current any, words []string) any {
	phrase := strings.Join(words, " ")
	switch v := current.(type) {
	case string:
		return v + " " + phrase
	case []any:
		return append(v, phrase)
	default:
		return phrase
	}
}
func cloneTypedAnswerV3(in RoleQualificationTypedOutputV3) RoleQualificationTypedOutputV3 {
	raw, _ := json.Marshal(in)
	var out RoleQualificationTypedOutputV3
	_ = json.Unmarshal(raw, &out)
	return out
}
