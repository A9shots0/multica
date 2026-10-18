package service

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type ChatLibraryGroup struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	SessionIDs []string `json:"sessionIds"`
}
type ChatLibrary struct {
	Grouping string             `json:"grouping"`
	Filter   string             `json:"filter"`
	Groups   []ChatLibraryGroup `json:"groups"`
	Revision int64              `json:"revision"`
}
type ChatLibraryChange struct {
	Action    string       `json:"action"`
	ID        string       `json:"id,omitempty"`
	Name      string       `json:"name,omitempty"`
	SessionID string       `json:"session_id,omitempty"`
	Value     string       `json:"value,omitempty"`
	Library   *ChatLibrary `json:"library,omitempty"`
}

func EmptyChatLibrary() ChatLibrary {
	return ChatLibrary{Grouping: "recent", Filter: "all", Groups: []ChatLibraryGroup{}}
}

// Each action merges into a locked current row. Concurrent changes to different
// groups never replace each other's whole library; repeated commands are safe.
func ApplyChatLibraryChange(current ChatLibrary, change ChatLibraryChange, owns func(string) bool) (ChatLibrary, error) {
	next := current
	next.Groups = make([]ChatLibraryGroup, len(current.Groups))
	for i, g := range current.Groups {
		next.Groups[i] = g
		next.Groups[i].SessionIDs = append([]string{}, g.SessionIDs...)
	}
	index := -1
	for i, g := range next.Groups {
		if g.ID == change.ID {
			index = i
		}
	}
	switch change.Action {
	case "set_grouping":
		next.Grouping = change.Value
	case "set_filter":
		next.Filter = change.Value
	case "create_group":
		if index >= 0 {
			if next.Groups[index].Name != strings.TrimSpace(change.Name) {
				return current, errors.New("group identity already exists")
			}
			return next, nil
		}
		next.Groups = append(next.Groups, ChatLibraryGroup{ID: change.ID, Name: strings.TrimSpace(change.Name), SessionIDs: []string{}})
		next.Grouping = "custom"
	case "rename_group":
		if index < 0 {
			return current, errors.New("group no longer exists")
		}
		next.Groups[index].Name = strings.TrimSpace(change.Name)
	case "delete_group":
		if index >= 0 {
			next.Groups = append(next.Groups[:index], next.Groups[index+1:]...)
		}
	case "move_chat":
		if change.ID != "" && index < 0 {
			return current, errors.New("group no longer exists")
		}
		if !owns(change.SessionID) {
			return current, errors.New("conversation is unavailable")
		}
		for i, g := range next.Groups {
			ids := []string{}
			for _, id := range g.SessionIDs {
				if id != change.SessionID {
					ids = append(ids, id)
				}
			}
			next.Groups[i].SessionIDs = ids
		}
		if index >= 0 {
			next.Groups[index].SessionIDs = append(next.Groups[index].SessionIDs, change.SessionID)
		}
	case "import":
		if current.Revision != 0 {
			return current, nil
		}
		if change.Library == nil {
			return current, errors.New("library is required")
		}
		next = *change.Library
		next.Groups = append([]ChatLibraryGroup{}, change.Library.Groups...)
		next.Revision = current.Revision
		// Older device preferences may reference deleted chats; retain only owned ones.
		for i, g := range next.Groups {
			ids := []string{}
			for _, id := range g.SessionIDs {
				if owns(id) {
					ids = append(ids, id)
				}
			}
			next.Groups[i].SessionIDs = ids
		}
	default:
		return current, errors.New("unknown library action")
	}
	if next.Grouping != "recent" && next.Grouping != "project" && next.Grouping != "custom" {
		return current, errors.New("invalid grouping")
	}
	if next.Filter != "all" && next.Filter != "unread" && next.Filter != "working" && next.Filter != "needs_you" {
		return current, errors.New("invalid filter")
	}
	if len(next.Groups) > 100 {
		return current, errors.New("at most 100 groups are supported")
	}
	seenIDs, seenNames, seenChats := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, g := range next.Groups {
		g.Name = strings.TrimSpace(g.Name)
		key := strings.ToLower(g.Name)
		if g.ID == "" || len(g.ID) > 128 || g.Name == "" || utf8.RuneCountInString(g.Name) > 60 || seenIDs[g.ID] || seenNames[key] {
			return current, errors.New("groups need unique identities and names of 1 to 60 characters")
		}
		seenIDs[g.ID] = true
		seenNames[key] = true
		for _, id := range g.SessionIDs {
			if seenChats[id] {
				return current, errors.New("a conversation can belong to only one custom group")
			}
			seenChats[id] = true
		}
		next.Groups[i] = g
	}
	if len(seenChats) > 10000 {
		return current, errors.New("too many grouped conversations")
	}
	next.Revision = current.Revision
	return next, nil
}
