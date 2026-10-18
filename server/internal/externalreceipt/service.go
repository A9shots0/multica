// Package externalreceipt admits observation-only provider receipts into the
// native workflow ledgers. It never accepts provider text as owner authority,
// production truth, or company-outcome completion.
package externalreceipt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workflowprojection"
	"github.com/multica-ai/multica/server/internal/workflowstore"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

var (
	ErrInvalidInput             = errors.New("invalid external outcome receipt")
	ErrIssueRevisionConflict    = errors.New("issue revision conflict")
	ErrIdempotencyConflict      = errors.New("external receipt idempotency conflict")
	ErrReceiptRevisionConflict  = errors.New("external receipt revision conflict")
	ErrWorkflowScopeUnavailable = errors.New("native workflow scope unavailable")
	ErrRecoveryRequired         = errors.New("external receipt was recorded without a complete admission result")
)

var identityNamespace = googleuuid.MustParse("53807844-8fe5-5e95-9988-75e7c8d466e5")

type Actor struct {
	Type       string
	ID         pgtype.UUID
	Credential publicapiv1.CredentialKind
}

type Request struct {
	WorkspaceID    pgtype.UUID
	IssueID        pgtype.UUID
	IssueRevision  int64
	IdempotencyKey string
	Actor          Actor
	Receipt        publicapiv1.OutcomeReceiptV02
}

type Result struct {
	ReceiptID              string
	EvidenceID             string
	ExecutionID            string
	ExecutionRevision      int64
	CurrentPictureRevision int64
	AuthorityClaimState    string
	DecisionProposalState  string
	Replay                 bool
}

type Service struct {
	Starter      workflowstore.TxStarter
	Queries      *db.Queries
	Now          func() time.Time
	OnProjection func(context.Context, pgtype.UUID, pgtype.UUID, int64)
}

func (s Service) Admit(ctx context.Context, request Request) (Result, error) {
	if s.Starter == nil || s.Queries == nil || !request.WorkspaceID.Valid || !request.IssueID.Valid ||
		!request.Actor.ID.Valid || request.IssueRevision < 1 {
		return Result{}, ErrInvalidInput
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey == "" || len(request.IdempotencyKey) > publicapiv1.MaxIdempotencyBytes ||
		!validActorCredential(request.Actor) {
		return Result{}, ErrInvalidInput
	}
	if err := request.Receipt.Validate(); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	now := s.Now().UTC()
	observedAt, _ := time.Parse(time.RFC3339Nano, request.Receipt.Freshness.ObservedAt)
	if observedAt.After(now.Add(5 * time.Minute)) {
		return Result{}, fmt.Errorf("%w: freshness.observed_at is in the future", ErrInvalidInput)
	}
	if expiresAt := request.Receipt.Freshness.ExpiresAt; expiresAt != nil {
		if absolute, parseErr := time.Parse(time.RFC3339Nano, *expiresAt); parseErr == nil && !absolute.After(now) {
			return Result{}, fmt.Errorf("%w: absolute freshness expiry has elapsed", ErrInvalidInput)
		}
	}
	raw, err := json.Marshal(request.Receipt)
	if err != nil {
		return Result{}, fmt.Errorf("%w: encode receipt: %v", ErrInvalidInput, err)
	}
	sum := sha256.Sum256(raw)
	requestHash := hex.EncodeToString(sum[:])

	var result Result
	err = workflowstore.InIssueWriteTransaction(ctx, s.Starter, s.Queries, request.WorkspaceID, request.IssueID, func(write *workflowstore.IssueWrite) error {
		if lockErr := write.LockExternalWorkReceiptStream(ctx, request.WorkspaceID, request.IssueID, request.Receipt.Provider, request.Receipt.WorkID); lockErr != nil {
			return lockErr
		}
		existing, lookupErr := write.GetExternalWorkReceiptByKey(ctx, request.WorkspaceID, request.IssueID, request.IdempotencyKey)
		if lookupErr == nil {
			if !sameAdmissionRequest(existing, request, requestHash) {
				return ErrIdempotencyConflict
			}
			var resultErr error
			result, resultErr = resultFromRow(existing, true)
			return resultErr
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return lookupErr
		}

		issue, issueErr := write.GetIssueForNotification(ctx, request.WorkspaceID, request.IssueID)
		if issueErr != nil {
			return issueErr
		}
		if issue.Revision != request.IssueRevision {
			return ErrIssueRevisionConflict
		}

		latest, latestErr := write.GetLatestExternalWorkReceipt(ctx, request.WorkspaceID, request.IssueID, request.Receipt.Provider, request.Receipt.WorkID)
		if errors.Is(latestErr, pgx.ErrNoRows) {
			if request.Receipt.ReceiptRevision != 1 {
				return ErrReceiptRevisionConflict
			}
		} else if latestErr != nil {
			return latestErr
		} else if request.Receipt.ReceiptRevision != latest.ReceiptRevision+1 || request.Receipt.SupersedesReceiptRevision == nil ||
			*request.Receipt.SupersedesReceiptRevision != latest.ReceiptRevision {
			// A same-key concurrent retry may have missed the first lookup and
			// observed the winner only through the provider/work revision index.
			// Re-read the idempotency row before classifying this as a revision
			// conflict.
			traced, traceErr := write.GetExternalWorkReceiptByKey(ctx, request.WorkspaceID, request.IssueID, request.IdempotencyKey)
			if traceErr == nil {
				if !sameAdmissionRequest(traced, request, requestHash) {
					return ErrIdempotencyConflict
				}
				var replayErr error
				result, replayErr = resultFromRow(traced, true)
				return replayErr
			}
			if !errors.Is(traceErr, pgx.ErrNoRows) {
				return traceErr
			}
			return ErrReceiptRevisionConflict
		}

		scope, scopeErr := write.GetLatestScope(ctx, request.WorkspaceID, request.IssueID)
		if errors.Is(scopeErr, pgx.ErrNoRows) {
			scope, scopeErr = bootstrapObservationScope(ctx, write, request, issue, now)
		}
		if scopeErr != nil || scope.State != "accepted" || scope.SupersededAt.Valid {
			return ErrWorkflowScopeUnavailable
		}
		authority, authorityErr := write.GetLatestAuthority(ctx, request.WorkspaceID, request.IssueID)
		if authorityErr != nil && !errors.Is(authorityErr, pgx.ErrNoRows) {
			return authorityErr
		}
		authorityState, authorityID, authorityRevision := verifyAuthorityClaim(request.Receipt, scope, authority, authorityErr == nil, now)
		decisionState := decisionProposalState(request.Receipt)

		receiptID := dbid.NewV7()
		executionID := stableID(request.WorkspaceID, request.IssueID, request.Receipt.Provider, request.Receipt.WorkID, "execution")
		evidenceID := stableID(request.WorkspaceID, request.IssueID, request.Receipt.Provider, request.Receipt.WorkID, fmt.Sprintf("evidence/%d", request.Receipt.ReceiptRevision))
		taskID := stableID(request.WorkspaceID, request.IssueID, request.Receipt.Provider, request.Receipt.WorkID, "run/"+request.Receipt.RunID)

		createdReceipt, createErr := write.AdmitExternalWorkReceipt(ctx, db.CreateExternalWorkReceiptParams{
			ID: receiptID, WorkspaceID: request.WorkspaceID, IssueID: request.IssueID,
			IdempotencyKey: request.IdempotencyKey, RequestHash: requestHash,
			Provider: request.Receipt.Provider, WorkID: request.Receipt.WorkID, RunID: request.Receipt.RunID,
			ReceiptRevision: request.Receipt.ReceiptRevision, SupersedesReceiptRevision: nullableInt64(request.Receipt.SupersedesReceiptRevision),
			ActorType: request.Actor.Type, ActorID: request.Actor.ID, CredentialKind: string(request.Actor.Credential),
			IssueRevision: request.IssueRevision, AuthorityClaimState: authorityState,
			DecisionProposalState: decisionState, RawReceipt: raw,
		})
		if createErr != nil {
			if errors.Is(createErr, pgx.ErrNoRows) {
				return ErrIdempotencyConflict
			}
			var pgErr *pgconn.PgError
			if errors.As(createErr, &pgErr) && pgErr.ConstraintName == "uq_issue_external_work_receipt_revision" {
				return ErrReceiptRevisionConflict
			}
			return createErr
		}
		// The admission lock protects workspace/issue deletion but deliberately
		// allows concurrent issue writers. A same-key retry can therefore miss
		// the initial lookup, wait on this unique insert, and receive the winner's
		// now-complete row from ON CONFLICT. Return that durable result instead of
		// attempting a second evidence/execution write.
		if createdReceipt.EvidenceID.Valid || createdReceipt.ExecutionID.Valid || createdReceipt.ExecutionRevision.Valid || createdReceipt.CurrentPictureRevision.Valid {
			var replayErr error
			result, replayErr = resultFromRow(createdReceipt, true)
			return replayErr
		}

		existingExecution, executionErr := write.GetExecution(ctx, db.GetIssueWorkflowExecutionParams{WorkspaceID: request.WorkspaceID, IssueID: request.IssueID, ID: executionID})
		executionRevision := int64(1)
		if executionErr == nil {
			executionRevision = existingExecution.Revision + 1
		} else if !errors.Is(executionErr, pgx.ErrNoRows) {
			return executionErr
		}

		coverage, limitations, contradictions := evidenceMetadata(request.Receipt, authorityState, decisionState)
		evidencePayload, _ := json.Marshal(map[string]any{
			"contract": "outcome_receipt_v0.2", "receipt": request.Receipt,
			"admission": map[string]any{"receipt_id": util.UUIDToString(createdReceipt.ID), "authority_claim_state": authorityState, "decision_proposal_state": decisionState},
		})
		validUntil := parseAbsoluteExpiry(request.Receipt.Freshness.ExpiresAt)
		if _, createErr = write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{
			ID: evidenceID, WorkspaceID: request.WorkspaceID, IssueID: request.IssueID,
			ConcernType: "execution", ConcernID: executionID, ConcernRevision: pgtype.Int8{Int64: executionRevision, Valid: true},
			EvidenceType: "external_outcome_receipt", Level: "observation", Source: "external_outcome_receipt_v0.2",
			ProducerType: "external_provider", ProducerID: request.Actor.ID,
			ObservedAt: pgtype.Timestamptz{Time: observedAt.UTC(), Valid: true}, ValidUntil: validUntil,
			Coverage: coverage, Limitations: limitations, Contradictions: contradictions, Payload: evidencePayload,
		}); createErr != nil {
			return createErr
		}

		checkpoint, expectedEvent, waitReason := executionPayloads(request.Receipt, evidenceID, authorityState, decisionState)
		state, terminalReason := executionState(request.Receipt)
		attemptCount := receiptAttemptCount(request.Receipt.ResourceUsage.Attempts)
		if executionErr == nil {
			updated, updateErr := write.UpdateExecution(ctx, db.UpdateIssueWorkflowExecutionCASParams{
				State: state, AuthorityID: authorityID, AuthorityRevision: authorityRevision,
				ActorType: pgtype.Text{String: "external_provider", Valid: true}, ActorID: request.Actor.ID,
				TaskID: taskID, Checkpoint: checkpoint, ExpectedEvent: expectedEvent, WaitReason: waitReason,
				AttemptCount: attemptCount, TerminalReason: terminalReason,
				WorkspaceID: request.WorkspaceID, IssueID: request.IssueID, ID: executionID, ExpectedRevision: existingExecution.Revision,
			})
			if updateErr != nil {
				return updateErr
			}
			executionRevision = updated.Revision
		} else {
			created, createExecutionErr := write.CreateExecution(ctx, db.CreateIssueWorkflowExecutionParams{
				ID: executionID, WorkspaceID: request.WorkspaceID, IssueID: request.IssueID,
				BranchKey:     "external/" + request.Receipt.Provider + "/" + request.Receipt.WorkID,
				OperationType: "external_observation", State: state, ScopeID: scope.ID, ScopeRevision: scope.Revision,
				AuthorityID: authorityID, AuthorityRevision: authorityRevision,
				ActorType: pgtype.Text{String: "external_provider", Valid: true}, ActorID: request.Actor.ID, TaskID: taskID,
				Checkpoint: checkpoint, ExpectedEvent: expectedEvent, WaitReason: waitReason, AttemptCount: attemptCount,
			})
			if createExecutionErr != nil {
				return createExecutionErr
			}
			executionRevision = created.Revision
		}

		projectionExpected, projectionRevision := int64(0), int64(1)
		current, currentErr := write.GetCurrentPicture(ctx, db.GetIssueCurrentPictureParams{WorkspaceID: request.WorkspaceID, IssueID: request.IssueID})
		if currentErr == nil {
			projectionExpected, projectionRevision = current.Revision, current.Revision+1
		} else if !errors.Is(currentErr, pgx.ErrNoRows) {
			return currentErr
		}
		if _, projectionErr := workflowprojection.ProjectInWrite(ctx, write, request.WorkspaceID, request.IssueID, projectionExpected, projectionRevision, now); projectionErr != nil {
			return projectionErr
		}
		stored, storeErr := write.SetExternalWorkReceiptResult(ctx, db.SetExternalWorkReceiptAdmissionResultParams{
			EvidenceID: evidenceID, ExecutionID: executionID,
			ExecutionRevision:      pgtype.Int8{Int64: executionRevision, Valid: true},
			CurrentPictureRevision: pgtype.Int8{Int64: projectionRevision, Valid: true},
			WorkspaceID:            request.WorkspaceID, IssueID: request.IssueID, ID: createdReceipt.ID,
		})
		if storeErr != nil {
			return storeErr
		}
		result, storeErr = resultFromRow(stored, false)
		return storeErr
	})
	if err != nil {
		return Result{}, err
	}
	if !result.Replay && s.OnProjection != nil {
		s.OnProjection(ctx, request.WorkspaceID, request.IssueID, result.CurrentPictureRevision)
	}
	return result, nil
}

// bootstrapObservationScope turns an owner's ordinary issue creation into the
// narrow native scope needed to record outside progress. It creates no effect
// authority and cannot complete the issue, approve a decision, or authorize a
// release. Agent-created issues still require an explicit native owner scope.
func bootstrapObservationScope(ctx context.Context, write *workflowstore.IssueWrite, request Request, issue db.Issue, now time.Time) (db.IssueWorkflowScope, error) {
	if issue.CreatorType != "member" || !issue.CreatorID.Valid {
		return db.IssueWorkflowScope{}, ErrWorkflowScopeUnavailable
	}
	// Different provider/work streams can reach an uninitialized issue at the
	// same time. Serialize only this one-time scaffold, then re-read after the
	// winning transaction commits.
	if err := write.LockExternalWorkReceiptStream(ctx, request.WorkspaceID, request.IssueID, "multica-native", "external-observation-scope"); err != nil {
		return db.IssueWorkflowScope{}, err
	}
	if existing, err := write.GetLatestScope(ctx, request.WorkspaceID, request.IssueID); err == nil {
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.IssueWorkflowScope{}, err
	}

	scopeID := stableID(request.WorkspaceID, request.IssueID, "native", "external-observation-scope")
	ownerEvidenceID := stableID(request.WorkspaceID, request.IssueID, "native", "owner-created-issue")
	observedAt := now
	if issue.CreatedAt.Valid {
		observedAt = issue.CreatedAt.Time.UTC()
	}
	ownerPayload, _ := json.Marshal(map[string]any{
		"source": "owner_created_issue", "issue_id": util.UUIDToString(issue.ID), "issue_revision": issue.Revision,
		"title": issue.Title, "description": issue.Description.String,
		"meaning": "The owner created this task; outside progress may be recorded as observation only.",
	})
	if _, err := write.CreateEvidence(ctx, db.CreateIssueWorkflowEvidenceParams{
		ID: ownerEvidenceID, WorkspaceID: request.WorkspaceID, IssueID: request.IssueID,
		ConcernType: "scope", ConcernID: scopeID, ConcernRevision: pgtype.Int8{Int64: 1, Valid: true},
		EvidenceType: "owner_request", Level: "owner", Source: "owner_created_issue",
		ProducerType: "owner", ProducerID: issue.CreatorID,
		ObservedAt: pgtype.Timestamptz{Time: observedAt, Valid: true},
		Coverage:   []byte(`{"external_progress_observation":true}`),
		Limitations: []byte(`[
			"does not grant production, decision, completion, or business acceptance authority"
		]`),
		Contradictions: []byte(`[]`), Payload: ownerPayload,
	}); err != nil {
		return db.IssueWorkflowScope{}, err
	}
	statement, _ := json.Marshal(map[string]any{
		"source": "owner_created_issue", "issue_revision": issue.Revision,
		"boundary": "Record outside progress for this issue as observation only.",
	})
	impact, _ := json.Marshal(map[string]any{
		"allows":  []string{"record_external_progress", "show_external_progress_in_inbox_taskboard_and_history"},
		"forbids": []string{"complete_company_outcome", "approve_owner_decision", "grant_effect_authority", "authorize_release"},
	})
	return write.CreateScope(ctx, db.CreateIssueWorkflowScopeParams{
		ID: scopeID, WorkspaceID: request.WorkspaceID, IssueID: request.IssueID,
		Revision: 1, AcceptanceVersion: 1, State: "accepted",
		OwnerEvidenceID: ownerEvidenceID, CreatedBy: issue.CreatorID,
		Statement: statement, Impact: impact,
	})
}

func sameAdmissionRequest(row db.IssueExternalWorkReceipt, request Request, requestHash string) bool {
	return row.RequestHash == requestHash && row.ActorType == request.Actor.Type && row.ActorID == request.Actor.ID &&
		row.CredentialKind == string(request.Actor.Credential) && row.IssueRevision == request.IssueRevision
}

func resultFromRow(row db.IssueExternalWorkReceipt, replay bool) (Result, error) {
	if !row.EvidenceID.Valid || !row.ExecutionID.Valid || !row.ExecutionRevision.Valid || !row.CurrentPictureRevision.Valid {
		return Result{}, ErrRecoveryRequired
	}
	return Result{
		ReceiptID: util.UUIDToString(row.ID), EvidenceID: util.UUIDToString(row.EvidenceID), ExecutionID: util.UUIDToString(row.ExecutionID),
		ExecutionRevision: row.ExecutionRevision.Int64, CurrentPictureRevision: row.CurrentPictureRevision.Int64,
		AuthorityClaimState: row.AuthorityClaimState, DecisionProposalState: row.DecisionProposalState, Replay: replay,
	}, nil
}

func executionState(receipt publicapiv1.OutcomeReceiptV02) (string, string) {
	if len(receipt.Dependencies) != 0 {
		return "blocked_external", ""
	}
	switch receipt.State {
	case "working":
		return "working", ""
	case "waiting_external", "waiting_owner", "blocked":
		return "blocked_external", ""
	case "complete":
		// This closes only the observed provider run. It never creates or updates
		// issue_workflow_outcome, release, health, or acceptance authority.
		return "completed", ""
	case "stopped":
		return "technically_stopped", "external provider reported that work stopped"
	default:
		return "received", ""
	}
}

func executionPayloads(receipt publicapiv1.OutcomeReceiptV02, evidenceID pgtype.UUID, authorityState, decisionState string) ([]byte, []byte, []byte) {
	checkpoint, _ := json.Marshal(map[string]any{
		"contract": "external_work_observation_v0.2", "provider": receipt.Provider, "work_id": receipt.WorkID,
		"run_id": receipt.RunID, "receipt_revision": receipt.ReceiptRevision, "reported_state": receipt.State, "objective": receipt.Objective,
		"current_milestone": receipt.CurrentMilestone, "change_since_last_receipt": receipt.ChangeSinceLastReceipt,
		"evidence": receipt.Evidence, "dependencies": receipt.Dependencies,
		"owner_decision_proposal": receipt.OwnerDecision,
		"next_action":             receipt.NextAction, "risk": receipt.Risk, "resource_usage": receipt.ResourceUsage,
		"effect_report": receipt.Effect, "acceptance_report": receipt.Acceptance,
		"evidence_id": util.UUIDToString(evidenceID), "authority_claim_state": authorityState,
		"decision_proposal_state": decisionState,
	})
	expected := map[string]any{}
	if receipt.State != "complete" && receipt.State != "stopped" {
		expected = map[string]any{"type": "provider_readback", "provider": receipt.Provider, "work_id": receipt.WorkID, "after_receipt_revision": receipt.ReceiptRevision}
	}
	expectedEvent, _ := json.Marshal(expected)
	wait := map[string]any{}
	if receipt.State == "waiting_external" || receipt.State == "waiting_owner" || receipt.State == "blocked" || len(receipt.Dependencies) != 0 {
		wait = map[string]any{"code": "external_receipt", "reported_state": receipt.State, "dependencies": receipt.Dependencies}
		if receipt.OwnerDecision.Required {
			wait["owner_decision_proposal"] = receipt.OwnerDecision
			wait["owner_decision_actionable"] = false
		}
	}
	waitReason, _ := json.Marshal(wait)
	return checkpoint, expectedEvent, waitReason
}

func evidenceMetadata(receipt publicapiv1.OutcomeReceiptV02, authorityState, decisionState string) ([]byte, []byte, []byte) {
	coverage, _ := json.Marshal(map[string]any{"receipt_fields": []string{"objective", "milestone", "change", "evidence", "dependencies", "next_action", "usage", "effect", "acceptance"}})
	limitations := []string{"provider-authored receipt is observation evidence, not company acceptance or production authority"}
	contradictions := []string{}
	if receipt.Freshness.ExpiresAt != nil {
		if _, err := time.Parse(time.RFC3339Nano, *receipt.Freshness.ExpiresAt); err != nil {
			limitations = append(limitations, "symbolic expiry requires dependency invalidation; it is not a wall-clock validity grant")
		}
	}
	if authorityState == "unverified" {
		contradictions = append(contradictions, "provider authority claim did not match current native authority")
	}
	if decisionState == "recorded_non_actionable" {
		limitations = append(limitations, "owner decision remains a non-actionable proposal without native premise and owner validation")
	}
	limitationsJSON, _ := json.Marshal(limitations)
	contradictionsJSON, _ := json.Marshal(contradictions)
	return coverage, limitationsJSON, contradictionsJSON
}

func verifyAuthorityClaim(receipt publicapiv1.OutcomeReceiptV02, scope db.IssueWorkflowScope, authority db.IssueWorkflowAuthority, found bool, now time.Time) (string, pgtype.UUID, pgtype.Int8) {
	needsEffectAuthority := receipt.RequestedMode == "change" || receipt.RequestedMode == "deploy" || receipt.Effect.ProductionChanged
	if !needsEffectAuthority {
		return "not_required_for_observation", pgtype.UUID{}, pgtype.Int8{}
	}
	if !found || authority.State != "active" || authority.ScopeID != scope.ID || authority.ScopeRevision != scope.Revision ||
		authority.RevokedAt.Valid || authority.SupersededAt.Valid || authority.ValidUntil.Valid && !authority.ValidUntil.Time.After(now) {
		return "unverified", pgtype.UUID{}, pgtype.Int8{}
	}
	var envelope struct {
		Reference      string   `json:"reference"`
		AllowedEffects []string `json:"allowed_effects"`
	}
	if json.Unmarshal(authority.Envelope, &envelope) != nil || strings.TrimSpace(envelope.Reference) != strings.TrimSpace(receipt.Authority.Reference) ||
		!containsAll(envelope.AllowedEffects, receipt.Authority.AllowedEffects) {
		return "unverified", pgtype.UUID{}, pgtype.Int8{}
	}
	return "matched_native_authority", authority.ID, pgtype.Int8{Int64: authority.Revision, Valid: true}
}

func decisionProposalState(receipt publicapiv1.OutcomeReceiptV02) string {
	if !receipt.OwnerDecision.Required {
		return "none"
	}
	return "recorded_non_actionable"
}

func stableID(workspaceID, issueID pgtype.UUID, parts ...string) pgtype.UUID {
	material := util.UUIDToString(workspaceID) + "/" + util.UUIDToString(issueID) + "/" + strings.Join(parts, "/")
	id := googleuuid.NewSHA1(identityNamespace, []byte(material))
	return pgtype.UUID{Bytes: id, Valid: true}
}

func nullableInt64(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func parseAbsoluteExpiry(value *string) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: parsed.UTC(), Valid: true}
}

func receiptAttemptCount(value *int64) int32 {
	if value == nil || *value <= 0 {
		return 0
	}
	if *value > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(*value)
}

func containsAll(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, value := range have {
		set[strings.TrimSpace(value)] = true
	}
	for _, value := range want {
		if !set[strings.TrimSpace(value)] {
			return false
		}
	}
	return true
}

func validCredential(value publicapiv1.CredentialKind) bool {
	switch value {
	case publicapiv1.CredentialUserOAuth, publicapiv1.CredentialPersonalAccess,
		publicapiv1.CredentialPluginInstall, publicapiv1.CredentialPluginInvocation:
		return true
	default:
		return false
	}
}

func validActorCredential(actor Actor) bool {
	if !validCredential(actor.Credential) {
		return false
	}
	switch actor.Type {
	case "member":
		// A UI/manual plugin callback is authenticated by a short-lived
		// invocation credential but deliberately acts as the member captured in
		// its signed grant. Preserve both facts in the audit row.
		return actor.Credential == publicapiv1.CredentialUserOAuth || actor.Credential == publicapiv1.CredentialPersonalAccess ||
			actor.Credential == publicapiv1.CredentialPluginInvocation
	case "plugin":
		return actor.Credential == publicapiv1.CredentialPluginInstall || actor.Credential == publicapiv1.CredentialPluginInvocation
	default:
		return false
	}
}
