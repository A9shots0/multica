package ownernotification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type Worker struct {
	Queries            *db.Queries
	Mode               string
	Destination        string
	DestinationKey     string
	Sender             Sender
	PollInterval       time.Duration
	LeaseDuration      time.Duration
	DeliveryCheckAfter time.Duration
	MaxDeliveryChecks  int32
	Now                func() time.Time
	Logger             *slog.Logger
}

func (worker Worker) defaults() Worker {
	if worker.PollInterval <= 0 {
		worker.PollInterval = 2 * time.Second
	}
	if worker.LeaseDuration <= 0 {
		worker.LeaseDuration = 30 * time.Second
	}
	if worker.DeliveryCheckAfter <= 0 {
		worker.DeliveryCheckAfter = 5 * time.Second
	}
	if worker.MaxDeliveryChecks <= 0 {
		worker.MaxDeliveryChecks = 12
	}
	if worker.Now == nil {
		worker.Now = time.Now
	}
	if worker.Logger == nil {
		worker.Logger = slog.Default()
	}
	return worker
}

func (worker Worker) Run(ctx context.Context) {
	worker = worker.defaults()
	if worker.Queries == nil || (worker.Mode != "dry_run" && worker.Mode != "live_test" && worker.Mode != "live") {
		return
	}
	ticker := time.NewTicker(worker.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := worker.Queries.RecoverExpiredOwnerNotificationClaims(ctx); err != nil {
			worker.Logger.Error("owner notification expired-claim recovery failed", "error", err)
		}
		if err := worker.ProcessOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			worker.Logger.Error("owner notification delivery failed", "mode", worker.Mode, "error", err)
		}
		if worker.Mode != "dry_run" {
			if err := worker.ReconcileOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				worker.Logger.Error("owner notification reconciliation failed", "mode", worker.Mode, "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (worker Worker) ProcessOne(ctx context.Context) error {
	worker = worker.defaults()
	lease := googleuuid.New()
	now := worker.Now().UTC()
	row, err := worker.Queries.ClaimNextOwnerNotification(ctx, db.ClaimNextOwnerNotificationParams{
		LeaseToken:            pgtype.UUID{Bytes: lease, Valid: true},
		LeaseExpiresAt:        pgtype.Timestamptz{Time: now.Add(worker.LeaseDuration), Valid: true},
		RequestedDeliveryMode: worker.Mode,
	})
	if err != nil {
		return err
	}
	if worker.DestinationKey == "" || row.DestinationKey != worker.DestinationKey {
		return worker.recordFailure(ctx, row.ID, lease, "failed", "destination_binding_mismatch", time.Time{})
	}
	requestKey := fmt.Sprintf("%s/attempt/%d", googleuuid.UUID(row.ID.Bytes), row.AttemptCount)
	if _, err := worker.Queries.CreateOwnerNotificationAttempt(ctx, db.CreateOwnerNotificationAttemptParams{
		ID: pgtype.UUID{Bytes: googleuuid.New(), Valid: true}, NotificationID: row.ID,
		RequestKey: requestKey, State: "started", Metadata: json.RawMessage(`{"schema_version":1}`),
	}); err != nil {
		return err
	}
	if worker.Mode == "dry_run" {
		_, _ = worker.Queries.CreateOwnerNotificationAttempt(ctx, db.CreateOwnerNotificationAttemptParams{
			ID: pgtype.UUID{Bytes: googleuuid.New(), Valid: true}, NotificationID: row.ID,
			RequestKey: requestKey + "/result", State: "dry_run_delivered", Metadata: json.RawMessage(`{"schema_version":1}`),
		})
		_, err = worker.Queries.CompleteOwnerNotificationDryRun(ctx, db.CompleteOwnerNotificationDryRunParams{ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}})
		return err
	}
	if worker.Sender == nil || worker.Destination == "" {
		return worker.recordFailure(ctx, row.ID, lease, "failed", "provider_not_configured", time.Time{})
	}
	result, sendErr := worker.Sender.Send(ctx, worker.Destination, row.Message)
	state := result.Disposition
	if state == "" {
		state = "ambiguous"
	}
	_, _ = worker.Queries.CreateOwnerNotificationAttempt(ctx, db.CreateOwnerNotificationAttemptParams{
		ID: pgtype.UUID{Bytes: googleuuid.New(), Valid: true}, NotificationID: row.ID,
		RequestKey: requestKey + "/result", State: attemptState(state),
		ProviderMessageID: text(result.ProviderMessageID), ProviderState: text(result.ProviderState),
		ErrorCode: text(result.ErrorCode), Metadata: json.RawMessage(`{"schema_version":1}`),
	})
	switch classifySendTransition(state, row.AttemptCount) {
	case sendTransitionAccepted:
		_, err = worker.Queries.RecordOwnerNotificationProviderAccepted(ctx, db.RecordOwnerNotificationProviderAcceptedParams{
			ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true},
			ProviderMessageID: text(result.ProviderMessageID), ProviderState: text(result.ProviderState),
			NextAttemptAt: pgtype.Timestamptz{Time: now.Add(worker.DeliveryCheckAfter), Valid: true},
		})
		return err
	case sendTransitionRetry:
		return worker.recordFailure(ctx, row.ID, lease, "retryable", result.ErrorCode, now.Add(time.Minute))
	case sendTransitionRejected:
		return worker.recordFailure(ctx, row.ID, lease, "rejected", result.ErrorCode, time.Time{})
	case sendTransitionFailed:
		return worker.recordFailure(ctx, row.ID, lease, "failed", result.ErrorCode, time.Time{})
	default:
		if sendErr != nil {
			worker.Logger.Warn("owner SMS send outcome is ambiguous", "error", sendErr)
		}
		return worker.recordFailure(ctx, row.ID, lease, "ambiguous", result.ErrorCode, time.Time{})
	}
}

func (worker Worker) recordFailure(ctx context.Context, id pgtype.UUID, lease googleuuid.UUID, state, providerState string, next time.Time) error {
	params := db.RecordOwnerNotificationFailureParams{ID: id, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, State: state, ProviderState: text(providerState)}
	if !next.IsZero() {
		params.NextAttemptAt = pgtype.Timestamptz{Time: next, Valid: true}
	}
	_, err := worker.Queries.RecordOwnerNotificationFailure(ctx, params)
	return err
}

func (worker Worker) ReconcileOne(ctx context.Context) error {
	worker = worker.defaults()
	if worker.Sender == nil {
		return pgx.ErrNoRows
	}
	lease := googleuuid.New()
	now := worker.Now().UTC()
	row, err := worker.Queries.ClaimNextOwnerNotificationDeliveryCheck(ctx, db.ClaimNextOwnerNotificationDeliveryCheckParams{
		LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, LeaseExpiresAt: pgtype.Timestamptz{Time: now.Add(worker.LeaseDuration), Valid: true}, RequestedDeliveryMode: worker.Mode,
	})
	if err != nil {
		return err
	}
	result, retrieveErr := worker.Sender.Retrieve(ctx, row.ProviderMessageID.String)
	if retrieveErr != nil || result.Disposition == "ambiguous" {
		result.ProviderState = "delivery_check_ambiguous"
	}
	switch classifyDeliveryTransition(result.Disposition, row.DeliveryCheckCount, worker.MaxDeliveryChecks) {
	case deliveryTransitionStop:
		_, err = worker.Queries.StopOwnerNotificationDeliveryChecks(ctx, db.StopOwnerNotificationDeliveryChecksParams{
			ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, ProviderState: text("delivery_unconfirmed_after_bounded_checks"),
		})
		return err
	case deliveryTransitionDelivered:
		_, err = worker.Queries.MarkOwnerNotificationDelivered(ctx, db.MarkOwnerNotificationDeliveredParams{ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, ProviderState: text(result.ProviderState)})
		return err
	case deliveryTransitionFailed:
		_, err = worker.Queries.FailOwnerNotificationDeliveryCheck(ctx, db.FailOwnerNotificationDeliveryCheckParams{
			ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, ProviderState: text(result.ProviderState),
		})
		return err
	case deliveryTransitionUnconfirmed:
		_, err = worker.Queries.StopOwnerNotificationDeliveryChecks(ctx, db.StopOwnerNotificationDeliveryChecksParams{
			ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, ProviderState: text(result.ProviderState),
		})
		return err
	default:
		_, err = worker.Queries.DeferOwnerNotificationDeliveryCheck(ctx, db.DeferOwnerNotificationDeliveryCheckParams{
			ID: row.ID, LeaseToken: pgtype.UUID{Bytes: lease, Valid: true}, ProviderState: text(result.ProviderState),
			NextAttemptAt: pgtype.Timestamptz{Time: now.Add(worker.DeliveryCheckAfter), Valid: true},
		})
		return err
	}
}

func attemptState(disposition string) string {
	if disposition == "accepted" {
		return "provider_accepted"
	}
	if disposition == "rejected" || disposition == "retryable" || disposition == "ambiguous" {
		return disposition
	}
	return "ambiguous"
}

type sendTransition string

const (
	sendTransitionAccepted  sendTransition = "accepted"
	sendTransitionRetry     sendTransition = "retry"
	sendTransitionRejected  sendTransition = "rejected"
	sendTransitionFailed    sendTransition = "failed"
	sendTransitionAmbiguous sendTransition = "ambiguous"
)

func classifySendTransition(disposition string, attemptCount int32) sendTransition {
	switch disposition {
	case "accepted":
		return sendTransitionAccepted
	case "retryable":
		if attemptCount >= 3 {
			return sendTransitionFailed
		}
		return sendTransitionRetry
	case "rejected":
		return sendTransitionRejected
	default:
		return sendTransitionAmbiguous
	}
}

type deliveryTransition string

const (
	deliveryTransitionDelivered   deliveryTransition = "delivered"
	deliveryTransitionFailed      deliveryTransition = "failed"
	deliveryTransitionUnconfirmed deliveryTransition = "unconfirmed"
	deliveryTransitionDefer       deliveryTransition = "defer"
	deliveryTransitionStop        deliveryTransition = "stop"
)

func classifyDeliveryTransition(disposition string, checkCount, maxChecks int32) deliveryTransition {
	switch disposition {
	case "delivered":
		return deliveryTransitionDelivered
	case "failed":
		return deliveryTransitionFailed
	case "unconfirmed":
		return deliveryTransitionUnconfirmed
	default:
		if checkCount >= maxChecks {
			return deliveryTransitionStop
		}
		return deliveryTransitionDefer
	}
}

func text(value string) pgtype.Text { return pgtype.Text{String: value, Valid: value != ""} }
