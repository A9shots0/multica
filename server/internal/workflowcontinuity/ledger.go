// Package workflowcontinuity derives the bounded task-facing continuity ledger
// from Multica's normalized Current Picture. It is a projection only: it grants
// no authority and never writes workflow state.
package workflowcontinuity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	SchemaVersion    = 1
	MaxLedgerBytes   = 64 << 10
	MaxGoalTextBytes = 4 << 10
	MaxTokenBytes    = 256
	MaxBlockers      = 128
)

var ErrInvalidCurrentPicture = errors.New("invalid workflow continuity current picture")

type Ledger struct {
	SchemaVersion          int        `json:"schema_version"`
	CurrentPictureRevision int64      `json:"current_picture_revision"`
	OriginalGoal           Goal       `json:"original_goal"`
	Acceptance             Acceptance `json:"acceptance"`
	Blockers               []Blocker  `json:"blockers"`
}

// Goal contains only owner-authored prose from known goal fields. The digests
// retain identity for a richer source object without copying arbitrary payload
// keys (or potentially private values) into a model prompt.
type Goal struct {
	OwnerRequest      OwnerRequest `json:"owner_request"`
	AcceptedScope     GoalSource   `json:"accepted_scope"`
	AcceptanceVersion int64        `json:"acceptance_version"`
}

type OwnerRequest struct {
	Available     bool     `json:"available"`
	EvidenceID    string   `json:"evidence_id,omitempty"`
	Text          GoalText `json:"text,omitempty"`
	PayloadSHA256 string   `json:"payload_sha256,omitempty"`
	Unavailable   string   `json:"unavailable_reason,omitempty"`
}

type GoalSource struct {
	Text            GoalText `json:"text,omitempty"`
	StatementSHA256 string   `json:"statement_sha256,omitempty"`
}

type GoalText struct {
	Goal        string `json:"goal,omitempty"`
	Objective   string `json:"objective,omitempty"`
	Request     string `json:"request,omitempty"`
	Description string `json:"description,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Title       string `json:"title,omitempty"`
	Boundary    string `json:"boundary,omitempty"`
}

type Acceptance struct {
	State                      string `json:"state"`
	AcceptanceVersion          int64  `json:"acceptance_version"`
	OutcomeID                  string `json:"outcome_id,omitempty"`
	OutcomeRevision            int64  `json:"outcome_revision,omitempty"`
	CompletionEvidenceID       string `json:"completion_evidence_id,omitempty"`
	OutcomeSatisfied           bool   `json:"outcome_satisfied"`
	OwnerAcceptanceReceiptID   string `json:"owner_acceptance_receipt_id,omitempty"`
	OwnerAcceptanceActorID     string `json:"owner_acceptance_actor_id,omitempty"`
	Accepted                   bool   `json:"accepted"`
	RequiresExternalAcceptance bool   `json:"requires_external_acceptance"`
}

// OwnerAcceptance is the authoritative human accept_result receipt admitted by
// Chat. A normalized satisfied outcome is evidence of delivery, not owner
// acceptance, so only this receipt may close the original goal in the ledger.
type OwnerAcceptance struct {
	ReceiptID       string
	ActorID         string
	OutcomeID       string
	OutcomeRevision int64
}

type BlockerOwner struct {
	System string `json:"system"`
	Role   string `json:"role"`
	ID     string `json:"id,omitempty"`
}

type WaitReason struct {
	Code          string `json:"code,omitempty"`
	ReportedState string `json:"reported_state,omitempty"`
}

type ExpectedEvent struct {
	Type                 string `json:"type,omitempty"`
	Provider             string `json:"provider,omitempty"`
	WorkID               string `json:"work_id,omitempty"`
	AfterReceiptRevision int64  `json:"after_receipt_revision,omitempty"`
}

type ResumeCondition struct {
	Type          string        `json:"type"`
	BlockerID     string        `json:"blocker_id"`
	ExpectedEvent ExpectedEvent `json:"expected_event"`
}

type NextSafeAction struct {
	Type string `json:"type"`
	At   string `json:"at,omitempty"`
}

type Evidence struct {
	WorkflowExecutionRevision int64 `json:"workflow_execution_revision"`
	CurrentPictureRevision    int64 `json:"current_picture_revision"`
}

type Blocker struct {
	ID              string          `json:"id"`
	Fingerprint     string          `json:"fingerprint"`
	Owner           BlockerOwner    `json:"owner"`
	WaitReason      WaitReason      `json:"wait_reason"`
	ResumeCondition ResumeCondition `json:"resume_condition"`
	NextSafeAction  NextSafeAction  `json:"next_safe_action"`
	Evidence        Evidence        `json:"evidence"`
}

type currentPicture struct {
	ContractVersion         int            `json:"contract_version"`
	OriginalBusinessRequest map[string]any `json:"original_business_request"`
	Outcome                 map[string]any `json:"outcome"`
	Scope                   map[string]any `json:"scope"`
	UnresolvedDependencies  []dependency   `json:"unresolved_dependencies"`
}

type dependency struct {
	ID            string         `json:"id"`
	Revision      int64          `json:"revision"`
	ActorType     string         `json:"actor_type"`
	ActorID       string         `json:"actor_id"`
	WaitReason    map[string]any `json:"wait_reason"`
	ExpectedEvent map[string]any `json:"expected_event"`
	Checkpoint    map[string]any `json:"checkpoint"`
	NextActionAt  string         `json:"next_action_at"`
}

// Build returns the canonical task-facing ledger for one authoritative Current
// Picture revision. Equivalent blocker semantics produce the same fingerprint
// across later observations, so resumed sessions can suppress repeated audits.
func Build(currentPictureRevision int64, document []byte, ownerAcceptance *OwnerAcceptance) (json.RawMessage, error) {
	if currentPictureRevision < 1 || len(document) == 0 || !json.Valid(document) {
		return nil, ErrInvalidCurrentPicture
	}
	var picture currentPicture
	if err := json.Unmarshal(document, &picture); err != nil || (picture.ContractVersion != 2 && picture.ContractVersion != 3) {
		return nil, ErrInvalidCurrentPicture
	}
	if len(picture.UnresolvedDependencies) > MaxBlockers {
		return nil, fmt.Errorf("%w: more than %d unresolved dependencies", ErrInvalidCurrentPicture, MaxBlockers)
	}

	outcomeState := safeToken(stringValue(picture.Outcome, "state"))
	outcomeID := safeToken(stringValue(picture.Outcome, "id"))
	outcomeRevision := int64Value(picture.Outcome, "revision")
	acceptanceVersion := int64Value(picture.Outcome, "acceptance_version")
	completionEvidenceID := safeToken(stringValue(picture.Outcome, "completion_evidence_id"))
	outcomeSatisfied := outcomeState == "satisfied" && completionEvidenceID != ""
	ownerAccepted := outcomeSatisfied && ownerAcceptance != nil && safeToken(ownerAcceptance.ReceiptID) != "" && safeToken(ownerAcceptance.ActorID) != "" &&
		safeToken(ownerAcceptance.OutcomeID) == outcomeID && ownerAcceptance.OutcomeRevision == outcomeRevision && outcomeID != "" && outcomeRevision > 0
	ledger := Ledger{
		SchemaVersion:          SchemaVersion,
		CurrentPictureRevision: currentPictureRevision,
		OriginalGoal: Goal{
			OwnerRequest:      ownerRequestFrom(picture.OriginalBusinessRequest),
			AcceptedScope:     goalSourceFrom(objectValue(picture.Scope, "statement")),
			AcceptanceVersion: int64Value(picture.Scope, "acceptance_version"),
		},
		Acceptance: Acceptance{
			State:                      outcomeState,
			AcceptanceVersion:          acceptanceVersion,
			OutcomeID:                  outcomeID,
			OutcomeRevision:            outcomeRevision,
			CompletionEvidenceID:       completionEvidenceID,
			OutcomeSatisfied:           outcomeSatisfied,
			Accepted:                   ownerAccepted,
			RequiresExternalAcceptance: !ownerAccepted,
		},
		Blockers: []Blocker{},
	}
	if ownerAccepted {
		ledger.Acceptance.OwnerAcceptanceReceiptID = safeToken(ownerAcceptance.ReceiptID)
		ledger.Acceptance.OwnerAcceptanceActorID = safeToken(ownerAcceptance.ActorID)
	}

	seen := make(map[string]struct{}, len(picture.UnresolvedDependencies))
	for _, item := range picture.UnresolvedDependencies {
		blocker, err := blockerFromDependency(currentPictureRevision, item)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[blocker.Fingerprint]; duplicate {
			continue
		}
		seen[blocker.Fingerprint] = struct{}{}
		ledger.Blockers = append(ledger.Blockers, blocker)
	}
	sort.Slice(ledger.Blockers, func(i, j int) bool {
		if ledger.Blockers[i].ID != ledger.Blockers[j].ID {
			return ledger.Blockers[i].ID < ledger.Blockers[j].ID
		}
		return ledger.Blockers[i].Fingerprint < ledger.Blockers[j].Fingerprint
	})
	if err := validateLedger(ledger); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(ledger)
	if err != nil {
		return nil, fmt.Errorf("%w: encode ledger: %v", ErrInvalidCurrentPicture, err)
	}
	if len(raw) > MaxLedgerBytes {
		return nil, fmt.Errorf("%w: ledger exceeds %d bytes", ErrInvalidCurrentPicture, MaxLedgerBytes)
	}
	return raw, nil
}

// Parse strictly validates prompt-bound wire data. Unknown fields, oversized
// text, non-token identifiers, or forged blocker fingerprints are rejected so
// prompt prose is not the security boundary for this optional metadata.
func Parse(raw []byte) (Ledger, error) {
	if len(raw) == 0 || len(raw) > MaxLedgerBytes || !json.Valid(raw) {
		return Ledger{}, ErrInvalidCurrentPicture
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var ledger Ledger
	if err := decoder.Decode(&ledger); err != nil {
		return Ledger{}, fmt.Errorf("%w: decode ledger: %v", ErrInvalidCurrentPicture, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Ledger{}, ErrInvalidCurrentPicture
	}
	if err := validateLedger(ledger); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func blockerFromDependency(currentPictureRevision int64, item dependency) (Blocker, error) {
	item.ID = safeToken(item.ID)
	if item.ID == "" || item.Revision < 1 {
		return Blocker{}, fmt.Errorf("%w: dependency identity is missing or unsafe", ErrInvalidCurrentPicture)
	}
	ownerSystem := safeToken(stringValue(item.Checkpoint, "provider"))
	if ownerSystem == "" {
		ownerSystem = safeToken(item.ActorType)
	}
	if ownerSystem == "" {
		ownerSystem = "unknown"
	}
	ownerRole := safeToken(item.ActorType)
	if ownerRole == "" {
		ownerRole = "accountable_dependency_owner"
	}
	owner := BlockerOwner{System: ownerSystem, Role: ownerRole, ID: safeToken(item.ActorID)}
	waitReason := WaitReason{
		Code:          safeToken(stringValue(item.WaitReason, "code")),
		ReportedState: safeToken(stringValue(item.WaitReason, "reported_state")),
	}
	expectedEvent := ExpectedEvent{
		Type:                 safeToken(stringValue(item.ExpectedEvent, "type")),
		Provider:             safeToken(stringValue(item.ExpectedEvent, "provider")),
		WorkID:               safeToken(stringValue(item.ExpectedEvent, "work_id")),
		AfterReceiptRevision: int64Value(item.ExpectedEvent, "after_receipt_revision"),
	}
	resume := ResumeCondition{Type: "current_picture_blocker_cleared", BlockerID: item.ID, ExpectedEvent: expectedEvent}
	nextSafeAction := NextSafeAction{Type: "wait_for_current_picture_change"}
	if expectedEvent.Type != "" {
		nextSafeAction.Type = "wait_for_expected_event"
	}
	if at := canonicalTime(item.NextActionAt); at != "" {
		nextSafeAction = NextSafeAction{Type: "wait_until", At: at}
	}
	evidence := Evidence{WorkflowExecutionRevision: item.Revision, CurrentPictureRevision: currentPictureRevision}
	blocker := Blocker{
		ID: item.ID, Owner: owner, WaitReason: waitReason, ResumeCondition: resume,
		NextSafeAction: nextSafeAction, Evidence: evidence,
	}
	blocker.Fingerprint = fingerprint(blocker)
	return blocker, nil
}

func validateLedger(ledger Ledger) error {
	if ledger.SchemaVersion != SchemaVersion || ledger.CurrentPictureRevision < 1 || len(ledger.Blockers) > MaxBlockers {
		return ErrInvalidCurrentPicture
	}
	if !validGoalText(ledger.OriginalGoal.OwnerRequest.Text) || !validGoalText(ledger.OriginalGoal.AcceptedScope.Text) ||
		!validDigest(ledger.OriginalGoal.OwnerRequest.PayloadSHA256) || !validDigest(ledger.OriginalGoal.AcceptedScope.StatementSHA256) {
		return ErrInvalidCurrentPicture
	}
	if ledger.OriginalGoal.OwnerRequest.EvidenceID != "" && safeToken(ledger.OriginalGoal.OwnerRequest.EvidenceID) != ledger.OriginalGoal.OwnerRequest.EvidenceID {
		return ErrInvalidCurrentPicture
	}
	ownerRequest := ledger.OriginalGoal.OwnerRequest
	if (!ownerRequest.Available && (ownerRequest.Unavailable != "owner_request_evidence_unavailable" || ownerRequest.EvidenceID != "" || ownerRequest.PayloadSHA256 != "" || !emptyGoalText(ownerRequest.Text))) ||
		(ownerRequest.Available && ownerRequest.Unavailable != "") {
		return ErrInvalidCurrentPicture
	}
	if ledger.Acceptance.State == "" || safeToken(ledger.Acceptance.State) != ledger.Acceptance.State ||
		!validOptionalToken(ledger.Acceptance.OutcomeID) || ledger.Acceptance.OutcomeRevision < 0 ||
		!validOptionalToken(ledger.Acceptance.CompletionEvidenceID) || !validOptionalToken(ledger.Acceptance.OwnerAcceptanceReceiptID) ||
		!validOptionalToken(ledger.Acceptance.OwnerAcceptanceActorID) || ledger.Acceptance.RequiresExternalAcceptance == ledger.Acceptance.Accepted {
		return ErrInvalidCurrentPicture
	}
	if ledger.Acceptance.Accepted && (!ledger.Acceptance.OutcomeSatisfied || ledger.Acceptance.OutcomeID == "" || ledger.Acceptance.OutcomeRevision < 1 ||
		ledger.Acceptance.OwnerAcceptanceReceiptID == "" || ledger.Acceptance.OwnerAcceptanceActorID == "") {
		return ErrInvalidCurrentPicture
	}
	if !ledger.Acceptance.Accepted && (ledger.Acceptance.OwnerAcceptanceReceiptID != "" || ledger.Acceptance.OwnerAcceptanceActorID != "") {
		return ErrInvalidCurrentPicture
	}
	wantOutcomeSatisfied := ledger.Acceptance.State == "satisfied" && ledger.Acceptance.CompletionEvidenceID != ""
	if ledger.Acceptance.OutcomeSatisfied != wantOutcomeSatisfied {
		return ErrInvalidCurrentPicture
	}
	for _, blocker := range ledger.Blockers {
		if safeToken(blocker.ID) != blocker.ID || safeToken(blocker.Owner.System) != blocker.Owner.System ||
			safeToken(blocker.Owner.Role) != blocker.Owner.Role || !validOptionalToken(blocker.Owner.ID) ||
			!validOptionalToken(blocker.WaitReason.Code) || !validOptionalToken(blocker.WaitReason.ReportedState) ||
			blocker.ResumeCondition.Type != "current_picture_blocker_cleared" || blocker.ResumeCondition.BlockerID != blocker.ID ||
			!validExpectedEvent(blocker.ResumeCondition.ExpectedEvent) || !validNextSafeAction(blocker.NextSafeAction) ||
			blocker.Evidence.WorkflowExecutionRevision < 1 || blocker.Evidence.CurrentPictureRevision != ledger.CurrentPictureRevision ||
			fingerprint(blocker) != blocker.Fingerprint {
			return ErrInvalidCurrentPicture
		}
	}
	return nil
}

func ownerRequestFrom(value map[string]any) OwnerRequest {
	available, _ := value["available"].(bool)
	if !available {
		return OwnerRequest{Available: false, Unavailable: "owner_request_evidence_unavailable"}
	}
	payload := objectValue(value, "payload")
	return OwnerRequest{
		Available: true, EvidenceID: safeToken(stringValue(value, "id")),
		Text: goalTextFrom(payload), PayloadSHA256: digest(payload),
	}
}

func goalSourceFrom(value map[string]any) GoalSource {
	return GoalSource{Text: goalTextFrom(value), StatementSHA256: digest(value)}
}

func goalTextFrom(value map[string]any) GoalText {
	return GoalText{
		Goal: boundedText(stringValue(value, "goal")), Objective: boundedText(stringValue(value, "objective")),
		Request: boundedText(stringValue(value, "request")), Description: boundedText(stringValue(value, "description")),
		Summary: boundedText(stringValue(value, "summary")),
		Title:   boundedText(stringValue(value, "title")), Boundary: boundedText(stringValue(value, "boundary")),
	}
}

func objectValue(values map[string]any, key string) map[string]any {
	value, _ := values[key].(map[string]any)
	return value
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func int64Value(values map[string]any, key string) int64 {
	switch value := values[key].(type) {
	case float64:
		return int64(value)
	case json.Number:
		result, _ := value.Int64()
		return result
	default:
		return 0
	}
}

func fingerprint(blocker Blocker) string {
	material := struct {
		ID              string          `json:"id"`
		Owner           BlockerOwner    `json:"owner"`
		WaitReason      WaitReason      `json:"wait_reason"`
		ResumeCondition ResumeCondition `json:"resume_condition"`
		NextSafeAction  NextSafeAction  `json:"next_safe_action"`
	}{blocker.ID, blocker.Owner, blocker.WaitReason, blocker.ResumeCondition, blocker.NextSafeAction}
	raw, _ := json.Marshal(material)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func digest(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func boundedText(value string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.TrimSpace(value) {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if space {
			if b.Len()+1 > MaxGoalTextBytes {
				break
			}
			b.WriteByte(' ')
			space = false
		}
		if b.Len()+utf8.RuneLen(r) > MaxGoalTextBytes {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func safeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > MaxTokenBytes {
		return ""
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("._:/@+-", r) {
			return ""
		}
	}
	return value
}

func canonicalTime(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func validOptionalToken(value string) bool { return value == "" || safeToken(value) == value }

func validDigest(value string) bool {
	if value == "" {
		return true
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func validGoalText(value GoalText) bool {
	for _, field := range []string{value.Goal, value.Objective, value.Request, value.Description, value.Summary, value.Title, value.Boundary} {
		if len(field) > MaxGoalTextBytes || boundedText(field) != field {
			return false
		}
	}
	return true
}

func emptyGoalText(value GoalText) bool {
	return value == (GoalText{})
}

func validExpectedEvent(value ExpectedEvent) bool {
	return validOptionalToken(value.Type) && validOptionalToken(value.Provider) && validOptionalToken(value.WorkID) && value.AfterReceiptRevision >= 0
}

func validNextSafeAction(value NextSafeAction) bool {
	switch value.Type {
	case "wait_for_current_picture_change", "wait_for_expected_event":
		return value.At == ""
	case "wait_until":
		return value.At != "" && canonicalTime(value.At) == value.At
	default:
		return false
	}
}
