package developmentmemory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/designgate"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type database interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Store struct{ db database }

func NewStore(db database) *Store { return &Store{db: db} }

type Completion struct {
	TaskID, WorkspaceID, IssueID string
	StartedAt, CompletedAt       time.Time
	Output, BranchName, Status   string
}

type Proposal struct {
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id"`
	IssueID         string         `json:"issue_id"`
	TaskID          string         `json:"task_id"`
	WorkstreamID    string         `json:"workstream_id"`
	AgentName       string         `json:"agent_name"`
	TaskStartedAt   time.Time      `json:"task_started_at"`
	TaskCompletedAt time.Time      `json:"task_completed_at"`
	TaskStatus      string         `json:"task_status"`
	Candidate       ProposalDraft  `json:"candidate"`
	ApprovedPayload *ProposalDraft `json:"approved_payload"`
	ReviewStatus    string         `json:"review_status"`
	ReviewedBy      *string        `json:"reviewed_by"`
	ReviewedAt      *time.Time     `json:"reviewed_at"`
	DeliveredAt     *time.Time     `json:"delivered_at"`
	DeadLetteredAt  *time.Time     `json:"dead_lettered_at"`
	ReviewNote      *string        `json:"review_note"`
	LastError       *string        `json:"last_error"`
	AttemptCount    int            `json:"attempt_count"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	IssueTitle      *string        `json:"issue_title,omitempty"`
}

func (s *Store) BindingForProposal(ctx context.Context, workspaceID, issueID, proposalID string) (Binding, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `
SELECT binding FROM development_memory_proposal
WHERE id=$3::uuid AND workspace_id=$1::uuid AND issue_id=$2::uuid`, workspaceID, issueID, proposalID).Scan(&raw)
	if err != nil {
		return Binding{}, err
	}
	var binding Binding
	if json.Unmarshal(raw, &binding) != nil || binding.SchemaVersion != 1 {
		return Binding{}, errors.New("development memory proposal binding is invalid")
	}
	return binding, nil
}

type Delivery struct {
	ProposalID, TaskID, WorkstreamID, AgentName string
	TaskStartedAt, TaskCompletedAt              time.Time
	TaskStatus                                  string
	Payload                                     ProposalDraft
	Repositories                                []BoundRepository
	LeaseToken                                  string
}

func (s *Store) BindTask(ctx context.Context, taskID string, binding Binding) error {
	if s == nil || s.db == nil {
		return errors.New("development memory store is unavailable")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
UPDATE agent_task_queue
SET context = COALESCE(context, '{}'::jsonb) || jsonb_build_object('development_memory_v1', $2::jsonb)
WHERE id = $1::uuid AND status = 'dispatched'`, taskID, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("development memory task binding lost its dispatched task")
	}
	return nil
}

func (s *Store) BindDesignDelivery(ctx context.Context, taskID string, binding designgate.Binding) error {
	if s == nil || s.db == nil {
		return errors.New("development memory store is unavailable")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
UPDATE agent_task_queue
SET context = COALESCE(context, '{}'::jsonb) || jsonb_build_object('design_delivery_v1', $2::jsonb)
WHERE id = $1::uuid AND status = 'dispatched'`, taskID, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("design delivery task binding lost its dispatched task")
	}
	return nil
}

func (s *Store) EnsureProposal(ctx context.Context, completion Completion) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("development memory store is unavailable")
	}
	var rawBinding []byte
	err := s.db.QueryRow(ctx, `SELECT context -> 'development_memory_v1' FROM agent_task_queue WHERE id = $1::uuid`, completion.TaskID).Scan(&rawBinding)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(rawBinding) == 0 || string(rawBinding) == "null" {
		return false, nil
	}
	status := normalizeTaskStatus(completion.Status)
	if status == "" {
		return false, errors.New("development memory completion status is invalid")
	}
	var binding Binding
	if json.Unmarshal(rawBinding, &binding) != nil || binding.SchemaVersion != 1 || binding.WorkstreamID == "" || len(binding.Repositories) == 0 {
		return false, errors.New("development memory task binding is invalid")
	}
	draft := parseProposal(completion.Output, completion.BranchName, status, binding)
	draft, reviewable := ReviewableProposal(draft)
	if !reviewable {
		return false, nil
	}
	candidate, err := json.Marshal(draft)
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256([]byte(completion.Output))
	tag, err := s.db.Exec(ctx, `
INSERT INTO development_memory_proposal (
    id, workspace_id, issue_id, task_id, workstream_id, agent_name,
    task_started_at, task_completed_at, task_status, binding, candidate, source_output_sha256
) VALUES (
    $1::uuid, $2::uuid, $3::uuid, $1::uuid, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11
)
ON CONFLICT (task_id) DO NOTHING`, completion.TaskID, completion.WorkspaceID, completion.IssueID,
		binding.WorkstreamID, binding.AgentName, completion.StartedAt, completion.CompletedAt, status, rawBinding, candidate, hex.EncodeToString(digest[:]))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) ListIssue(ctx context.Context, workspaceID, issueID string) ([]Proposal, error) {
	rows, err := s.db.Query(ctx, `
SELECT id::text, workspace_id::text, issue_id::text, task_id::text, workstream_id, agent_name,
       task_started_at, task_completed_at, task_status, candidate, approved_payload, review_status,
       reviewed_by::text, reviewed_at, review_note, delivered_at, attempt_count,
       last_error, dead_lettered_at, created_at, updated_at
FROM development_memory_proposal
WHERE workspace_id = $1::uuid AND issue_id = $2::uuid
ORDER BY created_at DESC`, workspaceID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Proposal{}
	for rows.Next() {
		proposal, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, proposal)
	}
	return result, rows.Err()
}

// ListAttention returns only owner-actionable memory work for a workspace.
// Rejected and delivered proposals remain in history but never clutter Inbox.
func (s *Store) ListAttention(ctx context.Context, workspaceID string) ([]Proposal, error) {
	rows, err := s.db.Query(ctx, `
SELECT p.id::text, p.workspace_id::text, p.issue_id::text, p.task_id::text, p.workstream_id, p.agent_name,
       p.task_started_at, p.task_completed_at, p.task_status, p.candidate, p.approved_payload, p.review_status,
       p.reviewed_by::text, p.reviewed_at, p.review_note, p.delivered_at, p.attempt_count,
       p.last_error, p.dead_lettered_at, p.created_at, p.updated_at, i.title
FROM development_memory_proposal p
JOIN issue i ON i.id = p.issue_id AND i.workspace_id = p.workspace_id
WHERE p.workspace_id = $1::uuid
  AND jsonb_array_length(COALESCE(p.candidate->'memory_items', '[]'::jsonb)) > 0
  AND (p.review_status = 'pending'
       OR (p.review_status = 'approved' AND p.delivered_at IS NULL AND p.dead_lettered_at IS NOT NULL))
ORDER BY p.created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Proposal{}
	for rows.Next() {
		var title string
		proposal, err := scanProposalWithExtras(rows, &title)
		if err != nil {
			return nil, err
		}
		proposal.IssueTitle = &title
		result = append(result, proposal)
	}
	return result, rows.Err()
}

func (s *Store) Approve(ctx context.Context, workspaceID, issueID, proposalID, reviewerID string, payload ProposalDraft, note string) (Proposal, error) {
	approved, err := json.Marshal(payload)
	if err != nil {
		return Proposal{}, err
	}
	row := s.db.QueryRow(ctx, `
UPDATE development_memory_proposal
SET review_status='approved', approved_payload=$5::jsonb, reviewed_by=$4::uuid,
    reviewed_at=now(), review_note=NULLIF($6, ''), next_attempt_at=now(),
    lease_token=NULL, last_error=NULL, dead_lettered_at=NULL, updated_at=now()
WHERE id=$3::uuid AND workspace_id=$1::uuid AND issue_id=$2::uuid AND review_status='pending'
AND EXISTS (SELECT 1 FROM workspace w WHERE w.id=development_memory_proposal.workspace_id)
RETURNING id::text, workspace_id::text, issue_id::text, task_id::text, workstream_id, agent_name,
          task_started_at, task_completed_at, task_status, candidate, approved_payload, review_status,
          reviewed_by::text, reviewed_at, review_note, delivered_at, attempt_count,
          last_error, dead_lettered_at, created_at, updated_at`, workspaceID, issueID, proposalID, reviewerID, approved, note)
	return scanProposal(row)
}

func (s *Store) Reject(ctx context.Context, workspaceID, issueID, proposalID, reviewerID, note string) (Proposal, error) {
	row := s.db.QueryRow(ctx, `
UPDATE development_memory_proposal
SET review_status='rejected', approved_payload=NULL, reviewed_by=$4::uuid,
    reviewed_at=now(), review_note=NULLIF($5, ''), lease_token=NULL, updated_at=now()
WHERE id=$3::uuid AND workspace_id=$1::uuid AND issue_id=$2::uuid AND review_status='pending'
RETURNING id::text, workspace_id::text, issue_id::text, task_id::text, workstream_id, agent_name,
          task_started_at, task_completed_at, task_status, candidate, approved_payload, review_status,
          reviewed_by::text, reviewed_at, review_note, delivered_at, attempt_count,
          last_error, dead_lettered_at, created_at, updated_at`, workspaceID, issueID, proposalID, reviewerID, note)
	return scanProposal(row)
}

func (s *Store) RetryDelivery(ctx context.Context, workspaceID, issueID, proposalID string) (Proposal, error) {
	row := s.db.QueryRow(ctx, `
UPDATE development_memory_proposal
SET attempt_count=0, next_attempt_at=now(), lease_token=NULL, last_error=NULL,
    dead_lettered_at=NULL, updated_at=now()
WHERE id=$3::uuid AND workspace_id=$1::uuid AND issue_id=$2::uuid
  AND review_status='approved' AND delivered_at IS NULL AND dead_lettered_at IS NOT NULL
AND EXISTS (SELECT 1 FROM workspace w WHERE w.id=development_memory_proposal.workspace_id)
RETURNING id::text, workspace_id::text, issue_id::text, task_id::text, workstream_id, agent_name,
          task_started_at, task_completed_at, task_status, candidate, approved_payload, review_status,
          reviewed_by::text, reviewed_at, review_note, delivered_at, attempt_count,
          last_error, dead_lettered_at, created_at, updated_at`, workspaceID, issueID, proposalID)
	return scanProposal(row)
}

func (s *Store) ClaimNext(ctx context.Context, lease time.Duration) (Delivery, error) {
	row := s.db.QueryRow(ctx, `
WITH due AS (
    SELECT id FROM development_memory_proposal
    WHERE review_status='approved' AND delivered_at IS NULL AND dead_lettered_at IS NULL
      AND next_attempt_at <= now()
      AND EXISTS (SELECT 1 FROM workspace w WHERE w.id=development_memory_proposal.workspace_id)
    ORDER BY next_attempt_at, created_at
    FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE development_memory_proposal AS p
SET lease_token=gen_random_uuid(), next_attempt_at=now()+make_interval(secs => $1), updated_at=now()
FROM due WHERE p.id=due.id
RETURNING p.id::text, p.task_id::text, p.workstream_id, p.agent_name,
          p.task_started_at, p.task_completed_at, p.task_status, p.approved_payload,
          p.binding -> 'repositories',
		  p.lease_token::text`, lease.Seconds())
	var delivery Delivery
	var payload, repositories []byte
	if err := row.Scan(&delivery.ProposalID, &delivery.TaskID, &delivery.WorkstreamID, &delivery.AgentName,
		&delivery.TaskStartedAt, &delivery.TaskCompletedAt, &delivery.TaskStatus, &payload, &repositories, &delivery.LeaseToken); err != nil {
		return Delivery{}, err
	}
	if json.Unmarshal(payload, &delivery.Payload) != nil || json.Unmarshal(repositories, &delivery.Repositories) != nil {
		return Delivery{}, errors.New("approved development memory delivery is malformed")
	}
	return delivery, nil
}

func (s *Store) MarkDelivered(ctx context.Context, proposalID, leaseToken string) error {
	tag, err := s.db.Exec(ctx, `UPDATE development_memory_proposal SET delivered_at=now(), attempt_count=attempt_count+1,
lease_token=NULL, last_error=NULL, updated_at=now() WHERE id=$1::uuid AND lease_token=$2::uuid`, proposalID, leaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("development memory delivery lease was lost")
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, proposalID, leaseToken string, deliveryErr error, maxAttempts int) error {
	if maxAttempts < 1 {
		maxAttempts = 10
	}
	message := "delivery failed"
	if deliveryErr != nil {
		message = redact.Text(deliveryErr.Error())
	}
	tag, err := s.db.Exec(ctx, `
UPDATE development_memory_proposal
SET attempt_count=attempt_count+1, last_error=left($3,1000), lease_token=NULL,
	    next_attempt_at=now() + make_interval(secs => LEAST(3600, 15 * power(2, LEAST(attempt_count, 8)))),
    dead_lettered_at=CASE WHEN attempt_count+1 >= $4 THEN now() ELSE NULL END,
    updated_at=now()
WHERE id=$1::uuid AND lease_token=$2::uuid`, proposalID, leaseToken, message, maxAttempts)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("development memory delivery lease was lost")
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanProposal(row rowScanner) (Proposal, error) {
	return scanProposalWithExtras(row)
}

func scanProposalWithExtras(row rowScanner, extras ...any) (Proposal, error) {
	var p Proposal
	var candidate, approved []byte
	var reviewedBy *string
	targets := []any{&p.ID, &p.WorkspaceID, &p.IssueID, &p.TaskID, &p.WorkstreamID, &p.AgentName,
		&p.TaskStartedAt, &p.TaskCompletedAt, &p.TaskStatus, &candidate, &approved, &p.ReviewStatus,
		&reviewedBy, &p.ReviewedAt, &p.ReviewNote, &p.DeliveredAt, &p.AttemptCount,
		&p.LastError, &p.DeadLetteredAt, &p.CreatedAt, &p.UpdatedAt}
	targets = append(targets, extras...)
	if err := row.Scan(targets...); err != nil {
		return Proposal{}, err
	}
	p.ReviewedBy = reviewedBy
	if err := json.Unmarshal(candidate, &p.Candidate); err != nil {
		return Proposal{}, fmt.Errorf("decode candidate: %w", err)
	}
	if len(approved) > 0 {
		var value ProposalDraft
		if err := json.Unmarshal(approved, &value); err != nil {
			return Proposal{}, fmt.Errorf("decode approved payload: %w", err)
		}
		p.ApprovedPayload = &value
	}
	return p, nil
}

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "complete", "completed":
		return "complete"
	case "failed":
		return "failed"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return ""
	}
}
