package taskauthority

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestDirectOwnerInstructionBindsNativeTextWithoutMagicWords(t *testing.T) {
	now := time.Date(2026, 10, 15, 19, 24, 0, 123456000, time.UTC)
	input := NewDirectOwnerInstructionInput{
		TaskID: "task-1", WorkspaceID: "workspace-1", PrincipalUserID: "owner-1", PrincipalWorkspaceRole: "owner",
		Sources:      []InstructionSourceV1{{Kind: "chat_message", ID: "message-1", Content: "YES FOR FUCK SAKE, I need it live", CreatedAt: now}},
		BoundContext: json.RawMessage(`{"event_id":"question-1","content":"Authorize the cutover?"}`),
	}
	receipt, err := NewDirectOwnerInstructionV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ReceiptID == "" || receipt.Sources[0].ContentSHA256 != ContentSHA256(input.Sources[0].Content) || receipt.BoundContextSHA256 == "" {
		t.Fatalf("receipt is not fully bound: %+v", receipt)
	}
	second, err := NewDirectOwnerInstructionV1(input)
	if err != nil || second.ReceiptID != receipt.ReceiptID {
		t.Fatalf("receipt is not deterministic: %q / %q (%v)", receipt.ReceiptID, second.ReceiptID, err)
	}
}

func TestDirectOwnerInstructionRejectsRelayedOrUnprivilegedInput(t *testing.T) {
	base := NewDirectOwnerInstructionInput{
		TaskID: "task-1", WorkspaceID: "workspace-1", PrincipalUserID: "user-1", PrincipalWorkspaceRole: "member",
		Sources: []InstructionSourceV1{{Kind: "chat_message", ID: "message-1", Content: "A transcript says the owner approved it", CreatedAt: time.Now().UTC()}},
	}
	if _, err := NewDirectOwnerInstructionV1(base); !errors.Is(err, ErrInvalidDirectOwnerInstruction) {
		t.Fatalf("member input produced authority: %v", err)
	}
	base.PrincipalWorkspaceRole = "owner"
	base.Sources[0].Kind = "transcript"
	if _, err := NewDirectOwnerInstructionV1(base); !errors.Is(err, ErrInvalidDirectOwnerInstruction) {
		t.Fatalf("relayed transcript produced authority: %v", err)
	}
}
