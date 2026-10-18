package systemgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrRegressionInvalid        = errors.New("invalid regression change")
	ErrRegressionUnauthorized   = errors.New("regression change is unauthorized")
	ErrRegressionConflict       = errors.New("regression change conflicts with registry history")
	ErrRegressionStale          = errors.New("regression change evidence is stale")
	ErrRegressionConsumerDenied = errors.New("regression registry consumer is not allowed")
	ErrRegressionHistoryCorrupt = errors.New("regression registry history is corrupt")
)

type RegressionChangeKind string

const (
	RegressionAdd       RegressionChangeKind = "add"
	RegressionUpdate    RegressionChangeKind = "update"
	RegressionSupersede RegressionChangeKind = "supersede"
	RegressionRetire    RegressionChangeKind = "retire"
)

type RegressionObligationStatus string

const (
	RegressionActive  RegressionObligationStatus = "active"
	RegressionRetired RegressionObligationStatus = "retired"
)

const (
	RegressionAuthorAccountableDomain = "accountable_outcome_owner"
	RegressionConsumerRelease         = "release_runtime_assurance"
	RegressionConsumerQualityChecker  = "independent_outcome_assurance"
)

// RegressionScope is a bounded system-graph selection. An obligation is
// relevant when at least one of its declared targets is affected.
type RegressionScope struct {
	JourneyIDs []string `json:"journey_ids,omitempty"`
	NodeIDs    []string `json:"node_ids,omitempty"`
	EdgeIDs    []string `json:"edge_ids,omitempty"`
}

// RegressionImpactScope is the controller's immutable declaration of the
// complete graph impact for one issue. ActiveFor accepts this bound receipt,
// rather than an ad-hoc subset supplied by a release or QC consumer.
type RegressionImpactScope struct {
	ControllerReceiptID string          `json:"controller_receipt_id"`
	GraphVersion        string          `json:"graph_version"`
	Source              string          `json:"source"`
	Complete            bool            `json:"complete"`
	Scope               RegressionScope `json:"scope"`
	BindingHash         string          `json:"binding_hash"`
}

type RegressionObligation struct {
	ID                      string                     `json:"id"`
	Version                 uint64                     `json:"version"`
	Title                   string                     `json:"title"`
	Requirement             string                     `json:"requirement"`
	ProofMethod             string                     `json:"proof_method"`
	AccountableDomainRoleID string                     `json:"accountable_domain_role_id"`
	Scope                   RegressionScope            `json:"scope"`
	Status                  RegressionObligationStatus `json:"status"`
	SupersedesVersion       uint64                     `json:"supersedes_version,omitempty"`
	ReviewAfter             time.Time                  `json:"review_after"`
}

// RegressionProposal is a complete proposed snapshot. ExpectedVersion makes
// concurrent or stale changes fail closed; ProposedVersion must be the next
// version. Supersede intentionally keeps the obligation active while recording
// that its prior semantics were replaced.
type RegressionProposal struct {
	ID              string               `json:"id"`
	Kind            RegressionChangeKind `json:"kind"`
	ExpectedVersion uint64               `json:"expected_version"`
	ProposedVersion uint64               `json:"proposed_version"`
	AuthorRoleID    string               `json:"author_role_id"`
	AuthorFunction  string               `json:"author_function"`
	CreatedAt       time.Time            `json:"created_at"`
	Obligation      RegressionObligation `json:"obligation"`
}

// RegressionDomainEvidence binds the proposal to evidence from the accountable
// domain. Exact scope equality prevents an author from approving a wider
// obligation than the evidence covers.
type RegressionDomainEvidence struct {
	ID                string          `json:"id"`
	ProposalID        string          `json:"proposal_id"`
	AuthorAgentID     string          `json:"author_agent_id"`
	AccountableRoleID string          `json:"accountable_role_id"`
	Scope             RegressionScope `json:"scope"`
	SourceRefs        []string        `json:"source_refs"`
	Source            string          `json:"source"`
	Immutable         bool            `json:"immutable"`
	ObservedAt        time.Time       `json:"observed_at"`
	ReviewAfter       time.Time       `json:"review_after"`
}

// RegressionApprovalReceipt proves that a separate assurance actor approved
// the exact proposal bytes. Approval does not grant authorship authority.
type RegressionApprovalReceipt struct {
	ID                  string    `json:"id"`
	ProposalID          string    `json:"proposal_id"`
	ProposalHash        string    `json:"proposal_hash"`
	ApproverAgentID     string    `json:"approver_agent_id"`
	ApproverRoleID      string    `json:"approver_role_id"`
	ApproverFunction    string    `json:"approver_function"`
	IndependentFromRole string    `json:"independent_from_role"`
	Source              string    `json:"source"`
	Approved            bool      `json:"approved"`
	Immutable           bool      `json:"immutable"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

type RegressionChangeRecord struct {
	RegistryVersion    uint64                    `json:"registry_version"`
	Proposal           RegressionProposal        `json:"proposal"`
	DomainEvidence     RegressionDomainEvidence  `json:"domain_evidence"`
	Approval           RegressionApprovalReceipt `json:"approval"`
	PreviousRecordHash string                    `json:"previous_record_hash,omitempty"`
	PreviousValueHash  string                    `json:"previous_value_hash,omitempty"`
	CurrentValueHash   string                    `json:"current_value_hash"`
	AppliedAt          time.Time                 `json:"applied_at"`
	RecordHash         string                    `json:"record_hash"`
}

type RegressionRegistryFinding struct {
	Sequence     uint64          `json:"sequence"`
	Kind         string          `json:"kind"`
	ProposalID   string          `json:"proposal_id"`
	ObligationID string          `json:"obligation_id,omitempty"`
	Scope        RegressionScope `json:"scope,omitempty"`
	Message      string          `json:"message"`
	ObservedAt   time.Time       `json:"observed_at"`
	PreviousHash string          `json:"previous_hash,omitempty"`
	FindingHash  string          `json:"finding_hash"`
}

// RegressionRejectedAttempt preserves a denied stale/conflicting submission
// as immutable evidence. RegistryVersion locates the exact accepted-history
// boundary at which the denial must reproduce after restart.
type RegressionRejectedAttempt struct {
	Sequence            uint64                    `json:"sequence"`
	RegistryVersion     uint64                    `json:"registry_version"`
	SubmissionHash      string                    `json:"submission_hash"`
	Proposal            RegressionProposal        `json:"proposal"`
	DomainEvidence      RegressionDomainEvidence  `json:"domain_evidence"`
	Approval            RegressionApprovalReceipt `json:"approval"`
	ErrorClass          string                    `json:"error_class"`
	ErrorMessage        string                    `json:"error_message"`
	Finding             RegressionRegistryFinding `json:"finding"`
	AttemptedAt         time.Time                 `json:"attempted_at"`
	PreviousAttemptHash string                    `json:"previous_attempt_hash,omitempty"`
	AttemptHash         string                    `json:"attempt_hash"`
}

type RegressionSelection struct {
	Consumer           string                      `json:"consumer"`
	RegistryVersion    uint64                      `json:"registry_version"`
	Obligations        []RegressionObligation      `json:"obligations"`
	StaleObligationIDs []string                    `json:"stale_obligation_ids,omitempty"`
	Findings           []RegressionRegistryFinding `json:"findings,omitempty"`
	Admission          string                      `json:"admission"`
}

type regressionAttempt struct {
	digest string
	record RegressionChangeRecord
	err    error
}

// RegressionRegistry is provider-free, in-memory foundation state. Its history
// is append-only and hash chained; callers receive defensive copies.
type RegressionRegistry struct {
	mu              sync.RWMutex
	current         map[string]RegressionObligation
	history         []RegressionChangeRecord
	findings        []RegressionRegistryFinding
	rejected        []RegressionRejectedAttempt
	attempts        map[string]regressionAttempt
	proposalDigests map[string]string
}

func NewRegressionRegistry() *RegressionRegistry {
	return &RegressionRegistry{
		current:         make(map[string]RegressionObligation),
		attempts:        make(map[string]regressionAttempt),
		proposalDigests: make(map[string]string),
	}
}

func (r *RegressionRegistry) Apply(proposal RegressionProposal, evidence RegressionDomainEvidence, approval RegressionApprovalReceipt, now time.Time) (RegressionChangeRecord, error) {
	proposal = normalizeRegressionProposal(proposal)
	evidence = normalizeRegressionEvidence(evidence)
	approval = normalizeRegressionApproval(approval)
	submissionHash, err := regressionHash(struct {
		Proposal RegressionProposal        `json:"proposal"`
		Evidence RegressionDomainEvidence  `json:"evidence"`
		Approval RegressionApprovalReceipt `json:"approval"`
	}{proposal, evidence, approval})
	if err != nil {
		return RegressionChangeRecord{}, fmt.Errorf("%w: hash submission: %v", ErrRegressionInvalid, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	attemptKey := proposal.ID + "\x00" + submissionHash
	if proposal.ID != "" {
		if prior, ok := r.attempts[attemptKey]; ok {
			return cloneRegressionRecord(prior.record), prior.err
		}
		if priorDigest, ok := r.proposalDigests[proposal.ID]; ok && priorDigest != submissionHash {
			err := fmt.Errorf("%w: proposal id %s was reused with different content", ErrRegressionConflict, proposal.ID)
			r.appendRejectedLocked(submissionHash, proposal, evidence, approval, err, now)
			r.attempts[attemptKey] = regressionAttempt{digest: submissionHash, err: err}
			return RegressionChangeRecord{}, err
		}
		r.proposalDigests[proposal.ID] = submissionHash
	}

	record, applyErr := r.applyLocked(proposal, evidence, approval, now)
	if errors.Is(applyErr, ErrRegressionStale) || errors.Is(applyErr, ErrRegressionConflict) {
		r.appendRejectedLocked(submissionHash, proposal, evidence, approval, applyErr, now)
	}
	if proposal.ID != "" {
		r.attempts[attemptKey] = regressionAttempt{digest: submissionHash, record: cloneRegressionRecord(record), err: applyErr}
	}
	return cloneRegressionRecord(record), applyErr
}

func (r *RegressionRegistry) applyLocked(proposal RegressionProposal, evidence RegressionDomainEvidence, approval RegressionApprovalReceipt, now time.Time) (RegressionChangeRecord, error) {
	proposalHash, err := regressionHash(proposal)
	if err != nil {
		return RegressionChangeRecord{}, fmt.Errorf("%w: hash proposal: %v", ErrRegressionInvalid, err)
	}
	if err := validateRegressionSubmission(proposal, evidence, approval, proposalHash, now); err != nil {
		return RegressionChangeRecord{}, err
	}

	prior, exists := r.current[proposal.Obligation.ID]
	switch proposal.Kind {
	case RegressionAdd:
		if exists {
			return RegressionChangeRecord{}, fmt.Errorf("%w: obligation %s already exists", ErrRegressionConflict, proposal.Obligation.ID)
		}
	case RegressionUpdate, RegressionSupersede, RegressionRetire:
		if !exists {
			return RegressionChangeRecord{}, fmt.Errorf("%w: obligation %s does not exist", ErrRegressionConflict, proposal.Obligation.ID)
		}
		if prior.Version != proposal.ExpectedVersion {
			return RegressionChangeRecord{}, fmt.Errorf("%w: obligation %s is version %d, proposal expected %d", ErrRegressionConflict, proposal.Obligation.ID, prior.Version, proposal.ExpectedVersion)
		}
		if prior.AccountableDomainRoleID != proposal.Obligation.AccountableDomainRoleID {
			return RegressionChangeRecord{}, fmt.Errorf("%w: accountable domain cannot be changed by registry proposal", ErrRegressionUnauthorized)
		}
		if prior.Status != RegressionActive {
			return RegressionChangeRecord{}, fmt.Errorf("%w: obligation %s is already %s", ErrRegressionConflict, prior.ID, prior.Status)
		}
	default:
		return RegressionChangeRecord{}, fmt.Errorf("%w: unsupported change kind %q", ErrRegressionInvalid, proposal.Kind)
	}

	previousValueHash := ""
	if exists {
		previousValueHash, _ = regressionHash(prior)
	}
	currentValueHash, err := regressionHash(proposal.Obligation)
	if err != nil {
		return RegressionChangeRecord{}, fmt.Errorf("%w: hash obligation: %v", ErrRegressionInvalid, err)
	}
	previousRecordHash := ""
	if len(r.history) > 0 {
		previousRecordHash = r.history[len(r.history)-1].RecordHash
	}
	record := RegressionChangeRecord{
		RegistryVersion:    uint64(len(r.history) + 1),
		Proposal:           proposal,
		DomainEvidence:     evidence,
		Approval:           approval,
		PreviousRecordHash: previousRecordHash,
		PreviousValueHash:  previousValueHash,
		CurrentValueHash:   currentValueHash,
		AppliedAt:          now.UTC(),
	}
	record.RecordHash, err = regressionRecordHash(record)
	if err != nil {
		return RegressionChangeRecord{}, fmt.Errorf("%w: hash record: %v", ErrRegressionInvalid, err)
	}
	r.current[proposal.Obligation.ID] = cloneRegressionObligation(proposal.Obligation)
	r.history = append(r.history, cloneRegressionRecord(record))
	return record, nil
}

func validateRegressionSubmission(proposal RegressionProposal, evidence RegressionDomainEvidence, approval RegressionApprovalReceipt, proposalHash string, now time.Time) error {
	if strings.TrimSpace(proposal.ID) == "" || strings.TrimSpace(proposal.AuthorRoleID) == "" || proposal.CreatedAt.IsZero() || proposal.CreatedAt.After(now) {
		return fmt.Errorf("%w: incomplete or future proposal", ErrRegressionInvalid)
	}
	roles, err := DefaultRoles()
	if err != nil {
		return fmt.Errorf("%w: role catalog unavailable: %v", ErrRegressionUnauthorized, err)
	}
	authorRole, authorExists := roles.Role(proposal.AuthorRoleID)
	if !authorExists || authorRole.Function != RegressionAuthorAccountableDomain || proposal.AuthorFunction != RegressionAuthorAccountableDomain {
		return fmt.Errorf("%w: only the accountable domain may author regressions", ErrRegressionUnauthorized)
	}
	obligation := proposal.Obligation
	if obligation.ID == "" || obligation.Title == "" || obligation.Requirement == "" || obligation.ProofMethod == "" || obligation.AccountableDomainRoleID != proposal.AuthorRoleID || obligation.ReviewAfter.IsZero() || obligation.Version != proposal.ProposedVersion || proposal.ProposedVersion != proposal.ExpectedVersion+1 {
		return fmt.Errorf("%w: incomplete obligation or invalid version transition", ErrRegressionInvalid)
	}
	if len(obligation.Scope.JourneyIDs)+len(obligation.Scope.NodeIDs)+len(obligation.Scope.EdgeIDs) == 0 {
		return fmt.Errorf("%w: obligation needs at least one graph target", ErrRegressionInvalid)
	}
	graph, err := Default()
	if err != nil {
		return fmt.Errorf("%w: system graph unavailable: %v", ErrRegressionInvalid, err)
	}
	if err := validateRegressionGraphScope(graph, authorRole, obligation.Scope); err != nil {
		return err
	}
	if proposal.Kind == RegressionAdd && proposal.ExpectedVersion != 0 {
		return fmt.Errorf("%w: add must start at version 1", ErrRegressionInvalid)
	}
	if proposal.Kind == RegressionSupersede && obligation.SupersedesVersion != proposal.ExpectedVersion {
		return fmt.Errorf("%w: supersede must identify the replaced version", ErrRegressionInvalid)
	}
	if proposal.Kind != RegressionSupersede && obligation.SupersedesVersion != 0 {
		return fmt.Errorf("%w: only supersede records a replaced version", ErrRegressionInvalid)
	}
	wantStatus := RegressionActive
	if proposal.Kind == RegressionRetire {
		wantStatus = RegressionRetired
	}
	if obligation.Status != wantStatus {
		return fmt.Errorf("%w: %s requires obligation status %s", ErrRegressionInvalid, proposal.Kind, wantStatus)
	}
	if evidence.ID == "" || evidence.ProposalID != proposal.ID || evidence.AuthorAgentID == "" || evidence.AccountableRoleID != proposal.AuthorRoleID || evidence.Source != "controller_record" || !evidence.Immutable || len(evidence.SourceRefs) == 0 || evidence.ObservedAt.IsZero() || evidence.ObservedAt.After(now) {
		return fmt.Errorf("%w: accountable domain evidence is incomplete", ErrRegressionUnauthorized)
	}
	if !sameRegressionScope(evidence.Scope, obligation.Scope) {
		return fmt.Errorf("%w: evidence scope does not equal obligation scope", ErrRegressionUnauthorized)
	}
	if evidence.ReviewAfter.IsZero() || !now.Before(evidence.ReviewAfter) || !evidence.ObservedAt.Before(evidence.ReviewAfter) {
		return fmt.Errorf("%w: accountable domain evidence expired", ErrRegressionStale)
	}
	if approval.ID == "" || approval.ProposalID != proposal.ID || approval.ProposalHash != proposalHash || approval.ApproverAgentID == "" || approval.ApproverRoleID == "" || approval.ApproverFunction != "independent_assurance" || approval.IndependentFromRole != proposal.AuthorRoleID || approval.Source != "controller_record" || !approval.Approved || !approval.Immutable {
		return fmt.Errorf("%w: independent approval receipt is incomplete or mismatched", ErrRegressionUnauthorized)
	}
	approverRole, approverExists := roles.Role(approval.ApproverRoleID)
	if !approverExists || (approverRole.Function != "independent_outcome_assurance" && approverRole.Function != "architecture_assurance") {
		return fmt.Errorf("%w: approval role is not independent assurance", ErrRegressionUnauthorized)
	}
	if approval.ApproverRoleID == proposal.AuthorRoleID || approval.ApproverAgentID == evidence.AuthorAgentID || approval.IssuedAt.IsZero() || approval.IssuedAt.Before(proposal.CreatedAt) || approval.IssuedAt.After(now) || approval.ExpiresAt.IsZero() || !now.Before(approval.ExpiresAt) {
		return fmt.Errorf("%w: approval is not independent and current", ErrRegressionUnauthorized)
	}
	return nil
}

func validateRegressionGraphScope(graph Catalog, authorRole RoleProfile, scope RegressionScope) error {
	if len(scope.JourneyIDs) == 0 {
		return fmt.Errorf("%w: obligation must name at least one journey", ErrRegressionInvalid)
	}
	journeys := make(map[string]Journey, len(graph.Journeys))
	for _, journey := range graph.Journeys {
		journeys[journey.ID] = journey
	}
	nodes := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = struct{}{}
	}
	edges := make(map[string]struct{}, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges[edge.ID] = struct{}{}
	}

	selected := make([]Journey, 0, len(scope.JourneyIDs))
	for _, journeyID := range scope.JourneyIDs {
		journey, ok := journeys[journeyID]
		if !ok {
			return fmt.Errorf("%w: unknown journey %s", ErrRegressionInvalid, journeyID)
		}
		if !slices.Contains(authorRole.OutcomeCapabilities, journey.AccountableCapability) {
			return fmt.Errorf("%w: role %s does not own journey %s capability %s", ErrRegressionUnauthorized, authorRole.ID, journey.ID, journey.AccountableCapability)
		}
		selected = append(selected, journey)
	}
	for _, nodeID := range scope.NodeIDs {
		if _, ok := nodes[nodeID]; !ok {
			return fmt.Errorf("%w: unknown node %s", ErrRegressionInvalid, nodeID)
		}
		belongs := false
		for _, journey := range selected {
			if slices.Contains(journey.NodeIDs, nodeID) {
				belongs = true
				break
			}
		}
		if !belongs {
			return fmt.Errorf("%w: node %s is outside the named journeys", ErrRegressionInvalid, nodeID)
		}
	}
	for _, edgeID := range scope.EdgeIDs {
		if _, ok := edges[edgeID]; !ok {
			return fmt.Errorf("%w: unknown edge %s", ErrRegressionInvalid, edgeID)
		}
		belongs := false
		for _, journey := range selected {
			if slices.Contains(journey.EdgeIDs, edgeID) {
				belongs = true
				break
			}
		}
		if !belongs {
			return fmt.Errorf("%w: edge %s is outside the named journeys", ErrRegressionInvalid, edgeID)
		}
	}
	return nil
}

// BuildRegressionImpactScope is the controller-side constructor for the exact
// impact closure of the named journeys. Consumers cannot narrow that closure.
func BuildRegressionImpactScope(graph Catalog, controllerReceiptID string, journeyIDs []string) (RegressionImpactScope, error) {
	controllerReceiptID = strings.TrimSpace(controllerReceiptID)
	journeyIDs = uniqueSorted(journeyIDs)
	if controllerReceiptID == "" || len(journeyIDs) == 0 {
		return RegressionImpactScope{}, fmt.Errorf("%w: controller receipt and affected journeys are required", ErrRegressionInvalid)
	}
	journeyByID := make(map[string]Journey, len(graph.Journeys))
	for _, journey := range graph.Journeys {
		journeyByID[journey.ID] = journey
	}
	scope := RegressionScope{JourneyIDs: journeyIDs}
	for _, journeyID := range journeyIDs {
		journey, ok := journeyByID[journeyID]
		if !ok {
			return RegressionImpactScope{}, fmt.Errorf("%w: unknown impacted journey %s", ErrRegressionInvalid, journeyID)
		}
		scope.NodeIDs = append(scope.NodeIDs, journey.NodeIDs...)
		scope.EdgeIDs = append(scope.EdgeIDs, journey.EdgeIDs...)
	}
	impact := RegressionImpactScope{
		ControllerReceiptID: controllerReceiptID,
		GraphVersion:        graph.Version,
		Source:              "controller_record",
		Complete:            true,
		Scope:               normalizeRegressionScope(scope),
	}
	var err error
	impact.BindingHash, err = regressionImpactHash(impact)
	if err != nil {
		return RegressionImpactScope{}, fmt.Errorf("%w: hash impact scope: %v", ErrRegressionInvalid, err)
	}
	return impact, nil
}

func validateRegressionImpactScope(impact RegressionImpactScope) (RegressionScope, error) {
	graph, err := Default()
	if err != nil {
		return RegressionScope{}, fmt.Errorf("%w: system graph unavailable: %v", ErrRegressionInvalid, err)
	}
	impact.ControllerReceiptID = strings.TrimSpace(impact.ControllerReceiptID)
	impact.GraphVersion = strings.TrimSpace(impact.GraphVersion)
	impact.Source = strings.TrimSpace(impact.Source)
	impact.Scope = normalizeRegressionScope(impact.Scope)
	impact.BindingHash = strings.TrimSpace(impact.BindingHash)
	if impact.ControllerReceiptID == "" || impact.Source != "controller_record" || !impact.Complete || impact.GraphVersion != graph.Version || impact.BindingHash == "" {
		return RegressionScope{}, fmt.Errorf("%w: impact scope is not a complete current controller receipt", ErrRegressionInvalid)
	}
	wantHash, err := regressionImpactHash(impact)
	if err != nil || wantHash != impact.BindingHash {
		return RegressionScope{}, fmt.Errorf("%w: impact scope binding mismatch", ErrRegressionInvalid)
	}
	want, err := BuildRegressionImpactScope(graph, impact.ControllerReceiptID, impact.Scope.JourneyIDs)
	if err != nil {
		return RegressionScope{}, err
	}
	if !sameRegressionScope(impact.Scope, want.Scope) {
		return RegressionScope{}, fmt.Errorf("%w: impact scope is unknown, out-of-journey, or incomplete", ErrRegressionInvalid)
	}
	return impact.Scope, nil
}

func (r *RegressionRegistry) ActiveFor(impact RegressionImpactScope, consumer string, now time.Time) (RegressionSelection, error) {
	if consumer != RegressionConsumerRelease && consumer != RegressionConsumerQualityChecker {
		return RegressionSelection{}, fmt.Errorf("%w: %s", ErrRegressionConsumerDenied, consumer)
	}
	scope, err := validateRegressionImpactScope(impact)
	if err != nil {
		return RegressionSelection{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	selection := RegressionSelection{Consumer: consumer, RegistryVersion: uint64(len(r.history)), Admission: "eligible"}
	for _, obligation := range r.current {
		if obligation.Status != RegressionActive || !regressionScopesIntersect(scope, obligation.Scope) {
			continue
		}
		selection.Obligations = append(selection.Obligations, cloneRegressionObligation(obligation))
		if obligation.ReviewAfter.IsZero() || !now.Before(obligation.ReviewAfter) {
			selection.StaleObligationIDs = append(selection.StaleObligationIDs, obligation.ID)
		}
	}
	for _, finding := range r.findings {
		if regressionScopesIntersect(scope, finding.Scope) {
			selection.Findings = append(selection.Findings, cloneRegressionFinding(finding))
		}
	}
	sort.Slice(selection.Obligations, func(i, j int) bool { return selection.Obligations[i].ID < selection.Obligations[j].ID })
	selection.StaleObligationIDs = uniqueSorted(selection.StaleObligationIDs)
	if len(selection.StaleObligationIDs) > 0 {
		selection.Admission = "blocked_stale_obligation"
	}
	return selection, nil
}

func (r *RegressionRegistry) History() []RegressionChangeRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RegressionChangeRecord, len(r.history))
	for i := range r.history {
		result[i] = cloneRegressionRecord(r.history[i])
	}
	return result
}

func (r *RegressionRegistry) Findings() []RegressionRegistryFinding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RegressionRegistryFinding, len(r.findings))
	for i := range r.findings {
		result[i] = cloneRegressionFinding(r.findings[i])
	}
	return result
}

func (r *RegressionRegistry) RejectedAttempts() []RegressionRejectedAttempt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RegressionRejectedAttempt, len(r.rejected))
	for i := range r.rejected {
		result[i] = cloneRegressionRejectedAttempt(r.rejected[i])
	}
	return result
}

// ReplayRegressionRegistry restores accepted history and, when supplied, the
// immutable rejected-attempt audit. The variadic form preserves callers that
// only hold legacy accepted history.
func ReplayRegressionRegistry(history []RegressionChangeRecord, rejectedAudit ...[]RegressionRejectedAttempt) (*RegressionRegistry, error) {
	registry := NewRegressionRegistry()
	for i, supplied := range history {
		if supplied.RegistryVersion != uint64(i+1) {
			return nil, fmt.Errorf("%w: record %d has registry version %d", ErrRegressionHistoryCorrupt, i, supplied.RegistryVersion)
		}
		wantPrevious := ""
		if i > 0 {
			wantPrevious = history[i-1].RecordHash
		}
		if supplied.PreviousRecordHash != wantPrevious {
			return nil, fmt.Errorf("%w: record %d breaks hash chain", ErrRegressionHistoryCorrupt, i)
		}
		wantHash, err := regressionRecordHash(supplied)
		if err != nil || wantHash != supplied.RecordHash {
			return nil, fmt.Errorf("%w: record %d hash mismatch", ErrRegressionHistoryCorrupt, i)
		}
	}
	if len(rejectedAudit) > 1 {
		return nil, fmt.Errorf("%w: multiple rejected-attempt histories supplied", ErrRegressionHistoryCorrupt)
	}
	audit := []RegressionRejectedAttempt(nil)
	if len(rejectedAudit) == 1 {
		audit = rejectedAudit[0]
	}
	historyIndex := 0
	for i, supplied := range audit {
		if supplied.Sequence != uint64(i+1) || supplied.RegistryVersion < uint64(historyIndex) || supplied.RegistryVersion > uint64(len(history)) {
			return nil, fmt.Errorf("%w: rejected attempt %d has invalid sequence boundary", ErrRegressionHistoryCorrupt, i)
		}
		for historyIndex < int(supplied.RegistryVersion) {
			if err := replayRegressionChange(registry, history[historyIndex], historyIndex); err != nil {
				return nil, err
			}
			historyIndex++
		}
		wantPrevious := ""
		if i > 0 {
			wantPrevious = audit[i-1].AttemptHash
		}
		if supplied.PreviousAttemptHash != wantPrevious {
			return nil, fmt.Errorf("%w: rejected attempt %d breaks hash chain", ErrRegressionHistoryCorrupt, i)
		}
		wantAttemptHash, err := regressionRejectedAttemptHash(supplied)
		if err != nil || wantAttemptHash != supplied.AttemptHash {
			return nil, fmt.Errorf("%w: rejected attempt %d hash mismatch", ErrRegressionHistoryCorrupt, i)
		}
		wantSubmissionHash, err := regressionSubmissionHash(supplied.Proposal, supplied.DomainEvidence, supplied.Approval)
		if err != nil || wantSubmissionHash != supplied.SubmissionHash {
			return nil, fmt.Errorf("%w: rejected attempt %d submission mismatch", ErrRegressionHistoryCorrupt, i)
		}
		wantFindingHash, err := regressionFindingHash(supplied.Finding)
		if err != nil || wantFindingHash != supplied.Finding.FindingHash || supplied.Finding.Sequence != uint64(i+1) {
			return nil, fmt.Errorf("%w: rejected attempt %d finding mismatch", ErrRegressionHistoryCorrupt, i)
		}
		wantFindingPrevious := ""
		if i > 0 {
			wantFindingPrevious = audit[i-1].Finding.FindingHash
		}
		if supplied.Finding.PreviousHash != wantFindingPrevious || supplied.Finding.ProposalID != supplied.Proposal.ID || supplied.Finding.Message != supplied.ErrorMessage || supplied.Finding.Kind != supplied.ErrorClass {
			return nil, fmt.Errorf("%w: rejected attempt %d finding binding mismatch", ErrRegressionHistoryCorrupt, i)
		}
		_, replayErr := registry.Apply(supplied.Proposal, supplied.DomainEvidence, supplied.Approval, supplied.AttemptedAt)
		if replayErr == nil || regressionErrorClass(replayErr) != supplied.ErrorClass || replayErr.Error() != supplied.ErrorMessage {
			return nil, fmt.Errorf("%w: rejected attempt %d does not reproduce", ErrRegressionHistoryCorrupt, i)
		}
		generated := registry.rejected[len(registry.rejected)-1]
		if generated.AttemptHash != supplied.AttemptHash || generated.Finding.FindingHash != supplied.Finding.FindingHash {
			return nil, fmt.Errorf("%w: rejected attempt %d immutable receipt differs", ErrRegressionHistoryCorrupt, i)
		}
	}
	for historyIndex < len(history) {
		if err := replayRegressionChange(registry, history[historyIndex], historyIndex); err != nil {
			return nil, err
		}
		historyIndex++
	}
	return registry, nil
}

func replayRegressionChange(registry *RegressionRegistry, supplied RegressionChangeRecord, index int) error {
	replayed, err := registry.Apply(supplied.Proposal, supplied.DomainEvidence, supplied.Approval, supplied.AppliedAt)
	if err != nil || replayed.RecordHash != supplied.RecordHash || replayed.CurrentValueHash != supplied.CurrentValueHash || replayed.PreviousValueHash != supplied.PreviousValueHash {
		return fmt.Errorf("%w: record %d does not reproduce", ErrRegressionHistoryCorrupt, index)
	}
	return nil
}

func (r *RegressionRegistry) appendRejectedLocked(submissionHash string, proposal RegressionProposal, evidence RegressionDomainEvidence, approval RegressionApprovalReceipt, rejection error, now time.Time) {
	kind := regressionErrorClass(rejection)
	previousFindingHash := ""
	if len(r.findings) > 0 {
		previousFindingHash = r.findings[len(r.findings)-1].FindingHash
	}
	finding := RegressionRegistryFinding{
		Sequence:     uint64(len(r.findings) + 1),
		Kind:         kind,
		ProposalID:   proposal.ID,
		ObligationID: proposal.Obligation.ID,
		Scope:        cloneRegressionScope(proposal.Obligation.Scope),
		Message:      rejection.Error(),
		ObservedAt:   now.UTC(),
		PreviousHash: previousFindingHash,
	}
	finding.FindingHash, _ = regressionFindingHash(finding)
	previousAttemptHash := ""
	if len(r.rejected) > 0 {
		previousAttemptHash = r.rejected[len(r.rejected)-1].AttemptHash
	}
	attempt := RegressionRejectedAttempt{
		Sequence:            uint64(len(r.rejected) + 1),
		RegistryVersion:     uint64(len(r.history)),
		SubmissionHash:      submissionHash,
		Proposal:            proposal,
		DomainEvidence:      evidence,
		Approval:            approval,
		ErrorClass:          kind,
		ErrorMessage:        rejection.Error(),
		Finding:             finding,
		AttemptedAt:         now.UTC(),
		PreviousAttemptHash: previousAttemptHash,
	}
	attempt.AttemptHash, _ = regressionRejectedAttemptHash(attempt)
	r.findings = append(r.findings, finding)
	r.rejected = append(r.rejected, attempt)
}

func regressionRecordHash(record RegressionChangeRecord) (string, error) {
	record.RecordHash = ""
	return regressionHash(record)
}

func regressionSubmissionHash(proposal RegressionProposal, evidence RegressionDomainEvidence, approval RegressionApprovalReceipt) (string, error) {
	return regressionHash(struct {
		Proposal RegressionProposal        `json:"proposal"`
		Evidence RegressionDomainEvidence  `json:"evidence"`
		Approval RegressionApprovalReceipt `json:"approval"`
	}{normalizeRegressionProposal(proposal), normalizeRegressionEvidence(evidence), normalizeRegressionApproval(approval)})
}

// RegressionProposalSHA256 returns the canonical proposal digest that an
// independent controller approval must bind. It does not validate or approve
// the proposal and grants no registry write authority.
func RegressionProposalSHA256(proposal RegressionProposal) (string, error) {
	return regressionHash(normalizeRegressionProposal(proposal))
}

func regressionImpactHash(impact RegressionImpactScope) (string, error) {
	impact.BindingHash = ""
	return regressionHash(impact)
}

func regressionFindingHash(finding RegressionRegistryFinding) (string, error) {
	finding.FindingHash = ""
	return regressionHash(finding)
}

func regressionRejectedAttemptHash(attempt RegressionRejectedAttempt) (string, error) {
	attempt.AttemptHash = ""
	return regressionHash(attempt)
}

func regressionErrorClass(err error) string {
	switch {
	case errors.Is(err, ErrRegressionStale):
		return "stale"
	case errors.Is(err, ErrRegressionConflict):
		return "conflict"
	case errors.Is(err, ErrRegressionUnauthorized):
		return "unauthorized"
	default:
		return "invalid"
	}
}

func regressionHash(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func normalizeRegressionProposal(value RegressionProposal) RegressionProposal {
	value.ID = strings.TrimSpace(value.ID)
	value.AuthorRoleID = strings.TrimSpace(value.AuthorRoleID)
	value.AuthorFunction = strings.TrimSpace(value.AuthorFunction)
	value.CreatedAt = value.CreatedAt.UTC()
	value.Obligation = cloneRegressionObligation(value.Obligation)
	value.Obligation.ID = strings.TrimSpace(value.Obligation.ID)
	value.Obligation.Title = strings.TrimSpace(value.Obligation.Title)
	value.Obligation.Requirement = strings.TrimSpace(value.Obligation.Requirement)
	value.Obligation.ProofMethod = strings.TrimSpace(value.Obligation.ProofMethod)
	value.Obligation.AccountableDomainRoleID = strings.TrimSpace(value.Obligation.AccountableDomainRoleID)
	value.Obligation.ReviewAfter = value.Obligation.ReviewAfter.UTC()
	value.Obligation.Scope = normalizeRegressionScope(value.Obligation.Scope)
	return value
}

func normalizeRegressionEvidence(value RegressionDomainEvidence) RegressionDomainEvidence {
	value.ID = strings.TrimSpace(value.ID)
	value.ProposalID = strings.TrimSpace(value.ProposalID)
	value.AuthorAgentID = strings.TrimSpace(value.AuthorAgentID)
	value.AccountableRoleID = strings.TrimSpace(value.AccountableRoleID)
	value.Source = strings.TrimSpace(value.Source)
	value.Scope = normalizeRegressionScope(value.Scope)
	value.SourceRefs = uniqueSorted(value.SourceRefs)
	value.ObservedAt = value.ObservedAt.UTC()
	value.ReviewAfter = value.ReviewAfter.UTC()
	return value
}

func normalizeRegressionApproval(value RegressionApprovalReceipt) RegressionApprovalReceipt {
	value.ID = strings.TrimSpace(value.ID)
	value.ProposalID = strings.TrimSpace(value.ProposalID)
	value.ProposalHash = strings.TrimSpace(value.ProposalHash)
	value.ApproverAgentID = strings.TrimSpace(value.ApproverAgentID)
	value.ApproverRoleID = strings.TrimSpace(value.ApproverRoleID)
	value.ApproverFunction = strings.TrimSpace(value.ApproverFunction)
	value.IndependentFromRole = strings.TrimSpace(value.IndependentFromRole)
	value.Source = strings.TrimSpace(value.Source)
	value.IssuedAt = value.IssuedAt.UTC()
	value.ExpiresAt = value.ExpiresAt.UTC()
	return value
}

func normalizeRegressionScope(value RegressionScope) RegressionScope {
	return RegressionScope{JourneyIDs: uniqueSorted(value.JourneyIDs), NodeIDs: uniqueSorted(value.NodeIDs), EdgeIDs: uniqueSorted(value.EdgeIDs)}
}

func sameRegressionScope(left, right RegressionScope) bool {
	left = normalizeRegressionScope(left)
	right = normalizeRegressionScope(right)
	return slices.Equal(left.JourneyIDs, right.JourneyIDs) && slices.Equal(left.NodeIDs, right.NodeIDs) && slices.Equal(left.EdgeIDs, right.EdgeIDs)
}

func regressionScopesIntersect(left, right RegressionScope) bool {
	return stringsIntersect(left.JourneyIDs, right.JourneyIDs) || stringsIntersect(left.NodeIDs, right.NodeIDs) || stringsIntersect(left.EdgeIDs, right.EdgeIDs)
}

func stringsIntersect(left, right []string) bool {
	for _, value := range left {
		if slices.Contains(right, value) {
			return true
		}
	}
	return false
}

func cloneRegressionScope(value RegressionScope) RegressionScope {
	return RegressionScope{JourneyIDs: slices.Clone(value.JourneyIDs), NodeIDs: slices.Clone(value.NodeIDs), EdgeIDs: slices.Clone(value.EdgeIDs)}
}

func cloneRegressionObligation(value RegressionObligation) RegressionObligation {
	value.Scope = cloneRegressionScope(value.Scope)
	return value
}

func cloneRegressionRecord(value RegressionChangeRecord) RegressionChangeRecord {
	value.Proposal = normalizeRegressionProposal(value.Proposal)
	value.DomainEvidence = normalizeRegressionEvidence(value.DomainEvidence)
	value.Approval = normalizeRegressionApproval(value.Approval)
	return value
}

func cloneRegressionFinding(value RegressionRegistryFinding) RegressionRegistryFinding {
	value.Scope = cloneRegressionScope(value.Scope)
	return value
}

func cloneRegressionRejectedAttempt(value RegressionRejectedAttempt) RegressionRejectedAttempt {
	value.Proposal = normalizeRegressionProposal(value.Proposal)
	value.DomainEvidence = normalizeRegressionEvidence(value.DomainEvidence)
	value.Approval = normalizeRegressionApproval(value.Approval)
	value.Finding = cloneRegressionFinding(value.Finding)
	return value
}
