package agentqualification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoleQualificationCaseGroupV1LoadsAsDraftProviderFreeDesign(t *testing.T) {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	if group.Status != "draft_prevalidation" {
		t.Fatalf("status = %q", group.Status)
	}
	if group.ProviderPolicy != "forbidden_draft_prevalidation" || group.RuntimeWiring != "none" || len(group.Models) != 0 {
		t.Fatalf("provider/runtime/model boundary is open: %+v", group)
	}
	if len(group.CandidateClasses) != 4 || len(group.Cases) != 12 {
		t.Fatalf("classes=%d cases=%d, want 4 and 12", len(group.CandidateClasses), len(group.Cases))
	}
}

func TestRoleQualificationCaseGroupV1HasThreeCompleteCasesPerCandidateFamily(t *testing.T) {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, candidate := range group.Cases {
		if len(candidate.ApplicableCandidateClasses) != 1 {
			t.Fatalf("%s applicable classes = %v", candidate.CaseID, candidate.ApplicableCandidateClasses)
		}
		counts[candidate.ApplicableCandidateClasses[0]]++
		if len(candidate.PassRequirements) < 3 || len(candidate.SubtleFailures) < 3 || len(candidate.AdversarialFailures) < 3 || len(candidate.RequiredArtifactIDs) < 3 {
			t.Fatalf("%s has incomplete grading declarations", candidate.CaseID)
		}
	}
	for _, classID := range []string{"mika_owner_interface", "quality_checker", "product_ui", "shared_windows_executor"} {
		if counts[classID] != 3 {
			t.Fatalf("%s cases = %d, want 3", classID, counts[classID])
		}
	}
}

func TestRoleQualificationCaseGroupV1CoversEveryRequiredBehavior(t *testing.T) {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, candidate := range group.Cases {
		for _, tag := range candidate.CoverageTags {
			covered[tag] = true
		}
	}
	for _, required := range group.RequiredCoverage {
		if !covered[required] {
			t.Errorf("required behavior %s has no case", required)
		}
	}
}

func TestRoleQualificationCaseGroupV1RejectsProviderModelRuntimeAndStatusDrift(t *testing.T) {
	base := roleQualificationGroupClone(t)

	mutated := base
	mutated.Status = "frozen_reviewed"
	assertRoleQualificationGroupRejected(t, mutated, "draft_prevalidation")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.ProviderPolicy = "allowed"
	assertRoleQualificationGroupRejected(t, mutated, "provider, runtime, or model")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.RuntimeWiring = "opencode"
	assertRoleQualificationGroupRejected(t, mutated, "provider, runtime, or model")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.Models = []string{"some-model"}
	assertRoleQualificationGroupRejected(t, mutated, "provider, runtime, or model")
}

func TestRoleQualificationCaseGroupV1RejectsApplicabilityAndArtifactDrift(t *testing.T) {
	base := roleQualificationGroupClone(t)

	mutated := roleQualificationGroupCloneFrom(t, base)
	mutated.CandidateClasses[0].ID = "unknown"
	assertRoleQualificationGroupRejected(t, mutated, "candidate classes")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.Cases[0].ApplicableCandidateClasses = []string{"quality_checker"}
	assertRoleQualificationGroupRejected(t, mutated, "candidate applicability")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.Cases[0].RequiredArtifactIDs[0] = "unknown"
	assertRoleQualificationGroupRejected(t, mutated, "unknown artifact")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.Cases[0].RequiredArtifactIDs = []string{
		"hosted_workflow_interaction_audit_20260910",
		"item_10_inbox_chat_acceptance",
		"mika_owner_interface_instructions_v1",
	}
	assertRoleQualificationGroupRejected(t, mutated, "omits the role decision register")
}

func TestRoleQualificationCaseGroupV1RejectsIncompleteGradesAndCoverage(t *testing.T) {
	base := roleQualificationGroupClone(t)

	mutated := roleQualificationGroupCloneFrom(t, base)
	mutated.Cases[0].PassRequirements = mutated.Cases[0].PassRequirements[:2]
	assertRoleQualificationGroupRejected(t, mutated, "pass requirements")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.Cases[0].SubtleFailures[1] = mutated.Cases[0].SubtleFailures[0]
	assertRoleQualificationGroupRejected(t, mutated, "duplicate subtle failures")

	mutated = roleQualificationGroupCloneFrom(t, base)
	mutated.Cases[0].AdversarialFailures[0] = ""
	assertRoleQualificationGroupRejected(t, mutated, "blank adversarial failures")

	mutated = roleQualificationGroupCloneFrom(t, base)
	for index := range mutated.Cases {
		filtered := mutated.Cases[index].CoverageTags[:0]
		for _, tag := range mutated.Cases[index].CoverageTags {
			if tag != "phone_farm_prohibition" {
				filtered = append(filtered, tag)
			}
		}
		mutated.Cases[index].CoverageTags = filtered
	}
	assertRoleQualificationGroupRejected(t, mutated, "do not cover every required behavior")
}

func TestRoleQualificationCaseGroupV1PinsCriticalOwnerAndHostBoundaries(t *testing.T) {
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]RoleQualificationCase{}
	for _, candidate := range group.Cases {
		cases[candidate.CaseID] = candidate
	}
	assertRoleQualificationCaseTextContains(t, cases["RQ-MIKA-01"], "Observed-Unknown-Checking", "inbound is not proven yet")
	assertRoleQualificationCaseTextContains(t, cases["RQ-MIKA-02"], "exactly one new issue", "Inbox item")
	assertRoleQualificationCaseTextContains(t, cases["RQ-MIKA-03"], "hypothesis", "genuine bounded owner decision")
	assertRoleQualificationCaseTextContains(t, cases["RQ-QC-01"], "diagnosis evidence", "employee business outcome")
	assertRoleQualificationCaseTextContains(t, cases["RQ-QC-02"], "Reject Live and Done", "verified baseline recovery")
	assertRoleQualificationCaseTextContains(t, cases["RQ-PUI-01"], "taste_decision_needed to none", "minimum, typical, maximum")
	assertRoleQualificationCaseTextContains(t, cases["RQ-PUI-02"], "exactly one bounded owner question", "routine design details")
	assertRoleQualificationCaseTextContains(t, cases["RQ-WIN-02"], "never reset, clean, stash, switch, delete", "separate requested evidence lanes")
	assertRoleQualificationCaseTextContains(t, cases["RQ-WIN-03"], "Windows-initiated SSH", "C:\\Dev\\SMC", "dedicated Mac")
}

func roleQualificationGroupClone(t *testing.T) RoleQualificationCaseGroup {
	t.Helper()
	group, err := CurrentRoleQualificationCaseGroupV1()
	if err != nil {
		t.Fatal(err)
	}
	return roleQualificationGroupCloneFrom(t, group)
}

func roleQualificationGroupCloneFrom(t *testing.T, group RoleQualificationCaseGroup) RoleQualificationCaseGroup {
	t.Helper()
	data, err := json.Marshal(group)
	if err != nil {
		t.Fatal(err)
	}
	var clone RoleQualificationCaseGroup
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func assertRoleQualificationGroupRejected(t *testing.T, group RoleQualificationCaseGroup, want string) {
	t.Helper()
	err := ValidateRoleQualificationCaseGroupV1(group)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want rejection containing %q", err, want)
	}
}

func assertRoleQualificationCaseTextContains(t *testing.T, candidate RoleQualificationCase, terms ...string) {
	t.Helper()
	parts := []string{candidate.Title, candidate.Purpose, candidate.Scenario}
	parts = append(parts, candidate.PassRequirements...)
	parts = append(parts, candidate.SubtleFailures...)
	parts = append(parts, candidate.AdversarialFailures...)
	text := strings.Join(parts, "\n")
	for _, term := range terms {
		if !strings.Contains(text, term) {
			t.Errorf("%s does not pin %q", candidate.CaseID, term)
		}
	}
}
