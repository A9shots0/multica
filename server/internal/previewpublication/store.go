package previewpublication

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var (
	ErrAuthorityUnavailable   = errors.New("preview publication authority unavailable")
	ErrPublicationConflict    = errors.New("preview publication conflicts with immutable record")
	ErrPublicationExpired     = errors.New("preview publication expired")
	ErrPublicationUnavailable = errors.New("preview publication unavailable")
)

type TxStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Store struct {
	DB  TxStarter
	Now func() time.Time
}

func replayMatches(stored, proposed Publication) bool {
	return reflect.DeepEqual(stored.Binding, proposed.Binding) &&
		stored.SchemaVersion == proposed.SchemaVersion &&
		stored.PreviewURL == proposed.PreviewURL &&
		stored.IdentityURL == proposed.IdentityURL &&
		stored.DataPolicy == proposed.DataPolicy &&
		stored.ExpiresAt.Equal(proposed.ExpiresAt)
}

func (s Store) Register(ctx context.Context, taskID pgtype.UUID, daemonID string, claim ProducerClaim) (Publication, bool, error) {
	if s.DB == nil || !taskID.Valid || strings.TrimSpace(daemonID) == "" {
		return Publication{}, false, ErrAuthorityUnavailable
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Publication{}, false, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	authority, err := q.LockPreviewPublicationAuthority(ctx, db.LockPreviewPublicationAuthorityParams{
		TaskID: taskID, DaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Publication{}, false, ErrAuthorityUnavailable
		}
		return Publication{}, false, err
	}
	binding := Binding{
		WorkspaceID:       util.UUIDToString(authority.WorkspaceID),
		IssueID:           util.UUIDToString(authority.IssueID),
		AgentID:           util.UUIDToString(authority.AgentID),
		AgentRunID:        util.UUIDToString(authority.AgentRunID),
		CandidateID:       util.UUIDToString(authority.CandidateID),
		CandidateRevision: authority.CandidateRevision,
		SourceRevision:    authority.SourceRevision,
		Worktree: WorktreeProvenance{
			DaemonID:  authority.DaemonID.String,
			RuntimeID: util.UUIDToString(authority.RuntimeID),
			Path:      authority.WorktreePath.String,
			Branch:    authority.Branch.String,
		},
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	publication, err := Seal(binding, claim, now)
	if err != nil {
		return Publication{}, false, err
	}
	policyJSON, err := json.Marshal(publication.DataPolicy)
	if err != nil {
		return Publication{}, false, err
	}
	publicationJSON, err := json.Marshal(publication)
	if err != nil {
		return Publication{}, false, err
	}
	inserted, err := q.InsertPreviewPublication(ctx, db.InsertPreviewPublicationParams{
		WorkspaceID: authority.WorkspaceID, IssueID: authority.IssueID,
		CandidateID: authority.CandidateID, CandidateRevision: authority.CandidateRevision,
		AgentID: authority.AgentID, AgentRunID: authority.AgentRunID,
		SourceRevision: authority.SourceRevision, DaemonID: authority.DaemonID.String,
		RuntimeID: authority.RuntimeID, WorktreePath: authority.WorktreePath.String,
		Branch: authority.Branch.String, PreviewUrl: publication.PreviewURL,
		IdentityUrl: publication.IdentityURL, DataPolicy: policyJSON,
		PublishedAt: pgtype.Timestamptz{Time: publication.PublishedAt, Valid: true},
		ExpiresAt:   pgtype.Timestamptz{Time: publication.ExpiresAt, Valid: true},
		Publication: publicationJSON,
	})
	if err != nil {
		return Publication{}, false, err
	}
	if inserted == 0 {
		existing, err := q.GetPreviewPublicationByIdentity(ctx, db.GetPreviewPublicationByIdentityParams{
			WorkspaceID: authority.WorkspaceID, IssueID: authority.IssueID,
			CandidateID: authority.CandidateID, CandidateRevision: authority.CandidateRevision,
		})
		if err != nil {
			return Publication{}, false, err
		}
		var stored Publication
		if json.Unmarshal(existing.Publication, &stored) != nil || !replayMatches(stored, publication) {
			return Publication{}, false, ErrPublicationConflict
		}
		publication = stored
	}
	if err := tx.Commit(ctx); err != nil {
		return Publication{}, false, err
	}
	return publication, inserted == 0, nil
}

func (s Store) Load(ctx context.Context, workspaceID, issueID pgtype.UUID) (Publication, error) {
	if s.DB == nil || !workspaceID.Valid || !issueID.Valid {
		return Publication{}, ErrPublicationUnavailable
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Publication{}, err
	}
	defer tx.Rollback(ctx)
	return s.loadWithQueries(ctx, db.New(tx), workspaceID, issueID)
}

func (s Store) loadWithQueries(ctx context.Context, q *db.Queries, workspaceID, issueID pgtype.UUID) (Publication, error) {
	row, err := q.GetLatestPreviewPublicationForIssue(ctx, db.GetLatestPreviewPublicationForIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Publication{}, ErrPublicationUnavailable
		}
		return Publication{}, err
	}
	var publication Publication
	if json.Unmarshal(row.Publication, &publication) != nil {
		return Publication{}, ErrPublicationUnavailable
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	if !publication.ExpiresAt.After(now) {
		return Publication{}, ErrPublicationExpired
	}
	eligible, err := q.PreviewPublicationIsEligible(ctx, db.PreviewPublicationIsEligibleParams{
		WorkspaceID: workspaceID, IssueID: issueID,
		CandidateID: row.CandidateID, CandidateRevision: row.CandidateRevision,
	})
	if err != nil {
		return Publication{}, err
	}
	if !eligible || Validate(publication, now) != nil {
		return Publication{}, ErrPublicationUnavailable
	}
	return publication, nil
}
