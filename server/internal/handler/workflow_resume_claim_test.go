package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/continuitycheckpoint"
	"github.com/multica-ai/multica/server/internal/developmentmemory"
	internalflags "github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workflowcheckpoint"
	"github.com/multica-ai/multica/server/internal/workflowresume"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/featureflag"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestClaimFreshTaskUsesExactVerifiedCheckpointAndDropsProviderSession(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Verified checkpoint resume runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Verified checkpoint resume agent")

	sourceID := uuid.NewString()
	dbfx.Exec(t, `INSERT INTO agent_task_queue (
		id, agent_id, runtime_id, issue_id, status, priority, attempt, max_attempts,
		session_id, work_dir, started_at, completed_at
	) VALUES ($1,$2,$3,$4,'completed',1,1,3,'provider-session-that-must-not-resume','/tmp/verified-workdir',now()-interval '2 minutes',now()-interval '1 minute')`,
		sourceID, agentID, runtimeID, issueID)
	targetID := createDispatchedClaimFixtureTask(t, ctx, agentID, runtimeID, issueID, "120 seconds", false)
	dbfx.Exec(t, `UPDATE agent_task_queue
		SET force_fresh_session=true, rerun_of_task_id=$2, parent_task_id=$2, attempt=2, max_attempts=3
		WHERE id=$1`, targetID, sourceID)

	scopeID := uuid.NewString()
	executionID := uuid.NewString()
	dbfx.Exec(t, `INSERT INTO issue_workflow_scope (
		id,workspace_id,issue_id,revision,acceptance_version,state,owner_evidence_id,created_by,statement
	) VALUES ($1,$2,$3,1,1,'accepted',$4,$4,'{}')`, scopeID, testWorkspaceID, issueID, testUserID)
	dbfx.Exec(t, `INSERT INTO issue_workflow_execution (
		id,workspace_id,issue_id,branch_key,operation_type,state,scope_id,scope_revision,
		actor_type,actor_id,task_id,checkpoint,expected_event,wait_reason
	) VALUES ($1,$2,$3,'main','implementation','working',$4,1,'agent',$5,$6,$7,'{}','{}')`,
		executionID, testWorkspaceID, issueID, scopeID, agentID, sourceID,
		`{"completed":["migration 636"],"next_action":"run the focused tests"}`)
	dbfx.Exec(t, `INSERT INTO issue_current_picture (
		workspace_id,issue_id,contract_version,revision,scope_revision,outcome_state,outcome_open,
		execution_state,release_state,health_state,attention_state,source_revisions,document,generated_at
	) VALUES ($1,$2,2,12,1,'open',true,'working','pending','unknown','resolved','[]',
		'{"contract_version":2,"original_business_request":{},"scope":{},"outcome":{"state":"open"},"unresolved_dependencies":[]}',now())`,
		testWorkspaceID, issueID)

	execution, err := testHandler.Queries.GetIssueWorkflowExecution(ctx, db.GetIssueWorkflowExecutionParams{
		WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(issueID), ID: parseUUID(executionID),
	})
	if err != nil {
		t.Fatal(err)
	}
	built, err := workflowcheckpoint.Build(execution, nil)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `INSERT INTO issue_workflow_execution_checkpoint_snapshot (
		id,workspace_id,issue_id,execution_id,execution_revision,task_id,actor_id,scope_id,
		scope_revision,branch_key,execution_state,checkpoint,snapshot_document,chain_anchor,snapshot_sha256
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,true,$14)`,
		uuid.NewString(), testWorkspaceID, issueID, executionID, execution.Revision, sourceID, agentID,
		scopeID, 1, execution.BranchKey, execution.State, built.Checkpoint, built.Document, built.Digest)
	resumeReceipt := continuitycheckpoint.Receipt{
		SchemaVersion: 1, PackageStep: 1, Summary: "migration checkpointed",
		Completed: []string{"migration 636"}, NextAction: "run the focused tests",
		Verification: []string{"snapshot verified"}, OpenRisks: []string{},
	}
	resumeRaw, err := json.Marshal(resumeReceipt)
	if err != nil {
		t.Fatal(err)
	}
	validatedResume, err := continuitycheckpoint.DecodeAndValidate(resumeRaw, 1)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `INSERT INTO agent_task_continuation_receipt (
		id,workspace_id,issue_id,root_task_id,source_task_id,successor_task_id,
		source_step,next_step,max_packages,checkpoint,checkpoint_sha256,execution_id,execution_revision
	) VALUES ($1,$2,$3,$4,$4,$5,1,2,3,$6,$7,$8,$9)`, uuid.NewString(), testWorkspaceID,
		issueID, sourceID, targetID, validatedResume.Canonical, validatedResume.SHA256, executionID, execution.Revision)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_continuation_receipt WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_workflow_execution_checkpoint_snapshot WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_workflow_execution WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_workflow_scope WHERE issue_id=$1`, issueID)
	})

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "verified-workflow-resume")
	req = withURLParam(req, "runtimeId", runtimeID)
	var response struct {
		Task *struct {
			ID                    string          `json:"id"`
			Attempt               int32           `json:"attempt"`
			WorkspaceID           string          `json:"workspace_id"`
			IssueID               string          `json:"issue_id"`
			PriorSessionID        string          `json:"prior_session_id"`
			PriorWorkDir          string          `json:"prior_work_dir"`
			WorkflowResumeCapsule json.RawMessage `json:"workflow_resume_capsule"`
		} `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
	if response.Task == nil || response.Task.ID != targetID {
		t.Fatalf("claimed task = %#v, want %s", response.Task, targetID)
	}
	if response.Task.PriorSessionID != "" || response.Task.PriorWorkDir != "/tmp/verified-workdir" {
		t.Fatalf("provider/workdir continuity = (%q,%q), want fresh provider and reused workdir", response.Task.PriorSessionID, response.Task.PriorWorkDir)
	}
	capsule, err := workflowresume.Parse(response.Task.WorkflowResumeCapsule, workflowresume.Target{
		TaskID: targetID, TaskAttempt: response.Task.Attempt,
		WorkspaceID: response.Task.WorkspaceID, IssueID: response.Task.IssueID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if capsule.Source.TaskID != sourceID || capsule.Source.ExecutionID != executionID ||
		!strings.Contains(string(capsule.Checkpoint), "run the focused tests") {
		t.Fatalf("resume capsule = %#v checkpoint=%s", capsule, capsule.Checkpoint)
	}
}

func TestContinuityObservationTargetingExcludesOtherProviders(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	const daemonID = "continuity-observation-claude-daemon"
	runtimeID := createClaimReclaimRuntime(t, ctx, "Continuity observation Claude runtime")
	dbfx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2, runtime_mode='local', provider='claude' WHERE id=$1`, runtimeID, daemonID)
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Continuity observation Claude agent")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID,
		"issue_id":   issueID,
		"status":     "queued",
	})

	priorFlags := testHandler.TaskService.FeatureFlags
	provider := featureflag.NewStaticProvider()
	provider.Set(internalflags.ContinuousWorkObservationV1, featureflag.Rule{
		AllowBy: "workspace_provider",
		Allow:   []string{testWorkspaceID + ":codex"},
	})
	testHandler.TaskService.FeatureFlags = featureflag.NewService(provider)
	t.Cleanup(func() { testHandler.TaskService.FeatureFlags = priorFlags })

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, daemonID)
	req = withURLParam(req, "runtimeId", runtimeID)
	var response struct {
		Task *struct {
			ID                    string `json:"id"`
			ContinuityObservation bool   `json:"continuity_observation_enabled"`
		} `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
	if response.Task == nil || response.Task.ID != taskID {
		t.Fatalf("claimed task = %#v, want %s", response.Task, taskID)
	}
	if response.Task.ContinuityObservation {
		t.Fatal("Claude task entered the Codex-only continuity observation cohort")
	}
}

func TestFeatureEnabledDaemonCheckpointCreatesOneVerifiedFreshHandoff(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	const (
		daemonID = "checkpoint-successor-claim-daemon"
		workDir  = "/tmp/multica-checkpoint-successor-claim"
	)
	runtimeID := createClaimReclaimRuntime(t, ctx, "Checkpoint successor claim runtime")
	dbfx.Exec(t, `UPDATE agent_runtime SET daemon_id=$2, runtime_mode='local', provider='codex' WHERE id=$1`, runtimeID, daemonID)
	agentID := dbfx.Agent(t, "Checkpoint successor claim agent", runtimeID)
	projectID := dbfx.Project(t, "Checkpoint successor claim project")
	dbfx.Insert(t, "project_resource", testutil.Cols{
		"project_id": projectID, "workspace_id": testWorkspaceID, "resource_type": "local_directory",
		"resource_ref": testutil.Raw(`'{"local_path":"/tmp/multica-checkpoint-successor-claim","daemon_id":"checkpoint-successor-claim-daemon","execution_mode":"in_place"}'::jsonb`),
		"position":     0, "created_by": testUserID,
	})
	dbfx.Insert(t, "project_resource", testutil.Cols{
		"project_id": projectID, "workspace_id": testWorkspaceID, "resource_type": "github_repo",
		"resource_ref": `{"url":"https://github.com/example/continuity-rehearsal"}`,
		"position":     1, "created_by": testUserID,
	})
	issueID := dbfx.Issue(t, "Checkpoint successor claim issue", testutil.Cols{"project_id": projectID})
	sourceID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "queued",
	})

	scopeID := uuid.NewString()
	executionID := uuid.NewString()
	dbfx.Exec(t, `INSERT INTO issue_workflow_scope (
		id,workspace_id,issue_id,revision,acceptance_version,state,owner_evidence_id,created_by,statement
	) VALUES ($1,$2,$3,1,1,'accepted',$4,$4,'{}')`, scopeID, testWorkspaceID, issueID, testUserID)
	dbfx.Exec(t, `INSERT INTO issue_workflow_execution (
		id,workspace_id,issue_id,branch_key,operation_type,state,scope_id,scope_revision,
		actor_type,actor_id,task_id,checkpoint,expected_event,wait_reason
	) VALUES ($1,$2,$3,'continuous-work','implementation','working',$4,1,'agent',$5,$6,'{}','{}','{}')`,
		executionID, testWorkspaceID, issueID, scopeID, agentID, sourceID)
	dbfx.Exec(t, `INSERT INTO issue_current_picture (
		workspace_id,issue_id,contract_version,revision,scope_revision,outcome_state,outcome_open,
		execution_state,release_state,health_state,attention_state,source_revisions,document,generated_at
	) VALUES ($1,$2,2,1,1,'open',true,'working','pending','unknown','resolved','[]',
		'{"contract_version":2,"original_business_request":{},"scope":{},"outcome":{"state":"open"},"unresolved_dependencies":[]}',now())`,
		testWorkspaceID, issueID)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_continuation_receipt WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_workflow_execution_checkpoint_snapshot WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_current_picture WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_workflow_execution WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_workflow_scope WHERE issue_id=$1`, issueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id=$1`, issueID)
	})

	priorFlags := testHandler.TaskService.FeatureFlags
	priorHandlerFlags := testHandler.FeatureFlags
	provider := featureflag.NewStaticProvider()
	provider.Set(internalflags.ContinuousWorkCheckpointV1, featureflag.Rule{Default: true})
	provider.Set(internalflags.ContinuousWorkObservationV1, featureflag.Rule{
		AllowBy: "workspace_provider",
		Allow:   []string{testWorkspaceID + ":codex"},
	})
	flags := featureflag.NewService(provider)
	testHandler.TaskService.FeatureFlags = flags
	testHandler.FeatureFlags = flags
	priorMemory, priorMemoryStore := testHandler.DevelopmentMemory, testHandler.DevelopmentMemoryStore
	memory := &recordingDevelopmentMemory{packet: json.RawMessage(`{"schema_version":1,"workstreams":[{"id":"current-intelligence-step-one"}]}`)}
	testHandler.DevelopmentMemory = memory
	testHandler.DevelopmentMemoryStore = developmentmemory.NewStore(testPool)
	t.Cleanup(func() {
		testHandler.TaskService.FeatureFlags = priorFlags
		testHandler.FeatureFlags = priorHandlerFlags
		testHandler.DevelopmentMemory, testHandler.DevelopmentMemoryStore = priorMemory, priorMemoryStore
	})

	claim := func() *struct {
		ID                       string                     `json:"id"`
		Attempt                  int32                      `json:"attempt"`
		WorkspaceID              string                     `json:"workspace_id"`
		IssueID                  string                     `json:"issue_id"`
		PriorSessionID           string                     `json:"prior_session_id"`
		PriorWorkDir             string                     `json:"prior_work_dir"`
		WorkflowResumeCapsule    json.RawMessage            `json:"workflow_resume_capsule"`
		ContinuityCheckpointPlan *continuitycheckpoint.Plan `json:"continuity_checkpoint_plan"`
		BrainContext             json.RawMessage            `json:"brain_context"`
		DevelopmentContext       json.RawMessage            `json:"development_context"`
		ContinuityObservation    bool                       `json:"continuity_observation_enabled"`
	} {
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, daemonID)
		req = withURLParam(req, "runtimeId", runtimeID)
		req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityContinuityCheckpointV1)
		var response struct {
			Task *struct {
				ID                       string                     `json:"id"`
				Attempt                  int32                      `json:"attempt"`
				WorkspaceID              string                     `json:"workspace_id"`
				IssueID                  string                     `json:"issue_id"`
				PriorSessionID           string                     `json:"prior_session_id"`
				PriorWorkDir             string                     `json:"prior_work_dir"`
				WorkflowResumeCapsule    json.RawMessage            `json:"workflow_resume_capsule"`
				ContinuityCheckpointPlan *continuitycheckpoint.Plan `json:"continuity_checkpoint_plan"`
				BrainContext             json.RawMessage            `json:"brain_context"`
				DevelopmentContext       json.RawMessage            `json:"development_context"`
				ContinuityObservation    bool                       `json:"continuity_observation_enabled"`
			} `json:"task"`
		}
		testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&response)
		return response.Task
	}

	first := claim()
	if first == nil || first.ID != sourceID || first.ContinuityCheckpointPlan == nil || !first.ContinuityObservation ||
		first.ContinuityCheckpointPlan.PackageStep != 1 || first.ContinuityCheckpointPlan.MaxPackages != 3 {
		t.Fatalf("feature-enabled source claim = %#v", first)
	}
	startReq := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+sourceID+"/start", nil, testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.StartTask, withURLParam(startReq, "taskId", sourceID)).Want(http.StatusOK)
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET session_id='provider-session-that-must-not-resume' WHERE id=$1`, sourceID); err != nil {
		t.Fatal(err)
	}
	receipt := continuitycheckpoint.Receipt{
		SchemaVersion: 1, PackageStep: 1, Summary: "package one complete",
		Completed: []string{"bounded implementation"}, NextAction: "run the next package",
		Verification: []string{"focused tests passed"}, OpenRisks: []string{},
	}
	complete := func() {
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+sourceID+"/complete", map[string]any{
			"output": "Package one is safely parked.", "work_dir": workDir, "continuity_checkpoint": receipt,
		}, testWorkspaceID, daemonID)
		testutil.Call(t, testHandler.CompleteTask, withURLParam(req, "taskId", sourceID)).Want(http.StatusOK)
	}
	complete()
	complete() // The daemon may replay a terminal callback after losing the response.
	var successorID string
	var successors int
	dbfx.QueryRow(t, `SELECT count(*), min(id::text) FROM agent_task_queue WHERE continuation_of_task_id=$1`, sourceID).Scan(&successors, &successorID)
	if successors != 1 {
		t.Fatalf("checkpoint callback created %d successors, want exactly one", successors)
	}
	var storedSession *string
	var storedWorkDir string
	var forceFresh bool
	dbfx.QueryRow(t, `SELECT session_id, work_dir, force_fresh_session FROM agent_task_queue WHERE id=$1`, successorID).Scan(&storedSession, &storedWorkDir, &forceFresh)
	if storedSession != nil || storedWorkDir != workDir || !forceFresh {
		t.Fatalf("stored successor continuity = session:%v workdir:%q fresh:%t", storedSession, storedWorkDir, forceFresh)
	}
	memory.packet = json.RawMessage(`{"schema_version":1,"workstreams":[{"id":"current-intelligence-step-two"}]}`)

	second := claim()
	if second == nil || second.ID != successorID {
		t.Fatalf("claimed task = %#v, want checkpoint successor %s", second, successorID)
	}
	if second.PriorSessionID != "" {
		t.Errorf("prior_session_id = %q, want a fresh provider session", second.PriorSessionID)
	}
	if second.PriorWorkDir != workDir {
		t.Errorf("prior_work_dir = %q, want exact durable in-place directory %q", second.PriorWorkDir, workDir)
	}
	plan := second.ContinuityCheckpointPlan
	if plan == nil || plan.RootTaskID != sourceID || plan.PackageStep != 2 || plan.MaxPackages != 3 {
		t.Errorf("continuity checkpoint plan = %#v, want root=%s step=2 max=3", plan, sourceID)
	}
	capsule, err := workflowresume.Parse(second.WorkflowResumeCapsule, workflowresume.Target{
		TaskID: second.ID, TaskAttempt: second.Attempt,
		WorkspaceID: second.WorkspaceID, IssueID: second.IssueID,
	})
	if err != nil {
		t.Fatalf("verify resume capsule: %v", err)
	}
	if capsule.Source.TaskID != sourceID || capsule.Source.ExecutionID != executionID ||
		!strings.Contains(string(capsule.Checkpoint), "run the next package") {
		t.Fatalf("resume capsule = %#v checkpoint=%s", capsule, capsule.Checkpoint)
	}
	if strings.Contains(string(second.WorkflowResumeCapsule), "current-intelligence") {
		t.Fatalf("claim-time Intelligence was copied into the durable capsule: %s", second.WorkflowResumeCapsule)
	}
	if len(second.BrainContext) == 0 || !json.Valid(second.BrainContext) {
		t.Fatalf("claim-time Brain reference missing or invalid: %s", second.BrainContext)
	}
	if !strings.Contains(string(second.DevelopmentContext), "current-intelligence-step-two") ||
		strings.Contains(string(second.DevelopmentContext), "current-intelligence-step-one") {
		t.Fatalf("successor did not receive current, separately fetched Intelligence: %s", second.DevelopmentContext)
	}

	// A later workflow revision invalidates the exact receipt/snapshot binding.
	// A continuation step may never silently downgrade to a capsule-free run.
	dbfx.Exec(t, `UPDATE issue_workflow_execution SET revision=revision+1, updated_at=now() WHERE id=$1`, executionID)
	loadedSuccessor, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(successorID))
	if err != nil {
		t.Fatal(err)
	}
	if capsule, err := testHandler.freshWorkflowResumeCapsule(ctx, loadedSuccessor, testWorkspaceID); err == nil || capsule != nil {
		t.Fatalf("stale continuation resumed without an exact checkpoint: capsule=%s err=%v", capsule, err)
	}
}
