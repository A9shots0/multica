package handler

import (
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestExactChatReplyContextPreservesSingleOwnedSnapshot(t *testing.T) {
	raw := []byte(`{"schema_version":1,"event_id":"event-1","actor":"assistant","message_kind":"message","content":"older exact answer","created_at":"2026-10-04T12:00:00Z","attachments":[],"artifacts":[]}`)
	got, err := exactChatReplyContext([]db.ChatMessage{{Content: "hidden kickoff"}, {Content: "current request", ReplyContext: raw}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("reply context changed: %s", got)
	}
}

func TestExactChatReplyContextFailsClosedOnCorruptInput(t *testing.T) {
	for name, messages := range map[string][]db.ChatMessage{
		"invalid JSON":     {{ReplyContext: []byte(`{"broken"`)}},
		"multiple targets": {{ReplyContext: []byte(`{"event_id":"one"}`)}, {ReplyContext: []byte(`{"event_id":"two"}`)}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := exactChatReplyContext(messages); err == nil {
				t.Fatal("expected corrupt reply context to fail")
			}
		})
	}
}
