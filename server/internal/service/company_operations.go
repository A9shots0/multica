package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

var (
	ErrCompanyServiceNotFound      = errors.New("company service not found")
	ErrCompanySignalConflict       = errors.New("company signal event id conflict")
	ErrCompanySignalPredecessor    = errors.New("company signal predecessor mismatch")
	ErrCompanySignalServiceBinding = errors.New("company signal service binding mismatch")
	ErrCompanyServiceTopology      = errors.New("company service topology mapping is invalid")
	ErrCompanyWorkTopology         = errors.New("company work topology mapping is invalid")
	ErrCompanyWorkIssueNotFound    = errors.New("company work issue not found")
)

type CompanyServiceInput struct {
	DisplayName    string         `json:"display_name"`
	AreaKey        string         `json:"area_key"`
	Environment    string         `json:"environment"`
	SourceSystem   string         `json:"source_system"`
	SourceNativeID string         `json:"source_native_id"`
	ProjectID      pgtype.UUID    `json:"-"`
	Authority      string         `json:"authority"`
	FreshnessState string         `json:"freshness_state"`
	PrivacyClass   string         `json:"privacy_class"`
	ObservedAt     time.Time      `json:"observed_at"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	SystemNodeIDs  []string       `json:"system_node_ids,omitempty"`
	JourneyIDs     []string       `json:"journey_ids,omitempty"`
}

type CompanySignalInput struct {
	ContractVersion    int32     `json:"contract_version"`
	EventID            string    `json:"event_id"`
	SourceSystem       string    `json:"source_system"`
	SourceInstance     string    `json:"source_instance"`
	SourceNativeID     string    `json:"source_native_id"`
	SourceRevision     string    `json:"source_revision"`
	EventKind          string    `json:"event_kind"`
	SourceLane         string    `json:"source_lane"`
	LifecycleState     string    `json:"lifecycle_state"`
	Severity           string    `json:"severity"`
	ActionRequired     bool      `json:"action_required"`
	Summary            string    `json:"summary"`
	NextStep           string    `json:"next_step"`
	AffectedRefs       []string  `json:"affected_refs"`
	ServiceKey         string    `json:"service_ref"`
	Environment        string    `json:"environment_ref"`
	ProjectRef         string    `json:"project_ref,omitempty"`
	OutcomeRef         string    `json:"outcome_ref,omitempty"`
	EvidenceRefs       []string  `json:"evidence_refs"`
	PredecessorEventID string    `json:"predecessor_event_id,omitempty"`
	PrivacyClass       string    `json:"privacy_class"`
	RawPayloadHash     string    `json:"raw_payload_hash"`
	CorrelationKey     string    `json:"correlation_key"`
	IncidentGeneration int64     `json:"incident_generation"`
	ObservedAt         time.Time `json:"observed_at"`
}

type CompanyWorkTopologyInput struct {
	GraphVersion  string
	SystemNodeIDs []string
	JourneyIDs    []string
	Authority     string
	EvidenceRefs  []string
	ObservedAt    time.Time
	ExpiresAt     *time.Time
}

type CompanySignalResult struct {
	EventID            string `json:"event_id"`
	IncidentID         string `json:"incident_id"`
	CorrelationKey     string `json:"correlation_key"`
	IncidentGeneration int64  `json:"incident_generation"`
	IncidentRevision   int64  `json:"incident_revision"`
	LifecycleState     string `json:"lifecycle_state"`
	VerificationState  string `json:"verification_state"`
	Replayed           bool   `json:"replayed"`
	ReceiptState       string `json:"receipt_state"`
}

type CompanyEvidenceLane struct {
	Lane             string   `json:"lane"`
	SourceSystem     string   `json:"source_system"`
	EventCount       int      `json:"event_count"`
	LifecycleState   string   `json:"lifecycle_state"`
	Severity         string   `json:"severity"`
	LatestSummary    string   `json:"latest_summary"`
	AffectedRefs     []string `json:"affected_refs"`
	EvidenceRefs     []string `json:"evidence_refs"`
	LatestObservedAt string   `json:"latest_observed_at"`
}

type CompanyIncidentProjection struct {
	ID                string                `json:"id"`
	CorrelationKey    string                `json:"correlation_key"`
	Generation        int64                 `json:"generation"`
	ServiceKey        string                `json:"service_ref"`
	Environment       string                `json:"environment_ref"`
	Title             string                `json:"title"`
	LifecycleState    string                `json:"lifecycle_state"`
	Severity          string                `json:"severity"`
	ActionRequired    bool                  `json:"action_required"`
	NextStep          string                `json:"next_step"`
	VerificationState string                `json:"verification_state"`
	ProjectID         *string               `json:"project_id,omitempty"`
	LinkedIssueID     *string               `json:"linked_issue_id,omitempty"`
	OpenedAt          string                `json:"opened_at"`
	LastObservedAt    string                `json:"last_observed_at"`
	ResolvedAt        *string               `json:"resolved_at,omitempty"`
	Revision          int64                 `json:"revision"`
	AffectedRefs      []string              `json:"affected_refs"`
	EvidenceLanes     []CompanyEvidenceLane `json:"evidence_lanes"`
	SystemNodeIDs     []string              `json:"system_node_ids"`
	JourneyIDs        []string              `json:"journey_ids"`
}

type CompanyServiceProjection struct {
	ServiceKey       string                   `json:"service_key"`
	DisplayName      string                   `json:"display_name"`
	AreaKey          string                   `json:"area_key"`
	Environment      string                   `json:"environment"`
	SourceSystem     string                   `json:"source_system"`
	SourceNativeID   string                   `json:"source_native_id"`
	ProjectID        *string                  `json:"project_id,omitempty"`
	PortfolioBinding *CompanyPortfolioBinding `json:"portfolio_binding,omitempty"`
	Authority        string                   `json:"authority"`
	FreshnessState   string                   `json:"freshness_state"`
	PrivacyClass     string                   `json:"privacy_class"`
	ObservedAt       string                   `json:"observed_at"`
	ExpiresAt        *string                  `json:"expires_at,omitempty"`
	Metadata         map[string]any           `json:"metadata"`
	SystemNodeIDs    []string                 `json:"system_node_ids"`
	JourneyIDs       []string                 `json:"journey_ids"`
	Revision         int64                    `json:"revision"`
}

// CompanyPortfolioBinding is a read-only identity projection. ProjectID stays
// Multica's local UUID; this binding never changes workspace membership,
// permissions, routing, or task authorization.
type CompanyPortfolioBinding struct {
	CanonicalProjectID      string                      `json:"canonical_project_id,omitempty"`
	CompanyMembership       string                      `json:"company_membership"`
	PortfolioClassification string                      `json:"portfolio_classification"`
	VerificationState       string                      `json:"verification_state"`
	Provenance              *CompanyPortfolioProvenance `json:"provenance,omitempty"`
}

type CompanyPortfolioProvenance struct {
	SourceSystem       string `json:"source_system"`
	SourceRepository   string `json:"source_repository"`
	SourceReference    string `json:"source_reference"`
	SourceRevision     string `json:"source_revision"`
	SourceSHA256       string `json:"source_sha256,omitempty"`
	SourceObservedAt   string `json:"source_observed_at"`
	EvidenceStage      string `json:"evidence_stage"`
	ProgramID          string `json:"program_id,omitempty"`
	ProjectionSystem   string `json:"projection_system"`
	ProjectionRevision string `json:"projection_revision"`
	ProjectedAt        string `json:"projected_at"`
}

type CompanyPortfolioResolver interface {
	ResolveCompanyProjects(ctx context.Context, workspaceID string, localProjectIDs []string) (map[string]CompanyPortfolioBinding, error)
}

type CompanyOperatingPicture struct {
	ContractVersion   int                                 `json:"contract_version"`
	GeneratedAt       string                              `json:"generated_at"`
	Services          []CompanyServiceProjection          `json:"services"`
	Incidents         []CompanyIncidentProjection         `json:"incidents"`
	CurrentEffort     []CompanyCurrentEffortProjection    `json:"current_effort"`
	WorkTopology      []CompanyWorkTopologyProjection     `json:"work_topology"`
	WorkRelationships []CompanyWorkRelationshipProjection `json:"work_relationships"`
}

// CompanyCurrentEffortProjection contains only work that can still consume
// capacity or still needs an outcome decision. Terminal task history and
// natively accepted outcomes remain in History instead of inflating HQ.
type CompanyCurrentEffortProjection struct {
	ID                 string   `json:"id"`
	SourceSystem       string   `json:"source_system"`
	ProviderReportOnly bool     `json:"provider_report_only"`
	WorkID             string   `json:"work_id"`
	RunID              *string  `json:"run_id,omitempty"`
	IssueID            *string  `json:"issue_id,omitempty"`
	ProjectID          *string  `json:"project_id,omitempty"`
	Title              string   `json:"title"`
	ActorID            *string  `json:"actor_id,omitempty"`
	ActorName          *string  `json:"actor_name,omitempty"`
	WorkSource         string   `json:"work_source"`
	State              string   `json:"state"`
	ReportedState      string   `json:"reported_state"`
	NativeOutcomeState *string  `json:"native_outcome_state,omitempty"`
	ObservedAt         string   `json:"observed_at"`
	FreshnessState     string   `json:"freshness_state"`
	SystemNodeIDs      []string `json:"system_node_ids"`
	JourneyIDs         []string `json:"journey_ids"`
}

type CompanyWorkTopologyProjection struct {
	IssueID        string   `json:"issue_id"`
	GraphVersion   string   `json:"graph_version"`
	SystemNodeIDs  []string `json:"system_node_ids"`
	JourneyIDs     []string `json:"journey_ids"`
	Authority      string   `json:"authority"`
	EvidenceRefs   []string `json:"evidence_refs"`
	ObservedAt     string   `json:"observed_at"`
	ExpiresAt      *string  `json:"expires_at,omitempty"`
	FreshnessState string   `json:"freshness_state"`
	Revision       int64    `json:"revision"`
}

type CompanyWorkRelationshipProjection struct {
	ID               string `json:"id"`
	SourceIssueID    string `json:"source_issue_id"`
	AffectedIssueID  string `json:"affected_issue_id"`
	RelationshipKind string `json:"relationship_kind"`
	GraphVersion     string `json:"graph_version"`
	CommittedAt      string `json:"committed_at"`
}

type CompanyOperationsService struct {
	Queries           *db.Queries
	TxStarter         TxStarter
	Now               func() time.Time
	PortfolioResolver CompanyPortfolioResolver
}

func NewCompanyOperationsService(q *db.Queries, tx TxStarter) *CompanyOperationsService {
	return &CompanyOperationsService{Queries: q, TxStarter: tx, Now: time.Now}
}

func (s *CompanyOperationsService) UpsertService(ctx context.Context, workspaceID pgtype.UUID, serviceKey string, in CompanyServiceInput) (db.CompanyService, error) {
	if err := validateCompanyServiceTopology(&in); err != nil {
		return db.CompanyService{}, err
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(in.Metadata)
	if err != nil {
		return db.CompanyService{}, fmt.Errorf("marshal company service metadata: %w", err)
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.CompanyService{}, fmt.Errorf("begin company service update: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	if err := qtx.LockCompanyServiceKey(ctx, "company-service:"+util.UUIDToString(workspaceID)+":"+serviceKey); err != nil {
		return db.CompanyService{}, fmt.Errorf("lock company service: %w", err)
	}
	if in.ProjectID.Valid {
		if _, err := qtx.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: in.ProjectID, WorkspaceID: workspaceID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.CompanyService{}, ErrProjectNotFound
			}
			return db.CompanyService{}, fmt.Errorf("verify company service project: %w", err)
		}
	}
	expiresAt := pgtype.Timestamptz{}
	if in.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{Time: in.ExpiresAt.UTC(), Valid: true}
	}
	row, err := qtx.UpsertCompanyService(ctx, db.UpsertCompanyServiceParams{
		WorkspaceID: workspaceID, ServiceKey: serviceKey, DisplayName: in.DisplayName,
		AreaKey: in.AreaKey, Environment: in.Environment, SourceSystem: in.SourceSystem,
		SourceNativeID: in.SourceNativeID, ProjectID: in.ProjectID, Authority: in.Authority,
		FreshnessState: in.FreshnessState, PrivacyClass: in.PrivacyClass,
		ObservedAt: pgtype.Timestamptz{Time: in.ObservedAt.UTC(), Valid: true}, ExpiresAt: expiresAt,
		Metadata: metadata, SystemNodeIds: in.SystemNodeIDs, JourneyIds: in.JourneyIDs,
	})
	if err != nil {
		return db.CompanyService{}, fmt.Errorf("upsert company service: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.CompanyService{}, fmt.Errorf("commit company service: %w", err)
	}
	return row, nil
}

func (s *CompanyOperationsService) UpsertWorkTopology(ctx context.Context, workspaceID, issueID pgtype.UUID, in CompanyWorkTopologyInput) (db.CompanyWorkTopologyBinding, error) {
	if err := validateCompanyWorkTopology(&in); err != nil {
		return db.CompanyWorkTopologyBinding{}, err
	}
	if _, err := s.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.CompanyWorkTopologyBinding{}, ErrCompanyWorkIssueNotFound
		}
		return db.CompanyWorkTopologyBinding{}, fmt.Errorf("verify company work issue: %w", err)
	}
	expiresAt := pgtype.Timestamptz{}
	if in.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{Time: in.ExpiresAt.UTC(), Valid: true}
	}
	row, err := s.Queries.UpsertCompanyWorkTopologyBinding(ctx, db.UpsertCompanyWorkTopologyBindingParams{
		WorkspaceID: workspaceID, IssueID: issueID, GraphVersion: in.GraphVersion,
		SystemNodeIds: in.SystemNodeIDs, JourneyIds: in.JourneyIDs, Authority: in.Authority,
		EvidenceRefs: in.EvidenceRefs, ObservedAt: pgtype.Timestamptz{Time: in.ObservedAt.UTC(), Valid: true},
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return db.CompanyWorkTopologyBinding{}, fmt.Errorf("upsert company work topology: %w", err)
	}
	return row, nil
}

func (s *CompanyOperationsService) IngestSignal(ctx context.Context, binding db.ExternalIncidentSourceBinding, in CompanySignalInput) (CompanySignalResult, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return CompanySignalResult{}, fmt.Errorf("marshal company signal: %w", err)
	}
	digest := sha256.Sum256(payload)
	requestHash := hex.EncodeToString(digest[:])
	incidentID := deterministicCompanyIncidentID(binding.WorkspaceID, in.CorrelationKey, in.IncidentGeneration)
	if in.AffectedRefs == nil {
		in.AffectedRefs = []string{}
	}
	if in.EvidenceRefs == nil {
		in.EvidenceRefs = []string{}
	}
	affected, _ := json.Marshal(in.AffectedRefs)
	evidence, _ := json.Marshal(in.EvidenceRefs)

	receipt, err := s.Queries.AdmitCompanySignalEvent(ctx, db.AdmitCompanySignalEventParams{
		WorkspaceID: binding.WorkspaceID, SourceKey: binding.SourceKey, EventID: in.EventID,
		RequestHash: requestHash, ContractVersion: in.ContractVersion, SourceSystem: in.SourceSystem,
		SourceInstance: in.SourceInstance, SourceNativeID: in.SourceNativeID, SourceRevision: in.SourceRevision,
		EventKind: in.EventKind, SourceLane: in.SourceLane, LifecycleState: in.LifecycleState,
		Severity: in.Severity, ActionRequired: in.ActionRequired, Summary: in.Summary, NextStep: in.NextStep,
		AffectedRefs: affected, ServiceKey: in.ServiceKey, Environment: in.Environment,
		ProjectRef: util.StrToText(in.ProjectRef), OutcomeRef: util.StrToText(in.OutcomeRef),
		EvidenceRefs: evidence, PredecessorEventID: util.StrToText(in.PredecessorEventID),
		PrivacyClass: in.PrivacyClass, RawPayloadHash: in.RawPayloadHash, CorrelationKey: in.CorrelationKey,
		IncidentGeneration: in.IncidentGeneration, IncidentID: incidentID,
		ObservedAt: pgtype.Timestamptz{Time: in.ObservedAt.UTC(), Valid: true},
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return CompanySignalResult{}, fmt.Errorf("admit company signal: %w", err)
		}
		stored, loadErr := s.Queries.GetCompanySignalEvent(ctx, db.GetCompanySignalEventParams{
			WorkspaceID: binding.WorkspaceID, SourceKey: binding.SourceKey, EventID: in.EventID,
		})
		if loadErr != nil {
			return CompanySignalResult{}, fmt.Errorf("load company signal conflict: %w", loadErr)
		}
		if stored.RequestHash != requestHash {
			_, quarantineErr := s.Queries.QuarantineCompanySignalConflict(ctx, db.QuarantineCompanySignalConflictParams{
				ID: dbid.NewV7(), WorkspaceID: binding.WorkspaceID, SourceKey: binding.SourceKey,
				EventID: in.EventID, StoredRequestHash: stored.RequestHash, ReceivedRequestHash: requestHash,
				ReasonCode: "event_id_payload_changed", ReceivedPayload: payload,
			})
			if quarantineErr != nil {
				return CompanySignalResult{}, fmt.Errorf("quarantine company signal conflict: %w", quarantineErr)
			}
			return CompanySignalResult{}, ErrCompanySignalConflict
		}
		receipt = stored
	}
	if receipt.State == "applied" {
		return decodeCompanySignalResult(receipt.Result, true)
	}
	if receipt.State == "rejected" {
		return CompanySignalResult{}, ErrCompanySignalPredecessor
	}

	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CompanySignalResult{}, fmt.Errorf("begin company signal apply: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	if err := qtx.LockCompanyIncidentKey(ctx, "company-incident:"+util.UUIDToString(binding.WorkspaceID)+":"+in.CorrelationKey+fmt.Sprintf(":%d", in.IncidentGeneration)); err != nil {
		return CompanySignalResult{}, fmt.Errorf("lock company incident: %w", err)
	}
	lockedReceipt, err := qtx.GetCompanySignalEvent(ctx, db.GetCompanySignalEventParams{
		WorkspaceID: binding.WorkspaceID, SourceKey: binding.SourceKey, EventID: in.EventID,
	})
	if err != nil {
		return CompanySignalResult{}, fmt.Errorf("reload company signal under lock: %w", err)
	}
	if lockedReceipt.RequestHash != requestHash {
		return CompanySignalResult{}, ErrCompanySignalConflict
	}
	if lockedReceipt.State == "applied" {
		return decodeCompanySignalResult(lockedReceipt.Result, true)
	}
	if lockedReceipt.State == "rejected" {
		return CompanySignalResult{}, ErrCompanySignalPredecessor
	}
	receipt = lockedReceipt
	serviceRow, err := qtx.GetCompanyService(ctx, db.GetCompanyServiceParams{WorkspaceID: binding.WorkspaceID, ServiceKey: in.ServiceKey})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CompanySignalResult{}, ErrCompanyServiceNotFound
		}
		return CompanySignalResult{}, fmt.Errorf("load company service: %w", err)
	}
	if serviceRow.Environment != in.Environment || (serviceRow.ProjectID.Valid && serviceRow.ProjectID != binding.ProjectID) {
		return CompanySignalResult{}, ErrCompanySignalServiceBinding
	}
	if err := validateCompanySignalOrdering(ctx, qtx, receipt); err != nil {
		result, _ := json.Marshal(map[string]any{"receipt_state": "rejected", "error_code": "predecessor_mismatch"})
		if _, markErr := qtx.MarkCompanySignalRejected(ctx, db.MarkCompanySignalRejectedParams{
			Result: result, ErrorCode: util.StrToText("predecessor_mismatch"), WorkspaceID: binding.WorkspaceID,
			SourceKey: binding.SourceKey, EventID: in.EventID, RequestHash: requestHash,
		}); markErr != nil {
			return CompanySignalResult{}, fmt.Errorf("record company signal rejection: %w", markErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return CompanySignalResult{}, fmt.Errorf("commit company signal rejection: %w", commitErr)
		}
		return CompanySignalResult{}, err
	}

	incident, err := qtx.GetCompanyIncident(ctx, db.GetCompanyIncidentParams{
		WorkspaceID: binding.WorkspaceID, CorrelationKey: in.CorrelationKey, Generation: in.IncidentGeneration,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		incident, err = qtx.CreateCompanyIncident(ctx, db.CreateCompanyIncidentParams{
			ID: incidentID, WorkspaceID: binding.WorkspaceID, CorrelationKey: in.CorrelationKey,
			Generation: in.IncidentGeneration, ServiceKey: in.ServiceKey, Environment: in.Environment,
			Title: in.Summary, LifecycleState: "open", Severity: in.Severity,
			ActionRequired: in.ActionRequired, NextStep: in.NextStep, VerificationState: "unverified",
			ProjectID:      binding.ProjectID,
			OpenedAt:       pgtype.Timestamptz{Time: in.ObservedAt.UTC(), Valid: true},
			LastObservedAt: pgtype.Timestamptz{Time: in.ObservedAt.UTC(), Valid: true},
		})
	} else if err == nil && (incident.ServiceKey != in.ServiceKey || incident.Environment != in.Environment) {
		return CompanySignalResult{}, ErrCompanySignalServiceBinding
	}
	if err != nil {
		return CompanySignalResult{}, fmt.Errorf("load or create company incident: %w", err)
	}

	latest, err := qtx.ListLatestCompanyIncidentSignals(ctx, db.ListLatestCompanyIncidentSignalsParams{
		WorkspaceID: binding.WorkspaceID, IncidentID: incident.ID,
	})
	if err != nil {
		return CompanySignalResult{}, fmt.Errorf("load company incident signals: %w", err)
	}
	aggregate := aggregateCompanyIncident(incident, latest)
	incident, err = qtx.UpdateCompanyIncident(ctx, db.UpdateCompanyIncidentParams{
		Title: incident.Title, LifecycleState: aggregate.LifecycleState, Severity: aggregate.Severity,
		ActionRequired: aggregate.ActionRequired, NextStep: aggregate.NextStep,
		VerificationState: aggregate.VerificationState, ProjectID: incident.ProjectID,
		LastObservedAt: aggregate.LastObservedAt, ResolvedAt: aggregate.ResolvedAt,
		WorkspaceID: binding.WorkspaceID, ID: incident.ID, ExpectedRevision: incident.Revision,
	})
	if err != nil {
		return CompanySignalResult{}, fmt.Errorf("update company incident: %w", err)
	}
	result := CompanySignalResult{
		EventID: in.EventID, IncidentID: util.UUIDToString(incident.ID), CorrelationKey: in.CorrelationKey,
		IncidentGeneration: in.IncidentGeneration, IncidentRevision: incident.Revision,
		LifecycleState: incident.LifecycleState, VerificationState: incident.VerificationState,
		ReceiptState: "applied",
	}
	resultJSON, _ := json.Marshal(result)
	if _, err := qtx.MarkCompanySignalApplied(ctx, db.MarkCompanySignalAppliedParams{
		Result: resultJSON, WorkspaceID: binding.WorkspaceID, SourceKey: binding.SourceKey,
		EventID: in.EventID, RequestHash: requestHash,
	}); err != nil {
		return CompanySignalResult{}, fmt.Errorf("complete company signal: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CompanySignalResult{}, fmt.Errorf("commit company signal: %w", err)
	}
	return result, nil
}

func (s *CompanyOperationsService) OperatingPicture(ctx context.Context, workspaceID pgtype.UUID) (CompanyOperatingPicture, error) {
	services, err := s.Queries.ListCompanyServices(ctx, workspaceID)
	if err != nil {
		return CompanyOperatingPicture{}, fmt.Errorf("list company services: %w", err)
	}
	incidents, err := s.Queries.ListCompanyIncidents(ctx, db.ListCompanyIncidentsParams{WorkspaceID: workspaceID, ResultLimit: 100})
	if err != nil {
		return CompanyOperatingPicture{}, fmt.Errorf("list company incidents: %w", err)
	}
	ids := make([]pgtype.UUID, 0, len(incidents))
	for _, incident := range incidents {
		ids = append(ids, incident.ID)
	}
	signals := []db.CompanySignalEvent{}
	if len(ids) > 0 {
		signals, err = s.Queries.ListCompanySignalsForIncidents(ctx, db.ListCompanySignalsForIncidentsParams{
			WorkspaceID: workspaceID, IncidentIds: ids, ResultLimit: 2000,
		})
		if err != nil {
			return CompanyOperatingPicture{}, fmt.Errorf("list company incident signals: %w", err)
		}
	}
	relationshipRows, err := s.Queries.ListWorkspaceIssueWorkflowConsequenceReceipts(ctx, db.ListWorkspaceIssueWorkflowConsequenceReceiptsParams{
		WorkspaceID: workspaceID,
		ResultLimit: 200,
	})
	if err != nil {
		return CompanyOperatingPicture{}, fmt.Errorf("list company work relationships: %w", err)
	}
	activeTasks, err := s.Queries.ListCompanyActiveAgentTasks(ctx, db.ListCompanyActiveAgentTasksParams{
		WorkspaceID: workspaceID,
		ResultLimit: 200,
	})
	if err != nil {
		return CompanyOperatingPicture{}, fmt.Errorf("list company active tasks: %w", err)
	}
	externalReceipts, err := s.Queries.ListCompanyLatestExternalWorkReceipts(ctx, db.ListCompanyLatestExternalWorkReceiptsParams{
		WorkspaceID: workspaceID,
		ResultLimit: 200,
	})
	if err != nil {
		return CompanyOperatingPicture{}, fmt.Errorf("list company external work: %w", err)
	}
	topologyRows, err := s.Queries.ListCompanyWorkTopologyBindings(ctx, db.ListCompanyWorkTopologyBindingsParams{
		WorkspaceID: workspaceID,
		ResultLimit: 500,
	})
	if err != nil {
		return CompanyOperatingPicture{}, fmt.Errorf("list company work topology: %w", err)
	}
	byIncident := make(map[string][]db.CompanySignalEvent)
	for _, signal := range signals {
		key := util.UUIDToString(signal.IncidentID)
		byIncident[key] = append(byIncident[key], signal)
	}
	serviceProjections := make([]CompanyServiceProjection, 0, len(services))
	servicesByKey := make(map[string]db.CompanyService, len(services))
	for _, companyService := range services {
		serviceProjections = append(serviceProjections, ProjectCompanyService(companyService))
		servicesByKey[companyService.ServiceKey] = companyService
	}
	s.attachCompanyPortfolioBindings(ctx, util.UUIDToString(workspaceID), serviceProjections)
	projections := make([]CompanyIncidentProjection, 0, len(incidents))
	for _, incident := range incidents {
		projections = append(projections, projectCompanyIncident(
			incident,
			byIncident[util.UUIDToString(incident.ID)],
			servicesByKey[incident.ServiceKey],
		))
	}
	workRelationships := make([]CompanyWorkRelationshipProjection, 0, len(relationshipRows))
	for _, row := range relationshipRows {
		workRelationships = append(workRelationships, CompanyWorkRelationshipProjection{
			ID: util.UUIDToString(row.ID), SourceIssueID: util.UUIDToString(row.SourceIssueID),
			AffectedIssueID: util.UUIDToString(row.AffectedIssueID), RelationshipKind: row.RelationshipKind,
			GraphVersion: row.GraphVersion, CommittedAt: row.CommittedAt.Time.UTC().Format(time.RFC3339Nano),
		})
	}
	workTopology, topologyByIssue := projectCompanyWorkTopology(topologyRows, s.now().UTC())
	currentEffort, err := projectCompanyCurrentEffort(activeTasks, externalReceipts, topologyByIssue, s.now().UTC())
	if err != nil {
		return CompanyOperatingPicture{}, err
	}
	return CompanyOperatingPicture{
		ContractVersion: 1, GeneratedAt: s.now().UTC().Format(time.RFC3339Nano),
		Services: serviceProjections, Incidents: projections, CurrentEffort: currentEffort, WorkTopology: workTopology,
		WorkRelationships: workRelationships,
	}, nil
}

func projectCompanyCurrentEffort(
	activeTasks []db.ListCompanyActiveAgentTasksRow,
	externalReceipts []db.ListCompanyLatestExternalWorkReceiptsRow,
	topologyByIssue map[string]CompanyWorkTopologyProjection,
	now time.Time,
) ([]CompanyCurrentEffortProjection, error) {
	items := make([]CompanyCurrentEffortProjection, 0, len(activeTasks)+len(externalReceipts))
	for _, task := range activeTasks {
		state := "active"
		switch task.Status {
		case "queued":
			state = "queued"
		case "waiting_local_directory":
			state = "waiting"
		}
		observedAt := task.CreatedAt.Time.UTC()
		if task.DispatchedAt.Valid {
			observedAt = task.DispatchedAt.Time.UTC()
		}
		if task.StartedAt.Valid {
			observedAt = task.StartedAt.Time.UTC()
		}
		title := strings.TrimSpace(task.IssueTitle.String)
		if title == "" {
			title = strings.TrimSpace(task.TriggerSummary.String)
		}
		if title == "" {
			title = "Multica work"
		}
		actorID, actorName := util.UUIDToString(task.AgentID), task.AgentName
		item := CompanyCurrentEffortProjection{
			ID: util.UUIDToString(task.ID), SourceSystem: "multica", WorkID: util.UUIDToString(task.ID),
			Title: title, ActorID: &actorID, ActorName: &actorName, WorkSource: task.WorkSource,
			State: state, ReportedState: task.Status, ObservedAt: observedAt.Format(time.RFC3339Nano),
			FreshnessState: "current",
		}
		if task.IssueID.Valid {
			value := util.UUIDToString(task.IssueID)
			item.IssueID = &value
			if topology, ok := topologyByIssue[value]; ok && topology.FreshnessState == "current" {
				item.SystemNodeIDs = append([]string(nil), topology.SystemNodeIDs...)
				item.JourneyIDs = append([]string(nil), topology.JourneyIDs...)
			}
		}
		if task.ProjectID.Valid {
			value := util.UUIDToString(task.ProjectID)
			item.ProjectID = &value
		}
		items = append(items, item)
	}
	for _, row := range externalReceipts {
		var receipt publicapiv1.OutcomeReceiptV02
		if err := json.Unmarshal(row.RawReceipt, &receipt); err != nil {
			return nil, fmt.Errorf("decode admitted external work receipt %s: %w", util.UUIDToString(row.ID), err)
		}
		observedAt, err := time.Parse(time.RFC3339Nano, receipt.Freshness.ObservedAt)
		if err != nil {
			return nil, fmt.Errorf("decode admitted external work freshness %s: %w", util.UUIDToString(row.ID), err)
		}
		state := "active"
		switch receipt.State {
		case "waiting_external", "waiting_owner", "blocked":
			state = "waiting"
		case "complete":
			state = "reported_done_unaccepted"
		case "stopped":
			state = "stopped"
		}
		freshness := "current"
		if receipt.Freshness.ExpiresAt != nil {
			if expiresAt, parseErr := time.Parse(time.RFC3339Nano, *receipt.Freshness.ExpiresAt); parseErr == nil && !expiresAt.After(now) {
				freshness = "stale"
				if state == "active" || state == "waiting" {
					state = "stale"
				}
			}
		}
		runID := row.RunID
		issueID := util.UUIDToString(row.IssueID)
		item := CompanyCurrentEffortProjection{
			ID: util.UUIDToString(row.ID), SourceSystem: row.Provider, ProviderReportOnly: true,
			WorkID: row.WorkID, RunID: &runID, IssueID: &issueID, Title: row.IssueTitle,
			WorkSource: "external_receipt", State: state, ReportedState: receipt.State,
			ObservedAt: observedAt.UTC().Format(time.RFC3339Nano), FreshnessState: freshness,
		}
		if topology, ok := topologyByIssue[issueID]; ok && topology.FreshnessState == "current" {
			item.SystemNodeIDs = append([]string(nil), topology.SystemNodeIDs...)
			item.JourneyIDs = append([]string(nil), topology.JourneyIDs...)
		}
		if row.ProjectID.Valid {
			value := util.UUIDToString(row.ProjectID)
			item.ProjectID = &value
		}
		if row.OutcomeState.Valid {
			value := row.OutcomeState.String
			item.NativeOutcomeState = &value
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ObservedAt == items[j].ObservedAt {
			return items[i].ID < items[j].ID
		}
		return items[i].ObservedAt > items[j].ObservedAt
	})
	return items, nil
}

func projectCompanyWorkTopology(rows []db.CompanyWorkTopologyBinding, now time.Time) ([]CompanyWorkTopologyProjection, map[string]CompanyWorkTopologyProjection) {
	items := make([]CompanyWorkTopologyProjection, 0, len(rows))
	byIssue := make(map[string]CompanyWorkTopologyProjection, len(rows))
	for _, row := range rows {
		item := ProjectCompanyWorkTopology(row, now)
		items = append(items, item)
		byIssue[item.IssueID] = item
	}
	return items, byIssue
}

func ProjectCompanyWorkTopology(row db.CompanyWorkTopologyBinding, now time.Time) CompanyWorkTopologyProjection {
	freshness := "current"
	if row.ExpiresAt.Valid && !row.ExpiresAt.Time.After(now) {
		freshness = "stale"
	}
	item := CompanyWorkTopologyProjection{
		IssueID: util.UUIDToString(row.IssueID), GraphVersion: row.GraphVersion,
		SystemNodeIDs: append([]string(nil), row.SystemNodeIds...), JourneyIDs: append([]string(nil), row.JourneyIds...),
		Authority: row.Authority, EvidenceRefs: append([]string(nil), row.EvidenceRefs...),
		ObservedAt: row.ObservedAt.Time.UTC().Format(time.RFC3339Nano), FreshnessState: freshness, Revision: row.Revision,
	}
	if row.ExpiresAt.Valid {
		value := row.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
		item.ExpiresAt = &value
	}
	return item
}

func ProjectCompanyService(row db.CompanyService) CompanyServiceProjection {
	metadata := map[string]any{}
	_ = json.Unmarshal(row.Metadata, &metadata)
	projection := CompanyServiceProjection{
		ServiceKey: row.ServiceKey, DisplayName: row.DisplayName, AreaKey: row.AreaKey,
		Environment: row.Environment, SourceSystem: row.SourceSystem, SourceNativeID: row.SourceNativeID,
		Authority: row.Authority, FreshnessState: row.FreshnessState, PrivacyClass: row.PrivacyClass,
		ObservedAt: row.ObservedAt.Time.UTC().Format(time.RFC3339Nano), Metadata: metadata, Revision: row.Revision,
		SystemNodeIDs: append([]string(nil), row.SystemNodeIds...),
		JourneyIDs:    append([]string(nil), row.JourneyIds...),
	}
	if row.ProjectID.Valid {
		value := util.UUIDToString(row.ProjectID)
		projection.ProjectID = &value
	}
	if row.ExpiresAt.Valid {
		value := row.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
		projection.ExpiresAt = &value
	}
	return projection
}

func (s *CompanyOperationsService) attachCompanyPortfolioBindings(ctx context.Context, workspaceID string, services []CompanyServiceProjection) {
	localProjectIDs := make([]string, 0, len(services))
	seen := make(map[string]struct{}, len(services))
	for i := range services {
		if services[i].ProjectID == nil {
			continue
		}
		localID := *services[i].ProjectID
		services[i].PortfolioBinding = &CompanyPortfolioBinding{
			CompanyMembership:       "unknown",
			PortfolioClassification: "unknown",
			VerificationState:       "unverified",
		}
		if _, ok := seen[localID]; !ok {
			seen[localID] = struct{}{}
			localProjectIDs = append(localProjectIDs, localID)
		}
	}
	if len(localProjectIDs) == 0 || s.PortfolioResolver == nil {
		return
	}
	bindings, err := s.PortfolioResolver.ResolveCompanyProjects(ctx, workspaceID, localProjectIDs)
	if err != nil {
		return
	}
	for i := range services {
		if services[i].ProjectID == nil {
			continue
		}
		binding, ok := bindings[*services[i].ProjectID]
		if !ok || !validCompanyPortfolioBinding(binding) {
			continue
		}
		copy := binding
		services[i].PortfolioBinding = &copy
	}
}

func validCompanyPortfolioBinding(binding CompanyPortfolioBinding) bool {
	if binding.VerificationState != "verified" || binding.CanonicalProjectID == "" || binding.Provenance == nil {
		return false
	}
	expectedMembership := "unknown"
	switch binding.PortfolioClassification {
	case "business_application", "company_infrastructure":
		expectedMembership = "company"
	case "personal":
		expectedMembership = "personal"
	case "experimental", "archived", "unknown":
	default:
		return false
	}
	if binding.CompanyMembership != expectedMembership {
		return false
	}
	p := binding.Provenance
	return p.SourceSystem == "silver-mediums-knowledge" &&
		p.SourceRepository == "A9shots0/Silver-Mediums-Knowledge" &&
		p.SourceReference != "" && p.SourceRevision != "" && p.SourceObservedAt != "" &&
		p.EvidenceStage != "" && p.ProjectionSystem == "silver-mediums-brain" &&
		p.ProjectionRevision != "" && p.ProjectedAt != ""
}

func validateCompanyServiceTopology(in *CompanyServiceInput) error {
	graph, err := systemgraph.Default()
	if err != nil {
		return fmt.Errorf("load system graph for company service: %w", err)
	}
	nodes := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = struct{}{}
	}
	journeys := make(map[string]struct{}, len(graph.Journeys))
	for _, journey := range graph.Journeys {
		journeys[journey.ID] = struct{}{}
	}
	in.SystemNodeIDs = uniqueCompanyTopologyIDs(in.SystemNodeIDs)
	in.JourneyIDs = uniqueCompanyTopologyIDs(in.JourneyIDs)
	for _, id := range in.SystemNodeIDs {
		if _, ok := nodes[id]; !ok {
			return fmt.Errorf("%w: unknown system node %q", ErrCompanyServiceTopology, id)
		}
	}
	for _, id := range in.JourneyIDs {
		if _, ok := journeys[id]; !ok {
			return fmt.Errorf("%w: unknown journey %q", ErrCompanyServiceTopology, id)
		}
	}
	return nil
}

func validateCompanyWorkTopology(in *CompanyWorkTopologyInput) error {
	graph, err := systemgraph.Default()
	if err != nil {
		return fmt.Errorf("load system graph for company work: %w", err)
	}
	in.GraphVersion = strings.TrimSpace(in.GraphVersion)
	in.SystemNodeIDs = uniqueCompanyTopologyIDs(in.SystemNodeIDs)
	in.JourneyIDs = uniqueCompanyTopologyIDs(in.JourneyIDs)
	in.EvidenceRefs = uniqueCompanyTopologyIDs(in.EvidenceRefs)
	if in.GraphVersion != graph.Version || len(in.SystemNodeIDs)+len(in.JourneyIDs) == 0 || in.ObservedAt.IsZero() {
		return ErrCompanyWorkTopology
	}
	if in.Authority != "owner" && in.Authority != "cited" || in.Authority == "cited" && len(in.EvidenceRefs) == 0 {
		return ErrCompanyWorkTopology
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(in.ObservedAt) {
		return ErrCompanyWorkTopology
	}
	nodes := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = struct{}{}
	}
	journeys := make(map[string]struct{}, len(graph.Journeys))
	for _, journey := range graph.Journeys {
		journeys[journey.ID] = struct{}{}
	}
	for _, id := range in.SystemNodeIDs {
		if _, ok := nodes[id]; !ok {
			return fmt.Errorf("%w: unknown system node %q", ErrCompanyWorkTopology, id)
		}
	}
	for _, id := range in.JourneyIDs {
		if _, ok := journeys[id]; !ok {
			return fmt.Errorf("%w: unknown journey %q", ErrCompanyWorkTopology, id)
		}
	}
	return nil
}

func uniqueCompanyTopologyIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type companyIncidentAggregate struct {
	LifecycleState, Severity, NextStep, VerificationState string
	ActionRequired                                        bool
	LastObservedAt, ResolvedAt                            pgtype.Timestamptz
}

func aggregateCompanyIncident(current db.CompanyIncident, latest []db.CompanySignalEvent) companyIncidentAggregate {
	result := companyIncidentAggregate{
		LifecycleState: current.LifecycleState, Severity: current.Severity, NextStep: current.NextStep,
		VerificationState: current.VerificationState, LastObservedAt: current.LastObservedAt,
	}
	controlActive, controlRecovering, controlNeedsHuman, allControlTerminal, sawControl := false, false, false, true, false
	verifiedRecovery, sawVerification := false, false
	for _, signal := range latest {
		if signal.ObservedAt.Time.After(result.LastObservedAt.Time) {
			result.LastObservedAt = signal.ObservedAt
		}
		if severityRank(signal.Severity) > severityRank(result.Severity) && !isCompanySignalTerminal(signal.LifecycleState) {
			result.Severity = signal.Severity
		}
		if signal.ActionRequired && !isCompanySignalTerminal(signal.LifecycleState) {
			result.ActionRequired = true
			result.NextStep = signal.NextStep
		}
		if signal.SourceLane == "verification" {
			sawVerification = true
			if signal.LifecycleState == "recovered" || signal.LifecycleState == "retired" {
				verifiedRecovery = true
			}
		}
		if signal.SourceLane == "support_impact" {
			continue
		}
		sawControl = true
		terminal := isCompanySignalTerminal(signal.LifecycleState)
		allControlTerminal = allControlTerminal && terminal
		controlActive = controlActive || !terminal
		controlRecovering = controlRecovering || signal.LifecycleState == "recovering"
		controlNeedsHuman = controlNeedsHuman || signal.LifecycleState == "needs_human"
	}
	switch {
	case verifiedRecovery:
		result.LifecycleState = "recovered"
		result.VerificationState = "verified"
		result.ActionRequired = false
		result.ResolvedAt = result.LastObservedAt
	case sawControl && !controlActive && allControlTerminal:
		result.LifecycleState = "recovering"
		result.VerificationState = "observed"
		result.ActionRequired = false
		result.NextStep = "Verify the real user workflow before accepting recovery."
	case controlNeedsHuman:
		result.LifecycleState = "needs_human"
	case controlRecovering:
		result.LifecycleState = "recovering"
		result.VerificationState = "observed"
	default:
		result.LifecycleState = "open"
		if sawVerification {
			result.VerificationState = "observed"
		}
	}
	return result
}

func validateCompanySignalOrdering(ctx context.Context, q *db.Queries, current db.CompanySignalEvent) error {
	latest, err := q.GetLatestCompanySignalStreamEvent(ctx, db.GetLatestCompanySignalStreamEventParams{
		WorkspaceID: current.WorkspaceID, SourceKey: current.SourceKey, SourceInstance: current.SourceInstance,
		SourceNativeID: current.SourceNativeID, SourceLane: current.SourceLane,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if current.PredecessorEventID.Valid {
			return ErrCompanySignalPredecessor
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load company signal ordering head: %w", err)
	}
	if !current.PredecessorEventID.Valid || current.PredecessorEventID.String != latest.EventID {
		return ErrCompanySignalPredecessor
	}
	predecessor, err := q.GetCompanySignalPredecessor(ctx, db.GetCompanySignalPredecessorParams{
		WorkspaceID: current.WorkspaceID, SourceKey: current.SourceKey, EventID: current.PredecessorEventID.String,
	})
	if err != nil || predecessor.CorrelationKey != current.CorrelationKey || predecessor.IncidentGeneration != current.IncidentGeneration {
		return ErrCompanySignalPredecessor
	}
	return nil
}

func projectCompanyIncident(incident db.CompanyIncident, signals []db.CompanySignalEvent, services ...db.CompanyService) CompanyIncidentProjection {
	affectedSet := map[string]struct{}{}
	type laneState struct {
		lane CompanyEvidenceLane
		last time.Time
	}
	lanes := map[string]laneState{}
	for _, signal := range signals {
		var affected, evidence []string
		_ = json.Unmarshal(signal.AffectedRefs, &affected)
		_ = json.Unmarshal(signal.EvidenceRefs, &evidence)
		for _, ref := range affected {
			affectedSet[ref] = struct{}{}
		}
		key := signal.SourceLane + "\x00" + signal.SourceSystem
		state := lanes[key]
		state.lane.Lane, state.lane.SourceSystem = signal.SourceLane, signal.SourceSystem
		state.lane.EventCount++
		if !signal.ObservedAt.Time.Before(state.last) {
			state.last = signal.ObservedAt.Time
			state.lane.LifecycleState = signal.LifecycleState
			state.lane.Severity = signal.Severity
			state.lane.LatestSummary = signal.Summary
			state.lane.AffectedRefs = affected
			state.lane.EvidenceRefs = evidence
			state.lane.LatestObservedAt = signal.ObservedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		lanes[key] = state
	}
	affected := make([]string, 0, len(affectedSet))
	for ref := range affectedSet {
		affected = append(affected, ref)
	}
	sort.Strings(affected)
	evidenceLanes := make([]CompanyEvidenceLane, 0, len(lanes))
	for _, state := range lanes {
		evidenceLanes = append(evidenceLanes, state.lane)
	}
	sort.Slice(evidenceLanes, func(i, j int) bool {
		if evidenceLanes[i].Lane == evidenceLanes[j].Lane {
			return evidenceLanes[i].SourceSystem < evidenceLanes[j].SourceSystem
		}
		return evidenceLanes[i].Lane < evidenceLanes[j].Lane
	})
	projection := CompanyIncidentProjection{
		ID: util.UUIDToString(incident.ID), CorrelationKey: incident.CorrelationKey, Generation: incident.Generation,
		ServiceKey: incident.ServiceKey, Environment: incident.Environment, Title: incident.Title,
		LifecycleState: incident.LifecycleState, Severity: incident.Severity, ActionRequired: incident.ActionRequired,
		NextStep: incident.NextStep, VerificationState: incident.VerificationState,
		OpenedAt:       incident.OpenedAt.Time.UTC().Format(time.RFC3339Nano),
		LastObservedAt: incident.LastObservedAt.Time.UTC().Format(time.RFC3339Nano), Revision: incident.Revision,
		AffectedRefs: affected, EvidenceLanes: evidenceLanes,
	}
	if len(services) > 0 {
		projection.SystemNodeIDs = append([]string(nil), services[0].SystemNodeIds...)
		projection.JourneyIDs = append([]string(nil), services[0].JourneyIds...)
	}
	if incident.ProjectID.Valid {
		value := util.UUIDToString(incident.ProjectID)
		projection.ProjectID = &value
	}
	if incident.LinkedIssueID.Valid {
		value := util.UUIDToString(incident.LinkedIssueID)
		projection.LinkedIssueID = &value
	}
	if incident.ResolvedAt.Valid {
		value := incident.ResolvedAt.Time.UTC().Format(time.RFC3339Nano)
		projection.ResolvedAt = &value
	}
	return projection
}

func deterministicCompanyIncidentID(workspaceID pgtype.UUID, correlationKey string, generation int64) pgtype.UUID {
	name := strings.Join([]string{"multica-company-incident", util.UUIDToString(workspaceID), correlationKey, fmt.Sprintf("%d", generation)}, ":")
	return pgtype.UUID{Bytes: googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte(name)), Valid: true}
}

func decodeCompanySignalResult(raw []byte, replayed bool) (CompanySignalResult, error) {
	var result CompanySignalResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CompanySignalResult{}, fmt.Errorf("decode company signal result: %w", err)
	}
	result.Replayed = replayed
	return result, nil
}

func isCompanySignalTerminal(state string) bool {
	return state == "recovered" || state == "suppressed" || state == "retired"
}

func severityRank(value string) int {
	switch value {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (s *CompanyOperationsService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
