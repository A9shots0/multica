package workflowaction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestFakeReleaseProviderObserveApplyAndGenerationFence(t *testing.T) {
	provider := NewFakeReleaseProvider()
	action := fakeReleaseAction(t, "deploy_exact_release", 0, "")
	observed, err := provider.Observe(context.Background(), action)
	if err != nil || observed.Kind != ObservationAbsent {
		t.Fatalf("initial observe=(%s,%v), want absent", observed.Kind, err)
	}
	provider.SetUnknown(action.EffectKey.String, true)
	if observed, err = provider.Observe(context.Background(), action); observed.Kind != ObservationUnknown || !errors.Is(err, ErrEffectUnknown) {
		t.Fatalf("unknown observe=(%s,%v)", observed.Kind, err)
	}
	provider.SetUnknown(action.EffectKey.String, false)
	applied, err := provider.Apply(context.Background(), action)
	if err != nil || applied.Kind != ObservationFound || provider.ApplyCount(action.EffectKey.String) != 1 {
		t.Fatalf("apply=(%s,%v) count=%d", applied.Kind, err, provider.ApplyCount(action.EffectKey.String))
	}
	if observed, err = provider.Observe(context.Background(), action); err != nil || observed.Kind != ObservationFound || string(observed.Result) != string(applied.Result) {
		t.Fatalf("reconcile observe=(%s,%v)", observed.Kind, err)
	}
	if replay, err := provider.Apply(context.Background(), action); err != nil || replay.Kind != ObservationFound || provider.ApplyCount(action.EffectKey.String) != 1 {
		t.Fatalf("idempotent apply=(%s,%v) count=%d", replay.Kind, err, provider.ApplyCount(action.EffectKey.String))
	}

	conflict := action
	conflict.Payload = append([]byte(nil), action.Payload...)
	conflict.Payload[len(conflict.Payload)-2] = 'x'
	if got, err := provider.Apply(context.Background(), conflict); got.Kind != ObservationUnknown || !errors.Is(err, ErrEffectMismatch) {
		t.Fatalf("semantic mismatch=(%s,%v)", got.Kind, err)
	}

	occupied := NewFakeReleaseProvider()
	occupied.SeedTarget(action.WorkspaceID, "production", uuid.New().String(), 1)
	blocked := fakeReleaseAction(t, "deploy_exact_release", 0, "")
	blocked.WorkspaceID = action.WorkspaceID
	if got, err := occupied.Apply(context.Background(), blocked); got.Kind != ObservationUnknown || !errors.Is(err, ErrEffectMismatch) {
		t.Fatalf("occupied generation=(%s,%v)", got.Kind, err)
	}
}

func TestFakeReleaseProviderNamespacesTargetsAndRejectsIncompleteSnapshots(t *testing.T) {
	provider := NewFakeReleaseProvider()
	workspaceA := fakeReleaseAction(t, "deploy_exact_release", 0, "")
	workspaceB := fakeReleaseAction(t, "deploy_exact_release", 0, "")
	if _, err := provider.Apply(context.Background(), workspaceA); err != nil {
		t.Fatalf("workspace A apply: %v", err)
	}
	if _, err := provider.Apply(context.Background(), workspaceB); err != nil {
		t.Fatalf("workspace B same target apply: %v", err)
	}

	// Distinct issues in one workspace still serialize on the provider target;
	// an issue-local receipt cannot overwrite another issue's generation.
	sameWorkspace := fakeReleaseAction(t, "deploy_exact_release", 0, "")
	sameWorkspace.WorkspaceID = workspaceA.WorkspaceID
	if sameWorkspace.IssueID == workspaceA.IssueID {
		t.Fatal("fixture did not create distinct issues")
	}
	if got, err := provider.Apply(context.Background(), sameWorkspace); got.Kind != ObservationUnknown || !errors.Is(err, ErrEffectMismatch) {
		t.Fatalf("same workspace occupied target=(%s,%v)", got.Kind, err)
	}

	for _, field := range []string{"command_hash", "release_revision", "build_manifest_digest", "artifact_digest", "runtime_contract_digest", "exposure_policy", "audience_policy", "risk_envelope", "budget_envelope", "recovery_policy", "recovery_policy_revision", "expected"} {
		t.Run(field, func(t *testing.T) {
			action := fakeReleaseAction(t, "deploy_exact_release", 0, "")
			var payload map[string]any
			if err := json.Unmarshal(action.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			delete(payload, field)
			action.Payload, _ = json.Marshal(payload)
			if got, err := provider.Apply(context.Background(), action); got.Kind != ObservationUnknown || !errors.Is(err, ErrEffectMismatch) || provider.ApplyCount(action.EffectKey.String) != 0 {
				t.Fatalf("incomplete %s=(%s,%v) applies=%d", field, got.Kind, err, provider.ApplyCount(action.EffectKey.String))
			}
		})
	}
}

func TestFakeReleaseProviderRollbackUsesANewGenerationAndCannotOverwriteNewerHead(t *testing.T) {
	predecessorID := uuid.New().String()
	action := fakeReleaseAction(t, "rollback_exact_release", 7, predecessorID)
	var payload map[string]any
	if err := json.Unmarshal(action.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["release_kind"] = "recovery"
	payload["recovery_of_release_id"] = predecessorID
	action.Payload, _ = json.Marshal(payload)

	provider := NewFakeReleaseProvider()
	provider.SeedTarget(action.WorkspaceID, "production", predecessorID, 7)
	if observed, err := provider.Observe(context.Background(), action); err != nil || observed.Kind != ObservationAbsent {
		t.Fatalf("rollback initial observe=(%s,%v)", observed.Kind, err)
	}
	applied, err := provider.Apply(context.Background(), action)
	if err != nil || applied.Kind != ObservationFound || provider.ApplyCount(action.EffectKey.String) != 1 {
		t.Fatalf("rollback apply=(%s,%v) count=%d", applied.Kind, err, provider.ApplyCount(action.EffectKey.String))
	}
	if replay, err := provider.Apply(context.Background(), action); err != nil || replay.Kind != ObservationFound || provider.ApplyCount(action.EffectKey.String) != 1 {
		t.Fatalf("rollback replay=(%s,%v) count=%d", replay.Kind, err, provider.ApplyCount(action.EffectKey.String))
	}

	stale := NewFakeReleaseProvider()
	stale.SeedTarget(action.WorkspaceID, "production", uuid.New().String(), 9)
	if got, err := stale.Apply(context.Background(), action); got.Kind != ObservationUnknown || !errors.Is(err, ErrEffectMismatch) || stale.ApplyCount(action.EffectKey.String) != 0 {
		t.Fatalf("stale rollback overwrote newer head=(%s,%v) applies=%d", got.Kind, err, stale.ApplyCount(action.EffectKey.String))
	}
}

func fakeReleaseAction(t *testing.T, actionType string, predecessor int64, predecessorID string) db.IssueWorkflowActionReceipt {
	t.Helper()
	id := uuid.New()
	workspaceID := uuid.New()
	operatorID, verifierID := uuid.New(), uuid.New()
	expected := map[string]int64{"outcome": 1, "scope": 1, "authority": 1, "execution": 1, "review": 1, "release": 1, "attention": 1, "evidence": 1, "budget": 1}
	requestHash := "sha256:command"
	payload, err := json.Marshal(map[string]any{
		"contract_version": 1, "effect_contract": "observe_before_apply_v1", "command_hash": requestHash,
		"operation_id": uuid.New().String(), "branch_key": "main", "release_id": uuid.New().String(), "release_revision": 1,
		"candidate_id": uuid.New().String(), "candidate_revision": 1, "scope_id": uuid.New().String(),
		"scope_revision": 1, "acceptance_version": 1, "review_id": uuid.New().String(), "review_revision": 1,
		"authority_id": uuid.New().String(), "authority_revision": 1, "authority_capability": "deploy_exact_release",
		"authority_envelope_digest": "sha256:authority", "authority_evidence_id": uuid.New().String(), "owner_evidence_revision": 1,
		"target_key":                 "production",
		"expected_target_release_id": predecessorID, "expected_target_generation": predecessor,
		"desired_target_generation": predecessor + 1, "release_operator_id": operatorID.String(), "planned_verifier_id": verifierID.String(),
		"build_manifest_digest": "sha256:manifest", "artifact_digest": "sha256:artifact", "runtime_contract_digest": "sha256:runtime",
		"exposure_policy": map[string]any{"mode": "canary"}, "audience_policy": map[string]any{"segment": "internal"},
		"risk_envelope": map[string]any{"level": "bounded"}, "exclusions": []string{}, "budget_envelope": map[string]any{"attempts": 1},
		"recovery_policy": map[string]any{"mode": "rollback"}, "recovery_policy_revision": 1, "rollback_target_release_id": predecessorID,
		"irreversible_effects": []string{}, "expected": expected,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db.IssueWorkflowActionReceipt{ID: pgtype.UUID{Bytes: id, Valid: true}, WorkspaceID: pgtype.UUID{Bytes: workspaceID, Valid: true}, IssueID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, ActionType: actionType, Payload: payload, RequestHash: requestHash, EffectKey: pgtype.Text{String: actionType + "/" + id.String(), Valid: true}, Revision: 2}
}
