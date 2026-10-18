package service

import "testing"

func TestChatLibraryMergesAndMoves(t *testing.T) {
	owns := func(id string) bool { return id == "chat" }
	library := EmptyChatLibrary()
	for _, change := range []ChatLibraryChange{{Action: "create_group", ID: "a", Name: "This week"}, {Action: "create_group", ID: "b", Name: "Research"}, {Action: "move_chat", ID: "a", SessionID: "chat"}, {Action: "move_chat", ID: "b", SessionID: "chat"}} {
		var err error
		library, err = ApplyChatLibraryChange(library, change, owns)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(library.Groups) != 2 || len(library.Groups[0].SessionIDs) != 0 || len(library.Groups[1].SessionIDs) != 1 {
		t.Fatalf("lost or duplicated membership: %+v", library)
	}
	for _, change := range []ChatLibraryChange{{Action: "move_chat", ID: "a", SessionID: "foreign"}, {Action: "rename_group", ID: "b", Name: "This week"}, {Action: "create_group", ID: "a", Name: "Overwrite"}, {Action: "set_grouping", Value: "bad"}} {
		if _, err := ApplyChatLibraryChange(library, change, owns); err == nil {
			t.Fatalf("accepted invalid change: %+v", change)
		}
	}
	saved, err := ApplyChatLibraryChange(library, ChatLibraryChange{Action: "delete_group", ID: "b"}, owns)
	if err != nil || len(saved.Groups) != 1 {
		t.Fatal("delete failed")
	}
	if len(library.Groups) != 2 || len(library.Groups[1].SessionIDs) != 1 {
		t.Fatal("mutated input")
	}
}
func TestChatLibraryImportCannotOverwriteOtherDevice(t *testing.T) {
	saved := EmptyChatLibrary()
	saved.Revision = 1
	old := ChatLibrary{Grouping: "custom", Filter: "all", Groups: []ChatLibraryGroup{{ID: "old", Name: "Old", SessionIDs: []string{"foreign"}}}}
	next, err := ApplyChatLibraryChange(saved, ChatLibraryChange{Action: "import", Library: &old}, func(string) bool { return false })
	if err != nil || len(next.Groups) != 0 || next.Revision != 1 {
		t.Fatal("old device replaced saved organization")
	}
	next, err = ApplyChatLibraryChange(EmptyChatLibrary(), ChatLibraryChange{Action: "import", Library: &old}, func(string) bool { return false })
	if err != nil || len(next.Groups) != 1 || len(next.Groups[0].SessionIDs) != 0 {
		t.Fatal("import retained inaccessible chat")
	}
}
