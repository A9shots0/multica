package developmentmemory

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type Deliverer interface {
	Deliver(context.Context, Delivery) error
}

type deliveryStore interface {
	ClaimNext(context.Context, time.Duration) (Delivery, error)
	MarkDelivered(context.Context, string, string) error
	MarkFailed(context.Context, string, string, error, int) error
}

type Worker struct {
	Store         deliveryStore
	Deliverer     Deliverer
	PollInterval  time.Duration
	LeaseDuration time.Duration
	MaxAttempts   int
}

func (w *Worker) Run(ctx context.Context) {
	interval := w.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		w.runOne(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) runOne(ctx context.Context) {
	if w == nil || w.Store == nil || w.Deliverer == nil {
		return
	}
	lease := w.LeaseDuration
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	delivery, err := w.Store.ClaimNext(ctx, lease)
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		slog.Warn("development memory: claim delivery failed", "error", err)
		return
	}
	if err := w.Deliverer.Deliver(ctx, delivery); err != nil {
		slog.Warn("development memory: delivery failed", "proposal_id", delivery.ProposalID, "error", redact.Text(err.Error()))
		if settleErr := w.Store.MarkFailed(ctx, delivery.ProposalID, delivery.LeaseToken, err, w.MaxAttempts); settleErr != nil {
			slog.Error("development memory: failed to settle delivery error", "proposal_id", delivery.ProposalID, "error", settleErr)
		}
		return
	}
	if err := w.Store.MarkDelivered(ctx, delivery.ProposalID, delivery.LeaseToken); err != nil {
		slog.Error("development memory: failed to settle delivered proposal", "proposal_id", delivery.ProposalID, "error", err)
	}
}
