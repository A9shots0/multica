package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// This file pins Change 2 of the agent-authorization-guardrails audit:
// project / squad / label / skill / runtime deletion are irreversible,
// workspace-role-gated operations (owner/admin, or — for runtimes — the
// owning member) but none of them checked WHO is making the call: a
// dispatched agent task holding its owner's mat_ task token could call any
// of them as long as the owning human happened to hold owner/admin.
// RequireHumanActor (actor_guards.go) already exists and is wired at
// router.go for billing/usage-style routes; these deletes get the same
// treatment, both at the router (r.With(handler.RequireHumanActor)) and as
// an in-handler backstop (isMachineCredentialActor), matching the pattern
// already used by PostIssueOperatorCompletion / PostIssueOwnerDirective /
// PostIssueWorkflowOwnerDecision. The backstop is what these tests exercise
// directly, since internal/handler tests call handler methods directly and
// cannot go through cmd/server's chi router (that package imports this one,
// not the other way around) — router.go wiring is inspected separately,
// not re-derived here.
//
// autopilot delete is a sixth candidate the audit named but is deliberately
// excluded here — see the comment where its table entry used to be, just
// above the "skill" case below.
//
// Each case proves two things: a task-token request (the owning human is
// workspace owner, so the pre-existing role check alone would have allowed
// it) is rejected with 403, and an ordinary human-session request from that
// same owner still succeeds and actually deletes the row.

// deleteRouteCase describes one destructive route under test.
type deleteRouteCase struct {
	name string
	// seed creates a fresh entity to delete and returns (id, request-builder).
	// request(asTaskToken) returns a *http.Request ready to hand to handle.
	seed   func(t *testing.T) (id string, request func(asTaskToken bool) *http.Request)
	handle func(http.ResponseWriter, *http.Request)
	// stillExists reports whether the entity with the given id is still
	// present (soft or hard) after a call, for the "task token changed
	// nothing" assertion.
	stillExists func(t *testing.T, id string) bool
}

// taskTokenHeaders stamps the machine-credential shape onto req: a running
// task owned by ownerUserID. Mirrors the header set used throughout this
// package's other actor-source tests (e.g. workflow_chat_operational_guard_test.go).
func taskTokenHeaders(req *http.Request, taskID, agentID, ownerUserID string) *http.Request {
	req.Header.Set("X-Actor-Source", "task_token")
	req.Header.Set("X-Task-ID", taskID)
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-User-ID", ownerUserID)
	return req
}

func TestDestructiveRoutesRejectTaskTokenButAllowHumanOwner(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	cases := []deleteRouteCase{
		{
			name: "project",
			seed: func(t *testing.T) (string, func(bool) *http.Request) {
				projectID := dbfx.Project(t, "guardrail delete target project")
				agentID := createHandlerTestAgent(t, "project delete guard agent", nil)
				taskID := dbfx.Task(t, agentID, testutil.Cols{
					"runtime_id": handlerTestRuntimeID(t),
					"status":     "running",
					"started_at": testutil.Raw("now()"),
				})
				return projectID, func(asTaskToken bool) *http.Request {
					req := withURLParam(newRequest(http.MethodDelete, "/api/projects/"+projectID, nil), "id", projectID)
					if asTaskToken {
						req = taskTokenHeaders(req, taskID, agentID, testUserID)
					}
					return req
				}
			},
			handle: testHandler.DeleteProject,
			stillExists: func(t *testing.T, id string) bool {
				return dbfx.Count(t, `SELECT count(*) FROM project WHERE id = $1`, id) == 1
			},
		},
		{
			name: "squad",
			seed: func(t *testing.T) (string, func(bool) *http.Request) {
				leaderAgentID := createHandlerTestAgent(t, "squad delete guard leader", nil)
				squad := createSquadAs(t, "", "guardrail delete target squad", leaderAgentID)
				agentID := createHandlerTestAgent(t, "squad delete guard agent", nil)
				taskID := dbfx.Task(t, agentID, testutil.Cols{
					"runtime_id": handlerTestRuntimeID(t),
					"status":     "running",
					"started_at": testutil.Raw("now()"),
				})
				return squad.ID, func(asTaskToken bool) *http.Request {
					req := squadScopeReq("", http.MethodDelete, "/api/squads", nil, map[string]string{"id": squad.ID})
					if asTaskToken {
						req = taskTokenHeaders(req, taskID, agentID, testUserID)
					}
					return req
				}
			},
			handle: testHandler.DeleteSquad,
			stillExists: func(t *testing.T, id string) bool {
				return dbfx.Count(t, `SELECT count(*) FROM squad WHERE id = $1 AND archived_at IS NULL`, id) == 1
			},
		},
		{
			name: "label",
			seed: func(t *testing.T) (string, func(bool) *http.Request) {
				w := httptest.NewRecorder()
				req := newRequest(http.MethodPost, "/api/labels", map[string]any{"name": "guardrail-delete-label-" + uuid.NewString()[:8], "color": "#123456"})
				testHandler.CreateLabel(w, req)
				if w.Code != http.StatusCreated {
					t.Fatalf("seed CreateLabel: expected 201, got %d: %s", w.Code, w.Body.String())
				}
				var created LabelResponse
				if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
					t.Fatal(err)
				}
				agentID := createHandlerTestAgent(t, "label delete guard agent", nil)
				taskID := dbfx.Task(t, agentID, testutil.Cols{
					"runtime_id": handlerTestRuntimeID(t),
					"status":     "running",
					"started_at": testutil.Raw("now()"),
				})
				return created.ID, func(asTaskToken bool) *http.Request {
					req := withURLParam(newRequest(http.MethodDelete, "/api/labels/"+created.ID, nil), "id", created.ID)
					if asTaskToken {
						req = taskTokenHeaders(req, taskID, agentID, testUserID)
					}
					return req
				}
			},
			handle: testHandler.DeleteLabel,
			stillExists: func(t *testing.T, id string) bool {
				return dbfx.Count(t, `SELECT count(*) FROM issue_label WHERE id = $1`, id) == 1
			},
		},
		// NOTE: autopilot delete is deliberately NOT in this table. Unlike the
		// other five, DeleteAutopilot already judges the acting human via
		// requireAutopilotWrite (MUL-7108) instead of a blanket role check —
		// it legitimately allows an agent task token when the run's originator
		// has write access, so a blanket RequireHumanActor gate would break
		// intended behavior (confirmed by running the full suite: it regressed
		// TestAutopilotWrites_JudgeTheActingHuman in
		// autopilot_acting_member_test.go). See the comment on DeleteAutopilot.
		{
			name: "skill",
			seed: func(t *testing.T) (string, func(bool) *http.Request) {
				w := httptest.NewRecorder()
				req := newRequest(http.MethodPost, "/api/skills", map[string]any{"name": "guardrail-delete-skill-" + uuid.NewString()[:8]})
				testHandler.CreateSkill(w, req)
				if w.Code != http.StatusCreated {
					t.Fatalf("seed CreateSkill: expected 201, got %d: %s", w.Code, w.Body.String())
				}
				var created SkillResponse
				if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
					t.Fatal(err)
				}
				agentID := createHandlerTestAgent(t, "skill delete guard agent", nil)
				taskID := dbfx.Task(t, agentID, testutil.Cols{
					"runtime_id": handlerTestRuntimeID(t),
					"status":     "running",
					"started_at": testutil.Raw("now()"),
				})
				return created.ID, func(asTaskToken bool) *http.Request {
					req := withURLParam(newRequest(http.MethodDelete, "/api/skills/"+created.ID, nil), "id", created.ID)
					if asTaskToken {
						req = taskTokenHeaders(req, taskID, agentID, testUserID)
					}
					return req
				}
			},
			handle: testHandler.DeleteSkill,
			stillExists: func(t *testing.T, id string) bool {
				return dbfx.Count(t, `SELECT count(*) FROM skill WHERE id = $1`, id) == 1
			},
		},
		{
			name: "runtime",
			seed: func(t *testing.T) (string, func(bool) *http.Request) {
				runtimeID := dbfx.Runtime(t, "guardrail delete target runtime")
				agentID := createHandlerTestAgent(t, "runtime delete guard agent", nil)
				taskID := dbfx.Task(t, agentID, testutil.Cols{
					"runtime_id": handlerTestRuntimeID(t),
					"status":     "running",
					"started_at": testutil.Raw("now()"),
				})
				return runtimeID, func(asTaskToken bool) *http.Request {
					req := withURLParam(newRequest(http.MethodDelete, "/api/runtimes/"+runtimeID, nil), "runtimeId", runtimeID)
					if asTaskToken {
						req = taskTokenHeaders(req, taskID, agentID, testUserID)
					}
					return req
				}
			},
			handle: testHandler.DeleteAgentRuntime,
			stillExists: func(t *testing.T, id string) bool {
				return dbfx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id = $1`, id) == 1
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/task_token_rejected", func(t *testing.T) {
			id, request := tc.seed(t)
			w := httptest.NewRecorder()
			tc.handle(w, request(true))
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s delete via task token: expected 403, got %d: %s", tc.name, w.Code, w.Body.String())
			}
			if !tc.stillExists(t, id) {
				t.Fatalf("%s delete via task token: entity %s was deleted despite 403", tc.name, id)
			}
		})

		t.Run(tc.name+"/human_owner_allowed", func(t *testing.T) {
			id, request := tc.seed(t)
			w := httptest.NewRecorder()
			tc.handle(w, request(false))
			if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
				t.Fatalf("%s delete as human owner: expected 200/204, got %d: %s", tc.name, w.Code, w.Body.String())
			}
			if tc.stillExists(t, id) {
				t.Fatalf("%s delete as human owner: entity %s still present after a successful delete", tc.name, id)
			}
		})
	}
}

// TestUnbindAgentsAndDeleteRuntimeRejectsTaskToken pins the confirmed-delete
// runtime variant separately: it shares DeleteAgentRuntime's irreversible
// outcome (the runtime row is gone at the end) but takes a POST body instead
// of a bare DELETE, so it needs its own request shape.
func TestUnbindAgentsAndDeleteRuntimeRejectsTaskToken(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "guardrail unbind-delete target runtime")
	agentID := createHandlerTestAgent(t, "unbind-delete guard agent", nil)
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": handlerTestRuntimeID(t),
		"status":     "running",
		"started_at": testutil.Raw("now()"),
	})

	req := withURLParam(newRequest(http.MethodPost, "/api/runtimes/"+runtimeID+"/unbind-agents-and-delete", map[string]any{}), "runtimeId", runtimeID)
	req = taskTokenHeaders(req, taskID, agentID, testUserID)
	w := httptest.NewRecorder()
	testHandler.UnbindAgentsAndDeleteRuntime(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unbind-agents-and-delete via task token: expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id = $1`, runtimeID) != 1 {
		t.Fatal("unbind-agents-and-delete via task token: runtime was deleted despite 403")
	}

	// Human owner: the runtime has no bound agents, so plain DELETE is the
	// documented path and this confirmed variant is exercised elsewhere
	// (runtime_unbind_delete_test.go) for its full unbind side effects; here
	// we only need to confirm the gate does not block a legitimate human
	// call in the same shape.
	humanReq := withURLParam(newRequest(http.MethodPost, "/api/runtimes/"+runtimeID+"/unbind-agents-and-delete", map[string]any{}), "runtimeId", runtimeID)
	w = httptest.NewRecorder()
	testHandler.UnbindAgentsAndDeleteRuntime(w, humanReq)
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("unbind-agents-and-delete as human owner: expected 200/204, got %d: %s", w.Code, w.Body.String())
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id = $1`, runtimeID) != 0 {
		t.Fatal("unbind-agents-and-delete as human owner: runtime still present after a successful delete")
	}
}
