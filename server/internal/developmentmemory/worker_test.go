package developmentmemory

import (
	"context"
	"errors"
	"testing"
	"time"
)

type workerStoreFake struct {
	delivery  Delivery
	claimErr  error
	delivered bool
	failed    bool
}

func (f *workerStoreFake) ClaimNext(context.Context, time.Duration) (Delivery, error) {
	return f.delivery, f.claimErr
}
func (f *workerStoreFake) MarkDelivered(context.Context, string, string) error {
	f.delivered = true
	return nil
}
func (f *workerStoreFake) MarkFailed(context.Context, string, string, error, int) error {
	f.failed = true
	return nil
}

type delivererFake struct{ err error }

func (f delivererFake) Deliver(context.Context, Delivery) error { return f.err }

func TestWorkerSettlesSuccessfulApprovedDelivery(t *testing.T) {
	store := &workerStoreFake{delivery: Delivery{ProposalID: "p1", LeaseToken: "lease"}}
	(&Worker{Store: store, Deliverer: delivererFake{}}).runOne(context.Background())
	if !store.delivered || store.failed {
		t.Fatalf("delivered=%v failed=%v", store.delivered, store.failed)
	}
}

func TestWorkerRetainsFailedApprovedDeliveryForRetry(t *testing.T) {
	store := &workerStoreFake{delivery: Delivery{ProposalID: "p1", LeaseToken: "lease"}}
	(&Worker{Store: store, Deliverer: delivererFake{err: errors.New("intelligence unavailable")}}).runOne(context.Background())
	if store.delivered || !store.failed {
		t.Fatalf("delivered=%v failed=%v", store.delivered, store.failed)
	}
}
