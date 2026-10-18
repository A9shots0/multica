package workflowaction

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkflowTaskContextsPreserveGuardedDispatchAndRejectUnbound(t *testing.T) {
	artifact, err := agentqualification.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.release_runtime_assurance")
	if err != nil {
		t.Fatal(err)
	}
	envelope := agentqualification.GuardedDispatchEnvelopeV1{
		Version:                 agentqualification.GuardedDispatchEnvelopeVersionV2,
		WorkspaceID:             util.UUIDToString(pgtype.UUID{Bytes: [16]byte{2}, Valid: true}),
		IssueID:                 util.UUIDToString(pgtype.UUID{Bytes: [16]byte{3}, Valid: true}),
		ConfigurationID:         "candidate-config",
		RoleID:                  artifact.RoleID,
		ExecutorAgentID:         "agent-1",
		RuntimeID:               "runtime-1",
		RuntimeProvider:         "candidate-provider",
		Model:                   "candidate-model",
		ReasoningEffort:         "candidate-effort",
		ArtifactBindingID:       artifact.BindingID,
		ArtifactBinding:         artifact,
		RuntimeBindingReceiptID: "unbound-receipt",
	}
	action := db.IssueWorkflowActionReceipt{
		ID:          pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		IssueID:     pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		EffectKey:   pgtype.Text{String: "effect/guarded", Valid: true},
	}
	correctionSpec := db.GetWorkflowCorrectionTaskSpecRow{AgentID: pgtype.UUID{Bytes: [16]byte{4}, Valid: true}, RuntimeID: pgtype.UUID{Bytes: [16]byte{5}, Valid: true}}
	reviewSpec := db.GetWorkflowReviewTaskSpecRow{AgentID: correctionSpec.AgentID, RuntimeID: correctionSpec.RuntimeID}
	releaseSpec := db.GetWorkflowReleaseVerificationTaskSpecRow{AgentID: correctionSpec.AgentID, RuntimeID: correctionSpec.RuntimeID}
	envelope.ExecutorAgentID = util.UUIDToString(correctionSpec.AgentID)
	envelope.RuntimeID = util.UUIDToString(correctionSpec.RuntimeID)
	payload, err := AttachGuardedDispatchToTaskContextV1([]byte(`{}`), envelope)
	if err != nil {
		t.Fatal(err)
	}
	action.Payload = payload
	contexts := map[string][]byte{
		"correction":           expectedTaskContext(action, correctionSpec),
		"review":               expectedReviewTaskContext(action, reviewSpec),
		"release_verification": expectedReleaseVerificationContext(action, releaseSpec),
	}
	for name, taskContext := range contexts {
		t.Run(name, func(t *testing.T) {
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal(taskContext, &decoded); err != nil {
				t.Fatal(err)
			}
			if len(decoded[agentqualification.GuardedDispatchTaskContextKey]) == 0 {
				t.Fatalf("guarded dispatch was dropped: %s", taskContext)
			}
			if _, err := agentqualification.ValidateGuardedTaskContextForExecutionV1(taskContext, nil, guardedExpectedIdentity(action, correctionSpec.AgentID, correctionSpec.RuntimeID), time.Now().UTC()); !errors.Is(err, agentqualification.ErrGuardedConfigurationUnbound) {
				t.Fatalf("admission error = %v, want guarded configuration unbound", err)
			}
		})
	}
}

func TestAttachGuardedDispatchToTaskContextV1RejectsHistoricalFallback(t *testing.T) {
	artifact, err := agentqualification.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		t.Fatal(err)
	}
	envelope := agentqualification.GuardedDispatchEnvelopeV1{
		Version:                 agentqualification.GuardedDispatchEnvelopeVersionV2,
		WorkspaceID:             "workspace-1",
		IssueID:                 "issue-1",
		ConfigurationID:         "data_sync.sol.high.v1",
		RoleID:                  artifact.RoleID,
		ExecutorAgentID:         "agent-1",
		RuntimeID:               "runtime-1",
		RuntimeProvider:         "candidate-provider",
		Model:                   "candidate-model",
		ReasoningEffort:         "candidate-effort",
		ArtifactBindingID:       "data_sync.sol.high.v1",
		ArtifactBinding:         artifact,
		RuntimeBindingReceiptID: "unbound-receipt",
	}
	if _, err := AttachGuardedDispatchToTaskContextV1([]byte(`{}`), envelope); err == nil {
		t.Fatal("historical configuration label was accepted as an artifact binding")
	}
}

func TestWorkflowTaskContextsPreserveLegacyAdmission(t *testing.T) {
	action := db.IssueWorkflowActionReceipt{
		ID:        pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		EffectKey: pgtype.Text{String: "effect/legacy", Valid: true},
		Payload:   []byte(`{}`),
	}
	for name, taskContext := range map[string][]byte{
		"correction":           expectedTaskContext(action, db.GetWorkflowCorrectionTaskSpecRow{}),
		"review":               expectedReviewTaskContext(action, db.GetWorkflowReviewTaskSpecRow{}),
		"release_verification": expectedReleaseVerificationContext(action, db.GetWorkflowReleaseVerificationTaskSpecRow{}),
	} {
		t.Run(name, func(t *testing.T) {
			admission, err := agentqualification.ValidateGuardedTaskContextV1(taskContext, nil, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if admission.Mode != agentqualification.GuardedDispatchModeLegacy {
				t.Fatalf("mode = %s, want legacy", admission.Mode)
			}
		})
	}
}

func TestWorkflowTaskEffectsValidateGuardBeforeInsert(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	dir := filepath.Dir(currentFile)
	for _, name := range []string{"local_task_effect.go", "review_task_effect.go", "release_verification_task_effect.go"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			guardAt := strings.Index(source, "agentqualification.ValidateGuardedTaskContextForExecutionV1(contextJSON")
			beginAt := strings.Index(source, "effect.Starter.Begin(ctx)")
			insertAt := strings.Index(source, "q.CreateWorkflowCorrectionTask(ctx")
			if guardAt < 0 || beginAt < 0 || insertAt < 0 || guardAt > beginAt || guardAt > insertAt {
				t.Fatalf("guard must run before transaction and insert: guard=%d begin=%d insert=%d", guardAt, beginAt, insertAt)
			}
		})
	}
}

func TestWorkflowTaskEffectsValidateGuardBeforeAcceptingObservedTask(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	dir := filepath.Dir(currentFile)
	for _, name := range []string{"local_task_effect.go", "review_task_effect.go", "release_verification_task_effect.go"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			observeEnd := strings.Index(source, "func (effect ")
			if observeEnd >= 0 {
				observeEnd = strings.Index(source[observeEnd+1:], "func (effect ") + observeEnd + 1
			}
			if observeEnd <= 0 {
				t.Fatal("could not isolate Observe method")
			}
			observeSource := source[:observeEnd]
			guardAt := strings.Index(observeSource, "agentqualification.ValidateGuardedTaskContextForExecutionV1(")
			foundAt := strings.LastIndex(observeSource, "ObservationFound")
			if guardAt < 0 || foundAt < 0 || guardAt > foundAt {
				t.Fatalf("Observe must validate guard before accepting Found: guard=%d found=%d", guardAt, foundAt)
			}
		})
	}
}
