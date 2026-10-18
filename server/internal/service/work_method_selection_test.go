package service

import (
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestSelectAutomaticWorkMethods(t *testing.T) {
	tests := []struct {
		name  string
		input WorkMethodSelectionInput
		want  []string
	}{
		{
			name:  "ordinary issue",
			input: WorkMethodSelectionInput{IssueID: "issue-1", ThreadName: "Add account export"},
			want:  []string{OutcomeOwnershipSkill},
		},
		{
			name:  "modernization audit",
			input: WorkMethodSelectionInput{IssueID: "issue-2", ThreadName: "Audit and modernize PhoneFarm connectivity"},
			want:  []string{OutcomeOwnershipSkill, EvidenceFirstSkill, LongWorkSkill, ResearchREASkill},
		},
		{
			name:  "plain explanation chat",
			input: WorkMethodSelectionInput{ChatMessage: "What does Multica do with that information?"},
			want:  nil,
		},
		{
			name:  "automatic follow up",
			input: WorkMethodSelectionInput{IssueID: "issue-3", ChatFollowUp: "summarize"},
			want:  nil,
		},
		{
			name:  "controller bound",
			input: WorkMethodSelectionInput{IssueID: "issue-4", ThreadName: "Audit failure", ControllerBound: true},
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			methods := SelectAutomaticWorkMethods(tc.input)
			got := make([]string, 0, len(methods))
			for _, method := range methods {
				got = append(got, method.Name)
			}
			if len(got) != len(tc.want) || (len(got) > 0 && !reflect.DeepEqual(got, tc.want)) {
				t.Fatalf("selected methods = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterAutomaticWorkMethodsKeepsOnlySelectedBuiltins(t *testing.T) {
	input := WorkMethodSelectionInput{IssueID: "issue-1", ThreadName: "Research a deployment failure"}
	inline := []AgentSkillData{
		{ID: "workspace-1", Name: OutcomeOwnershipSkill},
		{Name: PlatformSkillName},
		{Name: OutcomeOwnershipSkill},
		{Name: EvidenceFirstSkill},
		{Name: ResearchREASkill},
		{Name: LongWorkSkill},
	}
	filtered := FilterAutomaticWorkMethodSkills(inline, input)
	got := make([]string, 0, len(filtered))
	for _, skill := range filtered {
		got = append(got, skill.Name)
	}
	want := []string{OutcomeOwnershipSkill, PlatformSkillName, OutcomeOwnershipSkill, EvidenceFirstSkill, ResearchREASkill}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered inline skills = %v, want %v", got, want)
	}

	refs := []AgentSkillRefData{
		{ID: "workspace-1", Source: skillbundle.SourceWorkspace, Name: LongWorkSkill},
		{ID: BuiltinSkillID(PlatformSkillName), Source: skillbundle.SourceBuiltin, Name: PlatformSkillName},
		{ID: BuiltinSkillID(OutcomeOwnershipSkill), Source: skillbundle.SourceBuiltin, Name: OutcomeOwnershipSkill},
		{ID: BuiltinSkillID(EvidenceFirstSkill), Source: skillbundle.SourceBuiltin, Name: EvidenceFirstSkill},
		{ID: BuiltinSkillID(ResearchREASkill), Source: skillbundle.SourceBuiltin, Name: ResearchREASkill},
		{ID: BuiltinSkillID(LongWorkSkill), Source: skillbundle.SourceBuiltin, Name: LongWorkSkill},
	}
	filteredRefs := FilterAutomaticWorkMethodRefs(refs, input)
	got = got[:0]
	for _, ref := range filteredRefs {
		got = append(got, ref.Name)
	}
	want = []string{LongWorkSkill, PlatformSkillName, OutcomeOwnershipSkill, EvidenceFirstSkill, ResearchREASkill}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered skill refs = %v, want %v", got, want)
	}
}
