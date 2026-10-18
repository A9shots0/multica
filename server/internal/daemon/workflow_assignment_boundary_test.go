package daemon

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/workflowguard"
)

func TestLegacyWorkflowAssignmentPromptUsesOnlyBoundedPacket(t *testing.T) {
	packet := json.RawMessage(`{"contract_version":1,"task_kind":"implementation","assignment_packet":"EXACT_PACKET_ONLY","context_policy":{"load_issue":false,"load_comments":false,"load_repository_docs":false},"required_skills":[{"id":"skill-1","source":"workspace","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`)
	prompt := BuildPrompt(Task{
		IssueID: "issue-1", WorkflowPacket: packet,
		InitiatorName: "AMBIENT_INITIATOR", InitiatorEmail: "ambient@example.invalid",
		ConnectedApps: []ConnectedAppData{{Provider: "AMBIENT_CONNECTED_APP", ServerName: "ambient-server", ToolkitSlug: "ambient"}}, PriorSessionResumeUnavailable: true,
	}, "codex", WithSharedLocalDirectory(), WithWorktreeReplayConflicts([]string{"AMBIENT_CONFLICT"}))
	for _, forbidden := range []string{
		"multica issue get", "comment list", "ITEM-14-ROLE-BUILD-PROGRESS",
		"AMBIENT_INITIATOR", "ambient@example.invalid", "AMBIENT_CONNECTED_APP", "AMBIENT_CONFLICT",
		"Shared working directory", "Session continuity",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("bounded workflow prompt contains %q:\n%s", forbidden, prompt)
		}
	}
	if !strings.Contains(prompt, "EXACT_PACKET_ONLY") || !strings.Contains(prompt, "skill-1") {
		t.Fatalf("bounded workflow prompt omitted exact packet/skill:\n%s", prompt)
	}
}

func TestLegacyWorkflowAssignmentRuntimeBriefKeepsOnlyExactSkills(t *testing.T) {
	task := Task{WorkflowPacket: json.RawMessage(`{"contract_version":1}`)}
	ambient := execenv.TaskContextForEnv{
		IssueID: "AMBIENT_ISSUE", TriggerCommentID: "AMBIENT_COMMENT",
		ProjectTitle: "AMBIENT_PROJECT", InitiatorEmail: "ambient@example.invalid",
		ConnectedApps: []ConnectedAppData{{Provider: "AMBIENT_APP", ServerName: "AMBIENT_SERVER", ToolkitSlug: "ambient"}},
		AgentID:       "exact-agent", AgentName: "exact agent",
		AgentInstructions: "AMBIENT_AGENT_INSTRUCTIONS",
		AgentSkills:       []execenv.SkillContextForEnv{{Name: "EXACT_SKILL", Content: "EXACT_SKILL_CONTENT"}},
	}
	bounded := boundedWorkflowTaskContext(task, ambient)
	if bounded.AgentID != "exact-agent" || bounded.AgentName != "exact agent" ||
		len(bounded.AgentSkills) != 1 || bounded.AgentSkills[0].Content != "EXACT_SKILL_CONTENT" {
		t.Fatalf("bounded runtime context lost exact identity/skill: %#v", bounded)
	}
	raw, err := json.Marshal(bounded)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"AMBIENT_ISSUE", "AMBIENT_COMMENT", "AMBIENT_PROJECT", "ambient@example.invalid", "AMBIENT_APP", "AMBIENT_SERVER", "AMBIENT_AGENT_INSTRUCTIONS"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("bounded runtime context contains %q: %s", forbidden, raw)
		}
	}
	if got := boundedWorkflowTaskContext(Task{}, ambient); got.IssueID != ambient.IssueID || got.AgentInstructions != ambient.AgentInstructions {
		t.Fatalf("ordinary runtime context changed: %#v", got)
	}
}

func boundedAssignmentFixture() workflowguard.LegacyAssignmentV1 {
	return workflowguard.LegacyAssignmentV1{
		ContractVersion: workflowguard.LegacyAssignmentVersion,
		TaskKind:        "review", IssueID: "issue-1", IssueTitle: "issue",
		AcceptanceCriteria: json.RawMessage(`[]`), SourceTaskID: "source-1",
		WorkflowEventID: "event-1", WorkflowEventType: "implementation_ready", WorkflowEventReason: "ready",
		WorkflowEventPayload: json.RawMessage(`{}`), TargetAgentID: "agent-1", SquadID: "squad-1",
		RuntimeID: "runtime-1", RuntimeProvider: "opencode", Model: "fixture/model", ReasoningEffort: "medium",
		ExecutionPolicy: workflowguard.ExecutionPolicy{
			EnvironmentPolicy: workflowguard.LegacyEnvironmentPolicy,
			ToolPolicy:        workflowguard.LegacyToolPolicy, FreshTaskHome: true,
		},
		RequiredSkills: []workflowguard.RequiredSkill{{
			ID: "skill-1", Source: "workspace",
			SHA256:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SizeBytes: 7, FileCount: 0,
		}},
	}
}

func boundedTaskFixture() Task {
	assignment := boundedAssignmentFixture()
	packet, _ := json.Marshal(assignment)
	return Task{
		ID: "task-1", AgentID: assignment.TargetAgentID, RuntimeID: assignment.RuntimeID,
		IssueID: assignment.IssueID, WorkspaceID: "workspace-1", SquadID: assignment.SquadID,
		WorkflowPacket: packet, LeaderRoleResolved: true,
		Agent: &AgentData{
			ID: assignment.TargetAgentID, Name: "fixture", Model: assignment.Model, ThinkingLevel: assignment.ReasoningEffort,
			SkillRefs: []SkillRefData{{
				ID: "skill-1", Source: "workspace",
				Hash:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				SizeBytes: 7, FileCount: 0,
			}},
		},
	}
}

func TestBoundedWorkflowClaimExposureRejectsAmbientOrAlteredAuthority(t *testing.T) {
	assignment := boundedAssignmentFixture()
	if err := validateBoundedWorkflowClaimExposure(boundedTaskFixture(), "opencode", assignment); err != nil {
		t.Fatalf("exact bounded claim rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Task)
	}{
		{"model", func(task *Task) { task.Agent.Model = "altered/model" }},
		{"provider", func(task *Task) { assignment.RuntimeProvider = "altered" }},
		{"skill", func(task *Task) {
			task.Agent.SkillRefs[0].Hash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{"repository", func(task *Task) { task.Repos = []RepoData{{URL: "https://example.invalid/repo"}} }},
		{"custom environment", func(task *Task) { task.Agent.CustomEnv = map[string]string{"SECRET": "value"} }},
		{"prior session", func(task *Task) { task.PriorSessionID = "ambient-session" }},
		{"workspace context", func(task *Task) { task.WorkspaceContext = "ambient" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boundedTaskFixture()
			want := assignment
			if tt.name == "provider" {
				want.RuntimeProvider = "altered"
			} else {
				tt.mutate(&got)
			}
			if err := validateBoundedWorkflowClaimExposure(got, "opencode", want); err == nil {
				t.Fatal("altered or ambient claim was accepted")
			}
		})
	}
}

func TestBoundedWorkflowResolvedSkillsAndPromptAreExact(t *testing.T) {
	assignment := boundedAssignmentFixture()
	task := boundedTaskFixture()
	task.Agent.Skills = []SkillData{{
		ID: "skill-1", Source: "workspace", Hash: assignment.RequiredSkills[0].SHA256,
		SizeBytes: 7, Content: "EXACT_SKILL_CONTENT",
	}}
	if err := validateBoundedWorkflowResolvedSkills(task, assignment); err != nil {
		t.Fatalf("exact skill payload rejected: %v", err)
	}
	prompt, err := appendBoundedWorkflowSkillPayload("EXACT_PACKET", task.Agent.Skills)
	if err != nil || !strings.Contains(prompt, "EXACT_PACKET") || !strings.Contains(prompt, "EXACT_SKILL_CONTENT") {
		t.Fatalf("exact prompt payload = %q, err %v", prompt, err)
	}
	task.Agent.Skills[0].Hash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := validateBoundedWorkflowResolvedSkills(task, assignment); err == nil {
		t.Fatal("altered resolved skill payload was accepted")
	}
}

func TestBoundedWorkflowChildEnvironmentDoesNotInheritParentCredentialsOrTools(t *testing.T) {
	// This models a Windows child even when the test runner is Unix.
	t.Setenv("SYSTEMROOT", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "parent-secret")
	t.Setenv("MULTICA_TOKEN", "parent-token")
	t.Setenv("OPENROUTER_API_KEY", "parent-router-secret")
	root := t.TempDir()
	executable := filepath.Join(root, "provider.exe")
	env, keys, err := buildGuardedChildEnvironment("opencode", "windows", executable, root, "fixture/model", "medium")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ANTHROPIC_API_KEY", "MULTICA_TOKEN", "OPENROUTER_API_KEY"} {
		if _, present := env[forbidden]; present {
			t.Fatalf("parent credential %s entered child environment", forbidden)
		}
	}
	if env["PATH"] != filepath.Dir(executable) || !strings.HasPrefix(env["HOME"], root) || len(env) != len(keys) {
		t.Fatalf("child environment is not exact/task-local: env=%#v keys=%#v", env, keys)
	}
}

func TestBoundedWorkflowBrokerEnvironmentKeepsOnlyTaskCapability(t *testing.T) {
	t.Setenv("SYSTEMROOT", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "parent-secret")
	t.Setenv("OPENROUTER_API_KEY", "parent-router-secret")
	root := t.TempDir()
	executable := filepath.Join(root, "broker.exe")
	provider := filepath.Join(root, "provider.exe")
	task := boundedTaskFixture()
	task.AuthToken = "mat_exact_task"
	authority := &agentqualification.GuardedWorkflowBrokerV1{
		AgentName: "temporary-reviewer", TaskKind: "review",
		ProviderExecutableSHA256: strings.Repeat("a", 64),
		ProviderProfileKind:      agentqualification.GuardedWorkflowBrokerProfileFixtureV1,
	}
	env, keys, err := buildBoundedWorkflowBrokerEnvironment(
		"claude", "windows", executable, provider, root, "claude-sonnet-4-6", "medium",
		task, authority, "http://127.0.0.1:18081",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ANTHROPIC_API_KEY", "OPENROUTER_API_KEY", "CODEX_HOME"} {
		if _, present := env[forbidden]; present {
			t.Fatalf("ambient value %s entered broker environment", forbidden)
		}
	}
	if env["MULTICA_TOKEN"] != task.AuthToken || env["MULTICA_TASK_ID"] != task.ID ||
		env["MULTICA_WORKFLOW_TASK_KIND"] != "review" ||
		env["ITEM14_PROVIDER_CHILD_PATH"] != provider || env["ITEM14_PROVIDER_CHILD_SHA256"] != strings.Repeat("a", 64) ||
		env["MULTICA_ISSUE_ID"] != task.IssueID || env["MULTICA_SERVER_URL"] != "http://127.0.0.1:18081" {
		t.Fatalf("broker task capability mismatch: %#v", env)
	}
	if len(env) != len(keys) {
		t.Fatalf("broker environment/key counts = %d/%d", len(env), len(keys))
	}

	task.AuthToken = "owner-token"
	if _, _, err := buildBoundedWorkflowBrokerEnvironment("claude", "windows", executable, provider, root, "claude-sonnet-4-6", "medium", task, authority, "http://127.0.0.1:18081"); err == nil {
		t.Fatal("non-task token was accepted by broker environment")
	}
}
