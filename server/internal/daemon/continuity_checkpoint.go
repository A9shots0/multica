package daemon

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/multica-ai/multica/server/internal/continuitycheckpoint"
)

const (
	continuityCheckpointOpenTag  = "<continuity-checkpoint>"
	continuityCheckpointCloseTag = "</continuity-checkpoint>"
)

// Keep daemon wire names readable while sharing one canonical contract with
// the server completion transaction.
type ContinuityCheckpointPlan = continuitycheckpoint.Plan
type ContinuityCheckpointReceipt = continuitycheckpoint.Receipt

func validateContinuityCheckpointPlan(plan *ContinuityCheckpointPlan) error {
	if plan == nil {
		return errors.New("continuity checkpoint plan is missing")
	}
	return continuitycheckpoint.ValidatePlan(*plan)
}

func parseContinuityCheckpoint(output string, plan *ContinuityCheckpointPlan) (*ContinuityCheckpointReceipt, string, error) {
	openCount := strings.Count(output, continuityCheckpointOpenTag)
	closeCount := strings.Count(output, continuityCheckpointCloseTag)
	// A plan is an authenticated option, not an order to continue. A genuinely
	// complete goal stays a normal completion and remains byte-compatible.
	if openCount == 0 && closeCount == 0 {
		return nil, output, nil
	}
	if openCount != 1 || closeCount != 1 {
		return nil, "", errors.New("continuity checkpoint output must contain exactly one matched tagged receipt")
	}
	if err := validateContinuityCheckpointPlan(plan); err != nil {
		return nil, "", err
	}
	open := strings.Index(output, continuityCheckpointOpenTag)
	close := strings.Index(output, continuityCheckpointCloseTag)
	if close < open+len(continuityCheckpointOpenTag) {
		return nil, "", errors.New("continuity checkpoint tags are malformed")
	}
	contentStart := open + len(continuityCheckpointOpenTag)
	raw := strings.TrimSpace(output[contentStart:close])
	validated, err := continuitycheckpoint.DecodeAndValidate(json.RawMessage(raw), plan.PackageStep)
	if err != nil {
		return nil, "", err
	}
	if len(validated.Canonical) > plan.MaxCheckpointBytes {
		return nil, "", errors.New("continuity checkpoint canonical receipt exceeds its authenticated byte limit")
	}

	// The machine receipt must not become the owner-facing result. Preserve
	// only ordinary prose outside the single block.
	visible := strings.TrimSpace(output[:open] + output[close+len(continuityCheckpointCloseTag):])
	return &validated.Receipt, visible, nil
}

func parseTaskContinuityCheckpoint(output string, plan *ContinuityCheckpointPlan, measuredRequired bool) (*ContinuityCheckpointReceipt, string, error) {
	receipt, visible, err := parseContinuityCheckpoint(output, plan)
	if err == nil && measuredRequired && receipt == nil {
		return nil, "", errors.New("measured continuity checkpoint turn returned no checkpoint receipt")
	}
	return receipt, visible, err
}
