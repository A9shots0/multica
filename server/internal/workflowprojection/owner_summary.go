package workflowprojection

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const OwnerIssueSummarySchemaVersion int32 = 1

type OwnerIssueStage string

const (
	OwnerStageNeedsOwner        OwnerIssueStage = "needs_owner"
	OwnerStageRecovering        OwnerIssueStage = "recovering"
	OwnerStageTechnicalStop     OwnerIssueStage = "technical_stop"
	OwnerStageBlocked           OwnerIssueStage = "blocked"
	OwnerStageVerifying         OwnerIssueStage = "verifying"
	OwnerStageReleasing         OwnerIssueStage = "releasing"
	OwnerStageInReview          OwnerIssueStage = "in_review"
	OwnerStageActive            OwnerIssueStage = "active"
	OwnerStageIncoming          OwnerIssueStage = "incoming"
	OwnerStageComplete          OwnerIssueStage = "complete"
	OwnerStageStatusUnavailable OwnerIssueStage = "status_unavailable"
)

type OwnerIssueReference struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type OwnerIssueSignal struct {
	Code       string                `json:"code"`
	References []OwnerIssueReference `json:"references"`
}

type OwnerIssueNextAction struct {
	Kind      string `json:"kind"`
	Automatic bool   `json:"automatic"`
	ActorType string `json:"actor_type,omitempty"`
	ActorID   string `json:"actor_id,omitempty"`
}

// OwnerIssueSummary is the stable, bounded owner-facing projection. It carries
// codes and authoritative references, never mutable prose inferred by a client.
type OwnerIssueSummary struct {
	SchemaVersion          int32                  `json:"schema_version"`
	CurrentPictureRevision int64                  `json:"current_picture_revision"`
	Stage                  OwnerIssueStage        `json:"stage"`
	Signals                []OwnerIssueSignal     `json:"signals"`
	NextAction             *OwnerIssueNextAction  `json:"next_action"`
	LatestMeaningfulChange *OwnerMeaningfulChange `json:"latest_meaningful_change"`
}

type OwnerMeaningfulChange struct {
	Code       string              `json:"code"`
	ResultCode string              `json:"result_code"`
	OccurredAt string              `json:"occurred_at"`
	Reference  OwnerIssueReference `json:"reference"`
}

type ownerSummaryInput struct {
	ContractVersion        int32
	ReleaseStatus          *ReleaseStatus
	Revision               int64
	Outcome                map[string]any
	Scope                  map[string]any
	Execution              map[string]any
	Release                map[string]any
	Health                 string
	Attention              map[string]any
	UnresolvedDecisions    []map[string]any
	UnresolvedIncidents    []map[string]any
	UnresolvedDependencies []map[string]any
	NextAutomaticAction    map[string]any
	ActiveActor            map[string]any
	LatestMeaningfulChange map[string]any
	TerminalReason         string
	GeneratedAt            time.Time
}

// OwnerIssueSummaryFromDocument derives the bounded owner contract only from a
// projector-marked semantic document. Older or malformed documents are never
// interpreted through legacy status heuristics.
func OwnerIssueSummaryFromDocument(raw []byte, revision int64, generatedAt time.Time) OwnerIssueSummary {
	var document struct {
		ContractVersion           int32            `json:"contract_version"`
		OwnerSummarySchemaVersion int32            `json:"owner_summary_schema_version"`
		ReleaseStatus             *ReleaseStatus   `json:"release_status"`
		Outcome                   map[string]any   `json:"outcome"`
		Scope                     map[string]any   `json:"scope"`
		Execution                 map[string]any   `json:"execution"`
		Release                   map[string]any   `json:"release"`
		Health                    string           `json:"health"`
		Attention                 map[string]any   `json:"attention"`
		UnresolvedDecisions       []map[string]any `json:"unresolved_decisions"`
		UnresolvedIncidents       []map[string]any `json:"unresolved_incidents"`
		UnresolvedDependencies    []map[string]any `json:"unresolved_dependencies"`
		NextAutomaticAction       map[string]any   `json:"next_automatic_action"`
		ActiveActor               map[string]any   `json:"active_actor"`
		LatestMeaningfulChange    map[string]any   `json:"latest_meaningful_change"`
		TerminalReason            string           `json:"terminal_reason"`
	}
	if json.Unmarshal(raw, &document) != nil {
		return unavailableOwnerSummary(revision)
	}
	if generatedAt.IsZero() || (document.ContractVersion != 2 && document.ContractVersion != 3) ||
		document.OwnerSummarySchemaVersion != OwnerIssueSummarySchemaVersion {
		return unavailableOwnerSummary(revision)
	}
	return buildOwnerIssueSummary(ownerSummaryInput{
		ContractVersion: document.ContractVersion, ReleaseStatus: document.ReleaseStatus,
		Revision: revision, Outcome: document.Outcome, Scope: document.Scope,
		Execution: document.Execution, Release: document.Release, Health: document.Health,
		Attention: document.Attention, UnresolvedDecisions: document.UnresolvedDecisions,
		UnresolvedIncidents: document.UnresolvedIncidents, UnresolvedDependencies: document.UnresolvedDependencies,
		NextAutomaticAction: document.NextAutomaticAction, ActiveActor: document.ActiveActor,
		LatestMeaningfulChange: document.LatestMeaningfulChange, TerminalReason: document.TerminalReason,
		GeneratedAt: generatedAt,
	})
}

func unavailableOwnerSummary(revision int64) OwnerIssueSummary {
	return OwnerIssueSummary{
		SchemaVersion: OwnerIssueSummarySchemaVersion, CurrentPictureRevision: revision,
		Stage: OwnerStageStatusUnavailable, Signals: []OwnerIssueSignal{{Code: "projection_unavailable", References: []OwnerIssueReference{}}},
	}
}

// NormalizeOwnerIssueSummary rejects partial, future, or contradictory summary
// values without attempting to infer a replacement stage from legacy columns.
func NormalizeOwnerIssueSummary(summary OwnerIssueSummary, revision int64) OwnerIssueSummary {
	if summary.SchemaVersion != OwnerIssueSummarySchemaVersion || revision < 1 ||
		summary.CurrentPictureRevision != revision || !knownOwnerStage(summary.Stage) || summary.Signals == nil {
		return unavailableOwnerSummary(revision)
	}
	for _, signal := range summary.Signals {
		if !knownOwnerSignal(signal.Code) || signal.References == nil {
			return unavailableOwnerSummary(revision)
		}
		for _, ref := range signal.References {
			if strings.TrimSpace(ref.Type) == "" || strings.TrimSpace(ref.ID) == "" || ref.Revision < 1 {
				return unavailableOwnerSummary(revision)
			}
		}
	}
	if summary.NextAction != nil && (!knownOwnerAction(summary.NextAction.Kind) ||
		(summary.NextAction.Kind == "answer_decision" && (summary.NextAction.Automatic || summary.NextAction.ActorType != "owner")) ||
		(summary.NextAction.Kind == "continue_workflow" && !summary.NextAction.Automatic) ||
		(summary.NextAction.Kind == "inspect_incident" && (summary.NextAction.Automatic || summary.NextAction.ActorType != "owner"))) {
		return unavailableOwnerSummary(revision)
	}
	if change := summary.LatestMeaningfulChange; change != nil &&
		(!knownMeaningfulChange(change.Code) || !knownMeaningfulResult(change.ResultCode) ||
			!validRFC3339(change.OccurredAt) || strings.TrimSpace(change.Reference.Type) == "" ||
			strings.TrimSpace(change.Reference.ID) == "" || change.Reference.Revision < 1) {
		return unavailableOwnerSummary(revision)
	}
	codes := make(map[string]bool, len(summary.Signals))
	for _, signal := range summary.Signals {
		if codes[signal.Code] || !ownerSignalAllowed(summary.Stage, signal.Code) {
			return unavailableOwnerSummary(revision)
		}
		codes[signal.Code] = true
	}
	requires := func(code string) bool { return codes[code] }
	switch summary.Stage {
	case OwnerStageStatusUnavailable:
		if !requires("projection_unavailable") || summary.NextAction != nil {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageNeedsOwner:
		if !requires("owner_attention_required") || summary.NextAction == nil || summary.NextAction.Kind != "answer_decision" || summary.NextAction.Automatic || summary.NextAction.ActorType != "owner" {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageRecovering:
		if !requires("recovery_active") {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageTechnicalStop:
		if !requires("technical_stop") || summary.NextAction == nil || summary.NextAction.Kind != "inspect_incident" || summary.NextAction.Automatic || summary.NextAction.ActorType != "owner" {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageBlocked:
		if !requires("work_blocked") {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageVerifying:
		if !requires("verification_pending") {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageReleasing:
		if !requires("release_in_progress") {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageInReview:
		if !requires("independent_review") {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageActive:
		if !requires("work_active") && !requires("waiting_automatic") {
			return unavailableOwnerSummary(revision)
		}
		if requires("waiting_automatic") && (summary.NextAction == nil || summary.NextAction.Kind != "continue_workflow" || !summary.NextAction.Automatic) {
			return unavailableOwnerSummary(revision)
		}
	case OwnerStageComplete:
		if !requires("outcome_complete") || (!requires("outcome_done") && !requires("verified_live")) || summary.NextAction != nil {
			return unavailableOwnerSummary(revision)
		}
	}
	return summary
}

func knownOwnerAction(kind string) bool {
	return kind == "answer_decision" || kind == "continue_workflow" || kind == "inspect_incident"
}

func ownerSignalAllowed(stage OwnerIssueStage, code string) bool {
	allowed := map[OwnerIssueStage]map[string]bool{
		OwnerStageStatusUnavailable: {"projection_unavailable": true},
		OwnerStageTechnicalStop:     {"technical_stop": true, "current_incident": true, "owner_attention_required": true, "external_dependency": true},
		OwnerStageRecovering:        {"recovery_active": true, "current_incident": true, "current_regression": true, "rollback_in_progress": true, "service_restored_outcome_open": true, "owner_attention_required": true, "external_dependency": true},
		OwnerStageNeedsOwner:        {"owner_attention_required": true, "external_dependency": true},
		OwnerStageBlocked:           {"work_blocked": true, "external_dependency": true},
		OwnerStageVerifying:         {"verification_pending": true},
		OwnerStageReleasing:         {"release_in_progress": true},
		OwnerStageInReview:          {"independent_review": true},
		OwnerStageActive:            {"work_active": true, "waiting_automatic": true},
		OwnerStageIncoming:          {},
		OwnerStageComplete:          {"outcome_complete": true, "outcome_done": true, "verified_live": true},
	}
	return allowed[stage][code]
}

func knownOwnerStage(stage OwnerIssueStage) bool {
	switch stage {
	case OwnerStageNeedsOwner, OwnerStageRecovering, OwnerStageTechnicalStop, OwnerStageBlocked,
		OwnerStageVerifying, OwnerStageReleasing, OwnerStageInReview, OwnerStageActive,
		OwnerStageIncoming, OwnerStageComplete, OwnerStageStatusUnavailable:
		return true
	default:
		return false
	}
}

func knownOwnerSignal(code string) bool {
	switch code {
	case "projection_unavailable", "owner_attention_required", "current_incident", "external_dependency",
		"recovery_active", "technical_stop", "work_blocked", "verification_pending",
		"release_in_progress", "independent_review", "automatic_continuation", "work_active", "waiting_automatic",
		"outcome_complete", "outcome_done", "verified_live", "current_regression", "rollback_in_progress", "service_restored_outcome_open":
		return true
	default:
		return false
	}
}

func buildOwnerIssueSummary(input ownerSummaryInput) OwnerIssueSummary {
	if err := validateOwnerSummaryInput(input); err != nil {
		return unavailableOwnerSummary(input.Revision)
	}

	signals := make([]OwnerIssueSignal, 0, 8)
	addSignal := func(code string, refs ...OwnerIssueReference) {
		signals = append(signals, OwnerIssueSignal{Code: code, References: refs})
	}

	decisionRefs := references("decision", input.UnresolvedDecisions)
	incidentRefs := references("incident", input.UnresolvedIncidents)
	dependencyRefs := references("execution", input.UnresolvedDependencies)
	if len(decisionRefs) != 0 {
		addSignal("owner_attention_required", decisionRefs...)
	}
	if len(incidentRefs) != 0 {
		addSignal("current_incident", incidentRefs...)
	}
	if len(dependencyRefs) != 0 {
		addSignal("external_dependency", dependencyRefs...)
	}

	outcomeState := requiredString(input.Outcome, "state")
	executionState := requiredString(input.Execution, "state")
	releaseState := optionalStringValue(input.Release, "state")
	stage := OwnerStageStatusUnavailable
	switch {
	case technicallyStopped(executionState, releaseState, input.TerminalReason) || releasePhase(input) == "technical_stop":
		stage = OwnerStageTechnicalStop
		addSignal("technical_stop", referencesFromMap("execution", input.Execution)...)
	case recoveryCurrent(releasePhase(input)):
		stage = OwnerStageRecovering
		addSignal("recovery_active", referencesFromMap("release", input.Release)...)
		switch releasePhase(input) {
		case "regressed":
			addSignal("current_regression", referencesFromMap("release", input.Release)...)
		case "rolling_back":
			addSignal("rollback_in_progress", referencesFromMap("release", input.Release)...)
		case "rolled_back":
			addSignal("service_restored_outcome_open", referencesFromMap("release", input.Release)...)
		}
	case len(input.UnresolvedDecisions) != 0:
		stage = OwnerStageNeedsOwner
	case executionState == "blocked_external" || len(input.UnresolvedDependencies) != 0:
		stage = OwnerStageBlocked
		addSignal("work_blocked", dependencyRefs...)
	case releasePhase(input) == "deployed_checking":
		stage = OwnerStageVerifying
		addSignal("verification_pending", referencesFromMap("release", input.Release)...)
	case releasePhase(input) == "releasing":
		stage = OwnerStageReleasing
		addSignal("release_in_progress", referencesFromMap("release", input.Release)...)
	case executionState == "in_review" || executionState == "reviewing":
		stage = OwnerStageInReview
		addSignal("independent_review", referencesFromMap("execution", input.Execution)...)
	case viableContinuation(input):
		stage = OwnerStageActive
		if requiredString(input.Execution, "state") == "waiting_automatic" {
			addSignal("waiting_automatic", referencesFromMap("execution", input.Execution)...)
		} else {
			addSignal("work_active", referencesFromMap("execution", input.Execution)...)
		}
	case incomingExecution(executionState):
		stage = OwnerStageIncoming
	case exactCompletion(input, outcomeState):
		stage = OwnerStageComplete
		addSignal("outcome_complete", referencesFromMap("outcome", input.Outcome)...)
		if releasePhase(input) == "verified_live" {
			addSignal("verified_live", referencesFromMap("release", input.Release)...)
		} else {
			addSignal("outcome_done", referencesFromMap("outcome", input.Outcome)...)
		}
	}

	sort.Slice(signals, func(i, j int) bool { return signals[i].Code < signals[j].Code })
	summary := OwnerIssueSummary{
		SchemaVersion: OwnerIssueSummarySchemaVersion, CurrentPictureRevision: input.Revision,
		Stage: stage, Signals: signals, NextAction: ownerNextAction(input),
		LatestMeaningfulChange: meaningfulChange(input.LatestMeaningfulChange),
	}
	if stage == OwnerStageTechnicalStop {
		summary.NextAction = &OwnerIssueNextAction{Kind: "inspect_incident", Automatic: false, ActorType: "owner"}
	}
	if stage == OwnerStageComplete || stage == OwnerStageStatusUnavailable {
		summary.NextAction = nil
	}
	if summary.Signals == nil {
		summary.Signals = []OwnerIssueSignal{}
	}
	return summary
}

func meaningfulChange(record map[string]any) *OwnerMeaningfulChange {
	reference := exactReference(record, optionalStringValue(record, "record_type"))
	if reference == nil {
		return nil
	}
	change := &OwnerMeaningfulChange{
		Code: optionalStringValue(record, "change_code"), ResultCode: optionalStringValue(record, "result_code"),
		OccurredAt: optionalStringValue(record, "occurred_at"), Reference: *reference,
	}
	if !knownMeaningfulChange(change.Code) || !knownMeaningfulResult(change.ResultCode) || !validRFC3339(change.OccurredAt) {
		return nil
	}
	return change
}

func knownMeaningfulChange(code string) bool {
	switch code {
	case "outcome_changed", "scope_changed", "authority_changed", "execution_changed", "decision_requested",
		"decision_answered", "candidate_changed", "review_recorded", "release_changed", "evidence_recorded",
		"incident_changed", "action_changed", "budget_changed", "target_changed":
		return true
	default:
		return false
	}
}

func knownMeaningfulResult(code string) bool {
	return code == "recorded" || code == "started" || code == "succeeded" || code == "failed" || code == "resolved" || code == "superseded" || code == "changed"
}

func validRFC3339(value string) bool {
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validateOwnerSummaryInput(input ownerSummaryInput) error {
	if input.Revision < 1 || input.Outcome == nil || input.Scope == nil || input.Execution == nil ||
		input.Release == nil || input.Attention == nil || input.NextAutomaticAction == nil ||
		input.ActiveActor == nil || input.LatestMeaningfulChange == nil || input.UnresolvedDecisions == nil ||
		input.UnresolvedIncidents == nil || input.UnresolvedDependencies == nil {
		return fmt.Errorf("missing current picture field")
	}
	if requiredString(input.Outcome, "state") == "" || requiredString(input.Execution, "state") == "" || strings.TrimSpace(input.Health) == "" {
		return fmt.Errorf("missing current picture state")
	}
	if !oneOfString(requiredString(input.Outcome, "state"), "open", "satisfied", "cancelled", "superseded") ||
		!oneOfString(requiredString(input.Execution, "state"), "received", "queued", "working", "waiting_automatic", "in_review", "correction", "blocked_external", "technically_stopped", "paused", "completed", "failed", "cancelled", "superseded") ||
		!oneOfString(input.Health, "unknown", "unverified", "verifying", "healthy", "degraded", "restored") {
		return fmt.Errorf("unknown current picture state")
	}
	if input.ContractVersion == 3 {
		if !validReleaseStatus(input.ReleaseStatus, input.Release, input.Health) {
			return fmt.Errorf("invalid release status")
		}
	} else if input.ContractVersion != 2 || releaseRequiresV3(optionalStringValue(input.Release, "state")) {
		return fmt.Errorf("release truth requires current picture v3")
	}
	if len(input.UnresolvedIncidents) != 0 && input.ContractVersion != 3 {
		return fmt.Errorf("incident truth requires current picture v3")
	}
	for _, records := range [][]map[string]any{input.UnresolvedDecisions, input.UnresolvedIncidents, input.UnresolvedDependencies} {
		for _, record := range records {
			if exactReference(record, "record") == nil {
				return fmt.Errorf("malformed current picture reference")
			}
		}
	}
	return nil
}

func recoveryCurrent(phase string) bool {
	return phase == "regressed" || phase == "rolling_back" || phase == "rolled_back"
}

func technicallyStopped(executionState, releaseState, reason string) bool {
	return strings.TrimSpace(reason) != "" || executionState == "technically_stopped" || executionState == "failed" || releaseState == "failed"
}

func incomingExecution(state string) bool {
	switch state {
	case "received", "planned", "pending":
		return true
	default:
		return false
	}
}

func viableContinuation(input ownerSummaryInput) bool {
	switch requiredString(input.Execution, "state") {
	case "waiting_automatic":
		dueAt, err := time.Parse(time.RFC3339Nano, optionalStringValue(input.Execution, "next_action_at"))
		return err == nil && dueAt.After(input.GeneratedAt) && knownAutomaticEvent(input.Execution["expected_event"])
	case "working", "correction":
		// A finite lease cannot be treated as current from ambient wall-clock time.
		// Active is reserved for a durable, non-expiring task binding.
		return optionalStringValue(input.Execution, "task_id") != "" &&
			optionalStringValue(input.Execution, "actor_type") != "" && optionalStringValue(input.Execution, "actor_id") != "" &&
			optionalStringValue(input.Execution, "lease_expires_at") == ""
	default:
		return false
	}
}

func knownAutomaticEvent(value any) bool {
	event, ok := value.(map[string]any)
	if !ok {
		return false
	}
	switch optionalStringValue(event, "type") {
	case "task.completed", "provider_readback", "deploy_exact_release", "review_correction", "review_candidate",
		"finalize_non_live", "resume_exact_operation", "replan_exact_operation", "verify_runtime_exact_release",
		"verify_business_exact_release", "monitor_exact_release", "verify_restoration_exact_release",
		"rollback_exact_release", "contain_exact_release":
		return true
	default:
		return false
	}
}

func exactCompletion(input ownerSummaryInput, outcomeState string) bool {
	if outcomeState != "satisfied" {
		return false
	}
	requiresDeployment, ok := input.Scope["requires_deployment"].(bool)
	if !ok {
		return false
	}
	if !requiresDeployment {
		return optionalStringValue(input.Release, "state") == "not_required"
	}
	return releasePhase(input) == "verified_live" && input.Health == "healthy"
}

func releasePhase(input ownerSummaryInput) string {
	if input.ReleaseStatus == nil {
		return ""
	}
	return input.ReleaseStatus.Phase
}

func releaseRequiresV3(state string) bool {
	switch state {
	case "", "pending", "not_required":
		return false
	default:
		return true
	}
}

func oneOfString(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func ownerNextAction(input ownerSummaryInput) *OwnerIssueNextAction {
	if len(input.UnresolvedDecisions) != 0 {
		return &OwnerIssueNextAction{Kind: "answer_decision", Automatic: false, ActorType: "owner"}
	}
	if !knownAutomaticEvent(input.NextAutomaticAction) {
		return nil
	}
	return &OwnerIssueNextAction{
		Kind: "continue_workflow", Automatic: true,
		ActorType: optionalStringValue(input.ActiveActor, "actor_type"),
		ActorID:   optionalStringValue(input.ActiveActor, "actor_id"),
	}
}

func references(recordType string, records []map[string]any) []OwnerIssueReference {
	result := make([]OwnerIssueReference, 0, len(records))
	for _, record := range records {
		if ref := exactReference(record, recordType); ref != nil {
			result = append(result, *ref)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		if result[i].ID != result[j].ID {
			return result[i].ID < result[j].ID
		}
		return result[i].Revision < result[j].Revision
	})
	return result
}

func referencesFromMap(recordType string, record map[string]any) []OwnerIssueReference {
	if ref := exactReference(record, recordType); ref != nil {
		return []OwnerIssueReference{*ref}
	}
	return []OwnerIssueReference{}
}

func exactReference(record map[string]any, recordType string) *OwnerIssueReference {
	id := optionalStringValue(record, "id")
	revision := intValue(record["revision"])
	if strings.TrimSpace(recordType) == "" || id == "" || revision < 1 {
		return nil
	}
	return &OwnerIssueReference{Type: recordType, ID: id, Revision: revision}
}

func requiredString(record map[string]any, key string) string {
	return optionalStringValue(record, key)
}

func optionalStringValue(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return strings.TrimSpace(value)
}
