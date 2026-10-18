package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// deployHandoffNote is the prompt handed to the dispatched agent when the
// owner triggers a real deploy. It deliberately does not hand the agent a
// generic "run docker" recipe — every project in this workspace deploys
// differently (some Railway, some local Docker Compose, some Mac-hosted),
// and guessing wrong on a live service is the one mistake this endpoint
// exists to prevent. The agent is told exactly where the real, per-project
// process is documented and told to stop and report back rather than
// improvise when it isn't.
//
// batchLabels lists every issue this one deploy covers (identifier + title),
// always including the dispatched issue itself. A project deploys as one
// unit — rebuild and ship whatever is currently merged — so shipping N
// ready issues on the same project is ONE redeploy, not N repeated ones;
// the agent is told the full scope so its verification and changelog entry
// account for everything, not just the issue the task happens to be
// attached to.
func deployHandoffNote(projectTitle string, batchLabels []string) string {
	target := "this issue's project"
	if projectTitle != "" {
		target = fmt.Sprintf("%q", projectTitle)
	}
	scope := "This deploy covers this one issue."
	if len(batchLabels) > 1 {
		scope = fmt.Sprintf(
			"This is ONE deploy covering %d issues together, not %d separate deploys — %s. Ship them as a single rebuild/redeploy of %s's current state; do not repeat the deploy process once per issue.",
			len(batchLabels), len(batchLabels), strings.Join(batchLabels, "; "), target,
		)
	}
	return fmt.Sprintf(`[MULTICA_DEPLOY_TASK] The owner approved deploying to production, from inside Multica — this is a real deploy request, not a drill.

%s

Before touching anything live:
1. Look up %s's documented deploy process in the knowledge hub (C:\Dev\Silver-Mediums-Knowledge\projects\<project-id>\CONTEXT.md, "Deployment/runtime" section, and its CHANGELOG.md for the most recent deploy).
2. If no deploy process is documented, or you are not confident what "deploy" means for this project, STOP here. Do not guess at production commands. Report back on this issue explaining what's missing and what you'd need to proceed.
3. Check what's actually running right now BEFORE assuming a deploy is needed: compare the live service's current commit/image/version against what this issue's change requires. The owner cannot know whether something was already shipped by an earlier session — that judgment is yours to make with real access, not something to ask him to guess. If it's already live and matches, don't redeploy: report back on this issue that it's already up to date and confirm how you verified that.
4. If a real deploy is needed: run the project's real test/build suite and confirm it's green. Do not deploy on top of a red or unverified build.
5. Note what's currently live right now (the running image/commit/version) as your rollback target BEFORE you overwrite it, so a bad deploy can be reversed quickly if regressions show up.
6. Deploy following the documented process exactly (e.g. docker rm -f + docker run/compose up — never docker restart, it does not pick up a rebuilt image), then verify the service is actually healthy afterward with a real check, not an assumption.
7. After a verified deploy, update the knowledge hub's CHANGELOG.md for that project with what was deployed (all issues in this batch), when, and the rollback target from step 5, per this repo's own write-back convention. Never report "deployed" without having confirmed it live. If anything looks wrong post-deploy, roll back to the noted target immediately rather than trying to forward-fix under pressure.`, scope, target)
}

// triggerDeployRequest optionally batches several ready issues on the SAME
// project into one deploy. additional_issue_ids are issues besides the one
// in the URL — a project deploys as a whole (rebuild + ship current state),
// so shipping 5 ready issues one at a time would be 5 redundant rebuilds of
// the identical thing. One real agent task is dispatched, attached to the
// URL issue; the other issues in the batch are recorded (MarkIssueDeployed)
// by the client only after that one task actually completes — see
// GetIssueDeployTaskStatus. Nothing here marks them deployed itself.
type triggerDeployRequest struct {
	AdditionalIssueIDs []string `json:"additional_issue_ids,omitempty"`
}

func decodeTriggerDeployRequest(r *http.Request) (triggerDeployRequest, error) {
	var req triggerDeployRequest
	if r.Body == nil {
		return req, nil
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return req, err
	}
	return req, nil
}

// TriggerIssueDeploy dispatches a real agent task to deploy this issue's
// changes (and, for a batch, every other issue passed in
// additional_issue_ids). It reuses the exact same enqueue path every
// "assign/promote" action in the app already goes through
// (EnqueueTaskForIssueWithHandoff) — deploying is not a separate, unproven
// pipeline, it is the ordinary "give an agent a real task with real shell
// access" mechanism, carrying a deploy-specific handoff note instead of an
// implementation one.
//
// Route-level gated to the workspace owner role (see router.go) — this is
// the one Multica action whose whole purpose is triggering a real,
// production-affecting side effect, so it does not accept a broader
// "member" actor the way most issue actions do.
func (h *Handler) TriggerIssueDeploy(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if !issue.AssigneeID.Valid {
		writeError(w, http.StatusBadRequest, "assign this issue to an agent before deploying it")
		return
	}

	req, err := decodeTriggerDeployRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
	batchLabels := []string{fmt.Sprintf("%s (%s)", service.IssueIdentifier(prefix, issue.Number), issue.Title)}
	batchIssueIDs := []string{uuidToString(issue.ID)}

	for _, rawID := range req.AdditionalIssueIDs {
		otherID, parseErr := util.ParseUUID(rawID)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid additional_issue_ids entry")
			return
		}
		other, loadErr := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
			ID:          otherID,
			WorkspaceID: issue.WorkspaceID,
		})
		if loadErr != nil {
			writeError(w, http.StatusBadRequest, "an additional issue in this batch was not found")
			return
		}
		// A batched deploy is one project's redeploy covering several of its
		// issues — issues from a different (or no) project have no business
		// riding along on someone else's rebuild.
		if !issue.ProjectID.Valid || !other.ProjectID.Valid || other.ProjectID != issue.ProjectID {
			writeError(w, http.StatusBadRequest, "every issue in a batched deploy must share the same project")
			return
		}
		batchLabels = append(batchLabels, fmt.Sprintf("%s (%s)", service.IssueIdentifier(prefix, other.Number), other.Title))
		batchIssueIDs = append(batchIssueIDs, uuidToString(other.ID))
	}

	projectTitle := ""
	if issue.ProjectID.Valid {
		if project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID:          issue.ProjectID,
			WorkspaceID: issue.WorkspaceID,
		}); err == nil {
			projectTitle = project.Title
		}
	}

	actorUserID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	task, err := h.TaskService.EnqueueTaskForIssueWithHandoff(r.Context(), issue, deployHandoffNote(projectTitle, batchLabels), parseUUID(actorUserID))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// idx_one_pending_task_per_issue_agent_thread: the assigned agent
			// already has a queued/dispatched task on this issue — a real state,
			// not a bug, e.g. the owner deployed twice or the agent is still
			// mid-run. Say so plainly instead of a generic failure.
			writeError(w, http.StatusConflict, "this agent already has an active task on this issue — wait for it to finish before deploying")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not start the deploy task")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id":         uuidToString(task.ID),
		"issue_id":        uuidToString(issue.ID),
		"assignee_id":     uuidToString(issue.AssigneeID),
		"batch_issue_ids": batchIssueIDs,
	})
}

// manualDeployNote records that the owner is asserting this was already
// deployed outside Multica (e.g. by a terminal session, another agent, or by
// hand) — distinct from an agent-dispatched deploy, and honest about that in
// the text itself. Shares the [MULTICA_DEPLOY_TASK] sentinel so it's picked
// up by ListRecentDeployTasks like any other deploy record.
func manualDeployNote() string {
	return "[MULTICA_DEPLOY_TASK] Recorded manually by the owner as already deployed outside Multica — no agent task ran for this deploy."
}

// MarkIssueDeployed records a deploy that already happened outside
// Multica's own dispatch mechanism, without starting any agent work. Exists
// because most of tonight's real deploys (including Multica's own) were run
// by hand in a terminal, not through TriggerIssueDeploy — Multica's deploy
// history has no way to know about those unless the owner tells it. Without
// this, clicking "Approve & Deploy" on an issue that's already live would
// dispatch a real, unnecessary agent task with nothing to actually do.
func (h *Handler) MarkIssueDeployed(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if !issue.AssigneeID.Valid {
		writeError(w, http.StatusBadRequest, "assign this issue to an agent before recording it as deployed")
		return
	}
	agent, err := h.Queries.GetAgent(r.Context(), issue.AssigneeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not resolve this issue's assigned agent")
		return
	}

	var initiatorID pgtype.UUID
	if actorUserID, ok := requireUserID(w, r); ok {
		initiatorID = parseUUID(actorUserID)
	} else {
		return
	}

	taskID, err := h.Queries.CreateManualDeploymentRecord(r.Context(), db.CreateManualDeploymentRecordParams{
		AgentID:         issue.AssigneeID,
		RuntimeID:       agent.RuntimeID,
		IssueID:         issue.ID,
		HandoffNote:     pgtype.Text{String: manualDeployNote(), Valid: true},
		InitiatorUserID: initiatorID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not record this deploy")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id":     uuidToString(taskID),
		"issue_id":    uuidToString(issue.ID),
		"assignee_id": uuidToString(issue.AssigneeID),
	})
}

// deployTaskStatusResponse is deliberately a small, stable subset of
// AgentTaskQueue — just enough for the Deployments page to render a real
// progress state (queued → dispatched → running → completed/failed/
// cancelled) instead of a static "Deploy started" that never updates.
type deployTaskStatusResponse struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

// GetIssueDeployTaskStatus lets the Deployments page poll a dispatched
// deploy task's real status. Scoped through the issue (not a bare
// /api/tasks/{id} lookup) so workspace membership is enforced the same way
// every other issue-scoped read already is.
func (h *Handler) GetIssueDeployTaskStatus(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	taskUUID, err := util.ParseUUID(chi.URLParam(r, "taskId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	task, err := h.Queries.GetAgentTask(r.Context(), taskUUID)
	if err != nil || task.IssueID != issue.ID {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	resp := deployTaskStatusResponse{TaskID: uuidToString(task.ID), Status: task.Status}
	if task.StartedAt.Valid {
		resp.StartedAt = task.StartedAt.Time.UTC().Format(time.RFC3339)
	}
	if task.CompletedAt.Valid {
		resp.CompletedAt = task.CompletedAt.Time.UTC().Format(time.RFC3339)
	}
	if task.Error.Valid {
		resp.Error = task.Error.String
	} else if task.FailureReason.Valid {
		resp.Error = task.FailureReason.String
	}
	writeJSON(w, http.StatusOK, resp)
}

const deploymentHistoryLimit = 100

type deploymentHistoryEntry struct {
	TaskID          string `json:"task_id"`
	IssueID         string `json:"issue_id"`
	IssueIdentifier string `json:"issue_identifier"`
	IssueTitle      string `json:"issue_title"`
	ProjectID       string `json:"project_id,omitempty"`
	ProjectTitle    string `json:"project_title,omitempty"`
	Status          string `json:"status"`
	CompletedAt     string `json:"completed_at,omitempty"`
	Error           string `json:"error,omitempty"`
}

type currentlyLiveProject struct {
	ProjectID       string `json:"project_id"`
	ProjectTitle    string `json:"project_title"`
	IssueID         string `json:"issue_id"`
	IssueIdentifier string `json:"issue_identifier"`
	IssueTitle      string `json:"issue_title"`
	DeployedAt      string `json:"deployed_at"`
}

type deploymentHistoryResponse struct {
	CurrentlyLive []currentlyLiveProject   `json:"currently_live"`
	History       []deploymentHistoryEntry `json:"history"`
}

// GetDeploymentHistory answers "what's actually live on each of my
// projects right now" and "what deployed recently" — the gap the owner
// called out: Multica dispatches real deploys (TriggerIssueDeploy) but had
// nowhere to show what any of them actually did afterward. Built off the
// same [MULTICA_DEPLOY_TASK]-tagged tasks the deploy trigger creates, no
// separate release ledger to keep in sync.
//
// "Currently live" per project is the most recent COMPLETED deploy task for
// that project — the query is already ordered newest-first, so the first
// completed row seen per project_id is that project's live state. A project
// with no completed deploy task simply has no entry: this never guesses.
func (h *Handler) GetDeploymentHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireUserID(w, r); !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	wsUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace")
		return
	}

	rows, err := h.Queries.ListRecentDeployTasks(r.Context(), db.ListRecentDeployTasksParams{
		WorkspaceID: wsUUID,
		Limit:       deploymentHistoryLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load deployment history")
		return
	}

	prefix := h.getIssuePrefix(r.Context(), wsUUID)
	history := make([]deploymentHistoryEntry, 0, len(rows))
	currentlyLive := make([]currentlyLiveProject, 0)
	seenLiveProject := map[string]bool{}

	for _, row := range rows {
		identifier := service.IssueIdentifier(prefix, row.IssueNumber)
		entry := deploymentHistoryEntry{
			TaskID:          uuidToString(row.TaskID),
			IssueID:         uuidToString(row.IssueID),
			IssueIdentifier: identifier,
			IssueTitle:      row.IssueTitle,
			Status:          row.Status,
		}
		if row.ProjectID.Valid {
			entry.ProjectID = uuidToString(row.ProjectID)
			entry.ProjectTitle = row.ProjectTitle.String
		}
		if row.CompletedAt.Valid {
			entry.CompletedAt = row.CompletedAt.Time.UTC().Format(time.RFC3339)
		}
		if row.Error.Valid {
			entry.Error = row.Error.String
		} else if row.FailureReason.Valid {
			entry.Error = row.FailureReason.String
		}
		history = append(history, entry)

		if row.Status == "completed" && row.ProjectID.Valid {
			projectKey := uuidToString(row.ProjectID)
			if !seenLiveProject[projectKey] {
				seenLiveProject[projectKey] = true
				currentlyLive = append(currentlyLive, currentlyLiveProject{
					ProjectID:       projectKey,
					ProjectTitle:    row.ProjectTitle.String,
					IssueID:         uuidToString(row.IssueID),
					IssueIdentifier: identifier,
					IssueTitle:      row.IssueTitle,
					DeployedAt:      entry.CompletedAt,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, deploymentHistoryResponse{CurrentlyLive: currentlyLive, History: history})
}
