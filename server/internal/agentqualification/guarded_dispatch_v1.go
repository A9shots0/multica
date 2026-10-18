package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

const (
	GuardedDispatchEnvelopeVersionV2 = "silver-mediums.guarded-dispatch.v2"
	QualifiedRuntimeBindingVersionV1 = "silver-mediums.qualified-runtime-binding.v1"
	QualifiedRuntimeBindingVersionV2 = "silver-mediums.qualified-runtime-binding.v2"
	GuardedDispatchTaskContextKey    = "guarded_dispatch"
	GuardedDispatchFailureInvalid    = "guarded_dispatch_invalid"
	GuardedConfigurationUnbound      = "guarded_configuration_unbound"
	GuardedAuthorityUnavailable      = "guarded_execution_authority_unavailable"
	GuardedDispatchModeLegacy        = "legacy"
	GuardedDispatchModeGuarded       = "guarded"
	qualifiedRuntimeBindingSourceV2  = "controller_record"
)

// ValidateGuardedTaskContextV1 extracts the one reserved guarded-dispatch key
// without imposing a new schema on historical task context. Empty, non-object,
// or malformed legacy context stays legacy when the reserved key was never
// decoded. Once that key is observed, duplicate, malformed, or trailing context
// fails closed rather than silently downgrading a guarded task to legacy.
func ValidateGuardedTaskContextV1(rawTaskContext json.RawMessage, runtimeBinding *QualifiedRuntimeBindingReceiptV2, now time.Time) (GuardedDispatchAdmissionV1, error) {
	legacy := GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeLegacy}
	if len(bytes.TrimSpace(rawTaskContext)) == 0 {
		return legacy, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(rawTaskContext))
	first, err := decoder.Token()
	if err != nil {
		return legacy, nil
	}
	delim, ok := first.(json.Delim)
	if !ok || delim != '{' {
		return legacy, nil
	}
	var guarded json.RawMessage
	guardedFound := false
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		if keyErr != nil {
			if guardedFound {
				return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: malformed task context after reserved key", ErrGuardedDispatchInvalid)
			}
			return legacy, nil
		}
		key, keyOK := keyToken.(string)
		if !keyOK {
			if guardedFound {
				return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: malformed task context key", ErrGuardedDispatchInvalid)
			}
			return legacy, nil
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			if guardedFound || key == GuardedDispatchTaskContextKey {
				return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: malformed reserved task context", ErrGuardedDispatchInvalid)
			}
			return legacy, nil
		}
		if key != GuardedDispatchTaskContextKey {
			continue
		}
		if guardedFound {
			return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: duplicate reserved task context key", ErrGuardedDispatchInvalid)
		}
		guardedFound = true
		guarded = append(guarded[:0], value...)
	}
	if _, err := decoder.Token(); err != nil {
		if guardedFound {
			return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: unterminated guarded task context", ErrGuardedDispatchInvalid)
		}
		return legacy, nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if guardedFound {
			return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, fmt.Errorf("%w: trailing guarded task context", ErrGuardedDispatchInvalid)
		}
		return legacy, nil
	}
	if !guardedFound {
		return legacy, nil
	}
	return ValidateGuardedDispatchV1(guarded, runtimeBinding, now)
}

var (
	ErrGuardedDispatchInvalid      = errors.New(GuardedDispatchFailureInvalid)
	ErrGuardedConfigurationUnbound = errors.New(GuardedConfigurationUnbound)
	ErrGuardedAuthorityUnavailable = errors.New(GuardedAuthorityUnavailable)
)

// GuardedDispatchEnvelopeV1 is the first implementation of the opt-in task
// envelope. Version v2 deliberately separates immutable role artifacts from
// runtime qualification: matching prompt files cannot qualify a model/runtime.
type GuardedDispatchEnvelopeV1 struct {
	Version                 string                `json:"version"`
	WorkspaceID             string                `json:"workspace_id"`
	IssueID                 string                `json:"issue_id"`
	ConfigurationID         string                `json:"configuration_id"`
	RoleID                  string                `json:"role_id"`
	ExecutorAgentID         string                `json:"executor_agent_id"`
	RuntimeID               string                `json:"runtime_id"`
	RuntimeProvider         string                `json:"runtime_provider"`
	Model                   string                `json:"model"`
	ReasoningEffort         string                `json:"reasoning_effort"`
	ArtifactBindingID       string                `json:"artifact_binding_id"`
	ArtifactBinding         RoleArtifactBindingV2 `json:"artifact_binding"`
	RuntimeBindingReceiptID string                `json:"runtime_binding_receipt_id"`
}

// QualifiedRuntimeBindingReceiptV2 is separate durable controller evidence
// that one exact runtime configuration and child-execution boundary have
// qualified. Version v1 predated ChildExecution and remains intentionally
// ineligible; changing its meaning in place would let an old stored receipt
// masquerade as evidence for a boundary it never evaluated.
type QualifiedRuntimeBindingReceiptV2 struct {
	Version               string                           `json:"version"`
	ID                    string                           `json:"id"`
	WorkspaceID           string                           `json:"workspace_id"`
	IssueID               string                           `json:"issue_id"`
	ExecutorAgentID       string                           `json:"executor_agent_id"`
	ConfigurationID       string                           `json:"configuration_id"`
	RoleID                string                           `json:"role_id"`
	RuntimeID             string                           `json:"runtime_id"`
	RuntimeProvider       string                           `json:"runtime_provider"`
	Model                 string                           `json:"model"`
	ReasoningEffort       string                           `json:"reasoning_effort"`
	ChildExecution        GuardedChildExecutionBoundaryV1  `json:"child_execution"`
	WorkflowBroker        *GuardedWorkflowBrokerTemplateV1 `json:"workflow_broker,omitempty"`
	ArtifactBindingSHA256 string                           `json:"artifact_binding_sha256"`
	QualificationStatus   string                           `json:"qualification_status"`
	EvidenceReceiptIDs    []string                         `json:"evidence_receipt_ids"`
	Source                string                           `json:"source"`
	Immutable             bool                             `json:"immutable"`
	Revoked               bool                             `json:"revoked"`
	IssuedAt              time.Time                        `json:"issued_at"`
	ExpiresAt             time.Time                        `json:"expires_at"`
}

// GuardedDispatchExpectedIdentityV1 is authoritative execution context, not
// task-supplied envelope data. Callers derive it from the claimed task/runtime.
type GuardedDispatchExpectedIdentityV1 struct {
	WorkspaceID     string
	IssueID         string
	ExecutorAgentID string
	RuntimeID       string
}

type GuardedDispatchAdmissionV1 struct {
	Mode           string
	Envelope       GuardedDispatchEnvelopeV1
	Artifact       RoleArtifactBindingV2
	RuntimeBinding QualifiedRuntimeBindingReceiptV2
}

// ValidateGuardedTaskContextForExecutionV1 adds actual task identity to the
// base envelope and receipt checks. Historical context remains legacy.
func ValidateGuardedTaskContextForExecutionV1(rawTaskContext json.RawMessage, runtimeBinding *QualifiedRuntimeBindingReceiptV2, expected GuardedDispatchExpectedIdentityV1, now time.Time) (GuardedDispatchAdmissionV1, error) {
	admission, validationErr := ValidateGuardedTaskContextV1(rawTaskContext, runtimeBinding, now)
	if admission.Mode == GuardedDispatchModeLegacy || (validationErr != nil && !errors.Is(validationErr, ErrGuardedConfigurationUnbound)) {
		return admission, validationErr
	}
	if strings.TrimSpace(expected.WorkspaceID) == "" || strings.TrimSpace(expected.IssueID) == "" || strings.TrimSpace(expected.ExecutorAgentID) == "" || strings.TrimSpace(expected.RuntimeID) == "" {
		return admission, fmt.Errorf("%w: incomplete expected execution identity", ErrGuardedDispatchInvalid)
	}
	envelope := admission.Envelope
	if envelope.WorkspaceID != expected.WorkspaceID || envelope.IssueID != expected.IssueID || envelope.ExecutorAgentID != expected.ExecutorAgentID || envelope.RuntimeID != expected.RuntimeID {
		return admission, fmt.Errorf("%w: guarded envelope does not match execution identity", ErrGuardedDispatchInvalid)
	}
	return admission, validationErr
}

// ValidateGuardedDispatchV1 preserves legacy behavior only when the envelope
// is absent. Any present envelope is strict, versioned, artifact-bound, and
// requires a separate qualified runtime receipt.
func ValidateGuardedDispatchV1(raw json.RawMessage, runtimeBinding *QualifiedRuntimeBindingReceiptV2, now time.Time) (GuardedDispatchAdmissionV1, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeLegacy}, nil
	}

	envelope, err := decodeGuardedDispatchEnvelopeV1(raw)
	if err != nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded}, err
	}
	artifact, err := ValidatedRoleArtifactBindingV2(envelope.ArtifactBindingID)
	if err != nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded, Envelope: envelope}, fmt.Errorf("%w: artifact binding: %v", ErrGuardedDispatchInvalid, err)
	}
	if envelope.RoleID != artifact.RoleID || envelope.ArtifactBindingID != artifact.BindingID || !reflect.DeepEqual(envelope.ArtifactBinding, artifact) {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded, Envelope: envelope}, fmt.Errorf("%w: exact role artifact binding mismatch", ErrGuardedDispatchInvalid)
	}
	if runtimeBinding == nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded, Envelope: envelope, Artifact: artifact}, ErrGuardedConfigurationUnbound
	}
	artifactHash, err := roleArtifactBindingSHA256V2(artifact)
	if err != nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded, Envelope: envelope, Artifact: artifact}, fmt.Errorf("%w: hash artifact binding: %v", ErrGuardedDispatchInvalid, err)
	}
	if err := validateQualifiedRuntimeBindingV2(envelope, *runtimeBinding, artifactHash, now); err != nil {
		return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded, Envelope: envelope, Artifact: artifact}, err
	}
	return GuardedDispatchAdmissionV1{Mode: GuardedDispatchModeGuarded, Envelope: envelope, Artifact: artifact, RuntimeBinding: *runtimeBinding}, nil
}

func decodeGuardedDispatchEnvelopeV1(raw json.RawMessage) (GuardedDispatchEnvelopeV1, error) {
	var envelope GuardedDispatchEnvelopeV1
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return envelope, fmt.Errorf("%w: decode envelope: %v", ErrGuardedDispatchInvalid, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return envelope, fmt.Errorf("%w: trailing envelope data", ErrGuardedDispatchInvalid)
	}
	if envelope.Version != GuardedDispatchEnvelopeVersionV2 {
		return envelope, fmt.Errorf("%w: unsupported envelope version", ErrGuardedDispatchInvalid)
	}
	if strings.TrimSpace(envelope.WorkspaceID) == "" || strings.TrimSpace(envelope.IssueID) == "" || strings.TrimSpace(envelope.ConfigurationID) == "" || strings.TrimSpace(envelope.RoleID) == "" || strings.TrimSpace(envelope.ExecutorAgentID) == "" || strings.TrimSpace(envelope.RuntimeID) == "" || strings.TrimSpace(envelope.RuntimeProvider) == "" || strings.TrimSpace(envelope.Model) == "" || strings.TrimSpace(envelope.ReasoningEffort) == "" || strings.TrimSpace(envelope.ArtifactBindingID) == "" || strings.TrimSpace(envelope.RuntimeBindingReceiptID) == "" {
		return envelope, fmt.Errorf("%w: incomplete guarded envelope", ErrGuardedDispatchInvalid)
	}
	return envelope, nil
}

func validateQualifiedRuntimeBindingV2(envelope GuardedDispatchEnvelopeV1, receipt QualifiedRuntimeBindingReceiptV2, artifactHash string, now time.Time) error {
	if receipt.Version != QualifiedRuntimeBindingVersionV2 || receipt.ID == "" || receipt.QualificationStatus != StatusQualifiedBounded || receipt.Source != qualifiedRuntimeBindingSourceV2 || !receipt.Immutable || receipt.Revoked || len(receipt.EvidenceReceiptIDs) == 0 || !now.Before(receipt.ExpiresAt) || now.Before(receipt.IssuedAt) {
		return ErrGuardedConfigurationUnbound
	}
	if err := validateGuardedChildExecutionBoundaryV1(receipt.ChildExecution); err != nil {
		return err
	}
	if err := validateGuardedChildExecutionProviderV1(receipt.ChildExecution, receipt.RuntimeProvider); err != nil {
		return err
	}
	if receipt.ID != envelope.RuntimeBindingReceiptID || receipt.WorkspaceID != envelope.WorkspaceID || receipt.IssueID != envelope.IssueID || receipt.ExecutorAgentID != envelope.ExecutorAgentID || receipt.ConfigurationID != envelope.ConfigurationID || receipt.RoleID != envelope.RoleID || receipt.RuntimeID != envelope.RuntimeID || receipt.RuntimeProvider != envelope.RuntimeProvider || receipt.Model != envelope.Model || receipt.ReasoningEffort != envelope.ReasoningEffort || receipt.ArtifactBindingSHA256 != artifactHash {
		return ErrGuardedConfigurationUnbound
	}
	return nil
}

func roleArtifactBindingSHA256V2(binding RoleArtifactBindingV2) (string, error) {
	canonical, err := json.Marshal(binding)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}
