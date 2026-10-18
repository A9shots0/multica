package handler

import (
	"strings"
	"testing"
)

func TestBuildChatSearchQuery_SingleTerm(t *testing.T) {
	query, args := buildChatSearchQuery("Deploy", []string{"Deploy"}, false)

	if args[0] != "deploy" {
		t.Errorf("expected phrase arg to be lowercased, got %q", args[0])
	}
	if strings.Contains(query, "ILIKE") {
		t.Error("query should not contain ILIKE")
	}
	if !strings.Contains(query, "LOWER(s.title) LIKE") {
		t.Error("query should match against the session title")
	}
	if !strings.Contains(query, "LOWER(m.content) LIKE") {
		t.Error("query should match against message content")
	}
	// Excludes archived by default.
	if !strings.Contains(query, "s.status <> 'archived'") {
		t.Error("query should exclude archived sessions by default")
	}
	// Scoped to the requester's own sessions — chat is private, not
	// workspace-shared like issues/projects.
	if !strings.Contains(query, "s.creator_id =") {
		t.Error("query must scope to creator_id — a chat search must never cross users")
	}
	if !strings.Contains(query, "cs.creator_id =") {
		t.Error("the matched_messages CTE must also scope to creator_id")
	}
}

func TestBuildChatSearchQuery_IncludeArchived(t *testing.T) {
	query, _ := buildChatSearchQuery("Deploy", []string{"Deploy"}, true)

	if strings.Contains(query, "status <> 'archived'") {
		t.Error("includeArchived=true should not filter out archived sessions")
	}
}

func TestBuildChatSearchQuery_ArgOrder(t *testing.T) {
	_, args := buildChatSearchQuery("hello world", []string{"hello", "world"}, false)

	// phrase, ws, user, then per-term params, then limit, offset.
	if len(args) < 6 {
		t.Fatalf("expected at least 6 args (phrase, ws, user, 2 terms, limit, offset), got %d", len(args))
	}
	if args[0] != "hello world" {
		t.Errorf("args[0] should be the lowercased phrase, got %v", args[0])
	}
	// args[1] and args[2] are ws/user placeholders (nil until the handler
	// fills them in) — SearchChatSessions relies on this fixed position.
	if args[1] != nil || args[2] != nil {
		t.Errorf("args[1]/args[2] should be nil placeholders for ws/user, got %v, %v", args[1], args[2])
	}
	last, secondLast := args[len(args)-1], args[len(args)-2]
	if last != nil || secondLast != nil {
		t.Errorf("last two args should be nil placeholders for limit/offset, got %v, %v", secondLast, last)
	}
}

func TestBuildChatSearchQuery_MultiWordRequiresAllTerms(t *testing.T) {
	query, _ := buildChatSearchQuery("deploy pipeline", []string{"deploy", "pipeline"}, false)

	if !strings.Contains(query, " AND ") {
		t.Error("a multi-word query should AND the per-term title conditions together")
	}
}
