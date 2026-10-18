package handler

import (
	"encoding/json"
	"testing"
)

func TestExternalWorkInboxProjectionFiltersRoutineUpdates(t *testing.T) {
	working := map[string]any{
		"external_work": map[string]any{
			"provider": "codex", "work_id": "work-1", "receipt_revision": 1,
			"reported_state": "working", "change_since_last_receipt": "Checks are still running.",
		},
	}
	raw, _ := json.Marshal(working)
	update, external := externalWorkInboxProjection(raw)
	if !external || update != nil {
		t.Fatalf("working receipt = %#v, external=%t; want recognized and omitted", update, external)
	}
}

func TestExternalWorkInboxProjectionKeepsImportantProviderFacts(t *testing.T) {
	blocked := map[string]any{
		"external_work": map[string]any{
			"provider": "claude", "work_id": "work-2", "receipt_revision": 4,
			"reported_state": "blocked", "current_milestone": "Release",
			"change_since_last_receipt": "The release could not continue.",
			"dependencies":              []string{"The deployment account is unavailable."},
			"next_action":               "Restore the account, then retry the release.",
		},
	}
	raw, _ := json.Marshal(blocked)
	update, external := externalWorkInboxProjection(raw)
	if !external || update == nil {
		t.Fatalf("blocked receipt was not projected")
	}
	if update.Provider != "claude" || update.WorkID != "work-2" || update.ReportedState != "blocked" ||
		update.ReceiptRevision != 4 || len(update.Dependencies) != 1 || update.NextAction == "" {
		t.Fatalf("blocked projection lost provider facts: %#v", update)
	}
}

func TestExternalWorkInboxProjectionKeepsCompletionClaimWithoutInventingBlocker(t *testing.T) {
	waiting := map[string]any{
		"external_work": map[string]any{
			"provider": "codex", "work_id": "work-3", "receipt_revision": 5,
			"reported_state": "waiting_external", "current_milestone": "Provider reports finished",
			"change_since_last_receipt": "Codex reports that its bounded work is finished.",
			"dependencies":              []string{},
			"next_action":               "Independently verify the result before accepting the issue.",
		},
	}
	raw, _ := json.Marshal(waiting)
	update, external := externalWorkInboxProjection(raw)
	if !external || update == nil {
		t.Fatalf("provider completion claim was not projected")
	}
	if update.ReportedState != "waiting_external" || len(update.Dependencies) != 0 || update.NextAction == "" {
		t.Fatalf("provider completion projection changed its trust boundary: %#v", update)
	}
}
