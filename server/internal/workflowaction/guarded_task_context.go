package workflowaction

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

// AttachGuardedDispatchToTaskContextV1 is the typed producer boundary for a
// guarded workflow task context. It accepts only an exact v2 role-artifact
// binding; it never consults or falls back to historical configuration labels.
// The resulting context remains intentionally unexecutable until the
// controller supplies a separately qualified runtime-binding receipt.
func AttachGuardedDispatchToTaskContextV1(base json.RawMessage, envelope agentqualification.GuardedDispatchEnvelopeV1) (json.RawMessage, error) {
	contextFields, err := decodeTaskContextObjectV1(base)
	if err != nil {
		return nil, err
	}
	if _, exists := contextFields[agentqualification.GuardedDispatchTaskContextKey]; exists {
		return nil, fmt.Errorf("task context already contains guarded dispatch")
	}
	encodedEnvelope, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode guarded dispatch envelope: %w", err)
	}
	contextFields[agentqualification.GuardedDispatchTaskContextKey] = encodedEnvelope
	result, err := json.Marshal(contextFields)
	if err != nil {
		return nil, fmt.Errorf("encode guarded task context: %w", err)
	}
	expected := agentqualification.GuardedDispatchExpectedIdentityV1{
		WorkspaceID:     envelope.WorkspaceID,
		IssueID:         envelope.IssueID,
		ExecutorAgentID: envelope.ExecutorAgentID,
		RuntimeID:       envelope.RuntimeID,
	}
	_, admissionErr := agentqualification.ValidateGuardedTaskContextForExecutionV1(result, nil, expected, time.Now().UTC())
	if !errors.Is(admissionErr, agentqualification.ErrGuardedConfigurationUnbound) {
		if admissionErr == nil {
			return nil, fmt.Errorf("guarded task context unexpectedly gained execution authority")
		}
		return nil, fmt.Errorf("validate guarded task context: %w", admissionErr)
	}
	return result, nil
}

func normalizeGuardedTaskContextV1(sourcePayload json.RawMessage, contextFields map[string]any, expected agentqualification.GuardedDispatchExpectedIdentityV1) (json.RawMessage, error) {
	admission, err := agentqualification.ValidateGuardedTaskContextForExecutionV1(sourcePayload, nil, expected, time.Now().UTC())
	if err == nil {
		if admission.Mode != agentqualification.GuardedDispatchModeLegacy {
			return nil, fmt.Errorf("guarded source unexpectedly gained execution authority")
		}
		return json.Marshal(contextFields)
	}
	if !errors.Is(err, agentqualification.ErrGuardedConfigurationUnbound) {
		return nil, err
	}
	return AttachGuardedDispatchToTaskContextV1(mustMarshalTaskContextV1(contextFields), admission.Envelope)
}

func decodeTaskContextObjectV1(raw json.RawMessage) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value map[string]json.RawMessage
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("task context must be a JSON object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("task context has trailing JSON")
	}
	return value, nil
}

func mustMarshalTaskContextV1(value map[string]any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
