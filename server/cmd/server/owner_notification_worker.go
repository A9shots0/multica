package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/ownernotification"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func ownerNotificationWorkerFromEnv(queries *db.Queries) (*ownernotification.Worker, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MULTICA_OWNER_SMS_MODE")))
	if mode == "" || mode == "off" {
		return nil, nil
	}
	if mode != "dry_run" && mode != "live_test" && mode != "live" {
		return nil, fmt.Errorf("MULTICA_OWNER_SMS_MODE must be off, dry_run, live_test, or live")
	}
	if mode == "live_test" && !envBool("MULTICA_OWNER_SMS_LIVE_TEST_ENABLED", false) {
		return nil, fmt.Errorf("live_test mode requires MULTICA_OWNER_SMS_LIVE_TEST_ENABLED=true")
	}
	if mode == "live" && !envBool("MULTICA_OWNER_SMS_LIVE_ENABLED", false) {
		return nil, fmt.Errorf("live mode requires MULTICA_OWNER_SMS_LIVE_ENABLED=true")
	}
	if mode != "dry_run" && !envBool("MULTICA_OWNER_SMS_DESTINATION_VERIFIED", false) {
		return nil, fmt.Errorf("live owner SMS requires MULTICA_OWNER_SMS_DESTINATION_VERIFIED=true")
	}
	destinationKey := strings.TrimSpace(os.Getenv("MULTICA_OWNER_SMS_DESTINATION_KEY"))
	if destinationKey == "" {
		destinationKey = "owner_primary"
	}
	worker := &ownernotification.Worker{
		Queries: queries, Mode: mode, DestinationKey: destinationKey,
		PollInterval: 2 * time.Second, LeaseDuration: 30 * time.Second,
		DeliveryCheckAfter: 5 * time.Second,
	}
	destination := strings.TrimSpace(os.Getenv("MULTICA_OWNER_SMS_DESTINATION"))
	if destination == "" {
		return nil, fmt.Errorf("enabled owner SMS requires MULTICA_OWNER_SMS_DESTINATION")
	}
	worker.Destination = destination
	if mode == "dry_run" {
		return worker, nil
	}
	apiKey := strings.TrimSpace(os.Getenv("TELNYX_API_KEY"))
	from := strings.TrimSpace(os.Getenv("TELNYX_PHONE_NUMBER"))
	if destination == "" || apiKey == "" || from == "" {
		return nil, fmt.Errorf("enabled owner SMS requires destination, TELNYX_API_KEY, and TELNYX_PHONE_NUMBER")
	}
	worker.Sender = ownernotification.TelnyxSender{
		APIKey: apiKey, FromNumber: from,
		MessagingProfileID: strings.TrimSpace(os.Getenv("TELNYX_MESSAGING_PROFILE_ID")),
		Client:             &http.Client{Timeout: 15 * time.Second},
	}
	return worker, nil
}

func configureOwnerNotificationBinding(ctx context.Context, queries *db.Queries, worker *ownernotification.Worker) error {
	if worker == nil {
		return nil
	}
	workspaceSlug := strings.TrimSpace(os.Getenv("MULTICA_OWNER_SMS_WORKSPACE_SLUG"))
	recipientEmail := strings.ToLower(strings.TrimSpace(os.Getenv("MULTICA_OWNER_SMS_RECIPIENT_EMAIL")))
	if workspaceSlug == "" || recipientEmail == "" {
		return fmt.Errorf("enabled owner SMS requires workspace slug and recipient email")
	}
	workspace, err := queries.GetWorkspaceBySlug(ctx, workspaceSlug)
	if err != nil {
		return fmt.Errorf("resolve owner SMS workspace: %w", err)
	}
	user, err := queries.GetUserByEmail(ctx, recipientEmail)
	if err != nil {
		return fmt.Errorf("resolve owner SMS recipient: %w", err)
	}
	member, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: user.ID, WorkspaceID: workspace.ID})
	if err != nil {
		return fmt.Errorf("resolve owner SMS membership: %w", err)
	}
	if !ownerNotificationRecipientRoleAllowed(member.Role) {
		return fmt.Errorf("owner SMS recipient must be the workspace owner")
	}
	_, err = queries.UpsertOwnerNotificationBinding(ctx, db.UpsertOwnerNotificationBindingParams{
		WorkspaceID: workspace.ID, RecipientID: user.ID,
		DestinationKey: worker.DestinationKey, DestinationFingerprint: ownerNotificationDestinationFingerprint(worker.Destination),
		DeliveryMode: worker.Mode, Enabled: true,
	})
	if err != nil {
		return fmt.Errorf("persist owner SMS binding: %w", err)
	}
	return nil
}

func ownerNotificationRecipientRoleAllowed(role string) bool {
	return role == "owner"
}

func ownerNotificationDestinationFingerprint(destination string) string {
	normalized := strings.TrimSpace(destination)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
