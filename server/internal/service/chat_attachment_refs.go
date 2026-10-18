package service

import (
	"net/url"
	"strings"

	"github.com/multica-ai/multica/server/internal/util"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// The visible message is durable input. A composer's transient upload list can
// be incomplete, so recover its explicit file references.
// These are candidates only: LinkAttachmentsToChatMessage still requires the
// same workspace/uploader and an unbound row. No URL is fetched here.
func chatAttachmentReferenceIDs(content, workspaceID string) []string {
	root := goldmark.DefaultParser().Parse(text.NewReader([]byte(content)))
	var ids []string
	seen := make(map[string]bool)
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var destination []byte
		switch node := node.(type) {
		case *ast.Image:
			destination = node.Destination
		case *ast.Link:
			destination = node.Destination
		default:
			return ast.WalkContinue, nil
		}
		id, valid := util.AttachmentIDFromDownloadURL(string(destination))
		if !valid {
			parsed, err := url.Parse(string(destination))
			if err != nil || (parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https") {
				return ast.WalkContinue, nil
			}
			prefix := "/uploads/workspaces/" + workspaceID + "/"
			if !strings.HasPrefix(parsed.Path, prefix) {
				return ast.WalkContinue, nil
			}
			filename := strings.TrimPrefix(parsed.Path, prefix)
			if strings.Contains(filename, "/") {
				return ast.WalkContinue, nil
			}
			candidate, _, _ := strings.Cut(filename, ".")
			parsedID, err := util.ParseUUID(candidate)
			if err != nil {
				return ast.WalkContinue, nil
			}
			id = util.UUIDToString(parsedID)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
		return ast.WalkContinue, nil
	})
	return ids
}
