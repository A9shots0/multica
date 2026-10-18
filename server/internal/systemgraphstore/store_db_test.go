package systemgraphstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const systemGraphStoreTestDatabaseEnv = "SYSTEM_GRAPH_STORE_TEST_DATABASE_URL"

const (
	testWorkspaceID       = "10000000-0000-0000-0000-000000000001"
	testSourceIssueID     = "20000000-0000-0000-0000-000000000001"
	testAffectedIssueID   = "20000000-0000-0000-0000-000000000002"
	testControlIssueID    = "20000000-0000-0000-0000-000000000003"
	testSourceOutcomeID   = "30000000-0000-0000-0000-000000000001"
	testAffectedOutcomeID = "30000000-0000-0000-0000-000000000002"
	testEvidenceID        = "40000000-0000-0000-0000-000000000001"
)

func TestStorePostgresConsequenceAndRegressionContracts(t *testing.T) {
	databaseURL := os.Getenv(systemGraphStoreTestDatabaseEnv)
	if databaseURL == "" {
		t.Skipf("integration test requires an isolated database at %s", systemGraphStoreTestDatabaseEnv)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to system graph store test database: %v", err)
	}
	schema := fmt.Sprintf("system_graph_store_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "SET search_path TO public")
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
		_ = conn.Close(context.Background())
	})

	createSystemGraphStorePrerequisites(t, ctx, conn)
	for version := 558; version <= 574; version++ {
		matches, globErr := filepath.Glob(filepath.Join("..", "..", "migrations", fmt.Sprintf("%d_*.up.sql", version)))
		if globErr != nil || len(matches) != 1 {
			t.Fatalf("resolve migration %d: matches=%v err=%v", version, matches, globErr)
		}
		body, readErr := os.ReadFile(matches[0])
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := conn.Exec(ctx, string(body)); execErr != nil {
			t.Fatalf("apply migration %d: %v", version, execErr)
		}
	}
	seedSystemGraphWorkflowState(t, ctx, conn)

	store := New(conn, db.New(conn))
	now := time.Date(2026, 9, 14, 12, 0, 0, 123456000, time.UTC)
	authority := ConsequenceAuthorityIssueV1{ReceiptID: "controller-consequence-1", ControllerID: "controller-1", RouteVersion: "route-v1"}
	input := consequenceInput()
	receipt, err := store.CreateConsequenceReceipt(ctx, testWorkspaceID, authority, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Source.Status != "live" || receipt.Affected.Status != "working" || receipt.Source.ScopeRevision != 3 || receipt.Affected.AuthorityRevision != 11 {
		t.Fatalf("receipt did not preserve independent snapshots: %+v", receipt)
	}
	replayed, err := store.CreateConsequenceReceipt(ctx, testWorkspaceID, authority, input, now.Add(time.Minute))
	if err != nil || replayed.ReceiptID != receipt.ReceiptID {
		t.Fatalf("exact consequence replay = %+v err=%v", replayed, err)
	}
	altered := input
	altered.RelationshipKind = "forged"
	if _, err := store.CreateConsequenceReceipt(ctx, testWorkspaceID, authority, altered, now); !errors.Is(err, systemgraph.ErrConsequenceReceiptConflict) {
		t.Fatalf("altered consequence replay error = %v", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_current_picture SET authority_revision = 12 WHERE issue_id = $1`, testAffectedIssueID); err != nil {
		t.Fatal(err)
	}
	stale := input
	stale.IdempotencyKey = "consequence-stale"
	stale.ControllerAuthorityReceiptID = "controller-consequence-2"
	staleAuthority := authority
	staleAuthority.ReceiptID = stale.ControllerAuthorityReceiptID
	if _, err := store.CreateConsequenceReceipt(ctx, testWorkspaceID, staleAuthority, stale, now); !errors.Is(err, systemgraph.ErrConsequenceReceiptInvalid) {
		t.Fatalf("stale snapshot error = %v", err)
	}
	for _, issueID := range []string{testSourceIssueID, testAffectedIssueID} {
		listed, listErr := store.ListConsequenceReceipts(ctx, testWorkspaceID, issueID)
		if listErr != nil || len(listed) != 1 || listed[0].ReceiptID != receipt.ReceiptID {
			t.Fatalf("list consequence for %s = %+v err=%v", issueID, listed, listErr)
		}
	}
	for _, statement := range []string{
		`UPDATE issue_workflow_consequence_receipt SET relationship_kind = 'mutated'`,
		`DELETE FROM issue_workflow_consequence_receipt`,
	} {
		if _, err := conn.Exec(ctx, statement); err == nil {
			t.Fatalf("append-only consequence storage accepted %q", statement)
		}
	}

	proposal, evidence, approval := regressionSubmission(t, "proposal-add", systemgraph.RegressionAdd, 0, "v1 behavior", now)
	record, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, proposal, evidence, approval, now)
	if err != nil || record.RegistryVersion != 1 {
		t.Fatalf("apply durable regression = %+v err=%v", record, err)
	}
	replayedRecord, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, proposal, evidence, approval, now)
	if err != nil || replayedRecord.RecordHash != record.RecordHash {
		t.Fatalf("exact regression replay = %+v err=%v", replayedRecord, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET superseded_at = $1 WHERE id = $2`, now, testEvidenceID); err != nil {
		t.Fatal(err)
	}
	replayedAfterSupersession, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, proposal, evidence, approval, now.Add(2*time.Hour))
	if err != nil || replayedAfterSupersession.RecordHash != record.RecordHash {
		t.Fatalf("exact regression replay after evidence supersession = %+v err=%v", replayedAfterSupersession, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE issue_workflow_evidence SET superseded_at = NULL WHERE id = $1`, testEvidenceID); err != nil {
		t.Fatal(err)
	}
	graph, err := systemgraph.Default()
	if err != nil {
		t.Fatal(err)
	}
	impact, err := systemgraph.BuildRegressionImpactScope(graph, "controller-impact-1", []string{"j_identity_access"})
	if err != nil {
		t.Fatal(err)
	}
	selection, err := store.ActiveRegressionsForWorkflow(ctx, testWorkspaceID, impact, systemgraph.RegressionConsumerRelease, now)
	if err != nil || len(selection.Obligations) != 1 || selection.Obligations[0].ID != proposal.Obligation.ID {
		t.Fatalf("durable release selection = %+v err=%v", selection, err)
	}

	missingProposal, missingEvidence, missingApproval := regressionSubmission(t, "proposal-missing-evidence", systemgraph.RegressionUpdate, 1, "missing evidence", now)
	missingEvidence.SourceRefs = []string{"workflow-evidence:40000000-0000-0000-0000-000000000099"}
	if _, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, missingProposal, missingEvidence, missingApproval, now); !errors.Is(err, ErrWorkflowEvidenceDenied) {
		t.Fatalf("missing workflow evidence error = %v", err)
	}
	bareRefProposal, bareRefEvidence, bareRefApproval := regressionSubmission(t, "proposal-bare-evidence", systemgraph.RegressionUpdate, 1, "bare evidence", now)
	bareRefEvidence.SourceRefs = []string{testEvidenceID}
	if _, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, bareRefProposal, bareRefEvidence, bareRefApproval, now); !errors.Is(err, ErrWorkflowEvidenceDenied) {
		t.Fatalf("untyped workflow evidence error = %v", err)
	}

	updateProposal, updateEvidence, updateApproval := regressionSubmission(t, "proposal-update", systemgraph.RegressionUpdate, 1, "v2 behavior", now)
	runConcurrentExactRegression(t, ctx, databaseURL, schema, updateProposal, updateEvidence, updateApproval, now)
	supersedeProposal, supersedeEvidence, supersedeApproval := regressionSubmission(t, "proposal-supersede", systemgraph.RegressionSupersede, 2, "v3 replacement", now)
	if _, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, supersedeProposal, supersedeEvidence, supersedeApproval, now); err != nil {
		t.Fatal(err)
	}
	retireProposal, retireEvidence, retireApproval := regressionSubmission(t, "proposal-retire", systemgraph.RegressionRetire, 3, "v4 retired", now)
	if _, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, retireProposal, retireEvidence, retireApproval, now); err != nil {
		t.Fatal(err)
	}

	conflictProposal, conflictEvidence, conflictApproval := regressionSubmission(t, "proposal-conflict", systemgraph.RegressionUpdate, 1, "stale update", now)
	if _, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, conflictProposal, conflictEvidence, conflictApproval, now); !errors.Is(err, systemgraph.ErrRegressionConflict) {
		t.Fatalf("conflicting durable regression error = %v", err)
	}
	restarted := New(conn, db.New(conn))
	registry, err := restarted.LoadRegressionRegistry(ctx, testWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.History()) != 4 || len(registry.RejectedAttempts()) != 1 || len(registry.Findings()) != 1 {
		t.Fatalf("restart replay lost history: accepted=%d rejected=%d findings=%d", len(registry.History()), len(registry.RejectedAttempts()), len(registry.Findings()))
	}
	selection, err = restarted.ActiveRegressionsForWorkflow(ctx, testWorkspaceID, impact, systemgraph.RegressionConsumerQualityChecker, now)
	if err != nil || len(selection.Obligations) != 0 {
		t.Fatalf("retired obligation survived restart: %+v err=%v", selection, err)
	}
	for _, statement := range []string{
		`UPDATE issue_workflow_regression_change_record SET proposal_id = 'mutated'`,
		`DELETE FROM issue_workflow_regression_change_record`,
		`UPDATE issue_workflow_regression_rejected_attempt SET proposal_id = 'mutated'`,
		`DELETE FROM issue_workflow_regression_rejected_attempt`,
	} {
		if _, err := conn.Exec(ctx, statement); err == nil {
			t.Fatalf("append-only regression storage accepted %q", statement)
		}
	}
}

func createSystemGraphStorePrerequisites(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	statements := []string{
		`CREATE TABLE workspace (id UUID NOT NULL PRIMARY KEY)`,
		`CREATE TABLE issue (id UUID NOT NULL PRIMARY KEY, workspace_id UUID NOT NULL)`,
		`CREATE TABLE issue_current_picture (workspace_id UUID NOT NULL, issue_id UUID NOT NULL, revision BIGINT NOT NULL, scope_revision BIGINT, authority_revision BIGINT)`,
		`CREATE TABLE issue_workflow_outcome (id UUID NOT NULL, workspace_id UUID NOT NULL, issue_id UUID NOT NULL, revision BIGINT NOT NULL, state TEXT NOT NULL, superseded_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE issue_workflow_evidence (id UUID NOT NULL, workspace_id UUID NOT NULL, issue_id UUID NOT NULL, observed_at TIMESTAMPTZ NOT NULL, valid_until TIMESTAMPTZ, contradictions JSONB NOT NULL DEFAULT '[]'::jsonb, superseded_at TIMESTAMPTZ)`,
	}
	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
}

func seedSystemGraphWorkflowState(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	if _, err := conn.Exec(ctx, `INSERT INTO workspace (id) VALUES ($1)`, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		issueID, outcomeID, state string
		scope, authority          int64
	}{
		{testSourceIssueID, testSourceOutcomeID, "live", 3, 7},
		{testAffectedIssueID, testAffectedOutcomeID, "working", 5, 11},
	} {
		if _, err := conn.Exec(ctx, `INSERT INTO issue (id, workspace_id) VALUES ($1, $2)`, row.issueID, testWorkspaceID); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue_current_picture (workspace_id, issue_id, revision, scope_revision, authority_revision) VALUES ($1, $2, 9, $3, $4)`, testWorkspaceID, row.issueID, row.scope, row.authority); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_outcome (id, workspace_id, issue_id, revision, state, updated_at) VALUES ($1, $2, $3, 1, $4, '2026-09-14T11:00:00Z')`, row.outcomeID, testWorkspaceID, row.issueID, row.state); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(ctx, `INSERT INTO issue_workflow_evidence (id, workspace_id, issue_id, observed_at, contradictions) VALUES ($1, $2, $3, '2026-09-14T11:00:00Z', '[]'::jsonb)`, testEvidenceID, testWorkspaceID, testSourceIssueID); err != nil {
		t.Fatal(err)
	}
}

func consequenceInput() systemgraph.ConsequenceReceiptInputV1 {
	return systemgraph.ConsequenceReceiptInputV1{
		IdempotencyKey: "consequence-1", ControllerAuthorityReceiptID: "controller-consequence-1",
		RelationshipKind:   "shared_contract_consequence",
		Source:             systemgraph.IssueOutcomeSnapshotV1{IssueID: testSourceIssueID, OutcomeID: testSourceOutcomeID, ScopeRevision: 3, Status: "live", AuthorityRevision: 7},
		Affected:           systemgraph.IssueOutcomeSnapshotV1{IssueID: testAffectedIssueID, OutcomeID: testAffectedOutcomeID, ScopeRevision: 5, Status: "working", AuthorityRevision: 11},
		ImpactSet:          systemgraph.ConsequenceImpactSetV1{AffectedIssueIDs: []string{testAffectedIssueID}, UnrelatedControlIssueIDs: []string{testControlIssueID}},
		SharedEvidenceRefs: []string{"workflow-evidence:" + testEvidenceID},
	}
}

func regressionSubmission(t *testing.T, id string, kind systemgraph.RegressionChangeKind, expected uint64, requirement string, now time.Time) (systemgraph.RegressionProposal, systemgraph.RegressionDomainEvidence, systemgraph.RegressionApprovalReceipt) {
	t.Helper()
	status := systemgraph.RegressionActive
	if kind == systemgraph.RegressionRetire {
		status = systemgraph.RegressionRetired
	}
	obligation := systemgraph.RegressionObligation{
		ID: "reg-account-assignment", Version: expected + 1,
		Title: "Account assignment remains consistent", Requirement: requirement,
		ProofMethod:             "verify stable CRM id through every affected projection",
		AccountableDomainRoleID: "crm_access_outcome",
		Scope:                   systemgraph.RegressionScope{JourneyIDs: []string{"j_identity_access"}, NodeIDs: []string{"crm_identity", "linkhub"}, EdgeIDs: []string{"e_identity_linkhub"}},
		Status:                  status, ReviewAfter: now.Add(24 * time.Hour),
	}
	if kind == systemgraph.RegressionSupersede {
		obligation.SupersedesVersion = expected
	}
	proposal := systemgraph.RegressionProposal{
		ID: id, Kind: kind, ExpectedVersion: expected, ProposedVersion: expected + 1,
		AuthorRoleID: "crm_access_outcome", AuthorFunction: systemgraph.RegressionAuthorAccountableDomain,
		CreatedAt: now.Add(-10 * time.Minute), Obligation: obligation,
	}
	evidence := systemgraph.RegressionDomainEvidence{
		ID: "evidence-" + id, ProposalID: id, AuthorAgentID: "domain-agent",
		AccountableRoleID: "crm_access_outcome", Scope: obligation.Scope,
		SourceRefs: []string{"workflow-evidence:" + testEvidenceID}, Source: "controller_record", Immutable: true,
		ObservedAt: now.Add(-20 * time.Minute), ReviewAfter: now.Add(time.Hour),
	}
	hash, err := systemgraph.RegressionProposalSHA256(proposal)
	if err != nil {
		t.Fatal(err)
	}
	approval := systemgraph.RegressionApprovalReceipt{
		ID: "approval-" + id, ProposalID: id, ProposalHash: hash,
		ApproverAgentID: "independent-agent", ApproverRoleID: "quality_checker",
		ApproverFunction: "independent_assurance", IndependentFromRole: "crm_access_outcome",
		Source: "controller_record", Approved: true, Immutable: true,
		IssuedAt: now.Add(-5 * time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	return proposal, evidence, approval
}

func runConcurrentExactRegression(t *testing.T, ctx context.Context, databaseURL, schema string, proposal systemgraph.RegressionProposal, evidence systemgraph.RegressionDomainEvidence, approval systemgraph.RegressionApprovalReceipt, now time.Time) {
	t.Helper()
	stores := make([]*Store, 0, 2)
	connections := make([]*pgx.Conn, 0, 2)
	for range 2 {
		conn, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, schema); err != nil {
			_ = conn.Close(ctx)
			t.Fatal(err)
		}
		connections = append(connections, conn)
		stores = append(stores, New(conn, db.New(conn)))
	}
	defer func() {
		for _, conn := range connections {
			_ = conn.Close(ctx)
		}
	}()
	errs := make(chan error, len(stores))
	hashes := make(chan string, len(stores))
	var wg sync.WaitGroup
	for _, store := range stores {
		wg.Add(1)
		go func(store *Store) {
			defer wg.Done()
			record, err := store.ApplyRegressionChange(ctx, testWorkspaceID, testSourceIssueID, proposal, evidence, approval, now)
			errs <- err
			hashes <- record.RecordHash
		}(store)
	}
	wg.Wait()
	close(errs)
	close(hashes)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent exact regression: %v", err)
		}
	}
	var first string
	for hash := range hashes {
		if first == "" {
			first = hash
		} else if hash != first {
			t.Fatalf("concurrent exact regression hashes differ: %s != %s", hash, first)
		}
	}
}

func TestDecodeStrictRejectsUnknownAndTrailingFields(t *testing.T) {
	var target systemgraph.ConsequenceReceiptV1
	for _, raw := range []string{`{"unknown":true}`, `{}` + `{}`} {
		if err := decodeStrict([]byte(raw), &target); err == nil {
			t.Fatalf("strict decoder accepted %q", raw)
		}
	}
	if !strings.Contains(ErrWorkflowEvidenceDenied.Error(), "workflow evidence") {
		t.Fatal("stable denial error lost its operator-facing meaning")
	}
}
