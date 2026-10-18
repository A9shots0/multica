package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestChatSendBindsVisibleScreenshotsDespiteIncompleteUploadList(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Fatal("isolated test database is required")
	}
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "no client list", true: "partial client list"}[partial], func(t *testing.T) {
			agent := createHandlerTestAgent(t, "Screenshot intake", nil)
			session := dbfx.ChatSession(t, agent, testutil.Cols{"explicitly_created_at": testutil.Raw("now()")})
			file := func(owner string, extra testutil.Cols) string {
				cols := testutil.Cols{"workspace_id": testWorkspaceID, "uploader_type": "member", "uploader_id": owner, "filename": "image.png", "url": "https://example.test/file.png", "content_type": "image/png", "size_bytes": 10}
				for key, value := range extra {
					cols[key] = value
				}
				return dbfx.Insert(t, "attachment", cols)
			}
			first, second := file(testUserID, nil), file(testUserID, nil)
			otherOwner := dbfx.User(t, "Other uploader", "other-uploader@example.test")
			foreign := file(otherOwner, nil)
			alreadyBound := file(testUserID, testutil.Cols{"issue_id": dbfx.Issue(t, "Existing file owner")})
			foreignWorkspace := file(testUserID, testutil.Cols{"workspace_id": dbfx.Workspace(t, "Other workspace", "other-screenshot-workspace")})
			content := "![image.png](/api/attachments/" + first + "/download)\n![image.png](/uploads/workspaces/" + testWorkspaceID + "/" + second + ".png)"
			for _, id := range []string{foreign, alreadyBound, foreignWorkspace} {
				content += "\n![unavailable](/api/attachments/" + id + "/download)"
			}
			body := map[string]any{"content": content}
			if partial {
				body["attachment_ids"] = []string{second}
			}
			req := withChatTestWorkspaceCtx(t, withURLParam(newRequest(http.MethodPost, "/api/chat-sessions/"+session+"/messages", body), "sessionId", session))
			var response SendChatMessageResponse
			testutil.Call(t, testHandler.SendChatMessage, req).Want(http.StatusCreated).JSON(&response)
			if len(response.AttachmentIDs) != 2 {
				t.Fatalf("visible files not returned: %v", response.AttachmentIDs)
			}
			if got := dbfx.Count(t, "SELECT count(*) FROM attachment WHERE chat_session_id=$1 AND chat_message_id IS NOT NULL AND id IN ($2,$3)", session, first, second); got != 2 {
				t.Fatalf("claim would receive %d of 2 screenshots", got)
			}
			if got := dbfx.Count(t, "SELECT count(*) FROM attachment WHERE chat_session_id=$1 AND id IN ($2,$3,$4)", session, foreign, alreadyBound, foreignWorkspace); got != 0 {
				t.Fatal("reference parsing bypassed attachment ownership")
			}
		})
	}
}
