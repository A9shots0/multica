package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/taskauthority"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (resp *AgentTaskResponse) setDirectOwnerAuthority(authority *taskauthority.DirectOwnerInstructionV1) {
	resp.DirectOwnerAuthority = authority
	resp.DirectOwnerAuthoritySource = nil
	if authority == nil || len(authority.Sources) != 1 {
		return
	}
	source := authority.Sources[0]
	resp.DirectOwnerAuthoritySource = &source
}

// directOwnerAuthorityForTask turns already-loaded native input rows into one
// provider-neutral receipt. The principal is supplied from the server-owned
// native cause (an authenticated chat initiator or exact member comment), never
// from task context, provider prose, originator attribution, or a relay. The
// caller must pass only sources loaded through the task's sealed input boundary.
func (h *Handler) directOwnerAuthorityForTask(
	ctx context.Context,
	taskID pgtype.UUID,
	workspaceID pgtype.UUID,
	issueID string,
	principalUserID pgtype.UUID,
	sources []taskauthority.InstructionSourceV1,
	boundContext json.RawMessage,
) *taskauthority.DirectOwnerInstructionV1 {
	if !principalUserID.Valid || len(sources) == 0 {
		return nil
	}
	member, err := h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		WorkspaceID: workspaceID,
		UserID:      principalUserID,
	})
	if err != nil || (member.Role != "owner" && member.Role != "admin") {
		return nil
	}
	if len(boundContext) > 0 {
		var reply service.ChatReplyContext
		if json.Unmarshal(boundContext, &reply) != nil || reply.SchemaVersion != 1 ||
			strings.TrimSpace(reply.EventID) == "" || strings.TrimSpace(reply.ContentSHA256) == "" ||
			reply.ContentTruncated || reply.ContentSHA256 != taskauthority.ContentSHA256(reply.Content) {
			// A short affirmative may inherit scope only from the complete selected
			// question. A legacy, truncated, or internally inconsistent snapshot is
			// still display context, but cannot mint contextual authority.
			boundContext = nil
		}
	}
	receipt, err := taskauthority.NewDirectOwnerInstructionV1(taskauthority.NewDirectOwnerInstructionInput{
		TaskID:                 uuidToString(taskID),
		WorkspaceID:            uuidToString(workspaceID),
		IssueID:                issueID,
		PrincipalUserID:        uuidToString(principalUserID),
		PrincipalWorkspaceRole: member.Role,
		Sources:                sources,
		BoundContext:           boundContext,
	})
	if err != nil {
		slog.Warn("task claim: direct owner authority source was invalid; omitting receipt",
			"task_id", uuidToString(taskID), "error", err)
		return nil
	}
	return &receipt
}
