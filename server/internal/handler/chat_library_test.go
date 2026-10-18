package handler

import (
	"context"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/http"
	"testing"
)

func TestChatLibraryDurableMergeAndOwnership(t *testing.T) {
	dbfx.Exec(t, `DELETE FROM chat_library WHERE workspace_id=$1 AND owner_id=$2`, testWorkspaceID, testUserID)
	t.Cleanup(func() {
		dbfx.Exec(t, `DELETE FROM chat_library WHERE workspace_id=$1 AND owner_id=$2`, testWorkspaceID, testUserID)
	})
	_, session, _, _ := setupDirectChatSession(t, context.Background(), "Grouped chat")
	change := func(input map[string]any, status int) service.ChatLibrary {
		t.Helper()
		r := withChatTestWorkspaceCtx(t, newRequest("PATCH", "/api/chat/library", input))
		var saved service.ChatLibrary
		result := testutil.Call(t, testHandler.ChangeChatLibrary, r).Want(status)
		if status == 200 {
			result.JSON(&saved)
		}
		return saved
	}
	change(map[string]any{"action": "create_group", "id": "one", "name": "Operations"}, 200)
	// A second device only changes the named group; it does not replace the first.
	saved := change(map[string]any{"action": "create_group", "id": "two", "name": "Ideas", "session_id": session}, 200)
	if len(saved.Groups) != 2 || saved.Revision != 2 {
		t.Fatalf("lost merged groups: %+v", saved)
	}
	saved = change(map[string]any{"action": "import", "library": service.EmptyChatLibrary()}, 200)
	if len(saved.Groups) != 2 {
		t.Fatal("old device replaced current library")
	}
	change(map[string]any{"action": "move_chat", "id": "one", "session_id": "00000000-0000-0000-0000-000000000001"}, 400)
	change(map[string]any{"action": "rename_group", "id": "one", "name": "ideas"}, 400)
	saved = change(map[string]any{"action": "move_chat", "id": "one", "session_id": session}, 200)
	if len(saved.Groups[0].SessionIDs) != 1 || len(saved.Groups[1].SessionIDs) != 0 {
		t.Fatal("move duplicated membership")
	}
	var reread service.ChatLibrary
	testutil.Call(t, testHandler.GetChatLibrary, withChatTestWorkspaceCtx(t, newRequest("GET", "/api/chat/library", nil))).Want(200).JSON(&reread)
	if reread.Revision != saved.Revision || len(reread.Groups[0].SessionIDs) != 1 {
		t.Fatal("organization did not persist")
	}
	change(map[string]any{"action": "delete_group", "id": "one"}, 200)
	if _, err := testHandler.Queries.GetChatSession(context.Background(), parseUUID(session)); err != nil {
		t.Fatal("removing group deleted chat")
	}
	r := withChatTestWorkspaceCtx(t, newRequest(http.MethodGet, "/api/chat/library", nil))
	r.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.GetChatLibrary, r).Want(403)
}
