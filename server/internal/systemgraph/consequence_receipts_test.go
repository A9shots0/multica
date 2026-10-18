package systemgraph

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestConsequenceReceiptPrepareCommitReplayPreservesIndependentIssues(t *testing.T) {
	store, err := newConsequenceReceiptTestStoreV1(ConsequenceReceiptJournalV1{})
	if err != nil {
		t.Fatal(err)
	}
	input := consequenceReceiptInputFixtureV1()
	prepared, err := store.Prepare(input)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Receipt.Source.Status != "live" || prepared.Receipt.Affected.Status != "working" {
		t.Fatal("prepare changed independent issue status")
	}
	if prepared.Receipt.Source.AuthorityRevision == prepared.Receipt.Affected.AuthorityRevision {
		t.Fatal("fixture must prove distinct authority revisions")
	}
	receipt, err := store.Commit(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ScopeEffect != referenceOnlyEffectV1 || receipt.StatusEffect != referenceOnlyEffectV1 || receipt.AuthorityEffect != referenceOnlyEffectV1 {
		t.Fatal("receipt does not explicitly prohibit issue-state merging")
	}
	replayed, err := store.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 || replayed[0].PayloadSHA256 != receipt.PayloadSHA256 {
		t.Fatalf("replay = %+v, want one exact receipt", replayed)
	}
	if len(replayed[0].SharedEvidenceRefs) != 2 {
		t.Fatalf("shared evidence refs = %v", replayed[0].SharedEvidenceRefs)
	}
}

func TestConsequenceReceiptIdempotentDuplicateAndConflict(t *testing.T) {
	store, err := newConsequenceReceiptTestStoreV1(ConsequenceReceiptJournalV1{})
	if err != nil {
		t.Fatal(err)
	}
	input := consequenceReceiptInputFixtureV1()
	first, err := store.Prepare(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Prepare(input)
	if err != nil || second.PayloadSHA256 != first.PayloadSHA256 {
		t.Fatalf("idempotent prepare = %+v, %v", second, err)
	}
	if _, err := store.Commit(first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(first); err != nil {
		t.Fatalf("idempotent commit: %v", err)
	}
	input.Affected.Status = "done"
	gate := store.gate.(*consequenceReceiptGateStubV1)
	gate.proof.Affected.Snapshot = input.Affected
	gate.proof.Affected.Provenance = snapshotProvenanceFixtureV1(input.Affected, "affected-current-v2")
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptConflict) {
		t.Fatalf("conflicting duplicate error = %v", err)
	}
}

func TestConsequenceReceiptCrashRecoveryCommitsPreparedOnce(t *testing.T) {
	firstStore, err := newConsequenceReceiptTestStoreV1(ConsequenceReceiptJournalV1{})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := firstStore.Prepare(consequenceReceiptInputFixtureV1())
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(firstStore.Journal())
	if err != nil {
		t.Fatal(err)
	}
	var recoveredJournal ConsequenceReceiptJournalV1
	if err := json.Unmarshal(serialized, &recoveredJournal); err != nil {
		t.Fatal(err)
	}
	recoveredStore, err := newConsequenceReceiptTestStoreV1(recoveredJournal)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := recoveredStore.Prepare(consequenceReceiptInputFixtureV1())
	if err != nil || recovered.PayloadSHA256 != prepared.PayloadSHA256 {
		t.Fatalf("recover prepare = %+v, %v", recovered, err)
	}
	firstCommit, err := recoveredStore.Commit(recovered)
	if err != nil {
		t.Fatal(err)
	}
	secondCommit, err := recoveredStore.Commit(recovered)
	if err != nil || secondCommit.ReceiptID != firstCommit.ReceiptID {
		t.Fatalf("recovered duplicate commit = %+v, %v", secondCommit, err)
	}
	replayed, err := recoveredStore.Replay()
	if err != nil || len(replayed) != 1 {
		t.Fatalf("replay after recovery = %+v, %v", replayed, err)
	}
}

func TestConsequenceReceiptExcludesUnrelatedControl(t *testing.T) {
	store, err := newConsequenceReceiptTestStoreV1(ConsequenceReceiptJournalV1{})
	if err != nil {
		t.Fatal(err)
	}
	input := consequenceReceiptInputFixtureV1()
	input.Affected = IssueOutcomeSnapshotV1{IssueID: "issue-control", OutcomeID: "outcome-control", ScopeRevision: 1, Status: "done", AuthorityRevision: 1}
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("unrelated control error = %v", err)
	}

	input = consequenceReceiptInputFixtureV1()
	input.ImpactSet.AffectedIssueIDs = append(input.ImpactSet.AffectedIssueIDs, "issue-control")
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("overlapping control error = %v", err)
	}
}

func TestConsequenceReceiptReplayFailsClosedOnTampering(t *testing.T) {
	store, err := newConsequenceReceiptTestStoreV1(ConsequenceReceiptJournalV1{})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := store.Prepare(consequenceReceiptInputFixtureV1())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(prepared); err != nil {
		t.Fatal(err)
	}
	journal := store.Journal()
	journal.Committed[0].Affected.AuthorityRevision++
	if _, err := newConsequenceReceiptTestStoreV1(journal); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("tampered journal error = %v", err)
	}
}

func TestConsequenceReceiptRejectsMissingOrSharedIdentity(t *testing.T) {
	tests := []func(*ConsequenceReceiptInputV1){
		func(in *ConsequenceReceiptInputV1) { in.Source.IssueID = in.Affected.IssueID },
		func(in *ConsequenceReceiptInputV1) { in.Source.OutcomeID = in.Affected.OutcomeID },
		func(in *ConsequenceReceiptInputV1) { in.SharedEvidenceRefs = nil },
		func(in *ConsequenceReceiptInputV1) { in.Affected.ScopeRevision = 0 },
	}
	for i, mutate := range tests {
		store, err := newConsequenceReceiptTestStoreV1(ConsequenceReceiptJournalV1{})
		if err != nil {
			t.Fatal(err)
		}
		input := consequenceReceiptInputFixtureV1()
		mutate(&input)
		if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
			t.Fatalf("case %d error = %v", i, err)
		}
	}
}

func TestConsequenceReceiptRejectsForgedControllerAuthority(t *testing.T) {
	input := consequenceReceiptInputFixtureV1()
	gate := consequenceReceiptGateFixtureV1()
	gate.proof.Authority.Authenticated = false
	store, err := NewConsequenceReceiptStoreV1(ConsequenceReceiptJournalV1{}, gate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("forged authority error = %v", err)
	}

	gate = consequenceReceiptGateFixtureV1()
	gate.proof.Authority.AffectedIssueID = "issue-other"
	store, err = NewConsequenceReceiptStoreV1(ConsequenceReceiptJournalV1{}, gate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("misbound authority error = %v", err)
	}
}

func TestConsequenceReceiptRejectsStaleOrUnprovenSnapshot(t *testing.T) {
	input := consequenceReceiptInputFixtureV1()
	gate := consequenceReceiptGateFixtureV1()
	gate.proof.Affected.Snapshot.Status = "done"
	gate.proof.Affected.Provenance = snapshotProvenanceFixtureV1(gate.proof.Affected.Snapshot, "affected-current")
	store, err := NewConsequenceReceiptStoreV1(ConsequenceReceiptJournalV1{}, gate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("stale snapshot error = %v", err)
	}

	gate = consequenceReceiptGateFixtureV1()
	gate.proof.Source.Provenance.SnapshotSHA256 = "forged"
	store, err = NewConsequenceReceiptStoreV1(ConsequenceReceiptJournalV1{}, gate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Prepare(input); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("forged provenance error = %v", err)
	}
}

func TestConsequenceReceiptRequiresControllerCreationGate(t *testing.T) {
	if _, err := NewConsequenceReceiptStoreV1(ConsequenceReceiptJournalV1{}, nil); !errors.Is(err, ErrConsequenceReceiptInvalid) {
		t.Fatalf("missing gate error = %v", err)
	}
}

func consequenceReceiptInputFixtureV1() ConsequenceReceiptInputV1 {
	return ConsequenceReceiptInputV1{
		IdempotencyKey:               "release-42:issue-affected:contract-change",
		ControllerAuthorityReceiptID: "controller-authority-42",
		RelationshipKind:             "shared_contract_consequence",
		Source: IssueOutcomeSnapshotV1{
			IssueID: "issue-source", OutcomeID: "outcome-source", ScopeRevision: 3,
			Status: "live", AuthorityRevision: 7,
		},
		Affected: IssueOutcomeSnapshotV1{
			IssueID: "issue-affected", OutcomeID: "outcome-affected", ScopeRevision: 5,
			Status: "working", AuthorityRevision: 11,
		},
		ImpactSet: ConsequenceImpactSetV1{
			AffectedIssueIDs:         []string{"issue-affected"},
			UnrelatedControlIssueIDs: []string{"issue-control"},
		},
		SharedEvidenceRefs: []string{"evidence-release-42", "evidence-contract-v8"},
	}
}

type consequenceReceiptGateStubV1 struct {
	proof ConsequenceCreationProofV1
	err   error
}

func (g *consequenceReceiptGateStubV1) AuthorizeAndLoadCurrentConsequenceV1(ConsequenceReceiptInputV1) (ConsequenceCreationProofV1, error) {
	return g.proof, g.err
}

func consequenceReceiptGateFixtureV1() *consequenceReceiptGateStubV1 {
	input := consequenceReceiptInputFixtureV1()
	return &consequenceReceiptGateStubV1{proof: ConsequenceCreationProofV1{
		Authority: ConsequenceControllerAuthorityV1{
			ReceiptID: "controller-authority-42", ControllerID: "controller-1",
			IssuerRole: "deterministic_controller", Source: "controller_record",
			Action: "record_cross_issue_consequence", RouteVersion: "route-v7",
			GraphVersion: "graph-v3", SourceIssueID: input.Source.IssueID,
			AffectedIssueID: input.Affected.IssueID, Authenticated: true, Immutable: true,
		},
		Source: CurrentIssueOutcomeSnapshotV1{
			Snapshot: input.Source, Provenance: snapshotProvenanceFixtureV1(input.Source, "source-current"),
		},
		Affected: CurrentIssueOutcomeSnapshotV1{
			Snapshot: input.Affected, Provenance: snapshotProvenanceFixtureV1(input.Affected, "affected-current"),
		},
	}}
}

func snapshotProvenanceFixtureV1(snapshot IssueOutcomeSnapshotV1, receiptID string) IssueOutcomeSnapshotProvenanceV1 {
	b, err := json.Marshal(snapshot)
	if err != nil {
		panic(fmt.Sprintf("marshal fixture snapshot: %v", err))
	}
	sum := sha256.Sum256(b)
	return IssueOutcomeSnapshotProvenanceV1{
		SnapshotReceiptID: receiptID, StoreRevision: "issue-ledger-v12",
		SnapshotSHA256: fmt.Sprintf("%x", sum),
	}
}

func newConsequenceReceiptTestStoreV1(journal ConsequenceReceiptJournalV1) (*ConsequenceReceiptStoreV1, error) {
	return NewConsequenceReceiptStoreV1(journal, consequenceReceiptGateFixtureV1())
}
