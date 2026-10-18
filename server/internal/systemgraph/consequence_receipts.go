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
)

const (
	ConsequenceReceiptVersionV1 = "consequence_receipt.v1"
	referenceOnlyEffectV1       = "reference_only_no_merge"
)

var (
	ErrConsequenceReceiptConflict = errors.New("consequence receipt conflict")
	ErrConsequenceReceiptInvalid  = errors.New("invalid consequence receipt")
)

// IssueOutcomeSnapshotV1 binds a consequence to an issue's independent scope,
// status, and authority. Sharing evidence never transfers any of these fields.
type IssueOutcomeSnapshotV1 struct {
	IssueID           string `json:"issue_id"`
	OutcomeID         string `json:"outcome_id"`
	ScopeRevision     int64  `json:"scope_revision"`
	Status            string `json:"status"`
	AuthorityRevision int64  `json:"authority_revision"`
}

type ConsequenceImpactSetV1 struct {
	AffectedIssueIDs         []string `json:"affected_issue_ids"`
	UnrelatedControlIssueIDs []string `json:"unrelated_control_issue_ids,omitempty"`
}

type ConsequenceReceiptInputV1 struct {
	IdempotencyKey               string                 `json:"idempotency_key"`
	ControllerAuthorityReceiptID string                 `json:"controller_authority_receipt_id"`
	RelationshipKind             string                 `json:"relationship_kind"`
	Source                       IssueOutcomeSnapshotV1 `json:"source"`
	Affected                     IssueOutcomeSnapshotV1 `json:"affected"`
	ImpactSet                    ConsequenceImpactSetV1 `json:"impact_set"`
	SharedEvidenceRefs           []string               `json:"shared_evidence_refs"`
}

// ConsequenceControllerAuthorityV1 is returned by the trusted controller gate,
// not accepted as caller-authored receipt data.
type ConsequenceControllerAuthorityV1 struct {
	ReceiptID       string `json:"receipt_id"`
	ControllerID    string `json:"controller_id"`
	IssuerRole      string `json:"issuer_role"`
	Source          string `json:"source"`
	Action          string `json:"action"`
	RouteVersion    string `json:"route_version"`
	GraphVersion    string `json:"graph_version"`
	SourceIssueID   string `json:"source_issue_id"`
	AffectedIssueID string `json:"affected_issue_id"`
	Authenticated   bool   `json:"authenticated"`
	Immutable       bool   `json:"immutable"`
}

type IssueOutcomeSnapshotProvenanceV1 struct {
	SnapshotReceiptID string `json:"snapshot_receipt_id"`
	StoreRevision     string `json:"store_revision"`
	SnapshotSHA256    string `json:"snapshot_sha256"`
}

type CurrentIssueOutcomeSnapshotV1 struct {
	Snapshot   IssueOutcomeSnapshotV1           `json:"snapshot"`
	Provenance IssueOutcomeSnapshotProvenanceV1 `json:"provenance"`
}

type ConsequenceCreationProofV1 struct {
	Authority ConsequenceControllerAuthorityV1 `json:"authority"`
	Source    CurrentIssueOutcomeSnapshotV1    `json:"source"`
	Affected  CurrentIssueOutcomeSnapshotV1    `json:"affected"`
}

// ConsequenceCreationGateV1 is the trust boundary. Its implementation must
// authenticate the deterministic controller and atomically load both current
// durable issue/outcome snapshots.
type ConsequenceCreationGateV1 interface {
	AuthorizeAndLoadCurrentConsequenceV1(input ConsequenceReceiptInputV1) (ConsequenceCreationProofV1, error)
}

// ConsequenceReceiptV1 is immutable once committed. The three effect fields
// make the limited semantics explicit: the receipt relates evidence; it cannot
// merge or mutate either issue's scope, status, or authority.
type ConsequenceReceiptV1 struct {
	Version             string                           `json:"version"`
	ReceiptID           string                           `json:"receipt_id"`
	IdempotencyKey      string                           `json:"idempotency_key"`
	ControllerAuthority ConsequenceControllerAuthorityV1 `json:"controller_authority"`
	RelationshipKind    string                           `json:"relationship_kind"`
	Source              IssueOutcomeSnapshotV1           `json:"source"`
	Affected            IssueOutcomeSnapshotV1           `json:"affected"`
	SourceProvenance    IssueOutcomeSnapshotProvenanceV1 `json:"source_provenance"`
	AffectedProvenance  IssueOutcomeSnapshotProvenanceV1 `json:"affected_provenance"`
	ImpactSet           ConsequenceImpactSetV1           `json:"impact_set"`
	SharedEvidenceRefs  []string                         `json:"shared_evidence_refs"`
	ScopeEffect         string                           `json:"scope_effect"`
	StatusEffect        string                           `json:"status_effect"`
	AuthorityEffect     string                           `json:"authority_effect"`
	PayloadSHA256       string                           `json:"payload_sha256"`
}

type PreparedConsequenceReceiptV1 struct {
	Version        string               `json:"version"`
	IdempotencyKey string               `json:"idempotency_key"`
	PayloadSHA256  string               `json:"payload_sha256"`
	Receipt        ConsequenceReceiptV1 `json:"receipt"`
}

// ConsequenceReceiptJournalV1 is the caller-durable state. A prepared record is
// written before commit, so recovery can finish it without creating a second
// receipt or changing its payload.
type ConsequenceReceiptJournalV1 struct {
	Version   string                         `json:"version"`
	Prepared  []PreparedConsequenceReceiptV1 `json:"prepared,omitempty"`
	Committed []ConsequenceReceiptV1         `json:"committed,omitempty"`
}

type ConsequenceReceiptStoreV1 struct {
	mu        sync.Mutex
	prepared  map[string]PreparedConsequenceReceiptV1
	committed map[string]ConsequenceReceiptV1
	gate      ConsequenceCreationGateV1
}

func NewConsequenceReceiptStoreV1(journal ConsequenceReceiptJournalV1, gate ConsequenceCreationGateV1) (*ConsequenceReceiptStoreV1, error) {
	if gate == nil {
		return nil, fmt.Errorf("%w: authenticated controller creation gate is required", ErrConsequenceReceiptInvalid)
	}
	if journal.Version != "" && journal.Version != ConsequenceReceiptVersionV1 {
		return nil, fmt.Errorf("%w: journal version %q", ErrConsequenceReceiptInvalid, journal.Version)
	}
	s := &ConsequenceReceiptStoreV1{
		prepared:  make(map[string]PreparedConsequenceReceiptV1),
		committed: make(map[string]ConsequenceReceiptV1),
		gate:      gate,
	}
	for _, prepared := range journal.Prepared {
		if err := validatePreparedConsequenceReceiptV1(prepared); err != nil {
			return nil, err
		}
		if existing, ok := s.prepared[prepared.IdempotencyKey]; ok && existing.PayloadSHA256 != prepared.PayloadSHA256 {
			return nil, fmt.Errorf("%w: conflicting prepared key %q", ErrConsequenceReceiptConflict, prepared.IdempotencyKey)
		}
		s.prepared[prepared.IdempotencyKey] = clonePreparedConsequenceReceiptV1(prepared)
	}
	for _, receipt := range journal.Committed {
		if err := validateConsequenceReceiptV1(receipt); err != nil {
			return nil, err
		}
		if existing, ok := s.committed[receipt.IdempotencyKey]; ok && existing.PayloadSHA256 != receipt.PayloadSHA256 {
			return nil, fmt.Errorf("%w: conflicting committed key %q", ErrConsequenceReceiptConflict, receipt.IdempotencyKey)
		}
		if prepared, ok := s.prepared[receipt.IdempotencyKey]; ok && prepared.PayloadSHA256 != receipt.PayloadSHA256 {
			return nil, fmt.Errorf("%w: prepared/committed mismatch for %q", ErrConsequenceReceiptConflict, receipt.IdempotencyKey)
		}
		s.committed[receipt.IdempotencyKey] = cloneConsequenceReceiptV1(receipt)
	}
	return s, nil
}

func (s *ConsequenceReceiptStoreV1) Prepare(input ConsequenceReceiptInputV1) (PreparedConsequenceReceiptV1, error) {
	proof, err := s.gate.AuthorizeAndLoadCurrentConsequenceV1(input)
	if err != nil {
		return PreparedConsequenceReceiptV1{}, fmt.Errorf("%w: controller creation gate: %v", ErrConsequenceReceiptInvalid, err)
	}
	receipt, err := buildConsequenceReceiptV1(input, proof)
	if err != nil {
		return PreparedConsequenceReceiptV1{}, err
	}
	prepared := PreparedConsequenceReceiptV1{
		Version:        ConsequenceReceiptVersionV1,
		IdempotencyKey: receipt.IdempotencyKey,
		PayloadSHA256:  receipt.PayloadSHA256,
		Receipt:        receipt,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.committed[prepared.IdempotencyKey]; ok {
		if existing.PayloadSHA256 != prepared.PayloadSHA256 {
			return PreparedConsequenceReceiptV1{}, consequenceConflict(prepared.IdempotencyKey)
		}
		return clonePreparedConsequenceReceiptV1(PreparedConsequenceReceiptV1{
			Version: ConsequenceReceiptVersionV1, IdempotencyKey: existing.IdempotencyKey,
			PayloadSHA256: existing.PayloadSHA256, Receipt: existing,
		}), nil
	}
	if existing, ok := s.prepared[prepared.IdempotencyKey]; ok {
		if existing.PayloadSHA256 != prepared.PayloadSHA256 {
			return PreparedConsequenceReceiptV1{}, consequenceConflict(prepared.IdempotencyKey)
		}
		return clonePreparedConsequenceReceiptV1(existing), nil
	}
	s.prepared[prepared.IdempotencyKey] = clonePreparedConsequenceReceiptV1(prepared)
	return clonePreparedConsequenceReceiptV1(prepared), nil
}

func (s *ConsequenceReceiptStoreV1) Commit(prepared PreparedConsequenceReceiptV1) (ConsequenceReceiptV1, error) {
	if err := validatePreparedConsequenceReceiptV1(prepared); err != nil {
		return ConsequenceReceiptV1{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.committed[prepared.IdempotencyKey]; ok {
		if existing.PayloadSHA256 != prepared.PayloadSHA256 {
			return ConsequenceReceiptV1{}, consequenceConflict(prepared.IdempotencyKey)
		}
		return cloneConsequenceReceiptV1(existing), nil
	}
	stored, ok := s.prepared[prepared.IdempotencyKey]
	if !ok || stored.PayloadSHA256 != prepared.PayloadSHA256 {
		return ConsequenceReceiptV1{}, fmt.Errorf("%w: no exact prepared record for %q", ErrConsequenceReceiptConflict, prepared.IdempotencyKey)
	}
	s.committed[prepared.IdempotencyKey] = cloneConsequenceReceiptV1(stored.Receipt)
	return cloneConsequenceReceiptV1(stored.Receipt), nil
}

// Replay validates all hashes and relationships before returning committed
// receipts. Prepared-only records remain available for an idempotent Commit.
func (s *ConsequenceReceiptStoreV1) Replay() ([]ConsequenceReceiptV1, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.committed))
	for key := range s.committed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]ConsequenceReceiptV1, 0, len(keys))
	for _, key := range keys {
		receipt := s.committed[key]
		if err := validateConsequenceReceiptV1(receipt); err != nil {
			return nil, err
		}
		if prepared, ok := s.prepared[key]; !ok || prepared.PayloadSHA256 != receipt.PayloadSHA256 {
			return nil, fmt.Errorf("%w: committed receipt %q lacks exact prepare", ErrConsequenceReceiptConflict, key)
		}
		result = append(result, cloneConsequenceReceiptV1(receipt))
	}
	return result, nil
}

func (s *ConsequenceReceiptStoreV1) Journal() ConsequenceReceiptJournalV1 {
	s.mu.Lock()
	defer s.mu.Unlock()
	journal := ConsequenceReceiptJournalV1{Version: ConsequenceReceiptVersionV1}
	for _, prepared := range s.prepared {
		journal.Prepared = append(journal.Prepared, clonePreparedConsequenceReceiptV1(prepared))
	}
	for _, receipt := range s.committed {
		journal.Committed = append(journal.Committed, cloneConsequenceReceiptV1(receipt))
	}
	sort.Slice(journal.Prepared, func(i, j int) bool { return journal.Prepared[i].IdempotencyKey < journal.Prepared[j].IdempotencyKey })
	sort.Slice(journal.Committed, func(i, j int) bool { return journal.Committed[i].IdempotencyKey < journal.Committed[j].IdempotencyKey })
	return journal
}

func buildConsequenceReceiptV1(input ConsequenceReceiptInputV1, proof ConsequenceCreationProofV1) (ConsequenceReceiptV1, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ControllerAuthorityReceiptID = strings.TrimSpace(input.ControllerAuthorityReceiptID)
	input.RelationshipKind = strings.TrimSpace(input.RelationshipKind)
	input.SharedEvidenceRefs = normalizedUniqueStrings(input.SharedEvidenceRefs)
	input.ImpactSet.AffectedIssueIDs = normalizedUniqueStrings(input.ImpactSet.AffectedIssueIDs)
	input.ImpactSet.UnrelatedControlIssueIDs = normalizedUniqueStrings(input.ImpactSet.UnrelatedControlIssueIDs)
	receipt := ConsequenceReceiptV1{
		Version: ConsequenceReceiptVersionV1, IdempotencyKey: input.IdempotencyKey,
		ControllerAuthority: proof.Authority,
		RelationshipKind:    input.RelationshipKind, Source: proof.Source.Snapshot, Affected: proof.Affected.Snapshot,
		SourceProvenance: proof.Source.Provenance, AffectedProvenance: proof.Affected.Provenance,
		ImpactSet: input.ImpactSet, SharedEvidenceRefs: input.SharedEvidenceRefs,
		ScopeEffect: referenceOnlyEffectV1, StatusEffect: referenceOnlyEffectV1,
		AuthorityEffect: referenceOnlyEffectV1,
	}
	if err := validateConsequenceCreationProofV1(input, proof); err != nil {
		return ConsequenceReceiptV1{}, err
	}
	if err := validateConsequenceReceiptBodyV1(receipt); err != nil {
		return ConsequenceReceiptV1{}, err
	}
	digest, err := consequenceReceiptDigestV1(receipt)
	if err != nil {
		return ConsequenceReceiptV1{}, err
	}
	receipt.PayloadSHA256 = digest
	receipt.ReceiptID = "consequence-" + digest[:24]
	return receipt, nil
}

func validatePreparedConsequenceReceiptV1(prepared PreparedConsequenceReceiptV1) error {
	if prepared.Version != ConsequenceReceiptVersionV1 || prepared.IdempotencyKey == "" || prepared.IdempotencyKey != prepared.Receipt.IdempotencyKey || prepared.PayloadSHA256 != prepared.Receipt.PayloadSHA256 {
		return fmt.Errorf("%w: prepared record identity mismatch", ErrConsequenceReceiptInvalid)
	}
	return validateConsequenceReceiptV1(prepared.Receipt)
}

func validateConsequenceReceiptV1(receipt ConsequenceReceiptV1) error {
	if err := validateConsequenceReceiptBodyV1(receipt); err != nil {
		return err
	}
	digest, err := consequenceReceiptDigestV1(receipt)
	if err != nil {
		return err
	}
	if receipt.PayloadSHA256 != digest || receipt.ReceiptID != "consequence-"+digest[:24] {
		return fmt.Errorf("%w: receipt hash mismatch", ErrConsequenceReceiptInvalid)
	}
	return nil
}

// ValidateConsequenceReceiptV1 lets durable controller stores verify a loaded
// receipt before returning it to workflow consumers. Creation remains gated by
// ConsequenceCreationGateV1; this function grants no creation authority.
func ValidateConsequenceReceiptV1(receipt ConsequenceReceiptV1) error {
	return validateConsequenceReceiptV1(receipt)
}

func validateConsequenceReceiptBodyV1(receipt ConsequenceReceiptV1) error {
	if receipt.Version != ConsequenceReceiptVersionV1 || strings.TrimSpace(receipt.IdempotencyKey) == "" || strings.TrimSpace(receipt.RelationshipKind) == "" {
		return fmt.Errorf("%w: version, idempotency key, and relationship kind are required", ErrConsequenceReceiptInvalid)
	}
	if err := validateIssueOutcomeSnapshotV1(receipt.Source); err != nil {
		return err
	}
	if err := validateIssueOutcomeSnapshotV1(receipt.Affected); err != nil {
		return err
	}
	if err := validateControllerAuthorityV1(receipt.ControllerAuthority, receipt.Source, receipt.Affected); err != nil {
		return err
	}
	if err := validateSnapshotProvenanceV1(receipt.Source, receipt.SourceProvenance); err != nil {
		return err
	}
	if err := validateSnapshotProvenanceV1(receipt.Affected, receipt.AffectedProvenance); err != nil {
		return err
	}
	if receipt.Source.IssueID == receipt.Affected.IssueID || receipt.Source.OutcomeID == receipt.Affected.OutcomeID {
		return fmt.Errorf("%w: source and affected issue/outcome must be distinct", ErrConsequenceReceiptInvalid)
	}
	if len(receipt.SharedEvidenceRefs) == 0 || !slices.Contains(receipt.ImpactSet.AffectedIssueIDs, receipt.Affected.IssueID) || slices.Contains(receipt.ImpactSet.UnrelatedControlIssueIDs, receipt.Affected.IssueID) {
		return fmt.Errorf("%w: affected issue must be related and not an unrelated control", ErrConsequenceReceiptInvalid)
	}
	for _, issueID := range receipt.ImpactSet.AffectedIssueIDs {
		if slices.Contains(receipt.ImpactSet.UnrelatedControlIssueIDs, issueID) {
			return fmt.Errorf("%w: impact and control sets overlap at %q", ErrConsequenceReceiptInvalid, issueID)
		}
	}
	if receipt.ScopeEffect != referenceOnlyEffectV1 || receipt.StatusEffect != referenceOnlyEffectV1 || receipt.AuthorityEffect != referenceOnlyEffectV1 {
		return fmt.Errorf("%w: consequence may only reference evidence without merging issue state", ErrConsequenceReceiptInvalid)
	}
	if !isNormalizedUnique(receipt.SharedEvidenceRefs) || !isNormalizedUnique(receipt.ImpactSet.AffectedIssueIDs) || !isNormalizedUnique(receipt.ImpactSet.UnrelatedControlIssueIDs) {
		return fmt.Errorf("%w: lists must be sorted, unique, and non-empty where required", ErrConsequenceReceiptInvalid)
	}
	return nil
}

func validateConsequenceCreationProofV1(input ConsequenceReceiptInputV1, proof ConsequenceCreationProofV1) error {
	if input.ControllerAuthorityReceiptID == "" || input.ControllerAuthorityReceiptID != proof.Authority.ReceiptID {
		return fmt.Errorf("%w: controller authority receipt mismatch", ErrConsequenceReceiptInvalid)
	}
	if input.Source != proof.Source.Snapshot || input.Affected != proof.Affected.Snapshot {
		return fmt.Errorf("%w: supplied issue/outcome snapshot is stale or forged", ErrConsequenceReceiptInvalid)
	}
	if err := validateControllerAuthorityV1(proof.Authority, proof.Source.Snapshot, proof.Affected.Snapshot); err != nil {
		return err
	}
	if err := validateSnapshotProvenanceV1(proof.Source.Snapshot, proof.Source.Provenance); err != nil {
		return err
	}
	return validateSnapshotProvenanceV1(proof.Affected.Snapshot, proof.Affected.Provenance)
}

func validateControllerAuthorityV1(authority ConsequenceControllerAuthorityV1, source, affected IssueOutcomeSnapshotV1) error {
	if authority.ReceiptID == "" || authority.ControllerID == "" || authority.IssuerRole != "deterministic_controller" || authority.Source != "controller_record" || authority.Action != "record_cross_issue_consequence" || authority.RouteVersion == "" || authority.GraphVersion == "" || !authority.Authenticated || !authority.Immutable {
		return fmt.Errorf("%w: authenticated immutable deterministic controller authority is required", ErrConsequenceReceiptInvalid)
	}
	if authority.SourceIssueID != source.IssueID || authority.AffectedIssueID != affected.IssueID {
		return fmt.Errorf("%w: controller authority issue binding mismatch", ErrConsequenceReceiptInvalid)
	}
	return nil
}

func validateSnapshotProvenanceV1(snapshot IssueOutcomeSnapshotV1, provenance IssueOutcomeSnapshotProvenanceV1) error {
	if provenance.SnapshotReceiptID == "" || provenance.StoreRevision == "" || provenance.SnapshotSHA256 == "" {
		return fmt.Errorf("%w: durable current snapshot provenance is required", ErrConsequenceReceiptInvalid)
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal issue/outcome snapshot: %w", err)
	}
	sum := sha256.Sum256(b)
	if provenance.SnapshotSHA256 != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("%w: current snapshot provenance hash mismatch", ErrConsequenceReceiptInvalid)
	}
	return nil
}

func validateIssueOutcomeSnapshotV1(snapshot IssueOutcomeSnapshotV1) error {
	if strings.TrimSpace(snapshot.IssueID) == "" || strings.TrimSpace(snapshot.OutcomeID) == "" || strings.TrimSpace(snapshot.Status) == "" || snapshot.ScopeRevision < 1 || snapshot.AuthorityRevision < 1 {
		return fmt.Errorf("%w: complete independent issue snapshot is required", ErrConsequenceReceiptInvalid)
	}
	return nil
}

func consequenceReceiptDigestV1(receipt ConsequenceReceiptV1) (string, error) {
	receipt.ReceiptID = ""
	receipt.PayloadSHA256 = ""
	b, err := json.Marshal(receipt)
	if err != nil {
		return "", fmt.Errorf("marshal consequence receipt: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func normalizedUniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func isNormalizedUnique(values []string) bool {
	for i, value := range values {
		if value == "" || strings.TrimSpace(value) != value || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}

func consequenceConflict(key string) error {
	return fmt.Errorf("%w: idempotency key %q has different immutable payload", ErrConsequenceReceiptConflict, key)
}

func clonePreparedConsequenceReceiptV1(prepared PreparedConsequenceReceiptV1) PreparedConsequenceReceiptV1 {
	prepared.Receipt = cloneConsequenceReceiptV1(prepared.Receipt)
	return prepared
}

func cloneConsequenceReceiptV1(receipt ConsequenceReceiptV1) ConsequenceReceiptV1 {
	receipt.SharedEvidenceRefs = slices.Clone(receipt.SharedEvidenceRefs)
	receipt.ImpactSet.AffectedIssueIDs = slices.Clone(receipt.ImpactSet.AffectedIssueIDs)
	receipt.ImpactSet.UnrelatedControlIssueIDs = slices.Clone(receipt.ImpactSet.UnrelatedControlIssueIDs)
	return receipt
}
