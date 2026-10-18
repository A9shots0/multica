package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// OwnerRuntimeRouteRequest is human authority over named existing runtimes for
// this issue, not a context flag that an agent can use to escape guarded work.
type OwnerRuntimeRouteRequest struct {
	SquadID                          string             `json:"squad_id"`
	DaemonID                         string             `json:"daemon_id"`
	Implementation                   OwnerRuntimeTarget `json:"implementation"`
	Review                           OwnerRuntimeTarget `json:"review"`
	Release                          OwnerRuntimeTarget `json:"release"`
	Verification                     OwnerRuntimeTarget `json:"verification"`
	AllowBriefDeploymentInterruption bool               `json:"allow_brief_deployment_interruption"`
}
type OwnerRuntimeTarget struct {
	AgentID   string `json:"agent_id"`
	RuntimeID string `json:"runtime_id"`
}
type OwnerRuntimeRoute struct {
	OwnerRuntimeRouteRequest
	Version       int    `json:"version"`
	WorkspaceID   string `json:"workspace_id"`
	IssueID       string `json:"issue_id"`
	OwnerID       string `json:"owner_id"`
	DirectiveID   string `json:"directive_id"`
	IssueRevision int64  `json:"issue_revision"`
	ScopeSHA256   string `json:"scope_sha256"`
}
type ownerRouteTask struct {
	DirectiveID string          `json:"directive_id"`
	Stage       string          `json:"stage"`
	EventID     string          `json:"event_id,omitempty"`
	Manifest    json.RawMessage `json:"manifest,omitempty"`
	Predecessor json.RawMessage `json:"predecessor_evidence,omitempty"`
}

func ParseOwnerRuntimeRoute(plan db.IssueWorkflowPlan) (*OwnerRuntimeRoute, error) {
	if len(plan.OwnerRuntimeRoute) == 0 {
		return nil, nil
	}
	var route OwnerRuntimeRoute
	if json.Unmarshal(plan.OwnerRuntimeRoute, &route) != nil || route.Version != 1 || route.WorkspaceID != util.UUIDToString(plan.WorkspaceID) || route.IssueID != util.UUIDToString(plan.IssueID) || route.SquadID != util.UUIDToString(plan.SquadID) || route.Implementation.AgentID != util.UUIDToString(plan.ImplementerID) || route.Review.AgentID != util.UUIDToString(plan.ReviewerID) || route.Release.AgentID != util.UUIDToString(plan.ReleaseAgentID) || route.Verification.AgentID != util.UUIDToString(plan.VerifierID) || route.OwnerID == "" || route.DirectiveID == "" || !plan.RequiresDeployment || plan.RequiresOwner {
		return nil, errors.New("owner workflow authority is invalid")
	}
	return &route, nil
}

func (route OwnerRuntimeRouteRequest) Validate() error {
	if _, err := util.ParseUUID(route.SquadID); err != nil {
		return errors.New("select an existing squad")
	}
	if strings.TrimSpace(route.DaemonID) == "" {
		return errors.New("confirm the exact host")
	}
	seen := map[string]string{}
	for _, target := range route.Targets() {
		if prior, ok := seen[target.AgentID]; ok && prior != target.RuntimeID {
			return errors.New("one agent must use the same exact runtime in every role")
		}
		seen[target.AgentID] = target.RuntimeID
		if _, err := util.ParseUUID(target.AgentID); err != nil {
			return errors.New("select all four existing role agents")
		}
		if _, err := util.ParseUUID(target.RuntimeID); err != nil {
			return errors.New("every role requires an exact runtime")
		}
	}
	if route.Implementation.AgentID == route.Review.AgentID || route.Release.AgentID == route.Verification.AgentID {
		return errors.New("review and live verification require independent agents")
	}
	return nil
}
func (route OwnerRuntimeRouteRequest) Targets() []OwnerRuntimeTarget {
	targets := []OwnerRuntimeTarget{route.Implementation, route.Review, route.Release, route.Verification}
	sort.Slice(targets, func(i, j int) bool { return targets[i].AgentID < targets[j].AgentID })
	return targets
}
func (route OwnerRuntimeRoute) Target(stage string) (OwnerRuntimeTarget, error) {
	switch stage {
	case "queued", "working", "correction":
		return route.Implementation, nil
	case "technical_review":
		return route.Review, nil
	case "deploying", "rolling_back":
		return route.Release, nil
	case "production_verifying":
		return route.Verification, nil
	}
	return OwnerRuntimeTarget{}, errors.New("owner workflow stage is not executable")
}

// ValidateOwnerRuntimeTargets runs while admission owns the issue lock. The
// same check is repeated at claim/start: changed permissions or host bindings
// never inherit the earlier owner's permission silently.
func (s *TaskService) ValidateOwnerRuntimeTargets(ctx context.Context, q *db.Queries, route OwnerRuntimeRoute) error {
	if err := route.Validate(); err != nil {
		return err
	}
	if s.CanInvokeOwnerRuntimeAgent == nil {
		return errors.New("owner invocation authority is unavailable")
	}
	workspace, _ := util.ParseUUID(route.WorkspaceID)
	owner, _ := util.ParseUUID(route.OwnerID)
	squad, _ := util.ParseUUID(route.SquadID)
	if _, err := q.LockSquadForAutopilotAssignment(ctx, db.LockSquadForAutopilotAssignmentParams{ID: squad, WorkspaceID: workspace}); err != nil {
		return errors.New("owner workflow squad unavailable")
	}
	seen := map[string]bool{}
	for _, target := range route.Targets() {
		if seen[target.AgentID] {
			continue
		}
		seen[target.AgentID] = true
		id, _ := util.ParseUUID(target.AgentID)
		agent, err := q.LockOwnerRouteAgent(ctx, db.LockOwnerRouteAgentParams{AgentID: id, WorkspaceID: workspace, SquadID: squad})
		if err != nil || util.UUIDToString(agent.RuntimeID) != target.RuntimeID || !s.CanInvokeOwnerRuntimeAgent(ctx, q, agent, owner, workspace) {
			return errors.New("owner workflow target invocation or membership changed")
		}
		runtime, err := q.LockOwnerRouteRuntime(ctx, db.LockOwnerRouteRuntimeParams{RuntimeID: agent.RuntimeID, WorkspaceID: workspace})
		if err != nil || runtime.OwnerID != owner || runtime.DaemonID.String != route.DaemonID || runtime.Status != "online" || !runtime.LastSeenAt.Valid || time.Since(runtime.LastSeenAt.Time) > 2*time.Minute {
			return errors.New("owner workflow runtime is not fresh on the declared owner's host")
		}
		if runtime.ProfileID.Valid {
			profile, err := q.GetRuntimeProfile(ctx, runtime.ProfileID)
			if err != nil || !profile.Enabled {
				return errors.New("owner workflow runtime profile is unavailable")
			}
		}
	}
	return nil
}

// FileManifestDigest is independent of Git. Canonical bytes are sorted UTF-8
// path + NUL + lowercase observed SHA256 + newline for each unique path.
// The server verifies this receipt, not the remote filesystem contents.
type FileManifest struct {
	SchemaVersion   int                `json:"schema_version"`
	Contract        string             `json:"contract"`
	Artifacts       []ManifestArtifact `json:"artifacts"`
	CandidateSHA256 string             `json:"candidate_sha256"`
}
type ManifestArtifact struct {
	Path           string `json:"path"`
	DeclaredSHA256 string `json:"declared_sha256"`
	ObservedSHA256 string `json:"observed_sha256"`
}

func ValidateFileManifest(raw []byte) (string, error) {
	var manifest FileManifest
	if json.Unmarshal(raw, &manifest) != nil || manifest.Contract != "file_manifest.v1" || manifest.SchemaVersion != 1 || len(manifest.Artifacts) < 1 || len(manifest.Artifacts) > 64 {
		return "", errors.New("a bounded file_manifest.v1 receipt is required")
	}
	sort.Slice(manifest.Artifacts, func(i, j int) bool { return manifest.Artifacts[i].Path < manifest.Artifacts[j].Path })
	hash := sha256.New()
	previous := ""
	for _, entry := range manifest.Artifacts {
		if entry.Path == "" || entry.Path != strings.TrimSpace(entry.Path) || len(entry.Path) > 1024 || strings.ContainsAny(entry.Path, "\x00\r\n") || entry.Path == previous {
			return "", errors.New("manifest paths must be nonempty, bounded and unique")
		}
		decoded, err := hex.DecodeString(entry.ObservedSHA256)
		if err != nil || len(decoded) != 32 || strings.ToLower(entry.ObservedSHA256) != entry.ObservedSHA256 || entry.DeclaredSHA256 != entry.ObservedSHA256 {
			return "", errors.New("manifest declared and observed SHA256 must match")
		}
		fmt.Fprintf(hash, "%s\x00%s\n", entry.Path, entry.ObservedSHA256)
		previous = entry.Path
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if manifest.CandidateSHA256 != digest {
		return "", errors.New("candidate SHA256 does not match canonical manifest")
	}
	return digest, nil
}

func ownerRouteTaskContext(ctx context.Context, q *db.Queries, plan db.IssueWorkflowPlan, route OwnerRuntimeRoute, eventID pgtype.UUID) ([]byte, error) {
	snapshot := ownerRouteTask{DirectiveID: route.DirectiveID, Stage: plan.CurrentState}
	if eventID.Valid {
		snapshot.EventID = util.UUIDToString(eventID)
		if eventID != mustRouteUUID(route.DirectiveID) {
			event, err := q.GetIssueWorkflowEvent(ctx, db.GetIssueWorkflowEventParams{ID: eventID, IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID})
			if err != nil {
				return nil, err
			}
			snapshot.Predecessor = event.Payload
		}
	}
	if plan.CurrentState != "queued" && plan.CurrentState != "working" && plan.CurrentState != "correction" {
		event, err := q.GetLatestWorkflowManifest(ctx, db.GetLatestWorkflowManifestParams{IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID})
		if err != nil {
			return nil, errors.New("review/release requires the actual implementation manifest")
		}
		if _, err = ValidateFileManifest(event.Payload); err != nil {
			return nil, err
		}
		snapshot.Manifest = event.Payload
	}
	return json.Marshal(map[string]any{"owner_runtime_route": snapshot})
}

// OwnerRuntimeRouteSourceError distinguishes unavailable source data from a
// route that was read successfully and no longer authorizes this task.
type OwnerRuntimeRouteSourceError struct {
	Source string
	Err    error
}

func (e *OwnerRuntimeRouteSourceError) Error() string { return e.Source + ": " + e.Err.Error() }
func (e *OwnerRuntimeRouteSourceError) Unwrap() error { return e.Err }

func (s *TaskService) ValidateOwnerRuntimeTask(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, starting bool) (bool, error) {
	if !task.IssueID.Valid {
		return false, nil
	}
	issue, err := q.GetIssue(ctx, task.IssueID)
	if err != nil {
		return false, &OwnerRuntimeRouteSourceError{Source: "issue", Err: err}
	}
	plan, err := q.GetIssueWorkflowPlan(ctx, db.GetIssueWorkflowPlanParams{IssueID: task.IssueID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, &OwnerRuntimeRouteSourceError{Source: "issue workflow plan", Err: err}
	}
	route, err := ParseOwnerRuntimeRoute(plan)
	if err != nil || route == nil {
		return route != nil, err
	}
	if issue.AssigneeType.String != "squad" || issue.AssigneeID != plan.SquadID || route.ScopeSHA256 != OwnerRuntimeScopeDigest(issue) {
		return true, errors.New("owner workflow issue scope or assignment changed")
	}
	if task.WorkflowActionID.Valid || task.AutopilotRunID.Valid || task.ChatSessionID.Valid || task.IsLeaderTask || task.SquadID != plan.SquadID || task.AccountableUserID != mustRouteUUID(route.OwnerID) {
		return true, errors.New("task does not carry this owner's workflow authority")
	}
	target, err := route.Target(plan.CurrentState)
	if err != nil || target.AgentID != util.UUIDToString(task.AgentID) || target.RuntimeID != util.UUIDToString(task.RuntimeID) {
		return true, errors.New("workflow role or stage changed")
	}
	if err = s.ValidateOwnerRuntimeTargets(ctx, q, *route); err != nil {
		return true, err
	}
	expected, err := ownerRouteTaskContext(ctx, q, plan, *route, task.TriggerEvidenceRefID)
	if err != nil {
		return true, err
	}
	var actualMap, expectedMap map[string]any
	if json.Unmarshal(task.Context, &actualMap) != nil || json.Unmarshal(expected, &expectedMap) != nil || len(actualMap) != 1 {
		return true, errors.New("workflow task has invalid route context")
	}
	if actual, ok := actualMap["owner_runtime_route"].(map[string]any); ok && actual["stage"] == "queued" && plan.CurrentState == "working" {
		actual["stage"] = "working"
	}
	if !reflect.DeepEqual(actualMap, expectedMap) {
		return true, errors.New("workflow task no longer matches its immutable route and candidate")
	}
	if task.TriggerEvidenceKind.String == "workflow_event" {
		event, err := q.GetIssueWorkflowEvent(ctx, db.GetIssueWorkflowEventParams{ID: task.TriggerEvidenceRefID, WorkspaceID: plan.WorkspaceID, IssueID: plan.IssueID})
		var decision struct {
			EnqueueAgentID string `json:"enqueue_agent_id"`
		}
		if err != nil || json.Unmarshal(event.Decision, &decision) != nil || decision.EnqueueAgentID != target.AgentID || event.SourceTaskID != task.DelegatedFromTaskID {
			return true, errors.New("workflow source event does not authorize this successor")
		}
	} else if task.TriggerEvidenceKind.String != "owner_workflow_entry" || task.TriggerEvidenceRefID != mustRouteUUID(route.DirectiveID) || (plan.CurrentState != "queued" && plan.CurrentState != "working") {
		return true, errors.New("workflow task has no admitted trigger")
	}
	if starting {
		other, err := q.HasOtherRunningOwnerWorkflowTask(ctx, db.HasOtherRunningOwnerWorkflowTaskParams{IssueID: task.IssueID, TaskID: task.ID})
		if err != nil || other {
			return true, errors.New("the preceding workflow worker has not finished")
		}
	}
	return true, nil
}

func (s *TaskService) enqueueOwnerRuntimeSuccessor(ctx context.Context, issue db.Issue, target pgtype.UUID, source db.AgentTaskQueue, eventID pgtype.UUID) (db.AgentTaskQueue, error) {
	if s.TxStarter == nil {
		return db.AgentTaskQueue{}, errors.New("owner workflow requires transactional admission")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	locked, err := q.LockIssueTaskAdmission(ctx, db.LockIssueTaskAdmissionParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	plan, err := q.GetIssueWorkflowPlanForUpdate(ctx, db.GetIssueWorkflowPlanForUpdateParams{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	route, err := ParseOwnerRuntimeRoute(plan)
	if err != nil || route == nil {
		return db.AgentTaskQueue{}, errors.New("owner workflow authority missing")
	}
	expected, err := route.Target(plan.CurrentState)
	if err != nil || expected.AgentID != util.UUIDToString(target) {
		return db.AgentTaskQueue{}, errors.New("successor target no longer matches owner route")
	}
	task, err := s.EnqueueOwnerRuntimeTask(ctx, tx, locked, plan, source, eventID, pgtype.UUID{})
	if err != nil {
		return task, err
	}
	if err = tx.Commit(ctx); err != nil {
		return db.AgentTaskQueue{}, err
	}
	s.PublishOwnerIntakeTask(ctx, task)
	return task, nil
}
func mustRouteUUID(value string) pgtype.UUID { id, _ := util.ParseUUID(value); return id }

// Reuse the normal terminal controller recovery after a lost post-commit
// callback. This deliberately selects only explicit owner-route carriers;
// legacy squad reconciliation and its leader policy are unchanged.
func (s *TaskService) recoverOwnerRuntimeCompletions(ctx context.Context, taskID pgtype.UUID) error {
	tasks, err := s.Queries.ListOwnerRuntimeCompletionRecoveryTasks(ctx, taskID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		s.recoverWorkflowTaskWithoutEvent(ctx, task)
	}
	return nil
}
func OwnerRuntimeScopeDigest(issue db.Issue) string {
	raw, _ := json.Marshal([]any{issue.Title, issue.Description, issue.AcceptanceCriteria, issue.ProjectID})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func (s *TaskService) EnqueueOwnerRuntimeTask(ctx context.Context, tx pgx.Tx, issue db.Issue, plan db.IssueWorkflowPlan, source db.AgentTaskQueue, eventID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	q := s.Queries.WithTx(tx)
	route, err := ParseOwnerRuntimeRoute(plan)
	if err != nil || route == nil {
		return db.AgentTaskQueue{}, errors.New("owner workflow receipt unavailable")
	}
	if err = s.ValidateOwnerRuntimeTargets(ctx, q, *route); err != nil {
		return db.AgentTaskQueue{}, err
	}
	target, err := route.Target(plan.CurrentState)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	agent, err := q.GetAgent(ctx, mustRouteUUID(target.AgentID))
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	var delegated, trigger pgtype.UUID
	kind := "owner_workflow_entry"
	trigger = mustRouteUUID(route.DirectiveID)
	if eventID.Valid {
		if prior, err := q.GetWorkflowEventCarrier(ctx, db.GetWorkflowEventCarrierParams{IssueID: issue.ID, EventID: eventID}); err == nil {
			return prior, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return db.AgentTaskQueue{}, err
		}
		current, err := q.GetAgentTask(ctx, source.ID)
		if err != nil || current.Status != "completed" {
			return db.AgentTaskQueue{}, errors.New("successor waits until the reporting task completes")
		}
		delegated, trigger, kind = source.ID, eventID, "workflow_event"
	}
	contextJSON, err := ownerRouteTaskContext(ctx, q, plan, *route, eventID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	// First-task context uses the directive as its exact trigger too.
	if !eventID.Valid {
		contextJSON, err = ownerRouteTaskContext(ctx, q, plan, *route, trigger)
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
	}
	worker := &TaskService{Queries: q, TxStarter: tx, Bus: events.New(), PrepareWorkflowGuardedAuthority: s.PrepareWorkflowGuardedAuthority, CanInvokeOwnerRuntimeAgent: s.CanInvokeOwnerRuntimeAgent, FeatureFlags: s.FeatureFlags, Entitlements: s.Entitlements, Composio: s.Composio}
	overlay := worker.buildRuntimeMCPOverlay(ctx, mustRouteUUID(route.OwnerID), agent)
	originatorSource := attribution.SourceDelegation
	if !eventID.Valid {
		originatorSource = attribution.SourceDirectHuman
	}
	return worker.createAgentTaskWithWorkflowAuthority(ctx, issue, agent, plan.CurrentState, db.CreateAgentTaskParams{
		TriggerCommentID: triggerCommentID,
		ID:               dbid.NewV7(), AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID, Priority: priorityToInt(issue.Priority), ForceFreshSession: pgtype.Bool{Bool: true, Valid: true}, SquadID: plan.SquadID,
		OriginatorUserID: mustRouteUUID(route.OwnerID), AccountableUserID: mustRouteUUID(route.OwnerID), OriginatorSource: pgtype.Text{String: string(originatorSource), Valid: true}, DelegatedFromTaskID: delegated,
		RuntimeMcpOverlay: overlay.Overlay, RuntimeConnectedApps: overlay.ConnectedApps,
		TriggerEvidenceKind: pgtype.Text{String: kind, Valid: true}, TriggerEvidenceRefID: trigger, TaskContext: contextJSON,
	})
}
