package service

import (
	"errors"
	"testing"
)

func TestAgentIssueCreateAdmissionKeepsOnlyMemberAuthorizedExceptions(t *testing.T) {
	tests := []struct {
		name          string
		requireMember bool
		creatorType   string
		wantBlocked   bool
	}{
		{name: "disabled policy keeps ordinary agent create", creatorType: "agent"},
		{name: "disabled policy keeps existing agent origin paths", creatorType: "agent"},
		{name: "enabled policy keeps member intake", requireMember: true, creatorType: "member"},
		{name: "enabled policy blocks root agent create", requireMember: true, creatorType: "agent", wantBlocked: true},
		{name: "enabled policy blocks child agent create", requireMember: true, creatorType: "agent", wantBlocked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := admitAgentIssueCreate(tt.requireMember, tt.creatorType)
			if tt.wantBlocked && !errors.Is(err, ErrAgentIssueCreationDisabled) {
				t.Fatalf("admitAgentIssueCreate() error = %v, want %v", err, ErrAgentIssueCreationDisabled)
			}
			if !tt.wantBlocked && err != nil {
				t.Fatalf("admitAgentIssueCreate() error = %v, want nil", err)
			}
		})
	}
}
