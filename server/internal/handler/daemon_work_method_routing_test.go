package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TestClaimTaskByRuntime_FiltersAutomaticMethodsAtTheServer pins the
// compatibility boundary: an old daemon that cannot render the new routing
// hint still receives exactly the selected built-in methods, not all six.
func TestClaimTaskByRuntime_FiltersAutomaticMethodsAtTheServer(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	tests := []struct {
		name         string
		fixture      string
		capabilities string
	}{
		{name: "old daemon inline skills", fixture: "methodrouteinline"},
		{name: "current daemon skill refs", fixture: "methodrouterefs", capabilities: protocol.DaemonCapabilitySkillBundlesV1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			runtimeID, taskID, _ := seedSkillLoadFixture(t, ctx, tc.fixture)
			var issueID string
			dbfx.QueryRow(t, `SELECT issue_id::text FROM agent_task_queue WHERE id = $1`, taskID).Scan(&issueID)
			dbfx.Exec(t, `UPDATE issue SET title = 'Audit and modernize PhoneFarm connectivity' WHERE id = $1`, issueID)

			req := newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, tc.fixture+"-daemon")
			if tc.capabilities != "" {
				req.Header.Set("X-Client-Capabilities", tc.capabilities)
			}
			req = withURLParam(req, "runtimeId", runtimeID)

			var response struct {
				Task struct {
					Agent *TaskAgentData `json:"agent"`
				} `json:"task"`
			}
			testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
			if response.Task.Agent == nil {
				t.Fatal("claim returned no agent payload")
			}

			names := map[string]bool{}
			for _, skill := range response.Task.Agent.Skills {
				names[skill.Name] = true
			}
			for _, ref := range response.Task.Agent.SkillRefs {
				names[ref.Name] = true
			}
			for _, want := range []string{
				service.PlatformSkillName,
				service.OutcomeOwnershipSkill,
				service.EvidenceFirstSkill,
				service.LongWorkSkill,
				service.ResearchREASkill,
			} {
				if !names[want] {
					t.Errorf("claim is missing selected method %q; names=%v", want, names)
				}
			}
			for _, reject := range []string{service.LearnAndHardenSkill, service.UIAcceptanceSkill} {
				if names[reject] {
					t.Errorf("claim included unrelated method %q; names=%v", reject, names)
				}
			}
		})
	}
}
