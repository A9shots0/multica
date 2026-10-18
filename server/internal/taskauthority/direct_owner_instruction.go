// Package taskauthority defines provider-neutral, server-authenticated task
// authority that is carried identically to every supported agent runtime.
package taskauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const DirectOwnerInstructionVersionV1 = "multica.direct-owner-instruction.v1"

var ErrInvalidDirectOwnerInstruction = errors.New("invalid direct owner instruction receipt")

// InstructionSourceV1 binds one immutable native input row to the exact text
// the owner supplied. Content is repeated intentionally: consumers must never
// recover authority by searching a provider transcript or accepting quoted
// text from another task.
type InstructionSourceV1 struct {
	Kind          string    `json:"kind"`
	ID            string    `json:"id"`
	Revision      int64     `json:"revision,omitempty"`
	Content       string    `json:"content"`
	ContentSHA256 string    `json:"content_sha256"`
	CreatedAt     time.Time `json:"created_at"`
}

// DirectOwnerInstructionV1 proves that the server authenticated a workspace
// owner/admin as the direct author of the task's native input. It authorizes
// only effects the bound instruction explicitly requests. It is not a blanket
// capability grant and cannot authorize unstated interruption, trust-policy,
// credential, budget, destructive-data, or third-party communication effects.
//
// A reply can carry a short answer such as "yes". BoundContextSHA256 ties that
// answer to the exact selected assistant event; the context supplies scope but
// never authority by itself.
type DirectOwnerInstructionV1 struct {
	Version                  string                `json:"version"`
	ReceiptID                string                `json:"receipt_id"`
	TaskID                   string                `json:"task_id"`
	WorkspaceID              string                `json:"workspace_id"`
	IssueID                  string                `json:"issue_id,omitempty"`
	PrincipalUserID          string                `json:"principal_user_id"`
	PrincipalWorkspaceRole   string                `json:"principal_workspace_role"`
	AuthorityClass           string                `json:"authority_class"`
	Coverage                 string                `json:"coverage"`
	Sources                  []InstructionSourceV1 `json:"sources"`
	BoundContextSHA256       string                `json:"bound_context_sha256,omitempty"`
	ExplicitEffectsOnly      bool                  `json:"explicit_effects_only"`
	ReplyContextMayBindScope bool                  `json:"reply_context_may_bind_scope"`
}

type NewDirectOwnerInstructionInput struct {
	TaskID                 string
	WorkspaceID            string
	IssueID                string
	PrincipalUserID        string
	PrincipalWorkspaceRole string
	Sources                []InstructionSourceV1
	BoundContext           json.RawMessage
}

func ContentSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func NewDirectOwnerInstructionV1(input NewDirectOwnerInstructionInput) (DirectOwnerInstructionV1, error) {
	receipt := DirectOwnerInstructionV1{
		Version:                  DirectOwnerInstructionVersionV1,
		TaskID:                   strings.TrimSpace(input.TaskID),
		WorkspaceID:              strings.TrimSpace(input.WorkspaceID),
		IssueID:                  strings.TrimSpace(input.IssueID),
		PrincipalUserID:          strings.TrimSpace(input.PrincipalUserID),
		PrincipalWorkspaceRole:   strings.TrimSpace(input.PrincipalWorkspaceRole),
		AuthorityClass:           "direct_owner_instruction",
		Coverage:                 "effects explicitly requested by the bound native owner instruction",
		Sources:                  append([]InstructionSourceV1(nil), input.Sources...),
		ExplicitEffectsOnly:      true,
		ReplyContextMayBindScope: len(input.BoundContext) > 0,
	}
	for i := range receipt.Sources {
		receipt.Sources[i].Kind = strings.TrimSpace(receipt.Sources[i].Kind)
		receipt.Sources[i].ID = strings.TrimSpace(receipt.Sources[i].ID)
		receipt.Sources[i].ContentSHA256 = ContentSHA256(receipt.Sources[i].Content)
		receipt.Sources[i].CreatedAt = receipt.Sources[i].CreatedAt.UTC().Truncate(time.Microsecond)
	}
	if len(input.BoundContext) > 0 {
		if !json.Valid(input.BoundContext) {
			return DirectOwnerInstructionV1{}, ErrInvalidDirectOwnerInstruction
		}
		sum := sha256.Sum256(input.BoundContext)
		receipt.BoundContextSHA256 = hex.EncodeToString(sum[:])
	}
	if err := receipt.validateCore(); err != nil {
		return DirectOwnerInstructionV1{}, err
	}
	identity := receipt
	identity.ReceiptID = ""
	raw, err := json.Marshal(identity)
	if err != nil {
		return DirectOwnerInstructionV1{}, err
	}
	sum := sha256.Sum256(raw)
	receipt.ReceiptID = "direct-owner-v1." + hex.EncodeToString(sum[:])
	return receipt, nil
}

func (r DirectOwnerInstructionV1) Validate() error {
	if err := r.validateCore(); err != nil || !strings.HasPrefix(r.ReceiptID, "direct-owner-v1.") {
		return ErrInvalidDirectOwnerInstruction
	}
	identity := r
	identity.ReceiptID = ""
	raw, err := json.Marshal(identity)
	if err != nil {
		return ErrInvalidDirectOwnerInstruction
	}
	sum := sha256.Sum256(raw)
	if r.ReceiptID != "direct-owner-v1."+hex.EncodeToString(sum[:]) {
		return ErrInvalidDirectOwnerInstruction
	}
	return nil
}

// BindsContext verifies that the immutable reply-context bytes delivered with
// a task are exactly the bytes the server hashed into this receipt. A context
// hash without its matching context (or vice versa) fails closed.
func (r DirectOwnerInstructionV1) BindsContext(context json.RawMessage) bool {
	if r.BoundContextSHA256 == "" {
		return len(context) == 0
	}
	if len(context) == 0 || !json.Valid(context) {
		return false
	}
	sum := sha256.Sum256(context)
	return r.BoundContextSHA256 == hex.EncodeToString(sum[:])
}

func (r DirectOwnerInstructionV1) validateCore() error {
	if r.Version != DirectOwnerInstructionVersionV1 || r.TaskID == "" || r.WorkspaceID == "" ||
		r.PrincipalUserID == "" || (r.PrincipalWorkspaceRole != "owner" && r.PrincipalWorkspaceRole != "admin") ||
		r.AuthorityClass != "direct_owner_instruction" || r.Coverage == "" || !r.ExplicitEffectsOnly ||
		len(r.Sources) == 0 || r.ReplyContextMayBindScope != (r.BoundContextSHA256 != "") {
		return ErrInvalidDirectOwnerInstruction
	}
	for _, source := range r.Sources {
		if (source.Kind != "chat_message" && source.Kind != "issue_comment") || source.ID == "" ||
			strings.TrimSpace(source.Content) == "" || source.ContentSHA256 != ContentSHA256(source.Content) ||
			source.CreatedAt.IsZero() || (source.Kind == "issue_comment" && source.Revision < 1) {
			return ErrInvalidDirectOwnerInstruction
		}
	}
	if r.BoundContextSHA256 != "" {
		decoded, err := hex.DecodeString(r.BoundContextSHA256)
		if err != nil || len(decoded) != sha256.Size {
			return ErrInvalidDirectOwnerInstruction
		}
	}
	return nil
}
