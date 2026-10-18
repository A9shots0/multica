package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/entitlement"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/issueposition"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const externalIncidentLeaseDuration = 30 * time.Second

var (
	ErrExternalIncidentSourceNotFound  = errors.New("external incident source binding not found")
	ErrExternalIncidentSourceDisabled  = errors.New("external incident source binding disabled")
	ErrExternalIncidentCredential      = errors.New("external incident source credential mismatch")
	ErrExternalIncidentCreatorAgent    = errors.New("external incident creator agent not found")
	ErrExternalIncidentConflict        = errors.New("external incident event key conflict")
	ErrExternalIncidentProcessing      = errors.New("external incident event is already processing")
	ErrExternalIncidentBindingRevision = errors.New("external incident source binding revision conflict")
)

// ExternalIncidentRejectedError is a deterministic lifecycle rejection. The
// occurrence receipt is retained with Code so retries return the same result.
type ExternalIncidentRejectedError struct {
	Code string
}

func (e *ExternalIncidentRejectedError) Error() string {
	return "external incident rejected: " + e.Code
}

type ExternalIncidentInput struct {
	EventKey            string    `json:"event_key"`
	FindingKey          string    `json:"finding_key"`
	IncidentKey         string    `json:"incident_key"`
	SourceSequence      int64     `json:"source_sequence"`
	PredecessorEventKey string    `json:"predecessor_event_key,omitempty"`
	EpisodeGeneration   int64     `json:"episode_generation"`
	Transition          string    `json:"transition"`
	Severity            string    `json:"severity"`
	ActionRequired      *bool     `json:"action_required"`
	Summary             string    `json:"summary"`
	NextStep            string    `json:"next_step"`
	ObservedAt          time.Time `json:"observed_at"`
	EvidenceRefs        []string  `json:"evidence_refs,omitempty"`
}

type ExternalIncidentResult struct {
	EventKey          string `json:"event_key"`
	FindingKey        string `json:"finding_key"`
	EpisodeGeneration int64  `json:"episode_generation"`
	Transition        string `json:"transition"`
	IssueID           string `json:"issue_id"`
	IssueNumber       int32  `json:"issue_number"`
	EpisodeRevision   int64  `json:"episode_revision"`
	Replayed          bool   `json:"replayed"`
	ReceiptState      string `json:"receipt_state"`
}

type ExternalIncidentSourceConfig struct {
	ProjectID        pgtype.UUID
	CreatorAgentID   pgtype.UUID
	Enabled          bool
	RotateCredential bool
	ExpectedRevision *int64
}

type ExternalIncidentSourceConfigResult struct {
	Binding    db.ExternalIncidentSourceBinding
	Credential string
}

type ExternalIncidentService struct {
	Queries      *db.Queries
	TxStarter    TxStarter
	Bus          *events.Bus
	IssueService *IssueService
	Now          func() time.Time
}

type externalIncidentApplyOutcome struct {
	Result   ExternalIncidentResult
	Issue    db.Issue
	Previous db.Issue
	ActorID  pgtype.UUID
	Created  bool
	Pending  bool
}

func NewExternalIncidentService(q *db.Queries, tx TxStarter, bus *events.Bus, issues *IssueService) *ExternalIncidentService {
	return &ExternalIncidentService{Queries: q, TxStarter: tx, Bus: bus, IssueService: issues, Now: time.Now}
}

// ConfigureSource binds one machine source to an existing Multica project and
// agent identity. The raw credential is returned only when the binding is first
// created or explicitly rotated; only its SHA-256 digest is retained.
func (s *ExternalIncidentService) ConfigureSource(ctx context.Context, workspaceID pgtype.UUID, sourceKey string, config ExternalIncidentSourceConfig) (ExternalIncidentSourceConfigResult, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("begin external incident source configuration: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	lockKey := fmt.Sprintf("external-incident-source:%s:%s", util.UUIDToString(workspaceID), sourceKey)
	if err := qtx.LockExternalIncidentSourceKey(ctx, lockKey); err != nil {
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("lock external incident source binding: %w", err)
	}
	if _, err := qtx.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: config.ProjectID, WorkspaceID: workspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalIncidentSourceConfigResult{}, ErrProjectNotFound
		}
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("verify external incident project: %w", err)
	}
	creatorAgent, err := qtx.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: config.CreatorAgentID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExternalIncidentSourceConfigResult{}, ErrExternalIncidentCreatorAgent
		}
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("verify external incident creator agent: %w", err)
	}
	if creatorAgent.ArchivedAt.Valid {
		return ExternalIncidentSourceConfigResult{}, ErrExternalIncidentCreatorAgent
	}

	credentialHash, credentialHint, rawCredential := "", "", ""
	existing, err := qtx.GetExternalIncidentSourceBinding(ctx, db.GetExternalIncidentSourceBindingParams{WorkspaceID: workspaceID, SourceKey: sourceKey})
	switch {
	case err == nil:
		changed := existing.ProjectID != config.ProjectID || existing.CreatorAgentID != config.CreatorAgentID || existing.Enabled != config.Enabled || config.RotateCredential
		if !changed {
			if err := tx.Commit(ctx); err != nil {
				return ExternalIncidentSourceConfigResult{}, fmt.Errorf("commit unchanged external incident source binding: %w", err)
			}
			return ExternalIncidentSourceConfigResult{Binding: existing}, nil
		}
		if changed && (config.ExpectedRevision == nil || *config.ExpectedRevision != existing.Revision) {
			return ExternalIncidentSourceConfigResult{}, ErrExternalIncidentBindingRevision
		}
		credentialHash, credentialHint = existing.CredentialHash, existing.CredentialHint
		if config.RotateCredential {
			rawCredential, credentialHash, credentialHint, err = newExternalIncidentCredential()
			if err != nil {
				return ExternalIncidentSourceConfigResult{}, err
			}
		}
	case errors.Is(err, pgx.ErrNoRows):
		if config.ExpectedRevision != nil && *config.ExpectedRevision != 0 {
			return ExternalIncidentSourceConfigResult{}, ErrExternalIncidentBindingRevision
		}
		rawCredential, credentialHash, credentialHint, err = newExternalIncidentCredential()
		if err != nil {
			return ExternalIncidentSourceConfigResult{}, err
		}
	default:
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("load external incident source binding: %w", err)
	}

	binding, err := qtx.UpsertExternalIncidentSourceBinding(ctx, db.UpsertExternalIncidentSourceBindingParams{
		WorkspaceID: workspaceID, SourceKey: sourceKey, CreatorAgentID: config.CreatorAgentID,
		ProjectID: config.ProjectID, CredentialHash: credentialHash, CredentialHint: credentialHint,
		Enabled: config.Enabled,
	})
	if err != nil {
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("save external incident source binding: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ExternalIncidentSourceConfigResult{}, fmt.Errorf("commit external incident source binding: %w", err)
	}
	return ExternalIncidentSourceConfigResult{Binding: binding, Credential: rawCredential}, nil
}

// AuthenticateSource resolves an external incident machine credential without
// entering Multica's human session/PAT middleware.
func (s *ExternalIncidentService) AuthenticateSource(ctx context.Context, workspaceID pgtype.UUID, sourceKey, credential string) (db.ExternalIncidentSourceBinding, error) {
	if !strings.HasPrefix(credential, "mei_") {
		return db.ExternalIncidentSourceBinding{}, ErrExternalIncidentCredential
	}
	binding, err := s.Queries.GetExternalIncidentSourceBinding(ctx, db.GetExternalIncidentSourceBindingParams{WorkspaceID: workspaceID, SourceKey: sourceKey})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ExternalIncidentSourceBinding{}, ErrExternalIncidentCredential
		}
		return db.ExternalIncidentSourceBinding{}, fmt.Errorf("load external incident source binding: %w", err)
	}
	got := externalIncidentCredentialHash(credential)
	if subtle.ConstantTimeCompare([]byte(got), []byte(binding.CredentialHash)) != 1 {
		return db.ExternalIncidentSourceBinding{}, ErrExternalIncidentCredential
	}
	if !binding.Enabled {
		return db.ExternalIncidentSourceBinding{}, ErrExternalIncidentSourceDisabled
	}
	return binding, nil
}

func newExternalIncidentCredential() (raw, hash, hint string, err error) {
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", fmt.Errorf("generate external incident credential: %w", err)
	}
	raw = "mei_" + hex.EncodeToString(secret)
	return raw, externalIncidentCredentialHash(raw), "mei_..." + raw[len(raw)-6:], nil
}

func externalIncidentCredentialHash(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func (s *ExternalIncidentService) Ingest(ctx context.Context, binding db.ExternalIncidentSourceBinding, in ExternalIncidentInput) (ExternalIncidentResult, error) {
	return s.ingest(ctx, binding, in)
}

func (s *ExternalIncidentService) ingest(ctx context.Context, binding db.ExternalIncidentSourceBinding, in ExternalIncidentInput) (ExternalIncidentResult, error) {
	if in.ActionRequired == nil {
		return ExternalIncidentResult{}, errors.New("external incident action_required is missing")
	}
	workspaceID, sourceKey := binding.WorkspaceID, binding.SourceKey
	payload, err := json.Marshal(in)
	if err != nil {
		return ExternalIncidentResult{}, fmt.Errorf("marshal external incident: %w", err)
	}
	digest := sha256.Sum256(payload)
	requestHash := hex.EncodeToString(digest[:])

	receipt, err := s.Queries.AdmitExternalIncidentOccurrence(ctx, db.AdmitExternalIncidentOccurrenceParams{
		WorkspaceID: workspaceID, SourceKey: sourceKey, EventKey: in.EventKey,
		RequestHash: requestHash, FindingKey: in.FindingKey, IncidentKey: in.IncidentKey,
		SourceSequence: in.SourceSequence, PredecessorEventKey: util.StrToText(in.PredecessorEventKey), EpisodeGeneration: in.EpisodeGeneration,
		LifecycleState: in.Transition, Severity: in.Severity, ActionRequired: *in.ActionRequired, Summary: in.Summary, NextStep: in.NextStep,
		ObservedAt: pgtype.Timestamptz{Time: in.ObservedAt.UTC(), Valid: true}, Payload: payload,
		BindingRevision: binding.Revision, CreatorAgentID: binding.CreatorAgentID, ProjectID: binding.ProjectID,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return ExternalIncidentResult{}, fmt.Errorf("admit external incident occurrence: %w", err)
		}
		stored, loadErr := s.Queries.GetExternalIncidentOccurrence(ctx, db.GetExternalIncidentOccurrenceParams{WorkspaceID: workspaceID, SourceKey: sourceKey, EventKey: in.EventKey})
		if loadErr != nil {
			return ExternalIncidentResult{}, fmt.Errorf("load conflicting external incident occurrence: %w", loadErr)
		}
		if stored.RequestHash == requestHash {
			receipt = stored
		} else {
			_, quarantineErr := s.Queries.QuarantineExternalIncidentConflict(ctx, db.QuarantineExternalIncidentConflictParams{
				ID: dbid.NewV7(), WorkspaceID: workspaceID, SourceKey: sourceKey, EventKey: in.EventKey,
				StoredRequestHash: stored.RequestHash, ReceivedRequestHash: requestHash,
				ReasonCode: "event_key_payload_changed", ReceivedPayload: payload,
			})
			if quarantineErr != nil {
				return ExternalIncidentResult{}, fmt.Errorf("quarantine external incident conflict: %w", quarantineErr)
			}
			return ExternalIncidentResult{}, ErrExternalIncidentConflict
		}
	}

	if receipt.RequestHash != requestHash {
		return ExternalIncidentResult{}, ErrExternalIncidentConflict
	}
	if receipt.State == "applied" {
		return decodeExternalIncidentResult(receipt.Result, true)
	}
	if receipt.State == "rejected" {
		return ExternalIncidentResult{}, &ExternalIncidentRejectedError{Code: receipt.ErrorCode.String}
	}

	now := s.now().UTC()
	leaseToken := dbid.NewV7()
	claimed, err := s.Queries.ClaimExternalIncidentOccurrence(ctx, db.ClaimExternalIncidentOccurrenceParams{
		LeaseToken: leaseToken, LeaseExpiresAt: pgtype.Timestamptz{Time: now.Add(externalIncidentLeaseDuration), Valid: true},
		WorkspaceID: workspaceID, SourceKey: sourceKey, EventKey: in.EventKey, RequestHash: requestHash,
		Now: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			latest, loadErr := s.Queries.GetExternalIncidentOccurrence(ctx, db.GetExternalIncidentOccurrenceParams{WorkspaceID: workspaceID, SourceKey: sourceKey, EventKey: in.EventKey})
			if loadErr == nil && latest.State == "applied" {
				return decodeExternalIncidentResult(latest.Result, true)
			}
			if loadErr == nil && latest.State == "rejected" {
				return ExternalIncidentResult{}, &ExternalIncidentRejectedError{Code: latest.ErrorCode.String}
			}
			return ExternalIncidentResult{}, ErrExternalIncidentProcessing
		}
		return ExternalIncidentResult{}, fmt.Errorf("claim external incident occurrence: %w", err)
	}

	outcome, err := s.applyClaimed(ctx, claimed, in, leaseToken)
	if err != nil {
		var rejected *ExternalIncidentRejectedError
		if errors.As(err, &rejected) {
			return ExternalIncidentResult{}, err
		}
		_, _ = s.Queries.MarkExternalIncidentOccurrenceFailed(ctx, db.MarkExternalIncidentOccurrenceFailedParams{
			ErrorCode: pgtype.Text{String: "processing_error", Valid: true}, WorkspaceID: workspaceID,
			SourceKey: sourceKey, EventKey: in.EventKey, RequestHash: requestHash, LeaseToken: leaseToken,
		})
		return ExternalIncidentResult{}, err
	}

	if outcome.Pending {
		return outcome.Result, nil
	}
	if s.Bus != nil {
		eventType := protocol.EventIssueUpdated
		if outcome.Created {
			eventType = protocol.EventIssueCreated
		}
		payload := s.externalIncidentRealtimePayload(ctx, outcome)
		s.Bus.Publish(events.Event{
			Type: eventType, WorkspaceID: util.UUIDToString(workspaceID), ActorType: "agent", ActorID: util.UUIDToString(outcome.ActorID),
			Payload: payload,
		})
	}
	return outcome.Result, nil
}

func (s *ExternalIncidentService) applyClaimed(ctx context.Context, receipt db.ExternalIncidentOccurrence, in ExternalIncidentInput, leaseToken pgtype.UUID) (externalIncidentApplyOutcome, error) {
	var issueServiceEntitlements entitlement.Provider
	if s.IssueService != nil {
		issueServiceEntitlements = s.IssueService.Entitlements
	}
	policy := ResolveIssueCountPolicy(ctx, issueServiceEntitlements, receipt.WorkspaceID)
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("begin external incident apply: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	lockKey := fmt.Sprintf("external-incident:%s:%s:%s", util.UUIDToString(receipt.WorkspaceID), receipt.SourceKey, receipt.FindingKey)
	if err := qtx.LockExternalIncidentEpisodeKey(ctx, lockKey); err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("lock external incident episode: %w", err)
	}

	orderingCode, pending, err := validateExternalIncidentOrdering(ctx, qtx, receipt)
	if err != nil {
		return externalIncidentApplyOutcome{}, err
	}
	if pending {
		result := pendingExternalIncidentResult(receipt)
		resultJSON, _ := json.Marshal(result)
		if _, err := qtx.MarkExternalIncidentOccurrencePending(ctx, db.MarkExternalIncidentOccurrencePendingParams{
			Result: resultJSON, WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey,
			EventKey: receipt.EventKey, RequestHash: receipt.RequestHash, LeaseToken: leaseToken,
		}); err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("record pending external incident predecessor: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("commit pending external incident predecessor: %w", err)
		}
		return externalIncidentApplyOutcome{Result: result, Pending: true}, nil
	}

	episode, exactErr := qtx.GetExternalIncidentEpisode(ctx, db.GetExternalIncidentEpisodeParams{
		WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, FindingKey: receipt.FindingKey, Generation: receipt.EpisodeGeneration,
	})
	hasEpisode := exactErr == nil
	if exactErr != nil && !errors.Is(exactErr, pgx.ErrNoRows) {
		return externalIncidentApplyOutcome{}, fmt.Errorf("load external incident episode: %w", exactErr)
	}
	code, validationErr := validateExternalIncidentTransition(ctx, qtx, receipt, episode, hasEpisode)
	if validationErr != nil {
		return externalIncidentApplyOutcome{}, validationErr
	}
	if code == "" {
		code = orderingCode
	}
	if code == "" && !hasEpisode {
		if policyErr := validateIssueCreatorForCreation(ctx, qtx, receipt.WorkspaceID, "agent"); policyErr != nil {
			if errors.Is(policyErr, ErrAgentIssueCreationDisabled) {
				code = "agent_issue_creation_disabled"
			} else {
				return externalIncidentApplyOutcome{}, policyErr
			}
		}
	}
	if code != "" {
		result := rejectedExternalIncidentResult(receipt, code)
		if _, markErr := qtx.MarkExternalIncidentOccurrenceRejected(ctx, db.MarkExternalIncidentOccurrenceRejectedParams{
			ErrorCode: pgtype.Text{String: code, Valid: true}, Result: result,
			WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, EventKey: receipt.EventKey,
			RequestHash: receipt.RequestHash, LeaseToken: leaseToken,
		}); markErr != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("record external incident rejection: %w", markErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("commit external incident rejection: %w", err)
		}
		return externalIncidentApplyOutcome{}, &ExternalIncidentRejectedError{Code: code}
	}

	episodeID := deterministicExternalIncidentEpisodeID(receipt.WorkspaceID, receipt.SourceKey, receipt.FindingKey, receipt.EpisodeGeneration)
	issueID := pgtype.UUID{Bytes: googleuuid.NewSHA1(googleuuid.UUID(episodeID.Bytes), []byte("issue")), Valid: true}
	created := false
	var issue, previousIssue db.Issue
	machineTitle := receipt.Summary
	machineDescription := externalIncidentDescription(receipt)
	machineStatus := externalIncidentIssueStatus(receipt.LifecycleState)
	machinePriority := externalIncidentIssuePriority(receipt.Severity)
	titleHumanOwned, descriptionHumanOwned, statusHumanOwned, priorityHumanOwned := false, false, false, false
	if hasEpisode {
		issueID = episode.IssueID
		issue, err = qtx.GetExternalIncidentIssueForUpdate(ctx, db.GetExternalIncidentIssueForUpdateParams{
			ID: issueID, WorkspaceID: receipt.WorkspaceID, OriginID: episodeID,
		})
		if err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("load external incident issue: %w", err)
		}
		previousIssue = issue
		titleHumanOwned = episode.TitleHumanOwned || issue.Title != episode.MachineTitle
		descriptionHumanOwned = episode.DescriptionHumanOwned || issue.Description.String != episode.MachineDescription || !issue.Description.Valid
		statusHumanOwned = episode.StatusHumanOwned || issue.Status != episode.MachineStatus
		priorityHumanOwned = episode.PriorityHumanOwned || issue.Priority != episode.MachinePriority
	} else {
		if _, err := qtx.LockWorkspaceForIssueCreate(ctx, receipt.WorkspaceID); err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("lock workspace issue creation policy: %w", err)
		}
		if _, err := qtx.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: receipt.ProjectID, WorkspaceID: receipt.WorkspaceID}); err != nil {
			return externalIncidentApplyOutcome{}, ErrProjectNotFound
		}
		number, err := AllocateIssueNumber(ctx, qtx, receipt.WorkspaceID, policy)
		if err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("allocate external incident issue number: %w", err)
		}
		position, err := issueposition.NextTopPosition(ctx, tx, receipt.WorkspaceID, machineStatus)
		if err != nil {
			return externalIncidentApplyOutcome{}, err
		}
		issue, err = qtx.CreateIssueWithOrigin(ctx, db.CreateIssueWithOriginParams{
			ID: issueID, WorkspaceID: receipt.WorkspaceID, Title: machineTitle,
			Description: pgtype.Text{String: machineDescription, Valid: true}, Status: machineStatus,
			Priority: machinePriority, CreatorType: "agent", CreatorID: receipt.CreatorAgentID,
			Position: position, Number: number, ProjectID: receipt.ProjectID,
			OriginType: pgtype.Text{String: "external_incident", Valid: true}, OriginID: episodeID,
		})
		if err != nil {
			return externalIncidentApplyOutcome{}, fmt.Errorf("create external incident issue: %w", err)
		}
		created = true
	}
	issueTitle, issueStatus, issuePriority := machineTitle, machineStatus, machinePriority
	issueDescription := pgtype.Text{String: machineDescription, Valid: true}
	if titleHumanOwned {
		issueTitle = issue.Title
	}
	if descriptionHumanOwned {
		issueDescription = issue.Description
	}
	if statusHumanOwned {
		issueStatus = issue.Status
	}
	if priorityHumanOwned {
		issuePriority = issue.Priority
	}

	metadata, err := externalIncidentMetadata(receipt, in, titleHumanOwned, descriptionHumanOwned, statusHumanOwned, priorityHumanOwned)
	if err != nil {
		return externalIncidentApplyOutcome{}, err
	}
	issue, err = qtx.UpdateExternalIncidentIssue(ctx, db.UpdateExternalIncidentIssueParams{
		Title: issueTitle, Description: issueDescription,
		Status: issueStatus, Priority: issuePriority,
		IncidentMetadata: metadata, ID: issueID, WorkspaceID: receipt.WorkspaceID, OriginID: episodeID,
	})
	if err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("update external incident issue: %w", err)
	}

	resolvedAt := pgtype.Timestamptz{}
	if isExternalIncidentTerminal(receipt.LifecycleState) {
		resolvedAt = receipt.ObservedAt
	}
	if hasEpisode {
		episode, err = qtx.AdvanceExternalIncidentEpisode(ctx, db.AdvanceExternalIncidentEpisodeParams{
			CurrentState: receipt.LifecycleState, Severity: receipt.Severity, CurrentEventKey: receipt.EventKey,
			CurrentSourceSequence: receipt.SourceSequence,
			MachineTitle:          machineTitle, MachineDescription: machineDescription, MachineStatus: machineStatus, MachinePriority: machinePriority,
			TitleHumanOwned: titleHumanOwned, DescriptionHumanOwned: descriptionHumanOwned,
			StatusHumanOwned: statusHumanOwned, PriorityHumanOwned: priorityHumanOwned,
			LastObservedAt: receipt.ObservedAt, ResolvedAt: resolvedAt,
			WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, FindingKey: receipt.FindingKey,
			Generation: receipt.EpisodeGeneration, ExpectedRevision: episode.Revision,
		})
	} else {
		episode, err = qtx.CreateExternalIncidentEpisode(ctx, db.CreateExternalIncidentEpisodeParams{
			WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, FindingKey: receipt.FindingKey, IncidentKey: receipt.IncidentKey,
			Generation: receipt.EpisodeGeneration, IssueID: issueID, CurrentState: receipt.LifecycleState,
			Severity: receipt.Severity, CurrentEventKey: receipt.EventKey, CurrentSourceSequence: receipt.SourceSequence,
			MachineTitle: machineTitle, MachineDescription: machineDescription, MachineStatus: machineStatus, MachinePriority: machinePriority,
			TitleHumanOwned: false, DescriptionHumanOwned: false, StatusHumanOwned: false, PriorityHumanOwned: false,
			OpenedAt:       receipt.ObservedAt,
			LastObservedAt: receipt.ObservedAt, ResolvedAt: resolvedAt,
		})
	}
	if err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("persist external incident episode: %w", err)
	}

	result := ExternalIncidentResult{
		EventKey: receipt.EventKey, FindingKey: receipt.FindingKey, EpisodeGeneration: receipt.EpisodeGeneration,
		Transition: receipt.LifecycleState, IssueID: util.UUIDToString(issue.ID), IssueNumber: issue.Number,
		EpisodeRevision: episode.Revision, ReceiptState: "applied",
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("marshal external incident result: %w", err)
	}
	if _, err := qtx.MarkExternalIncidentOccurrenceApplied(ctx, db.MarkExternalIncidentOccurrenceAppliedParams{
		IssueID: issue.ID, EpisodeRevision: pgtype.Int8{Int64: episode.Revision, Valid: true}, Result: resultJSON,
		WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, EventKey: receipt.EventKey,
		RequestHash: receipt.RequestHash, LeaseToken: leaseToken,
	}); err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("complete external incident occurrence: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return externalIncidentApplyOutcome{}, fmt.Errorf("commit external incident occurrence: %w", err)
	}
	return externalIncidentApplyOutcome{Result: result, Issue: issue, Previous: previousIssue, ActorID: receipt.CreatorAgentID, Created: created}, nil
}

func validateExternalIncidentTransition(ctx context.Context, q *db.Queries, receipt db.ExternalIncidentOccurrence, current db.ExternalIncidentEpisode, hasCurrent bool) (string, error) {
	if hasCurrent {
		if receipt.IncidentKey != current.IncidentKey {
			return "incident_key_changed", nil
		}
		if isExternalIncidentTerminal(current.CurrentState) {
			return "episode_terminal", nil
		}
		if receipt.ObservedAt.Time.Before(current.LastObservedAt.Time) {
			return "observation_older", nil
		}
		if receipt.LifecycleState == "new" && current.CurrentState != "new" {
			return "lifecycle_regression", nil
		}
		return "", nil
	}
	latest, err := q.GetLatestExternalIncidentEpisode(ctx, db.GetLatestExternalIncidentEpisodeParams{
		WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, FindingKey: receipt.FindingKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if receipt.EpisodeGeneration != 1 {
			return "first_generation_must_be_one", nil
		}
		if isExternalIncidentTerminal(receipt.LifecycleState) {
			return "terminal_event_requires_active_episode", nil
		}
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load latest external incident episode: %w", err)
	}
	if !isExternalIncidentTerminal(latest.CurrentState) {
		return "prior_episode_still_active", nil
	}
	if receipt.IncidentKey != latest.IncidentKey {
		return "incident_key_changed", nil
	}
	if receipt.EpisodeGeneration != latest.Generation+1 {
		return "generation_must_follow_terminal_episode", nil
	}
	if isExternalIncidentTerminal(receipt.LifecycleState) {
		return "terminal_event_requires_active_episode", nil
	}
	return "", nil
}

func validateExternalIncidentOrdering(ctx context.Context, q *db.Queries, receipt db.ExternalIncidentOccurrence) (code string, pending bool, err error) {
	if !receipt.PredecessorEventKey.Valid {
		_, err := q.GetLatestAppliedExternalIncidentOccurrence(ctx, db.GetLatestAppliedExternalIncidentOccurrenceParams{
			WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, FindingKey: receipt.FindingKey,
		})
		if err == nil {
			return "predecessor_required", false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", false, fmt.Errorf("load latest applied external incident occurrence: %w", err)
		}
		return "", false, nil
	}
	predecessor, err := q.GetExternalIncidentPredecessor(ctx, db.GetExternalIncidentPredecessorParams{
		WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, EventKey: receipt.PredecessorEventKey.String,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load external incident predecessor: %w", err)
	}
	if predecessor.FindingKey != receipt.FindingKey || predecessor.IncidentKey != receipt.IncidentKey || predecessor.SourceSequence >= receipt.SourceSequence {
		return "predecessor_mismatch", false, nil
	}
	if predecessor.State == "rejected" {
		return "predecessor_rejected", false, nil
	}
	if predecessor.State != "applied" {
		return "", true, nil
	}
	latest, err := q.GetLatestExternalIncidentEpisode(ctx, db.GetLatestExternalIncidentEpisodeParams{
		WorkspaceID: receipt.WorkspaceID, SourceKey: receipt.SourceKey, FindingKey: receipt.FindingKey,
	})
	if err == nil && latest.CurrentEventKey != predecessor.EventKey {
		return "predecessor_not_current", false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("load external incident ordering head: %w", err)
	}
	return "", false, nil
}

func externalIncidentMetadata(receipt db.ExternalIncidentOccurrence, in ExternalIncidentInput, titleHumanOwned, descriptionHumanOwned, statusHumanOwned, priorityHumanOwned bool) ([]byte, error) {
	value := map[string]any{
		"source_key": receipt.SourceKey, "finding_key": receipt.FindingKey, "incident_key": receipt.IncidentKey,
		"episode_generation": receipt.EpisodeGeneration, "transition": receipt.LifecycleState,
		"severity": receipt.Severity, "event_key": receipt.EventKey, "source_sequence": receipt.SourceSequence,
		"predecessor_event_key": util.TextToPtr(receipt.PredecessorEventKey),
		"action_required":       receipt.ActionRequired, "next_step": receipt.NextStep,
		"observed_at":      receipt.ObservedAt.Time.UTC().Format(time.RFC3339Nano),
		"binding_revision": receipt.BindingRevision, "evidence_refs": in.EvidenceRefs,
		"human_owned_fields": map[string]bool{
			"title": titleHumanOwned, "description": descriptionHumanOwned,
			"status": statusHumanOwned, "priority": priorityHumanOwned,
		},
	}
	return json.Marshal(value)
}

func pendingExternalIncidentResult(receipt db.ExternalIncidentOccurrence) ExternalIncidentResult {
	return ExternalIncidentResult{
		EventKey: receipt.EventKey, FindingKey: receipt.FindingKey,
		EpisodeGeneration: receipt.EpisodeGeneration, Transition: receipt.LifecycleState,
		ReceiptState: "pending_predecessor",
	}
}

func (s *ExternalIncidentService) externalIncidentRealtimePayload(ctx context.Context, outcome externalIncidentApplyOutcome) map[string]any {
	prefix := ""
	if workspace, err := s.Queries.GetWorkspace(ctx, outcome.Issue.WorkspaceID); err == nil {
		prefix = workspace.IssuePrefix
	}
	payload := map[string]any{
		"issue":             IssueToMapResolved(ctx, s.Queries, outcome.Issue, prefix),
		"external_incident": true,
	}
	if outcome.Created {
		return payload
	}
	payload["assignee_changed"] = false
	payload["project_changed"] = false
	payload["status_changed"] = outcome.Previous.Status != outcome.Issue.Status
	payload["priority_changed"] = outcome.Previous.Priority != outcome.Issue.Priority
	payload["title_changed"] = outcome.Previous.Title != outcome.Issue.Title
	payload["description_changed"] = outcome.Previous.Description != outcome.Issue.Description
	payload["prev_status"] = outcome.Previous.Status
	payload["prev_priority"] = outcome.Previous.Priority
	payload["prev_title"] = outcome.Previous.Title
	payload["prev_description"] = util.TextToPtr(outcome.Previous.Description)
	return payload
}

func externalIncidentDescription(receipt db.ExternalIncidentOccurrence) string {
	return "Summary:\n" + receipt.Summary + "\n\nNext step:\n" + receipt.NextStep
}

func externalIncidentIssueStatus(state string) string {
	if isExternalIncidentTerminal(state) {
		return "done"
	}
	if state == "needs_human" {
		return "blocked"
	}
	if state == "suppressed" {
		return "cancelled"
	}
	return "in_progress"
}

func externalIncidentIssuePriority(severity string) string {
	switch severity {
	case "critical":
		return "urgent"
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return "none"
	}
}

func isExternalIncidentTerminal(state string) bool { return state == "recovered" || state == "retired" }

func deterministicExternalIncidentEpisodeID(workspaceID pgtype.UUID, sourceKey, findingKey string, generation int64) pgtype.UUID {
	name := strings.Join([]string{"multica-external-incident", util.UUIDToString(workspaceID), sourceKey, findingKey, fmt.Sprintf("%d", generation)}, ":")
	return pgtype.UUID{Bytes: googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte(name)), Valid: true}
}

func decodeExternalIncidentResult(raw []byte, replayed bool) (ExternalIncidentResult, error) {
	var result ExternalIncidentResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return ExternalIncidentResult{}, fmt.Errorf("decode external incident result: %w", err)
	}
	result.Replayed = replayed
	return result, nil
}

func rejectedExternalIncidentResult(receipt db.ExternalIncidentOccurrence, code string) []byte {
	value, _ := json.Marshal(map[string]any{
		"event_key": receipt.EventKey, "finding_key": receipt.FindingKey,
		"episode_generation": receipt.EpisodeGeneration, "transition": receipt.LifecycleState, "receipt_state": "rejected", "error_code": code,
	})
	return value
}

func (s *ExternalIncidentService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
