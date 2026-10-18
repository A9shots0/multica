package handler

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Routes are infrastructure configuration, not an agent's task input. IDs are
// workspace-scoped and canonical so aliases/names cannot substitute a host.
func (h *Handler) validateProjectExecutionRoutes(ctx context.Context, workspaceID pgtype.UUID, routes map[string][]string) error {
	if len(routes) > 64 {
		return fmt.Errorf("at most 64 project execution routes are allowed")
	}
	runtimes, err := h.Queries.ListAgentRuntimes(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("cannot load workspace machines")
	}
	known := map[string]bool{}
	for _, rt := range runtimes {
		if rt.DaemonID.Valid {
			known[rt.DaemonID.String] = true
		}
	}
	for projectID, daemonIDs := range routes {
		id, err := uuid.Parse(projectID)
		if err != nil || id.String() != projectID {
			return fmt.Errorf("project execution route requires a canonical project ID")
		}
		if _, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: parseUUID(projectID), WorkspaceID: workspaceID}); err != nil {
			return fmt.Errorf("execution route project does not belong to this workspace")
		}
		if len(daemonIDs) > 8 {
			return fmt.Errorf("at most 8 machines are allowed per project")
		}
		seen := map[string]bool{}
		for _, daemonID := range daemonIDs {
			id, err := uuid.Parse(daemonID)
			if err != nil || id.String() != daemonID || !known[daemonID] || seen[daemonID] {
				return fmt.Errorf("execution route requires distinct registered workspace machine IDs")
			}
			seen[daemonID] = true
		}
	}
	return nil
}
