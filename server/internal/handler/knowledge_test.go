package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveKnowledgeHubFile_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "ok.md"), []byte("# ok"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// A real secret sitting just outside the hub root, exactly the kind of
	// file a traversal payload is trying to reach.
	if err := os.WriteFile(filepath.Join(filepath.Dir(root), "outside-secret.md"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	cases := []struct {
		name   string
		path   string
		wantOK bool
	}{
		{"ordinary nested file", "projects/ok.md", true},
		{"parent traversal", "../outside-secret.md", false},
		{"parent traversal with subdir prefix", "projects/../../outside-secret.md", false},
		// filepath.Join does not special-case a leading separator on a
		// non-first argument — it is not the OS-level meaning of "absolute
		// path", just a path element that starts with a separator — so this
		// joins and Cleans to a path safely inside root, on both Windows and
		// the Linux container this actually runs in. Real traversal payloads
		// use "..", covered by the cases above.
		{"leading slash is not an absolute-path escape", "/etc/passwd.md", true},
		{"empty path", "", false},
		{"non-markdown extension", "projects/ok.txt", false},
		{"traversal disguised without leading dots after join", "projects/../../../../../../etc/passwd.md", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, ok := resolveKnowledgeHubFile(root, tc.path)
			if ok != tc.wantOK {
				t.Fatalf("resolveKnowledgeHubFile(%q) ok = %v, want %v (resolved=%q)", tc.path, ok, tc.wantOK, resolved)
			}
			if ok {
				absRoot, _ := filepath.Abs(root)
				rel, err := filepath.Rel(absRoot, resolved)
				if err != nil || rel == ".." || len(rel) >= 2 && rel[:3] == "../" {
					t.Fatalf("resolved path %q escaped root %q", resolved, absRoot)
				}
			}
		})
	}
}

func TestFirstMarkdownHeading(t *testing.T) {
	dir := t.TempDir()
	withHeading := filepath.Join(dir, "with-heading.md")
	if err := os.WriteFile(withHeading, []byte("\n\n# Real Title\n\nBody text\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	noHeading := filepath.Join(dir, "no-heading.md")
	if err := os.WriteFile(noHeading, []byte("Just prose, no heading marker.\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if got := firstMarkdownHeading(withHeading); got != "Real Title" {
		t.Errorf("firstMarkdownHeading(withHeading) = %q, want %q", got, "Real Title")
	}
	if got := firstMarkdownHeading(noHeading); got != "" {
		t.Errorf("firstMarkdownHeading(noHeading) = %q, want empty", got)
	}
	if got := firstMarkdownHeading(filepath.Join(dir, "missing.md")); got != "" {
		t.Errorf("firstMarkdownHeading(missing) = %q, want empty", got)
	}
}

func TestGetKnowledgeHubTree_SkipsDebrisDirectories(t *testing.T) {
	root := t.TempDir()
	systemsDir := filepath.Join(root, "systems")
	if err := os.MkdirAll(filepath.Join(systemsDir, "agent-evals", "v1"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(systemsDir, "SYSTEM_MAP.md"), []byte("# System Map"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// A file inside the debris directory that would show up if the walk
	// failed to skip it.
	if err := os.WriteFile(filepath.Join(systemsDir, "agent-evals", "v1", "leaked.md"), []byte("# Leaked"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Setenv("KNOWLEDGE_HUB_PATH", root)
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-hub/tree", nil)
	rec := httptest.NewRecorder()
	h.GetKnowledgeHubTree(rec, req)

	var resp knowledgeHubTreeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Configured {
		t.Fatal("expected Configured=true")
	}
	for _, f := range resp.Files {
		if strings.Contains(f.Path, "agent-evals") {
			t.Fatalf("agent-evals leaked into tree: %+v", resp.Files)
		}
	}
	found := false
	for _, f := range resp.Files {
		if f.Path == "systems/SYSTEM_MAP.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected systems/SYSTEM_MAP.md in tree, got %+v", resp.Files)
	}
}

func TestKnowledgeHubRoleSummary(t *testing.T) {
	dir := t.TempDir()

	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		return path
	}

	withCitation := write("with-citation.md", "# CRM Context\n\n- Role:\n  - Central roster and access hub. Source: `C:/Dev/CRM/main.py`.\n- Repository/default branch:\n")
	if got, want := knowledgeHubRoleSummary(withCitation), "Central roster and access hub."; got != want {
		t.Errorf("with-citation summary = %q, want %q", got, want)
	}

	noCitation := write("no-citation.md", "# X\n\n- Role:\n  - Does one thing well.\n")
	if got, want := knowledgeHubRoleSummary(noCitation), "Does one thing well."; got != want {
		t.Errorf("no-citation summary = %q, want %q", got, want)
	}

	blockquoteBefore := write("blockquote-before.md", "# Phone Farm\n\n> An owner note before Role.\n\n- Role:\n  - Mobile fleet operations.\n")
	if got, want := knowledgeHubRoleSummary(blockquoteBefore), "Mobile fleet operations."; got != want {
		t.Errorf("blockquote-before summary = %q, want %q", got, want)
	}

	noRole := write("no-role.md", "# Y\n\nNo role section here at all.\n")
	if got := knowledgeHubRoleSummary(noRole); got != "" {
		t.Errorf("no-role summary = %q, want empty", got)
	}

	long := strings.Repeat("a very long sentence ", 30)
	longFile := write("long.md", "- Role:\n  - "+long+"\n")
	// "…" is U+2026, 3 bytes in UTF-8 — the byte-length budget is the
	// content cap plus that many bytes for the appended ellipsis rune, not +1.
	const ellipsisBytes = len("…")
	if got := knowledgeHubRoleSummary(longFile); len(got) > knowledgeHubRoleSummaryMaxLen+ellipsisBytes {
		t.Errorf("long summary not truncated: len=%d", len(got))
	}
	if got := knowledgeHubRoleSummary(longFile); !strings.HasSuffix(got, "…") {
		t.Errorf("long summary missing ellipsis suffix: %q", got)
	}

	if got := knowledgeHubRoleSummary(filepath.Join(dir, "missing.md")); got != "" {
		t.Errorf("missing file summary = %q, want empty", got)
	}
}
