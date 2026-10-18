package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/continuitycheckpoint"
	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workflowcontinuity"
	"github.com/multica-ai/multica/server/internal/workflowresume"
)

// TestSupervisorBaseContextBudgetSurvivesClaimWire exercises the two provider
// inputs from one claim-shaped payload. The narrower unit tests intentionally
// isolate Brain projection and runtime-brief layout; this regression joins
// them with Mika's actual compact product identity and a workspace-notes layer
// sized like the production sample that motivated the profile. The prompt
// ceiling covers platform-added base context; owner input and verified
// long-work continuity have separate additive budgets below.
func TestSupervisorBaseContextBudgetSurvivesClaimWire(t *testing.T) {
	const workspaceNotesBytes = 5265
	noteLine := "Route work through the current issue; preserve owner authority and require evidence.\n"
	workspaceNotes := strings.Repeat(noteLine, workspaceNotesBytes/len(noteLine)+1)[:workspaceNotesBytes]

	references := make([]map[string]any, 0, 45)
	for i := 0; i < 45; i++ {
		references = append(references, map[string]any{
			"id":       fmt.Sprintf("document-%03d", i),
			"revision": strings.Repeat(fmt.Sprintf("%x", i%16), 64),
			"state":    "reviewed",
			"excerpt":  strings.Repeat("company memory prose excluded from the compact index ", 6),
		})
	}
	brain, err := json.Marshal(map[string]any{
		"state":     "online",
		"project":   "multica",
		"lifecycle": "durable_source_bound",
		"data": map[string]any{
			"schemaVersion": "silvermediums.brain.v2",
			"references":    references,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	wire, err := json.Marshal(struct {
		Task *Task `json:"task"`
	}{Task: &Task{
		ID:                 "task-silv-101",
		IssueID:            "issue-silv-101",
		TaskContextProfile: supervisorCoordinationProfile,
		BrainContext:       brain,
		DevelopmentContext: json.RawMessage(`{"schema_version":1,"workstreams":[{"current_memory":[{"statement":"duplicate prose"}]}]}`),
		Agent: &AgentData{
			ID:           "mika-agent-id",
			Name:         "Mika",
			Instructions: service.ComposeMikaSupervisorInstructions("Mika", workspaceNotes),
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var claim struct {
		Task *Task `json:"task"`
	}
	if err := json.Unmarshal(wire, &claim); err != nil {
		t.Fatal(err)
	}
	if claim.Task == nil || claim.Task.Agent == nil {
		t.Fatal("claim wire lost task or agent")
	}

	prompt := BuildPrompt(*claim.Task, "codex")
	if len(prompt) > 8*1024 {
		t.Fatalf("supervisor prompt = %d bytes, want <= %d", len(prompt), 8*1024)
	}

	brief, err := execenv.InjectRuntimeConfig(t.TempDir(), "codex", execenv.TaskContextForEnv{
		ContextProfile:    claim.Task.TaskContextProfile,
		IssueID:           claim.Task.IssueID,
		AgentID:           claim.Task.Agent.ID,
		AgentName:         claim.Task.Agent.Name,
		AgentInstructions: claim.Task.Agent.Instructions,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(brief) > 12*1024 {
		t.Fatalf("supervisor runtime brief = %d bytes, want <= %d", len(brief), 12*1024)
	}

	t.Logf("claim-wire supervisor context: prompt=%d bytes, runtime_brief=%d bytes", len(prompt), len(brief))
}

func TestSupervisorLongWorkContextHasABoundedAdditiveBudget(t *testing.T) {
	const (
		taskID      = "11111111-1111-1111-1111-111111111111"
		workspaceID = "22222222-2222-2222-2222-222222222222"
		issueID     = "33333333-3333-3333-3333-333333333333"
	)
	dependencies := make([]map[string]any, 0, 80)
	for i := 0; i < 80; i++ {
		dependencies = append(dependencies, map[string]any{
			"id":         fmt.Sprintf("blocker-%03d", i),
			"revision":   i + 1,
			"actor_type": "external_provider",
			"actor_id":   fmt.Sprintf("provider-%03d", i),
			"wait_reason": map[string]any{
				"code":           "external_receipt",
				"reported_state": "pending",
			},
			"expected_event": map[string]any{
				"type":                   "provider_readback",
				"provider":               "railway",
				"work_id":                fmt.Sprintf("work-%03d", i),
				"after_receipt_revision": i + 1,
			},
			"checkpoint": map[string]any{"provider": "railway"},
		})
	}
	picture, err := json.Marshal(map[string]any{
		"contract_version": 2,
		"original_business_request": map[string]any{
			"available": true,
			"id":        "owner-request-1",
			"payload":   map[string]any{"goal": strings.Repeat("g", workflowcontinuity.MaxGoalTextBytes)},
		},
		"scope": map[string]any{
			"acceptance_version": 7,
			"statement":          map[string]any{"goal": strings.Repeat("s", workflowcontinuity.MaxGoalTextBytes)},
		},
		"outcome":                 map[string]any{"state": "open", "id": "outcome-1", "revision": 3, "acceptance_version": 7},
		"unresolved_dependencies": dependencies,
	})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := workflowcontinuity.Build(91, picture, nil)
	if err != nil {
		t.Fatal(err)
	}
	target := workflowresume.Target{TaskID: taskID, TaskAttempt: 2, WorkspaceID: workspaceID, IssueID: issueID}
	capsule, err := workflowresume.Build(target, workflowresume.Source{
		TaskID:            "44444444-4444-4444-4444-444444444444",
		ExecutionID:       "55555555-5555-5555-5555-555555555555",
		ExecutionRevision: 8,
		SnapshotSHA256:    strings.Repeat("a", 64),
	}, 91, json.RawMessage(`{"summary":"`+strings.Repeat("x", 1500)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	task := Task{
		ID:                    taskID,
		Attempt:               2,
		WorkspaceID:           workspaceID,
		IssueID:               issueID,
		TaskContextProfile:    supervisorCoordinationProfile,
		WorkflowContinuity:    ledger,
		WorkflowResumeCapsule: capsule,
		ContinuityCheckpointPlan: &ContinuityCheckpointPlan{
			SchemaVersion:      continuitycheckpoint.SchemaVersion,
			RootTaskID:         taskID,
			PackageStep:        2,
			MaxPackages:        continuitycheckpoint.MaxPackages,
			MaxCheckpointBytes: continuitycheckpoint.MaxCanonicalBytes,
		},
	}
	prompt := BuildPrompt(task, "codex")
	if len(prompt) > 16*1024 {
		t.Fatalf("long-work supervisor prompt = %d bytes, want <= %d", len(prompt), 16*1024)
	}
	for _, want := range []string{"Compact workflow continuity", `"blockers_omitted":`, "Verified fresh-session checkpoint", "Bounded continuity checkpoint"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("long-work supervisor prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, strings.Repeat("g", 128)) || strings.Contains(prompt, strings.Repeat("s", 128)) {
		t.Fatal("long-work supervisor prompt duplicated owner goal prose")
	}
	t.Logf("long-work supervisor prompt=%d bytes (ledger=%d, capsule=%d)", len(prompt), len(ledger), len(capsule))
}
