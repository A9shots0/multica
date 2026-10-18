package workflowaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowcommand"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// FakeReleaseProvider is a deterministic, process-local provider used to
// prove the release effect protocol without touching a hosted provider. It is
// intentionally not wired into the production server.
type FakeReleaseProvider struct {
	mu      sync.Mutex
	effects map[string]fakeReleaseRecord
	targets map[string]fakeReleaseTarget
	unknown map[string]bool
	apply   map[string]int
}

type fakeReleaseRecord struct {
	hash   string
	result []byte
}

type fakeReleaseTarget struct {
	releaseID  string
	generation int64
}

type fakeReleaseSnapshot struct {
	ContractVersion            int                                 `json:"contract_version"`
	EffectContract             string                              `json:"effect_contract"`
	CommandHash                string                              `json:"command_hash"`
	OperationID                string                              `json:"operation_id"`
	BranchKey                  string                              `json:"branch_key"`
	ReleaseID                  string                              `json:"release_id"`
	ReleaseKind                string                              `json:"release_kind,omitempty"`
	RecoveryOfReleaseID        string                              `json:"recovery_of_release_id,omitempty"`
	ReleaseRevision            int64                               `json:"release_revision"`
	CandidateID                string                              `json:"candidate_id"`
	CandidateRevision          int64                               `json:"candidate_revision"`
	ScopeID                    string                              `json:"scope_id"`
	ScopeRevision              int64                               `json:"scope_revision"`
	AcceptanceVersion          int64                               `json:"acceptance_version"`
	ReviewID                   string                              `json:"review_id"`
	ReviewRevision             int64                               `json:"review_revision"`
	AuthorityID                string                              `json:"authority_id"`
	AuthorityRevision          int64                               `json:"authority_revision"`
	AuthorityCapability        string                              `json:"authority_capability"`
	AuthorityEnvelopeDigest    string                              `json:"authority_envelope_digest"`
	AuthorityEvidenceID        string                              `json:"authority_evidence_id"`
	OwnerEvidenceRevision      int64                               `json:"owner_evidence_revision"`
	TargetKey                  string                              `json:"target_key"`
	ExpectedTargetReleaseID    string                              `json:"expected_target_release_id"`
	ExpectedTargetGeneration   int64                               `json:"expected_target_generation"`
	DesiredTargetGeneration    int64                               `json:"desired_target_generation"`
	ReleaseOperatorID          string                              `json:"release_operator_id"`
	PlannedVerifierID          string                              `json:"planned_verifier_id"`
	BuildManifestDigest        string                              `json:"build_manifest_digest"`
	ArtifactDigest             string                              `json:"artifact_digest"`
	RuntimeContractDigest      string                              `json:"runtime_contract_digest"`
	ExposurePolicy             map[string]any                      `json:"exposure_policy"`
	AudiencePolicy             map[string]any                      `json:"audience_policy"`
	RiskEnvelope               map[string]any                      `json:"risk_envelope"`
	Exclusions                 []string                            `json:"exclusions"`
	BudgetEnvelope             map[string]any                      `json:"budget_envelope"`
	RecoveryPolicy             map[string]any                      `json:"recovery_policy"`
	RecoveryPolicyRevision     int64                               `json:"recovery_policy_revision"`
	RollbackTargetReleaseID    string                              `json:"rollback_target_release_id"`
	KnownGoodCandidateID       string                              `json:"known_good_candidate_id,omitempty"`
	KnownGoodCandidateRevision int64                               `json:"known_good_candidate_revision,omitempty"`
	IrreversibleEffects        []string                            `json:"irreversible_effects"`
	Expected                   map[string]workflowstatev2.Revision `json:"expected"`
}

func NewFakeReleaseProvider() *FakeReleaseProvider {
	return &FakeReleaseProvider{effects: map[string]fakeReleaseRecord{}, targets: map[string]fakeReleaseTarget{}, unknown: map[string]bool{}, apply: map[string]int{}}
}

func (provider *FakeReleaseProvider) SetUnknown(effectKey string, unknown bool) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.ensure()
	provider.unknown[effectKey] = unknown
}

func (provider *FakeReleaseProvider) ApplyCount(effectKey string) int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.apply[effectKey]
}

func (provider *FakeReleaseProvider) SeedTarget(workspaceID pgtype.UUID, targetKey, releaseID string, generation int64) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.ensure()
	provider.targets[releaseProviderTargetKey(workspaceID, targetKey)] = fakeReleaseTarget{releaseID: releaseID, generation: generation}
}

func (provider *FakeReleaseProvider) Observe(_ context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.ensure()
	if provider.unknown[action.EffectKey.String] {
		return Observation{Kind: ObservationUnknown}, ErrEffectUnknown
	}
	record, ok := provider.effects[action.EffectKey.String]
	if !ok {
		return Observation{Kind: ObservationAbsent}, nil
	}
	hash := releaseEffectHash(action)
	if record.hash != hash {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	return Observation{Kind: ObservationFound, ReceiptRevision: action.Revision, Result: append([]byte(nil), record.result...)}, nil
}

func (provider *FakeReleaseProvider) Apply(_ context.Context, action db.IssueWorkflowActionReceipt) (Observation, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.ensure()
	key := action.EffectKey.String
	if provider.unknown[key] {
		return Observation{Kind: ObservationUnknown}, ErrEffectUnknown
	}
	hash := releaseEffectHash(action)
	if record, ok := provider.effects[key]; ok {
		if record.hash != hash {
			return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
		}
		return Observation{Kind: ObservationFound, ReceiptRevision: action.Revision, Result: append([]byte(nil), record.result...)}, nil
	}
	var snapshot fakeReleaseSnapshot
	decoder := json.NewDecoder(bytes.NewReader(action.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&snapshot) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validFakeReleaseSnapshot(action, snapshot) {
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	targetKey := releaseProviderTargetKey(action.WorkspaceID, snapshot.TargetKey)
	current, exists := provider.targets[targetKey]
	switch action.ActionType {
	case "deploy_exact_release":
		if snapshot.ExpectedTargetGeneration == 0 {
			if exists {
				return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
			}
		} else if !exists || current.generation != snapshot.ExpectedTargetGeneration || current.releaseID != snapshot.ExpectedTargetReleaseID {
			return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
		}
		provider.targets[targetKey] = fakeReleaseTarget{releaseID: snapshot.ReleaseID, generation: snapshot.DesiredTargetGeneration}
	case "expose_exact_release":
		if !exists || current.generation != snapshot.DesiredTargetGeneration || current.releaseID != snapshot.ReleaseID {
			return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
		}
	case "rollback_exact_release":
		if snapshot.ReleaseKind != "recovery" || snapshot.RecoveryOfReleaseID == "" || snapshot.RecoveryOfReleaseID != snapshot.ExpectedTargetReleaseID || !exists || current.generation != snapshot.ExpectedTargetGeneration || current.releaseID != snapshot.ExpectedTargetReleaseID {
			return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
		}
		provider.targets[targetKey] = fakeReleaseTarget{releaseID: snapshot.ReleaseID, generation: snapshot.DesiredTargetGeneration}
	default:
		return Observation{Kind: ObservationUnknown}, ErrEffectMismatch
	}
	provider.apply[key]++
	typ, evidence := releaseObservationShape(action.ActionType)
	result, _ := json.Marshal(workflowcommand.ReleaseEffectResultInput{
		Type: typ, EffectKey: key, ProviderReceiptID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(key)).String(),
		ReleaseID: snapshot.ReleaseID, CandidateID: snapshot.CandidateID, CandidateRevision: snapshot.CandidateRevision,
		ScopeID: snapshot.ScopeID, ScopeRevision: snapshot.ScopeRevision, AcceptanceVersion: snapshot.AcceptanceVersion,
		OperationID: snapshot.OperationID, BranchKey: snapshot.BranchKey, TargetKey: snapshot.TargetKey,
		TargetGeneration: snapshot.DesiredTargetGeneration, ArtifactDigest: snapshot.ArtifactDigest,
		RuntimeContractDigest: snapshot.RuntimeContractDigest, Evidence: evidence,
	})
	provider.effects[key] = fakeReleaseRecord{hash: hash, result: result}
	return Observation{Kind: ObservationFound, ReceiptRevision: action.Revision, Result: result}, nil
}

func validFakeReleaseSnapshot(action db.IssueWorkflowActionReceipt, snapshot fakeReleaseSnapshot) bool {
	if !action.EffectKey.Valid || snapshot.ContractVersion != 1 || snapshot.EffectContract != "observe_before_apply_v1" || snapshot.CommandHash == "" || snapshot.CommandHash != action.RequestHash ||
		snapshot.OperationID == "" || snapshot.BranchKey == "" || snapshot.ReleaseID == "" || snapshot.ReleaseRevision < 1 || snapshot.CandidateID == "" || snapshot.CandidateRevision < 1 || snapshot.ScopeID == "" || snapshot.ScopeRevision < 1 || snapshot.AcceptanceVersion < 1 ||
		snapshot.ReviewID == "" || snapshot.ReviewRevision < 1 || snapshot.AuthorityID == "" || snapshot.AuthorityRevision < 1 || snapshot.AuthorityCapability != "deploy_exact_release" || snapshot.AuthorityEnvelopeDigest == "" || snapshot.AuthorityEvidenceID == "" || snapshot.OwnerEvidenceRevision < 1 ||
		snapshot.TargetKey == "" || snapshot.DesiredTargetGeneration != snapshot.ExpectedTargetGeneration+1 || snapshot.ReleaseOperatorID == "" || snapshot.PlannedVerifierID == "" || snapshot.ReleaseOperatorID == snapshot.PlannedVerifierID ||
		snapshot.BuildManifestDigest == "" || snapshot.ArtifactDigest == "" || snapshot.RuntimeContractDigest == "" || len(snapshot.ExposurePolicy) == 0 || len(snapshot.AudiencePolicy) == 0 || len(snapshot.RiskEnvelope) == 0 || len(snapshot.BudgetEnvelope) == 0 || len(snapshot.RecoveryPolicy) == 0 || snapshot.RecoveryPolicyRevision < 1 || len(snapshot.Expected) != 9 {
		return false
	}
	if snapshot.ExpectedTargetGeneration == 0 {
		return strings.TrimSpace(snapshot.ExpectedTargetReleaseID) == ""
	}
	return strings.TrimSpace(snapshot.ExpectedTargetReleaseID) != ""
}

func (provider *FakeReleaseProvider) ensure() {
	if provider.effects == nil {
		provider.effects = map[string]fakeReleaseRecord{}
	}
	if provider.targets == nil {
		provider.targets = map[string]fakeReleaseTarget{}
	}
	if provider.unknown == nil {
		provider.unknown = map[string]bool{}
	}
	if provider.apply == nil {
		provider.apply = map[string]int{}
	}
}

func releaseEffectHash(action db.IssueWorkflowActionReceipt) string {
	sum := sha256.Sum256(append([]byte(action.ActionType+"/"+action.EffectKey.String+"/"), action.Payload...))
	return hex.EncodeToString(sum[:])
}

func releaseObservationShape(actionType string) (string, map[string]any) {
	switch actionType {
	case "deploy_exact_release":
		return "deployment", map[string]any{"provider_state": "deployed", "receipt_verified": true}
	case "expose_exact_release":
		return "exposure", map[string]any{"exposure_state": "exposed", "audience_verified": true}
	case "rollback_exact_release":
		return "rollback", map[string]any{"provider_state": "rolled_back", "receipt_verified": true}
	default:
		return "", nil
	}
}

func releaseProviderTargetKey(workspaceID pgtype.UUID, targetKey string) string {
	return uuid.UUID(workspaceID.Bytes).String() + "\x00" + targetKey
}
