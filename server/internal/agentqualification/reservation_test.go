package agentqualification

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestReserveNextScreeningLaunchClaimsSeededAssignmentOnce(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	root := t.TempDir()
	prepared, reservation, path, err := ReserveNextScreeningLaunch(root, batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z", "2026-09-11T12:01:01Z")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Assignment == nil || reservation.EvaluationBinding != prepared.Assignment.EvaluationBinding || reservation.AdmissionInputSHA256 == "" || path == "" {
		t.Fatalf("reservation incomplete: %+v %+v %q", prepared, reservation, path)
	}
	if _, err := os.Stat(filepath.Join(path, "reservation.json")); err != nil {
		t.Fatalf("durable reservation missing: %v", err)
	}
	if _, _, _, err := ReserveNextScreeningLaunch(root, batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z", "2026-09-11T12:01:02Z"); !errors.Is(err, ErrScreeningAssignmentReserved) {
		t.Fatalf("duplicate claim was not blocked: %v", err)
	}
}

func TestReserveNextScreeningLaunchIsAtomicAcrossConcurrentCallers(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	root := t.TempDir()
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, _, err := ReserveNextScreeningLaunch(root, batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z", "2026-09-11T12:01:01Z")
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded, blocked := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrScreeningAssignmentReserved):
			blocked++
		default:
			t.Fatalf("unexpected reservation error: %v", err)
		}
	}
	if succeeded != 1 || blocked != 1 {
		t.Fatalf("atomic claim result success=%d blocked=%d", succeeded, blocked)
	}
}

func TestIncompleteReservationRemainsAmbiguousAndBlocksRetry(t *testing.T) {
	batch, _ := CurrentScreeningBatch()
	assignments, _ := ScreeningAssignments(batch)
	root := t.TempDir()
	bindingHash := sha256Hex([]byte(assignments[0].EvaluationBinding))
	if err := os.Mkdir(filepath.Join(root, bindingHash), 0o700); err != nil {
		t.Fatal(err)
	}
	ledger := BatchUsageLedger{SchemaVersion: BatchUsageSchemaVersion, BatchID: batch.BatchID, TokenCeiling: batch.TokenCeiling}
	capacity := CapacitySnapshot{Provider: "codex", LimitID: "codex", AccountLane: "primary_subscription", WindowID: "weekly-1", UsedBasisPoints: 4800, ObservedAt: "2026-09-11T12:00:00Z", Source: "authoritative_account_usage"}
	if _, _, _, err := ReserveNextScreeningLaunch(root, batch, ledger, capacity, capacity, "2026-09-11T12:01:00Z", "2026-09-11T12:01:01Z"); !errors.Is(err, ErrScreeningAssignmentReserved) {
		t.Fatalf("ambiguous crash claim did not block retry: %v", err)
	}
}
