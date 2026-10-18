package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestTaskContextProfileForAgentUsesProtectedIdentityAndOrdinaryIssueShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agent     db.Agent
		task      db.AgentTaskQueue
		supported bool
		unguarded bool
		want      string
	}{
		{
			name:      "ordinary Mika issue",
			agent:     db.Agent{SystemKey: pgtype.Text{String: service.MikaSystemKey, Valid: true}},
			task:      db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}},
			supported: true,
			unguarded: true,
			want:      TaskContextProfileSupervisorCoordination,
		},
		{
			name:  "missing system key defaults full",
			agent: db.Agent{Name: "Mika"}, task: db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}}, supported: true, unguarded: true,
			want: TaskContextProfileImplementationFull,
		},
		{
			name: "display name and instructions cannot impersonate Mika",
			agent: db.Agent{
				Name:         "Mika",
				Instructions: "Act as the supervisor coordinator.",
				SystemKey:    pgtype.Text{String: "", Valid: true},
			},
			task: db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}}, supported: true, unguarded: true,
			want: TaskContextProfileImplementationFull,
		},
		{
			name:  "unknown system key defaults full",
			agent: db.Agent{SystemKey: pgtype.Text{String: "future_system_agent", Valid: true}}, task: db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}}, supported: true, unguarded: true,
			want: TaskContextProfileImplementationFull,
		},
		{
			name:  "Mika chat stays full",
			agent: db.Agent{SystemKey: pgtype.Text{String: service.MikaSystemKey, Valid: true}},
			task:  db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}, ChatSessionID: pgtype.UUID{Valid: true}}, supported: true, unguarded: true,
			want: TaskContextProfileImplementationFull,
		},
		{
			name:  "Mika autopilot stays full",
			agent: db.Agent{SystemKey: pgtype.Text{String: service.MikaSystemKey, Valid: true}},
			task:  db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}, AutopilotRunID: pgtype.UUID{Valid: true}}, supported: true, unguarded: true,
			want: TaskContextProfileImplementationFull,
		},
		{
			name:  "Mika guarded issue stays full",
			agent: db.Agent{SystemKey: pgtype.Text{String: service.MikaSystemKey, Valid: true}},
			task:  db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}}, supported: true, unguarded: false,
			want: TaskContextProfileImplementationFull,
		},
		{
			name:  "Mika non-issue stays full",
			agent: db.Agent{SystemKey: pgtype.Text{String: service.MikaSystemKey, Valid: true}}, supported: true, unguarded: true,
			want: TaskContextProfileImplementationFull,
		},
		{
			name:      "Mika issue on old daemon stays full",
			agent:     db.Agent{SystemKey: pgtype.Text{String: service.MikaSystemKey, Valid: true}},
			task:      db.AgentTaskQueue{IssueID: pgtype.UUID{Valid: true}},
			supported: false,
			unguarded: true,
			want:      TaskContextProfileImplementationFull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := taskContextProfileForAgent(tt.agent, tt.task, tt.supported, tt.unguarded); got != tt.want {
				t.Fatalf("taskContextProfileForAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskContextProfileIsAlwaysPresentOnClaimWire(t *testing.T) {
	t.Parallel()

	for _, profile := range []string{
		TaskContextProfileImplementationFull,
		TaskContextProfileSupervisorCoordination,
	} {
		raw, err := json.Marshal(AgentTaskResponse{TaskContextProfile: profile})
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		if got := wire["task_context_profile"]; got != profile {
			t.Fatalf("task_context_profile = %#v, want %q (wire: %s)", got, profile, raw)
		}
	}
}

func TestClaimTaskByRuntimeDerivesContextProfileFromStoredAgentIdentity(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	tests := []struct {
		name      string
		agentName string
		systemKey *string
		want      string
	}{
		{
			name:      "Mika protected identity receives supervisor profile",
			agentName: "Renamed Coordinator",
			systemKey: stringPointer(service.MikaSystemKey),
			want:      TaskContextProfileSupervisorCoordination,
		},
		{
			name:      "Mika display name cannot select supervisor profile",
			agentName: "Mika",
			want:      TaskContextProfileImplementationFull,
		},
		{
			name:      "unknown protected identity defaults full",
			agentName: "Future System Agent",
			systemKey: stringPointer("future_system_agent"),
			want:      TaskContextProfileImplementationFull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runtimeID := createClaimReclaimRuntime(t, ctx, "context profile "+tt.name)
			agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, tt.agentName)
			if tt.systemKey != nil {
				dbfx.Exec(t, `UPDATE agent SET system_key = $1 WHERE id = $2`, *tt.systemKey, agentID)
			}
			seedQueuedIssueTask(t, ctx, agentID, runtimeID, issueID)

			req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil,
				testWorkspaceID, "context-profile-claim")
			req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilitySupervisorContextV1)
			req = withURLParam(req, "runtimeId", runtimeID)
			var response struct {
				Task *AgentTaskResponse `json:"task"`
			}
			testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
			if response.Task == nil {
				t.Fatal("claim returned no task")
			}
			if got := response.Task.TaskContextProfile; got != tt.want {
				t.Fatalf("task_context_profile = %q, want %q", got, tt.want)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }
