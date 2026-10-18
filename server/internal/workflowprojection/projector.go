// Package workflowprojection builds the read-only Current Picture projection
// from normalized workflow v2 records.
package workflowprojection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const ContractVersion int32 = 2

// Record is one normalized ledger row. Project accepts this transport-neutral
// form so deterministic behavior can be tested without a database.
type Record struct {
	Type      string
	ID        string
	Revision  int64
	CreatedAt time.Time
	Payload   json.RawMessage
}

type SourceRevision struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

// Document is the complete semantic Current Picture. HistoricalCompletion is
// intentionally separate from current release health, so a later degradation
// or rollback does not erase the fact that the requested outcome was once met.
type Document struct {
	ContractVersion           int32            `json:"contract_version"`
	OwnerSummarySchemaVersion int32            `json:"owner_summary_schema_version"`
	ReleaseStatus             *ReleaseStatus   `json:"release_status,omitempty"`
	OriginalBusinessRequest   map[string]any   `json:"original_business_request"`
	Outcome                   map[string]any   `json:"outcome"`
	HistoricalCompletion      map[string]any   `json:"historical_completion"`
	Scope                     map[string]any   `json:"scope"`
	Authority                 map[string]any   `json:"authority"`
	Execution                 map[string]any   `json:"execution"`
	ActiveBranches            []map[string]any `json:"active_branches"`
	PrimaryBranch             map[string]any   `json:"primary_branch"`
	Release                   map[string]any   `json:"release"`
	ObservedRelease           map[string]any   `json:"observed_release"`
	Health                    string           `json:"health"`
	Attention                 map[string]any   `json:"attention"`
	CurrentMilestone          string           `json:"current_milestone"`
	LatestMeaningfulChange    map[string]any   `json:"latest_meaningful_change"`
	NextAutomaticAction       map[string]any   `json:"next_automatic_action"`
	ActiveActor               map[string]any   `json:"active_actor"`
	WaitReason                any              `json:"wait_reason"`
	Candidate                 map[string]any   `json:"candidate"`
	Target                    map[string]any   `json:"target"`
	EvidenceSummary           map[string]any   `json:"evidence_summary"`
	AuthoritySummary          map[string]any   `json:"authority_summary"`
	UnresolvedDecisions       []map[string]any `json:"unresolved_decisions"`
	UnresolvedIncidents       []map[string]any `json:"unresolved_incidents"`
	UnresolvedDependencies    []map[string]any `json:"unresolved_dependencies"`
	Budgets                   []map[string]any `json:"budgets"`
	Supersession              map[string]any   `json:"supersession"`
	TerminalReason            string           `json:"terminal_reason"`
	NextActor                 map[string]any   `json:"next_actor"`
}

// Projection carries both semantic bytes and the denormalized columns used by
// list/detail reads. GeneratedAt and Revision are write metadata, not part of
// Document or SourceRevisions equivalence.
type Projection struct {
	ContractVersion        int32
	Revision               int64
	ScopeRevision          *int64
	AuthorityRevision      *int64
	OutcomeState           string
	OutcomeOpen            bool
	ExecutionState         string
	ReleaseState           string
	HealthState            string
	AttentionState         string
	CurrentMilestone       string
	LatestMeaningfulChange []byte
	NextAutomaticAction    []byte
	ActiveActor            []byte
	WaitReason             []byte
	Candidate              []byte
	Target                 []byte
	EvidenceSummary        []byte
	AuthoritySummary       []byte
	UnresolvedDecisions    []byte
	UnresolvedIncidents    []byte
	UnresolvedDependencies []byte
	BudgetSummary          []byte
	SupersessionSummary    []byte
	TerminalReason         string
	NextActor              []byte
	SourceRevisions        []byte
	Document               []byte
	GeneratedAt            time.Time
}

type parsedRecord struct {
	Record
	fields map[string]any
}

var knownRecordTypes = map[string]bool{
	"outcome": true, "scope": true, "authority": true, "execution": true,
	"decision": true, "decision_response": true, "candidate": true,
	"review": true, "release": true, "evidence": true, "incident": true,
	"action_receipt": true, "attention": true, "budget": true, "target_head": true,
	"consequence": true, "regression_change": true, "regression_rejected": true,
}

var meaningfulRecordTypes = map[string]bool{
	"outcome": true, "scope": true, "authority": true, "execution": true,
	"decision": true, "decision_response": true, "candidate": true,
	"review": true, "release": true, "evidence": true, "incident": true,
	"consequence": true, "regression_change": true,
}

// Project deterministically derives one semantic picture from normalized rows.
func Project(records []Record, revision int64, generatedAt time.Time) (Projection, error) {
	if revision < 1 {
		return Projection{}, fmt.Errorf("invalid Current Picture revision %d", revision)
	}
	ordered := append([]Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
		}
		if ordered[i].Type != ordered[j].Type {
			return ordered[i].Type < ordered[j].Type
		}
		return ordered[i].ID < ordered[j].ID
	})

	parsed := make([]parsedRecord, 0, len(ordered))
	sources := make([]SourceRevision, 0, len(ordered))
	for _, record := range ordered {
		if !knownRecordTypes[record.Type] || record.ID == "" || record.Revision < 1 {
			return Projection{}, fmt.Errorf("invalid workflow projection record %q/%q revision %d", record.Type, record.ID, record.Revision)
		}
		fields := map[string]any{}
		if err := json.Unmarshal(record.Payload, &fields); err != nil {
			return Projection{}, fmt.Errorf("decode %s %s: %w", record.Type, record.ID, err)
		}
		if err := validateRecord(record, fields); err != nil {
			return Projection{}, err
		}
		parsed = append(parsed, parsedRecord{Record: record, fields: fields})
		sources = append(sources, SourceRevision{Type: record.Type, ID: record.ID, Revision: record.Revision})
	}
	if err := validateProjectionRelations(parsed); err != nil {
		return Projection{}, err
	}

	latest := map[string]parsedRecord{}
	latestAny := parsedRecord{}
	for _, record := range parsed {
		if meaningfulRecordTypes[record.Type] && laterRecord(record, latestAny) {
			latestAny = record
		}
		if isSuperseded(record.fields) {
			continue
		}
		if prior, ok := latest[record.Type]; !ok || laterRecord(record, prior) {
			latest[record.Type] = record
		}
	}

	outcome := summary(latest["outcome"], "id", "revision", "state", "acceptance_version", "completion_evidence_id", "terminal_reason")
	if len(outcome) == 0 {
		outcome = map[string]any{"state": "open"}
	}
	activeBranches, primary := projectBranches(parsed)
	execution := map[string]any{}
	scope := map[string]any{}
	authority := map[string]any{}
	candidate := map[string]any{}
	release := map[string]any{}
	target := map[string]any{}
	evidenceSummary := emptyEvidenceSummary()
	observedRelease := map[string]any{}
	primaryBranch := map[string]any{}
	if len(primary) != 0 {
		primaryBranch = map[string]any{"key": cloneMap(primary["key"].(map[string]any)), "selection": "active_then_latest_effective_change"}
		execution = cloneMap(primary["execution"].(map[string]any))
		scope = cloneMap(primary["scope"].(map[string]any))
		authority = cloneMap(primary["authority"].(map[string]any))
		candidate = cloneMap(primary["candidate"].(map[string]any))
		release = cloneMap(primary["release"].(map[string]any))
		target = cloneMap(primary["target"].(map[string]any))
		evidenceSummary = cloneMap(primary["evidence_summary"].(map[string]any))
	} else {
		scope = summary(latest["scope"], "id", "revision", "acceptance_version", "state", "owner_evidence_id", "statement", "impact", "requires_deployment", "source_plan_revision")
		authority = summary(latest["authority"], "id", "revision", "scope_revision", "state", "envelope", "valid_until", "owner_evidence_id")
	}
	if targetKey := stringValue(release["target_key"], ""); targetKey != "" {
		observedRelease, target, evidenceSummary = projectObservedTarget(parsed, targetKey)
		if len(observedRelease) != 0 && stringValue(observedRelease["release_kind"], "forward") == "recovery" {
			// The target head is deployment truth. A recovery generation can be
			// current while the primary implementation branch still points at the
			// failed forward release; the owner-visible release must follow the
			// observed target without erasing that branch history.
			release = cloneMap(observedRelease)
		}
	}

	unresolvedDecisions := collect(parsed, "decision", func(fields map[string]any) bool {
		return stringField(fields, "state") == "unanswered" && !isSuperseded(fields)
	}, "id", "revision", "decision_type", "state", "resolution_state", "question", "recommendation", "consequence", "options", "candidate_id", "target_key", "accountable_owner_id", "suspended_operation_id", "expires_at")
	unresolvedIncidents := collect(parsed, "incident", func(fields map[string]any) bool {
		state := stringField(fields, "state")
		return state != "resolved" && state != "closed" && !isSuperseded(fields)
	}, "id", "revision", "release_id", "release_revision", "candidate_id", "candidate_revision", "target_key", "target_generation", "incident_type", "severity", "state", "detection_evidence_id", "recovery_action_id", "compatibility_state", "regression_fingerprint", "recovery_release_id", "consequence", "recovery", "acknowledged_at")
	unresolvedDependencies := collect(parsed, "execution", func(fields map[string]any) bool {
		return stringField(fields, "state") == "blocked_external" && !isSuperseded(fields)
	}, "id", "revision", "branch_key", "operation_type", "state", "wait_reason", "expected_event", "checkpoint", "next_action_at", "actor_type", "actor_id")
	budgets := collect(parsed, "budget", func(fields map[string]any) bool {
		return stringField(fields, "state") != "superseded"
	}, "id", "revision", "concern_type", "concern_id", "state", "limits", "consumption", "exhausted_at")

	attention := summary(latestActiveAttention(parsed), "id", "revision", "attention_type", "decision_id", "incident_id", "consequence", "recommendation", "context", "next_action", "read_state", "acknowledged", "resolution_state", "snoozed_until")
	attentionState := "resolved"
	if len(attention) != 0 {
		attentionState = stringValue(attention["resolution_state"], "open")
	}
	if len(unresolvedDecisions) != 0 || attentionState == "resuming" {
		invalidateEvidenceSummary(evidenceSummary)
	}

	healthRelease := observedRelease
	if len(healthRelease) == 0 {
		healthRelease = release
	}
	health := deriveHealth(healthRelease, target, relatedIncidents(unresolvedIncidents, healthRelease, target), evidenceSummary)
	outcomeState := stringValue(outcome["state"], "open")
	executionState := stringValue(execution["state"], "received")
	releaseState := stringValue(release["state"], "pending")
	if releaseState == "live" && health != "healthy" {
		releaseState = "production_verifying"
	}
	if releaseState == "restored" && health != "restored" {
		releaseState = "rolled_back"
	}
	if len(release) != 0 {
		release["state"] = releaseState
	}
	terminalReason := stringValue(outcome["terminal_reason"], "")
	if terminalReason == "" {
		terminalReason = stringValue(execution["terminal_reason"], "")
	}

	activeActor := pick(execution, "actor_type", "actor_id", "task_id")
	nextActor := map[string]any{}
	if len(unresolvedDecisions) != 0 {
		nextActor = pick(unresolvedDecisions[0], "accountable_owner_id")
		nextActor["type"] = "owner"
	} else if len(activeActor) != 0 {
		nextActor = cloneMap(activeActor)
	}
	nextAutomaticAction := pick(execution, "expected_event", "next_action_at")
	if value, ok := attention["next_action"]; ok && attentionState != "resolved" {
		nextAutomaticAction = map[string]any{"attention": value}
	}
	waitReason := any(map[string]any{})
	if value, ok := execution["wait_reason"]; ok {
		waitReason = value
	}

	supersession := summarizeSupersession(parsed)
	latestChange := summary(latestAny, "id", "revision", "state", "updated_at", "created_at")
	if len(latestChange) != 0 {
		latestChange["record_type"] = latestAny.Type
		latestChange["change_code"] = meaningfulChangeCode(latestAny.Type)
		latestChange["result_code"] = meaningfulResultCode(latestAny.fields)
		latestChange["occurred_at"] = effectiveTime(latestAny).UTC().Format(time.RFC3339Nano)
		if external := externalWorkMeaningfulSummary(latestAny); len(external) != 0 {
			latestChange["external_work"] = external
		}
	}
	historical := historicalCompletion(parsed)
	releaseStatus := buildReleaseStatus(release, candidate, scope, target, evidenceSummary, execution, historical, health, terminalReason)
	contractVersion := ContractVersion
	currentMilestone := milestone(outcomeState, executionState, releaseState, health, attentionState)
	if releaseStatus != nil {
		contractVersion = 3
		currentMilestone = releaseStatus.Phase
	}
	document := Document{
		ContractVersion: contractVersion, OwnerSummarySchemaVersion: OwnerIssueSummarySchemaVersion, ReleaseStatus: releaseStatus,
		OriginalBusinessRequest: originalBusinessRequest(parsed),
		Outcome:                 outcome, HistoricalCompletion: historical,
		Scope: scope, Authority: authority, Execution: execution, ActiveBranches: activeBranches, PrimaryBranch: primaryBranch, Release: release, ObservedRelease: observedRelease,
		Health: health, Attention: attention, CurrentMilestone: currentMilestone,
		LatestMeaningfulChange: latestChange, NextAutomaticAction: nextAutomaticAction,
		ActiveActor: activeActor, WaitReason: waitReason, Candidate: candidate, Target: target,
		EvidenceSummary: evidenceSummary, AuthoritySummary: cloneMap(authority),
		UnresolvedDecisions: unresolvedDecisions, UnresolvedIncidents: unresolvedIncidents,
		UnresolvedDependencies: unresolvedDependencies, Budgets: budgets, Supersession: supersession,
		TerminalReason: terminalReason, NextActor: nextActor,
	}

	documentJSON, err := json.Marshal(document)
	if err != nil {
		return Projection{}, err
	}
	sourceJSON, err := json.Marshal(sources)
	if err != nil {
		return Projection{}, err
	}
	return Projection{
		ContractVersion: contractVersion,
		Revision:        revision, ScopeRevision: intPointer(scope["revision"]), AuthorityRevision: intPointer(authority["revision"]),
		OutcomeState: outcomeState, OutcomeOpen: outcomeState == "open", ExecutionState: executionState,
		ReleaseState: releaseState, HealthState: health, AttentionState: attentionState,
		CurrentMilestone: document.CurrentMilestone, LatestMeaningfulChange: mustJSON(latestChange),
		NextAutomaticAction: mustJSON(nextAutomaticAction), ActiveActor: mustJSON(activeActor),
		WaitReason: mustJSON(waitReason), Candidate: mustJSON(candidate), Target: mustJSON(target),
		EvidenceSummary: mustJSON(evidenceSummary), AuthoritySummary: mustJSON(authority),
		UnresolvedDecisions: mustJSON(unresolvedDecisions), UnresolvedIncidents: mustJSON(unresolvedIncidents),
		UnresolvedDependencies: mustJSON(unresolvedDependencies), BudgetSummary: mustJSON(budgets),
		SupersessionSummary: mustJSON(supersession), TerminalReason: terminalReason,
		NextActor: mustJSON(nextActor), SourceRevisions: sourceJSON, Document: documentJSON,
		GeneratedAt: generatedAt.UTC(),
	}, nil
}

func externalWorkMeaningfulSummary(record parsedRecord) map[string]any {
	if record.Type != "execution" || stringField(record.fields, "operation_type") != "external_observation" ||
		stringField(record.fields, "actor_type") != "external_provider" {
		return nil
	}
	checkpoint, ok := record.fields["checkpoint"].(map[string]any)
	if !ok || stringField(checkpoint, "contract") != "external_work_observation_v0.2" {
		return nil
	}
	return pick(checkpoint,
		"provider", "work_id", "run_id", "receipt_revision", "reported_state",
		"current_milestone", "change_since_last_receipt", "evidence", "dependencies",
		"next_action", "owner_decision_proposal", "acceptance_report")
}

func meaningfulChangeCode(recordType string) string {
	switch recordType {
	case "outcome":
		return "outcome_changed"
	case "scope":
		return "scope_changed"
	case "authority":
		return "authority_changed"
	case "execution":
		return "execution_changed"
	case "decision":
		return "decision_requested"
	case "decision_response":
		return "decision_answered"
	case "candidate":
		return "candidate_changed"
	case "review":
		return "review_recorded"
	case "release":
		return "release_changed"
	case "evidence":
		return "evidence_recorded"
	case "incident":
		return "incident_changed"
	case "consequence":
		return "cross_issue_consequence_recorded"
	case "regression_change":
		return "regression_obligation_changed"
	case "action_receipt":
		return "action_changed"
	case "budget":
		return "budget_changed"
	case "target_head":
		return "target_changed"
	default:
		return ""
	}
}

func meaningfulResultCode(fields map[string]any) string {
	if isSuperseded(fields) {
		return "superseded"
	}
	state := optionalStringValue(fields, "state")
	if state == "" {
		state = optionalStringValue(fields, "outcome")
	}
	switch state {
	case "satisfied", "completed", "passed", "live", "restored", "not_required", "settled":
		return "succeeded"
	case "failed", "degraded", "technically_stopped", "rejected", "cancelled":
		return "failed"
	case "resolved", "closed":
		return "resolved"
	case "received", "queued", "working", "waiting_automatic", "in_review", "pending", "authorized", "deploying", "admitted", "unanswered":
		return "started"
	case "":
		return "recorded"
	default:
		return "changed"
	}
}

// MeaningfulChangeFromRecord turns one durable normalized record into the
// bounded owner activity contract. Callers may isolate a malformed row without
// trusting its payload or failing an otherwise healthy page.
func MeaningfulChangeFromRecord(record Record) (OwnerMeaningfulChange, error) {
	if !meaningfulRecordTypes[record.Type] || record.ID == "" || record.Revision < 1 || record.CreatedAt.IsZero() {
		return OwnerMeaningfulChange{}, fmt.Errorf("invalid meaningful record %q/%q revision %d", record.Type, record.ID, record.Revision)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(record.Payload, &fields); err != nil {
		return OwnerMeaningfulChange{}, fmt.Errorf("decode %s %s: %w", record.Type, record.ID, err)
	}
	if err := validateRecord(record, fields); err != nil {
		return OwnerMeaningfulChange{}, err
	}
	code := meaningfulChangeCode(record.Type)
	if code == "" {
		return OwnerMeaningfulChange{}, fmt.Errorf("record type %q is not owner meaningful", record.Type)
	}
	return OwnerMeaningfulChange{
		Code: code, ResultCode: meaningfulResultCode(fields),
		OccurredAt: record.CreatedAt.UTC().Format(time.RFC3339Nano),
		Reference:  OwnerIssueReference{Type: record.Type, ID: record.ID, Revision: record.Revision},
	}, nil
}

func invalidateEvidenceSummary(summary map[string]any) {
	summary["runtime_current"], summary["business_current"], summary["regression_current"], summary["restoration_current"] = false, false, false, false
	summary["current_counts"] = map[string]int{}
	items, _ := summary["items"].([]map[string]any)
	for _, item := range items {
		freshness, _ := item["freshness"].(map[string]any)
		freshness["current"] = false
		freshness["invalidated_by_attention"] = true
	}
}

// ProjectAndStore reads and writes under the same issue admission lock. The
// generated Current Picture mutator remains encapsulated by workflowstore.
func ProjectAndStore(ctx context.Context, starter workflowstore.TxStarter, queries *db.Queries, workspaceID, issueID pgtype.UUID, expectedRevision, revision int64, generatedAt time.Time) (db.IssueCurrentPicture, error) {
	var stored db.IssueCurrentPicture
	err := workflowstore.InIssueWriteTransaction(ctx, starter, queries, workspaceID, issueID, func(write *workflowstore.IssueWrite) error {
		var err error
		stored, err = ProjectInWrite(ctx, write, workspaceID, issueID, expectedRevision, revision, generatedAt)
		return err
	})
	return stored, err
}

// ProjectInWrite rebuilds the display projection inside an already-admitted
// issue transaction. It exists so command admission can commit authoritative
// ledgers, promised actions, and the rebuild atomically without nesting a
// second transaction.
func ProjectInWrite(ctx context.Context, write *workflowstore.IssueWrite, workspaceID, issueID pgtype.UUID, expectedRevision, revision int64, generatedAt time.Time) (db.IssueCurrentPicture, error) {
	if write == nil {
		return db.IssueCurrentPicture{}, fmt.Errorf("projection requires an admitted issue write: %w", workflowstore.ErrInvalidIssueWriteCapability)
	}
	var stored db.IssueCurrentPicture
	err := write.WithIssueOperation(ctx, workspaceID, issueID, func(leasedCtx context.Context) error {
		rows, err := write.ListProjectionInputs(leasedCtx, db.ListIssueWorkflowProjectionInputsParams{WorkspaceID: workspaceID, IssueID: issueID})
		if err != nil {
			return err
		}
		records := make([]Record, 0, len(rows))
		for _, row := range rows {
			records = append(records, Record{Type: row.RecordType, ID: uuidString(row.ID), Revision: row.Revision, CreatedAt: row.CreatedAt.Time.UTC(), Payload: row.Payload})
		}
		projection, err := Project(records, revision, generatedAt)
		if err != nil {
			return err
		}
		stored, err = write.UpsertCurrentPictureCAS(leasedCtx, projection.params(workspaceID, issueID, expectedRevision))
		return err
	})
	if err != nil {
		if errors.Is(err, workflowstore.ErrInvalidIssueWriteCapability) {
			return db.IssueCurrentPicture{}, fmt.Errorf("projection requires an admitted issue write: %w", err)
		}
		return db.IssueCurrentPicture{}, err
	}
	return stored, nil
}

func (p Projection) params(workspaceID, issueID pgtype.UUID, expectedRevision int64) db.UpsertIssueCurrentPictureParams {
	return db.UpsertIssueCurrentPictureParams{
		WorkspaceID: workspaceID, IssueID: issueID, ContractVersion: p.ContractVersion, Revision: p.Revision,
		ScopeRevision: nullableInt8(p.ScopeRevision), AuthorityRevision: nullableInt8(p.AuthorityRevision),
		OutcomeState: p.OutcomeState, OutcomeOpen: p.OutcomeOpen, ExecutionState: p.ExecutionState,
		ReleaseState: p.ReleaseState, HealthState: p.HealthState, AttentionState: p.AttentionState,
		CurrentMilestone: p.CurrentMilestone, LatestMeaningfulChange: p.LatestMeaningfulChange,
		NextAutomaticAction: p.NextAutomaticAction, ActiveActor: p.ActiveActor, WaitReason: p.WaitReason,
		Candidate: p.Candidate, Target: p.Target, EvidenceSummary: p.EvidenceSummary,
		AuthoritySummary: p.AuthoritySummary, UnresolvedDecisions: p.UnresolvedDecisions,
		UnresolvedIncidents: p.UnresolvedIncidents, UnresolvedDependencies: p.UnresolvedDependencies,
		BudgetSummary: p.BudgetSummary, SupersessionSummary: p.SupersessionSummary,
		TerminalReason: p.TerminalReason, NextActor: p.NextActor, SourceRevisions: p.SourceRevisions,
		Document: p.Document, GeneratedAt: pgtype.Timestamptz{Time: p.GeneratedAt, Valid: true},
		ExpectedRevision: expectedRevision,
	}
}

func laterRecord(candidate, current parsedRecord) bool {
	if current.ID == "" {
		return true
	}
	candidateTime, currentTime := effectiveTime(candidate), effectiveTime(current)
	if !candidateTime.Equal(currentTime) {
		return candidateTime.After(currentTime)
	}
	// The observation-only scope bootstrap and the first external execution are
	// created in one PostgreSQL transaction, so now() gives them the same
	// timestamp. The execution is the event the owner needs to see; allowing the
	// generic scope row to win the deterministic type tie hides the provider
	// milestone and can create a vague "scope changed" Inbox item instead.
	if externalObservationFollowsBootstrap(candidate, current) {
		return true
	}
	if externalObservationFollowsBootstrap(current, candidate) {
		return false
	}
	if candidate.Revision != current.Revision {
		return candidate.Revision > current.Revision
	}
	if candidate.Type != current.Type {
		return candidate.Type > current.Type
	}
	return candidate.ID > current.ID
}

func externalObservationFollowsBootstrap(candidate, current parsedRecord) bool {
	if candidate.Type != "execution" || current.Type != "scope" ||
		stringField(candidate.fields, "operation_type") != "external_observation" ||
		stringField(candidate.fields, "actor_type") != "external_provider" {
		return false
	}
	statement, ok := current.fields["statement"].(map[string]any)
	return ok && stringField(statement, "source") == "owner_created_issue"
}

func effectiveTime(record parsedRecord) time.Time {
	for _, key := range []string{"updated_at", "observed_at", "created_at"} {
		if value := stringField(record.fields, key); value != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				return parsed.UTC()
			}
		}
	}
	return record.CreatedAt.UTC()
}

func latestActiveAttention(records []parsedRecord) parsedRecord {
	var latest parsedRecord
	for _, record := range records {
		if record.Type != "attention" || isSuperseded(record.fields) || stringField(record.fields, "resolution_state") == "resolved" {
			continue
		}
		if laterRecord(record, latest) {
			latest = record
		}
	}
	return latest
}

func summary(record parsedRecord, keys ...string) map[string]any {
	if record.ID == "" {
		return map[string]any{}
	}
	result := map[string]any{"id": record.ID, "revision": record.Revision}
	for _, key := range keys {
		if value, ok := record.fields[key]; ok && value != nil {
			result[key] = value
		}
	}
	return result
}

func collect(records []parsedRecord, recordType string, include func(map[string]any) bool, keys ...string) []map[string]any {
	result := make([]map[string]any, 0)
	for _, record := range records {
		if record.Type == recordType && include(record.fields) {
			result = append(result, summary(record, keys...))
		}
	}
	return result
}

func projectBranches(records []parsedRecord) ([]map[string]any, map[string]any) {
	byTypeID := make(map[string]map[string]parsedRecord)
	for _, record := range records {
		if isSuperseded(record.fields) {
			continue
		}
		if byTypeID[record.Type] == nil {
			byTypeID[record.Type] = make(map[string]parsedRecord)
		}
		if prior, ok := byTypeID[record.Type][record.ID]; !ok || laterRecord(record, prior) {
			byTypeID[record.Type][record.ID] = record
		}
	}

	executions := make([]parsedRecord, 0, len(byTypeID["execution"]))
	for _, execution := range byTypeID["execution"] {
		executions = append(executions, execution)
	}
	sort.Slice(executions, func(i, j int) bool {
		activeI, activeJ := executionActive(executions[i].fields), executionActive(executions[j].fields)
		if activeI != activeJ {
			return activeI
		}
		if laterRecord(executions[i], executions[j]) {
			return true
		}
		if laterRecord(executions[j], executions[i]) {
			return false
		}
		return executions[i].ID < executions[j].ID
	})

	branches := make([]map[string]any, 0, len(executions))
	for index, executionRecord := range executions {
		execution := summary(executionRecord, "branch_key", "operation_type", "state", "scope_id", "scope_revision", "authority_id", "authority_revision", "candidate_id", "target_key", "actor_type", "actor_id", "task_id", "checkpoint", "expected_event", "wait_reason", "lease_expires_at", "next_action_at", "attempt_count", "terminal_reason")
		scopeRecord := relatedByIDRevision(byTypeID["scope"], stringField(executionRecord.fields, "scope_id"), intValue(executionRecord.fields["scope_revision"]))
		authorityRecord := relatedByIDRevision(byTypeID["authority"], stringField(executionRecord.fields, "authority_id"), intValue(executionRecord.fields["authority_revision"]))
		candidateRecord := relatedByID(byTypeID["candidate"], stringField(executionRecord.fields, "candidate_id"))
		releaseRecord := relatedRelease(byTypeID["release"], executionRecord, candidateRecord)
		targetRecord := relatedTargetHead(byTypeID["target_head"], releaseRecord)

		scope := summary(scopeRecord, "acceptance_version", "state", "owner_evidence_id", "statement", "impact", "requires_deployment", "source_plan_revision")
		authority := summary(authorityRecord, "scope_id", "scope_revision", "state", "envelope", "valid_until", "owner_evidence_id")
		candidate := summary(candidateRecord, "scope_id", "scope_revision", "candidate_type", "state", "source_revision", "artifact", "build_contract")
		release := summary(releaseRecord, "candidate_id", "candidate_revision", "scope_id", "scope_revision", "authority_id", "authority_revision", "target_key", "target_generation", "state", "facets", "recovery_policy", "exposed_population", "started_at", "deployed_at", "restored_at", "attempt_contract_version", "acceptance_version", "operation_id", "branch_key", "expected_target_release_id", "expected_target_generation", "release_kind", "recovery_of_release_id", "head_observed_at", "release_operator_id", "planned_verifier_id", "build_manifest_digest", "artifact_digest", "runtime_contract_digest", "authority_capability", "authority_evidence_id", "recovery_policy_revision", "authority_envelope_digest", "owner_evidence_revision", "exposure_policy_digest", "rollback_target_release_id", "irreversible_effects_digest", "reviewed_review_id", "reviewed_review_revision")
		target := summary(targetRecord, "target_key", "generation", "release_id", "issue_id", "candidate_id", "state", "updated_at")
		branch := map[string]any{
			"key": map[string]any{
				"execution_id": executionRecord.ID, "branch_key": execution["branch_key"],
				"operation_type": execution["operation_type"], "candidate_id": candidate["id"],
				"release_id": release["id"], "target_key": release["target_key"],
			},
			"is_primary": index == 0, "active": executionActive(executionRecord.fields),
			"execution": execution, "scope": scope, "authority": authority, "candidate": candidate,
			"release": release, "target": target,
			"evidence_summary": summarizeEvidence(records, release, target, candidate, scope),
		}
		branches = append(branches, branch)
	}
	if len(branches) == 0 {
		return branches, map[string]any{}
	}
	active := make([]map[string]any, 0, len(branches))
	for _, branch := range branches {
		if branch["active"] == true {
			active = append(active, branch)
		}
	}
	return active, branches[0]
}

func executionActive(fields map[string]any) bool {
	switch stringField(fields, "state") {
	case "completed", "cancelled", "superseded", "failed", "technically_stopped":
		return false
	default:
		return true
	}
}

func relatedByID(records map[string]parsedRecord, id string) parsedRecord {
	return records[id]
}

func relatedByIDRevision(records map[string]parsedRecord, id string, revision int64) parsedRecord {
	record := records[id]
	if record.Revision != revision {
		return parsedRecord{}
	}
	return record
}

func relatedRelease(records map[string]parsedRecord, execution, candidate parsedRecord) parsedRecord {
	if candidate.ID == "" {
		return parsedRecord{}
	}
	var selected parsedRecord
	for _, release := range records {
		if stringField(release.fields, "candidate_id") != candidate.ID ||
			intValue(release.fields["candidate_revision"]) != candidate.Revision ||
			stringField(release.fields, "scope_id") != stringField(execution.fields, "scope_id") ||
			intValue(release.fields["scope_revision"]) != intValue(execution.fields["scope_revision"]) ||
			stringField(release.fields, "authority_id") != stringField(execution.fields, "authority_id") ||
			intValue(release.fields["authority_revision"]) != intValue(execution.fields["authority_revision"]) ||
			stringField(release.fields, "target_key") != stringField(execution.fields, "target_key") {
			continue
		}
		if laterRecord(release, selected) {
			selected = release
		}
	}
	return selected
}

func relatedTargetHead(records map[string]parsedRecord, release parsedRecord) parsedRecord {
	if release.ID == "" {
		return parsedRecord{}
	}
	for _, head := range records {
		if stringField(head.fields, "release_id") == release.ID &&
			stringField(head.fields, "target_key") == stringField(release.fields, "target_key") &&
			stringField(head.fields, "candidate_id") == stringField(release.fields, "candidate_id") &&
			intValue(head.fields["generation"]) == intValue(release.fields["target_generation"]) {
			return head
		}
	}
	return parsedRecord{}
}

func emptyEvidenceSummary() map[string]any {
	return map[string]any{"counts": map[string]int{}, "current_counts": map[string]int{}, "items": []map[string]any{}, "technical_reviews": []map[string]any{}, "runtime_current": false, "business_current": false, "regression_current": false, "restoration_current": false}
}

func summarizeEvidence(records []parsedRecord, release, target, candidate, scope map[string]any) map[string]any {
	counts := map[string]int{}
	current := map[string]int{}
	items := make([]map[string]any, 0)
	reviews := make([]map[string]any, 0)
	runtimeCurrent, businessCurrent, regressionCurrent, restorationCurrent := false, false, false, false
	for _, record := range records {
		if record.Type == "review" {
			if stringField(record.fields, "candidate_id") != stringValue(candidate["id"], "") ||
				intValue(record.fields["candidate_revision"]) != intValue(candidate["revision"]) ||
				stringField(record.fields, "scope_id") != stringValue(scope["id"], "") ||
				intValue(record.fields["scope_revision"]) != intValue(scope["revision"]) {
				continue
			}
			reviews = append(reviews, summary(record,
				"candidate_id", "candidate_revision", "scope_revision", "reviewer_id",
				"outcome", "criteria", "coverage", "gaps", "required_correction",
				"started_at", "completed_at", "superseded_at"))
			continue
		}
		if record.Type != "evidence" {
			continue
		}
		if !evidenceRelated(record.fields, release, candidate) {
			continue
		}
		typeName := stringField(record.fields, "evidence_type")
		if typeName == "" {
			typeName = "unknown"
		}
		counts[typeName]++
		payload, _ := record.fields["payload"].(map[string]any)
		// Finite deadlines cannot be evaluated from ambient projection time without
		// making rebuilds nondeterministic. Until an expiry action writes a durable
		// normalized state, deadline-bearing evidence is visible but fails closed.
		noPendingDeadline := stringValue(record.fields["valid_until"], "") == ""
		fresh := !isSuperseded(record.fields) &&
			emptyJSONValue(record.fields["contradictions"]) &&
			stringField(payload, "freshness") == "current" && noPendingDeadline
		item := summary(record,
			"concern_type", "concern_id", "concern_revision", "evidence_type", "level",
			"source", "producer_type", "producer_id", "candidate_id", "target_key",
			"observed_at", "valid_until", "coverage", "limitations", "contradictions",
			"superseded_at")
		item["freshness"] = map[string]any{
			"state":                      stringField(payload, "freshness"),
			"current":                    fresh,
			"valid_until":                record.fields["valid_until"],
			"requires_expiry_transition": !noPendingDeadline,
			"superseded":                 isSuperseded(record.fields),
			"contradicted":               !emptyJSONValue(record.fields["contradictions"]),
		}
		// The provider receipt is immutable deployment identity. Timestamps only
		// order observations and must never be promoted into a deployed revision.
		if providerReceiptID := stringField(payload, "provider_receipt_id"); providerReceiptID != "" {
			item["provider_receipt_id"] = providerReceiptID
		}
		item["applies"] = fresh && evidenceApplies(record.fields, release, target, candidate, scope)
		items = append(items, item)
		if fresh {
			current[typeName]++
		}
		if !fresh || !evidenceApplies(record.fields, release, target, candidate, scope) {
			continue
		}
		level := stringField(record.fields, "level")
		if typeName == "runtime" || level == "runtime" {
			runtimeCurrent = true
		}
		if typeName == "business" || level == "business" {
			businessCurrent = true
		}
		if typeName == "regression" || level == "regression" {
			regressionCurrent = true
		}
		if typeName == "restoration" || level == "restoration" {
			restorationCurrent = true
		}
	}
	return map[string]any{
		"counts": counts, "current_counts": current, "items": items, "technical_reviews": reviews,
		"runtime_current": runtimeCurrent, "business_current": businessCurrent, "regression_current": regressionCurrent, "restoration_current": restorationCurrent,
	}
}

func projectObservedTarget(records []parsedRecord, targetKey string) (map[string]any, map[string]any, map[string]any) {
	var head, release, candidate, scope parsedRecord
	for _, record := range records {
		if record.Type == "target_head" && stringField(record.fields, "target_key") == targetKey && laterRecord(record, head) {
			head = record
		}
	}
	if head.ID == "" {
		return map[string]any{}, map[string]any{}, emptyEvidenceSummary()
	}
	for _, record := range records {
		switch record.Type {
		case "release":
			if record.ID == stringField(head.fields, "release_id") && laterRecord(record, release) {
				release = record
			}
		}
	}
	if release.ID != "" {
		for _, record := range records {
			switch record.Type {
			case "candidate":
				if record.ID == stringField(release.fields, "candidate_id") && record.Revision == intValue(release.fields["candidate_revision"]) {
					candidate = record
				}
			case "scope":
				if record.ID == stringField(release.fields, "scope_id") && record.Revision == intValue(release.fields["scope_revision"]) {
					scope = record
				}
			}
		}
	}
	target := summary(head, "target_key", "generation", "release_id", "issue_id", "candidate_id", "state", "updated_at")
	releaseSummary := summary(release, "candidate_id", "candidate_revision", "scope_id", "scope_revision", "authority_id", "authority_revision", "target_key", "target_generation", "state", "facets", "recovery_policy", "exposed_population", "started_at", "deployed_at", "restored_at", "attempt_contract_version", "acceptance_version", "operation_id", "branch_key", "release_kind", "expected_target_release_id", "expected_target_generation", "recovery_of_release_id", "head_observed_at", "release_operator_id", "planned_verifier_id", "build_manifest_digest", "artifact_digest", "runtime_contract_digest", "authority_capability", "authority_evidence_id", "recovery_policy_revision", "authority_envelope_digest", "owner_evidence_revision", "exposure_policy_digest", "rollback_target_release_id", "irreversible_effects_digest", "reviewed_review_id", "reviewed_review_revision")
	if release.ID == "" {
		return releaseSummary, target, emptyEvidenceSummary()
	}
	return releaseSummary, target, summarizeEvidence(records, releaseSummary, target, summary(candidate, "scope_id", "scope_revision", "candidate_type", "state", "source_revision", "artifact", "build_contract"), summary(scope, "acceptance_version", "state", "owner_evidence_id", "statement", "impact", "requires_deployment", "source_plan_revision"))
}

func evidenceRelated(fields, release, candidate map[string]any) bool {
	if stringField(fields, "candidate_id") != stringValue(candidate["id"], "") || stringValue(candidate["id"], "") == "" {
		return false
	}
	if targetKey := stringField(fields, "target_key"); targetKey != "" && targetKey != stringValue(release["target_key"], "") {
		return false
	}
	payload, _ := fields["payload"].(map[string]any)
	if releaseID := stringField(payload, "release_id"); releaseID != "" && releaseID != stringValue(release["id"], "") {
		return false
	}
	return true
}

func evidenceApplies(fields, release, target, candidate, scope map[string]any) bool {
	payload, _ := fields["payload"].(map[string]any)
	facets, _ := release["facets"].(map[string]any)
	typedOperator, typedVerifier := stringValue(release["release_operator_id"], ""), stringValue(release["planned_verifier_id"], "")
	legacyOperator, legacyVerifier := stringValue(facets["release_operator_id"], ""), stringValue(facets["planned_verifier_id"], "")
	if (typedOperator != "" && legacyOperator != "" && typedOperator != legacyOperator) || (typedVerifier != "" && legacyVerifier != "" && typedVerifier != legacyVerifier) {
		return false
	}
	operatorID := stringValue(release["release_operator_id"], legacyOperator)
	verifierID := stringValue(release["planned_verifier_id"], legacyVerifier)
	return stringField(fields, "candidate_id") != "" &&
		stringField(fields, "producer_type") == "verifier" &&
		stringField(fields, "producer_id") != "" && stringField(fields, "producer_id") == verifierID && verifierID != operatorID &&
		stringField(fields, "candidate_id") == stringValue(candidate["id"], "") &&
		intValue(payload["candidate_revision"]) == intValue(candidate["revision"]) &&
		stringField(payload, "release_id") == stringValue(release["id"], "") &&
		stringValue(target["release_id"], "") == stringValue(release["id"], "") &&
		stringField(fields, "target_key") == stringValue(release["target_key"], "") &&
		stringValue(target["target_key"], "") == stringValue(release["target_key"], "") &&
		intValue(payload["target_generation"]) == intValue(release["target_generation"]) &&
		intValue(target["generation"]) == intValue(release["target_generation"]) &&
		stringField(payload, "scope_id") == stringValue(scope["id"], "") &&
		intValue(payload["scope_revision"]) == intValue(release["scope_revision"]) &&
		intValue(payload["acceptance_version"]) == intValue(scope["acceptance_version"])
}

func originalBusinessRequest(records []parsedRecord) map[string]any {
	var original parsedRecord
	for _, record := range records {
		if record.Type != "evidence" || isSuperseded(record.fields) ||
			stringField(record.fields, "evidence_type") != "owner_request" ||
			stringField(record.fields, "producer_type") != "owner" {
			continue
		}
		if original.ID == "" || effectiveTime(record).Before(effectiveTime(original)) ||
			(effectiveTime(record).Equal(effectiveTime(original)) && record.ID < original.ID) {
			original = record
		}
	}
	if original.ID == "" {
		return map[string]any{"available": false, "reason": "owner_request_evidence_unavailable"}
	}
	result := summary(original, "source", "producer_type", "producer_id", "observed_at", "payload")
	result["available"] = true
	return result
}

func summarizeSupersession(records []parsedRecord) map[string]any {
	counts := map[string]int{}
	superseded := make([]map[string]any, 0)
	for _, record := range records {
		if isSuperseded(record.fields) {
			counts[record.Type]++
			item := summary(record, "superseded_by", "superseded_at")
			item["record_type"] = record.Type
			superseded = append(superseded, item)
		}
	}
	return map[string]any{"counts": counts, "records": superseded, "total": sumCounts(counts)}
}

func historicalCompletion(records []parsedRecord) map[string]any {
	var latestSatisfied parsedRecord
	for _, record := range records {
		if record.Type != "outcome" || stringField(record.fields, "state") != "satisfied" {
			continue
		}
		if laterRecord(record, latestSatisfied) {
			latestSatisfied = record
		}
	}
	if latestSatisfied.ID == "" {
		return map[string]any{"satisfied": false}
	}
	result := map[string]any{
		"satisfied":        true,
		"outcome_id":       latestSatisfied.ID,
		"outcome_revision": latestSatisfied.Revision,
	}
	if evidenceID, ok := latestSatisfied.fields["completion_evidence_id"]; ok && evidenceID != nil {
		result["completion_evidence_id"] = evidenceID
	}
	return result
}

func deriveHealth(release, target map[string]any, incidents []map[string]any, evidence map[string]any) string {
	if len(incidents) != 0 {
		return "degraded"
	}
	exactHead := stringValue(release["id"], "") != "" &&
		stringValue(target["release_id"], "") == stringValue(release["id"], "") &&
		intValue(target["generation"]) == intValue(release["target_generation"]) &&
		stringValue(target["state"], "") == stringValue(release["state"], "")
	switch stringValue(release["state"], "pending") {
	case "degraded", "failed", "rolling_back":
		return "degraded"
	case "rolled_back":
		return "degraded"
	case "restored":
		if exactHead && evidence["runtime_current"] == true && evidence["business_current"] == true && evidence["regression_current"] == true && evidence["restoration_current"] == true {
			return "restored"
		}
		return "unverified"
	case "live":
		if exactHead && evidence["runtime_current"] == true && evidence["business_current"] == true && evidence["regression_current"] == true {
			return "healthy"
		}
		return "unverified"
	case "deployed", "exposed", "production_verifying":
		return "verifying"
	default:
		return "unknown"
	}
}

func relatedIncidents(incidents []map[string]any, release, target map[string]any) []map[string]any {
	result := make([]map[string]any, 0)
	for _, incident := range incidents {
		if stringValue(incident["release_id"], "") == stringValue(release["id"], "") &&
			stringValue(incident["target_key"], "") == stringValue(target["target_key"], "") &&
			intValue(incident["target_generation"]) == intValue(target["generation"]) {
			result = append(result, incident)
		}
	}
	return result
}

func milestone(outcome, execution, release, health, attention string) string {
	if attention != "resolved" {
		return "needs_attention"
	}
	if health == "degraded" || health == "restored" {
		return health
	}
	if release != "" && release != "pending" && release != "not_required" {
		return release
	}
	if outcome == "satisfied" || outcome == "cancelled" || outcome == "superseded" {
		return outcome
	}
	return execution
}

func isSuperseded(fields map[string]any) bool {
	return fields["superseded_at"] != nil && stringValue(fields["superseded_at"], "") != ""
}

type fieldKind byte

const (
	stringKind fieldKind = iota
	stringAllowEmptyKind
	integerKind
	objectKind
	arrayKind
)

var requiredShape = map[string]map[string]fieldKind{
	"outcome":             {"state": stringKind, "acceptance_version": integerKind},
	"scope":               {"state": stringKind, "acceptance_version": integerKind, "owner_evidence_id": stringKind, "created_by": stringKind, "statement": objectKind, "impact": objectKind},
	"authority":           {"state": stringKind, "scope_id": stringKind, "scope_revision": integerKind, "owner_evidence_id": stringKind, "created_by": stringKind, "envelope": objectKind},
	"execution":           {"branch_key": stringKind, "operation_type": stringKind, "state": stringKind, "scope_id": stringKind, "scope_revision": integerKind, "checkpoint": objectKind, "expected_event": objectKind, "wait_reason": objectKind, "attempt_count": integerKind},
	"decision":            {"decision_type": stringKind, "state": stringKind, "resolution_state": stringKind, "question": stringKind, "options": arrayKind, "requester_id": stringKind, "accountable_owner_id": stringKind, "suspended_operation_id": stringKind},
	"decision_response":   {"decision_id": stringKind, "decision_revision": integerKind, "response_type": stringKind, "actor_id": stringKind, "idempotency_key": stringKind, "request_hash": stringKind, "selected_options": arrayKind, "content": objectKind, "scope_change": objectKind},
	"candidate":           {"scope_id": stringKind, "scope_revision": integerKind, "candidate_type": stringKind, "state": stringKind, "artifact": objectKind, "build_contract": objectKind, "created_by": stringKind},
	"review":              {"candidate_id": stringKind, "candidate_revision": integerKind, "scope_id": stringKind, "scope_revision": integerKind, "reviewer_id": stringKind, "outcome": stringKind, "criteria": arrayKind, "coverage": objectKind, "evidence_refs": arrayKind, "gaps": arrayKind, "correction_packet": objectKind, "correction_count": integerKind, "max_corrections": integerKind},
	"release":             {"candidate_id": stringKind, "candidate_revision": integerKind, "scope_id": stringKind, "scope_revision": integerKind, "authority_id": stringKind, "authority_revision": integerKind, "target_key": stringAllowEmptyKind, "target_generation": integerKind, "state": stringKind, "facets": objectKind, "recovery_policy": objectKind, "exposed_population": objectKind},
	"evidence":            {"evidence_type": stringKind, "level": stringKind, "source": stringKind, "producer_type": stringKind, "observed_at": stringKind, "coverage": objectKind, "limitations": arrayKind, "contradictions": arrayKind, "payload": objectKind},
	"incident":            {"incident_type": stringKind, "severity": stringKind, "state": stringKind, "consequence": objectKind, "recovery": objectKind},
	"action_receipt":      {"action_key": stringKind, "request_hash": stringKind, "command_id": stringKind, "action_type": stringKind, "state": stringKind, "payload": objectKind, "attempt_count": integerKind, "max_attempts": integerKind},
	"attention":           {"attention_type": stringKind, "read_state": stringKind, "resolution_state": stringKind, "context": objectKind, "next_action": objectKind},
	"budget":              {"scope_id": stringKind, "concern_type": stringKind, "state": stringKind, "limits": objectKind, "consumption": objectKind},
	"target_head":         {"target_key": stringKind, "generation": integerKind, "release_id": stringKind, "issue_id": stringKind, "candidate_id": stringKind, "state": stringKind},
	"consequence":         {"source_issue_id": stringKind, "affected_issue_id": stringKind, "receipt_id": stringKind, "idempotency_key": stringKind, "relationship_kind": stringKind, "payload_sha256": stringKind, "receipt": objectKind},
	"regression_change":   {"registry_version": integerKind, "proposal_id": stringKind, "obligation_id": stringKind, "record_hash": stringKind, "record": objectKind},
	"regression_rejected": {"sequence": integerKind, "registry_version": integerKind, "proposal_id": stringKind, "obligation_id": stringKind, "submission_hash": stringKind, "attempt_hash": stringKind, "attempt": objectKind},
}

func validateRecord(record Record, fields map[string]any) error {
	if fields == nil {
		return fmt.Errorf("decode %s %s: payload must be an object", record.Type, record.ID)
	}
	if record.CreatedAt.IsZero() {
		return fmt.Errorf("invalid %s %s: envelope time is required", record.Type, record.ID)
	}
	idField := "id"
	if record.Type == "target_head" {
		idField = "release_id"
	}
	payloadID, ok := fields[idField].(string)
	if !ok || payloadID == "" || payloadID != record.ID {
		return fmt.Errorf("invalid %s %s: payload %s must equal envelope id", record.Type, record.ID, idField)
	}
	if record.Type == "review" || record.Type == "evidence" || record.Type == "decision_response" || record.Type == "consequence" {
		if record.Revision != 1 {
			return fmt.Errorf("invalid %s %s: immutable envelope revision must be 1", record.Type, record.ID)
		}
	} else if record.Type == "regression_change" {
		payloadRevision, ok := numberInt64(fields["registry_version"])
		if !ok || payloadRevision != record.Revision {
			return fmt.Errorf("invalid %s %s: payload registry_version must equal envelope revision", record.Type, record.ID)
		}
	} else if record.Type == "regression_rejected" {
		payloadRevision, ok := numberInt64(fields["sequence"])
		if !ok || payloadRevision != record.Revision {
			return fmt.Errorf("invalid %s %s: payload sequence must equal envelope revision", record.Type, record.ID)
		}
	} else {
		payloadRevision, ok := numberInt64(fields["revision"])
		if !ok || payloadRevision != record.Revision {
			return fmt.Errorf("invalid %s %s: payload revision must equal envelope revision", record.Type, record.ID)
		}
	}
	timeField := "created_at"
	if record.Type == "target_head" {
		timeField = "updated_at"
	}
	payloadTime, ok := fields[timeField].(string)
	if !ok {
		return fmt.Errorf("invalid %s %s: payload %s is required", record.Type, record.ID, timeField)
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, payloadTime)
	if err != nil || !parsedTime.Equal(record.CreatedAt) {
		return fmt.Errorf("invalid %s %s: payload %s must equal envelope time", record.Type, record.ID, timeField)
	}
	for name, kind := range requiredShape[record.Type] {
		value, exists := fields[name]
		if !exists || value == nil {
			return fmt.Errorf("invalid %s %s: required field %s is missing or null", record.Type, record.ID, name)
		}
		valid := false
		switch kind {
		case stringKind:
			text, isString := value.(string)
			valid = isString && text != ""
		case stringAllowEmptyKind:
			_, valid = value.(string)
		case integerKind:
			_, valid = numberInt64(value)
		case objectKind:
			_, valid = value.(map[string]any)
		case arrayKind:
			_, valid = value.([]any)
		}
		if !valid {
			return fmt.Errorf("invalid %s %s: required field %s has wrong type", record.Type, record.ID, name)
		}
	}
	if err := validateReleaseProjectionShape(record, fields); err != nil {
		return err
	}
	return nil
}

func validateReleaseProjectionShape(record Record, fields map[string]any) error {
	if record.Type != "release" && record.Type != "incident" {
		return nil
	}
	integerFields := []string{}
	stringFields := []string{}
	if record.Type == "release" {
		integerFields = []string{"attempt_contract_version", "acceptance_version", "expected_target_generation", "recovery_policy_revision", "owner_evidence_revision", "reviewed_review_revision"}
		stringFields = []string{"operation_id", "branch_key", "expected_target_release_id", "release_kind", "recovery_of_release_id", "head_observed_at", "release_operator_id", "planned_verifier_id", "build_manifest_digest", "artifact_digest", "runtime_contract_digest", "authority_capability", "authority_evidence_id", "authority_envelope_digest", "exposure_policy_digest", "rollback_target_release_id", "irreversible_effects_digest", "reviewed_review_id"}
	} else {
		integerFields = []string{"release_revision", "candidate_revision", "target_generation"}
		stringFields = []string{"candidate_id", "detection_evidence_id", "recovery_action_id", "compatibility_state"}
	}
	for _, name := range integerFields {
		if value, ok := fields[name]; ok && value != nil {
			if _, valid := numberInt64(value); !valid {
				return fmt.Errorf("invalid %s %s: optional field %s has wrong type", record.Type, record.ID, name)
			}
		}
	}
	for _, name := range stringFields {
		if value, ok := fields[name]; ok && value != nil {
			if _, valid := value.(string); !valid {
				return fmt.Errorf("invalid %s %s: optional field %s has wrong type", record.Type, record.ID, name)
			}
		}
	}
	return nil
}

func validateProjectionRelations(records []parsedRecord) error {
	byKey := make(map[string]parsedRecord, len(records))
	for _, record := range records {
		byKey[fmt.Sprintf("%s/%s/%d", record.Type, record.ID, record.Revision)] = record
		if record.Type == "release" && intValue(record.fields["attempt_contract_version"]) > 1 {
			version := intValue(record.fields["attempt_contract_version"])
			if (version != 2 && version != 3) || intValue(record.fields["acceptance_version"]) < 1 || stringField(record.fields, "operation_id") == "" || stringField(record.fields, "branch_key") == "" ||
				record.fields["expected_target_generation"] == nil || stringField(record.fields, "release_operator_id") == "" || stringField(record.fields, "planned_verifier_id") == "" ||
				stringField(record.fields, "release_operator_id") == stringField(record.fields, "planned_verifier_id") || stringField(record.fields, "build_manifest_digest") == "" ||
				stringField(record.fields, "artifact_digest") == "" || stringField(record.fields, "runtime_contract_digest") == "" || stringField(record.fields, "authority_capability") == "" ||
				stringField(record.fields, "authority_evidence_id") == "" || intValue(record.fields["recovery_policy_revision"]) < 1 {
				return fmt.Errorf("release %s has an incomplete v2 attempt snapshot", record.ID)
			}
			if version == 3 && (stringField(record.fields, "authority_envelope_digest") == "" || intValue(record.fields["owner_evidence_revision"]) < 1 || stringField(record.fields, "exposure_policy_digest") == "" || stringField(record.fields, "irreversible_effects_digest") == "" || stringField(record.fields, "reviewed_review_id") == "" || intValue(record.fields["reviewed_review_revision"]) < 1) {
				return fmt.Errorf("release %s has an incomplete v3 authority snapshot", record.ID)
			}
		}
	}
	for _, incident := range records {
		if incident.Type != "incident" {
			continue
		}
		exactFields := []string{"release_revision", "candidate_id", "candidate_revision", "target_generation", "detection_evidence_id"}
		present := 0
		for _, key := range exactFields {
			if incident.fields[key] != nil {
				present++
			}
		}
		if present == 0 {
			continue
		}
		if present != len(exactFields) || stringField(incident.fields, "release_id") == "" || stringField(incident.fields, "target_key") == "" || intValue(incident.fields["target_generation"]) < 1 {
			return fmt.Errorf("incident %s has a partial exact release binding", incident.ID)
		}
		release, ok := byKey[fmt.Sprintf("release/%s/%d", stringField(incident.fields, "release_id"), intValue(incident.fields["release_revision"]))]
		if !ok || stringField(incident.fields, "candidate_id") != stringField(release.fields, "candidate_id") || intValue(incident.fields["candidate_revision"]) != intValue(release.fields["candidate_revision"]) || stringField(incident.fields, "target_key") != stringField(release.fields, "target_key") || intValue(incident.fields["target_generation"]) != intValue(release.fields["target_generation"]) {
			return fmt.Errorf("incident %s disagrees with its exact release", incident.ID)
		}
		foundEvidence := false
		for _, evidence := range records {
			if evidence.Type == "evidence" && evidence.ID == stringField(incident.fields, "detection_evidence_id") {
				foundEvidence = true
				break
			}
		}
		if !foundEvidence {
			return fmt.Errorf("incident %s detection evidence is missing", incident.ID)
		}
	}
	return nil
}

func stringField(fields map[string]any, key string) string { return stringValue(fields[key], "") }

func stringValue(value any, fallback string) string {
	if text, ok := value.(string); ok && text != "" {
		return text
	}
	return fallback
}

func intValue(value any) int64 {
	n, _ := numberInt64(value)
	return n
}

func numberInt64(value any) (int64, bool) {
	switch value := value.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value || value < math.MinInt64 || value >= 9223372036854775808.0 {
			return 0, false
		}
		return int64(value), true
	case json.Number:
		n, err := value.Int64()
		return n, err == nil
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func intPointer(value any) *int64 {
	if n, ok := numberInt64(value); ok {
		return &n
	}
	return nil
}

func nullableInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func pick(source map[string]any, keys ...string) map[string]any {
	result := map[string]any{}
	for _, key := range keys {
		if value, ok := source[key]; ok && value != nil {
			result[key] = value
		}
	}
	return result
}

func cloneMap(source map[string]any) map[string]any {
	return pick(source, sortedKeys(source)...)
}

func sortedKeys(source map[string]any) []string {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func emptyJSONValue(value any) bool {
	if value == nil {
		return true
	}
	switch value := value.(type) {
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	case string:
		return strings.TrimSpace(value) == ""
	default:
		return false
	}
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

// SemanticEqual compares only rebuild-stable projection content.
func SemanticEqual(a, b Projection) bool {
	return bytes.Equal(a.Document, b.Document) && bytes.Equal(a.SourceRevisions, b.SourceRevisions)
}
