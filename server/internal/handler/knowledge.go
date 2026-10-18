package handler

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// knowledgeHubTopLevel is the allowlist of what actually IS the knowledge hub
// (projects/registry.json, systems/SYSTEM_MAP.md, decisions/DECISIONS.md, and
// friends — see projects/intelligence/CONTEXT.md's own audit of this same
// directory, written the session this page was built). The hub's repo root
// also carries ~18,800 files of leftover agent-task sandbox debris
// (.task25-*, .task44-canary-*, .pytest_cache, ...) that were never part of
// "the knowledge hub" in the first place; walking the whole root would serve
// that debris as if it were curated content. Listed explicitly rather than
// excluded by pattern so a new debris shape added later doesn't silently
// leak in — only paths named here are ever walked.
var knowledgeHubTopLevel = []string{
	"START_HERE.md",
	"GOVERNANCE.md",
	"CHANGELOG.md",
	"CURRENT_STATE.md",
	"projects",
	"systems",
	"decisions",
	"truth",
	"runtime",
	"initiatives",
	"inbox",
}

type knowledgeHubNode struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

type knowledgeHubTreeResponse struct {
	Configured bool               `json:"configured"`
	Files      []knowledgeHubNode `json:"files"`
}

// firstMarkdownHeading reads only as far as the first "# " line (or 64 lines,
// whichever comes first — a title lives at the top or not at all) so listing
// a large hub does not mean reading every file in full.
func firstMarkdownHeading(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for lines := 0; scanner.Scan() && lines < 64; lines++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// GetKnowledgeHubTree lists every markdown file under the mounted knowledge
// hub's allowlisted top-level entries (see knowledgeHubTopLevel). Read-only,
// owner-facing: the hub is a bind mount of the same repo agents already read
// directly (C:\Dev\Silver-Mediums-Knowledge), not a Multica-owned copy —
// this page is a viewer, not an editor.
func (h *Handler) GetKnowledgeHubTree(w http.ResponseWriter, r *http.Request) {
	root := strings.TrimSpace(os.Getenv("KNOWLEDGE_HUB_PATH"))
	if root == "" {
		writeJSON(w, http.StatusOK, knowledgeHubTreeResponse{Configured: false})
		return
	}

	var files []knowledgeHubNode
	for _, entry := range knowledgeHubTopLevel {
		entryPath := filepath.Join(root, entry)
		info, err := os.Stat(entryPath)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if strings.HasSuffix(strings.ToLower(entry), ".md") {
				files = append(files, knowledgeHubNode{
					Path:    entry,
					Title:   firstOr(firstMarkdownHeading(entryPath), entry),
					Size:    info.Size(),
					ModTime: info.ModTime().UTC().Format(time.RFC3339),
				})
			}
			continue
		}
		filepath.Walk(entryPath, func(path string, walkInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if walkInfo.IsDir() {
				// systems/agent-evals alone holds 1,069 of the 1,104 files
				// under systems/ — versioned eval run output (v1..v5r1,
				// runs/, .pytest_cache), not curated content, discovered
				// live when it made this endpoint hang for 15s+ walking
				// (and stat-ing, over a Docker bind mount) every one of
				// them. Skipped by name for the same reason
				// knowledgeHubTopLevel is explicit rather than pattern-based:
				// a directory this shape can reappear anywhere in the hub,
				// and guessing at a pattern is how the NEXT one slips
				// through silently. isSkippedKnowledgeHubDir also names
				// hidden/cache dirs generically on top of that.
				if isSkippedKnowledgeHubDir(walkInfo.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(path), ".md") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			files = append(files, knowledgeHubNode{
				Path:    rel,
				Title:   firstOr(firstMarkdownHeading(path), filepath.Base(path)),
				Size:    walkInfo.Size(),
				ModTime: walkInfo.ModTime().UTC().Format(time.RFC3339),
			})
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	writeJSON(w, http.StatusOK, knowledgeHubTreeResponse{Configured: true, Files: files})
}

// knowledgeHubRegistryProject mirrors the fields of one projects/registry.json
// entry this handler actually uses. `.loose()`-equivalent: extra registry
// fields (remote_sha, recovery_bundle, ...) are agent-facing provenance, not
// anything a human summary card needs, and are silently dropped by omission
// rather than modeled here.
type knowledgeHubRegistryProject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Context   string `json:"context"`
	Changelog string `json:"changelog"`
}

type knowledgeHubRegistry struct {
	Projects []knowledgeHubRegistryProject `json:"projects"`
}

type knowledgeHubProjectSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Context   string `json:"context"`
	Changelog string `json:"changelog"`
}

type knowledgeHubProjectsResponse struct {
	Configured bool                         `json:"configured"`
	Projects   []knowledgeHubProjectSummary `json:"projects"`
}

// knowledgeHubRoleSummaryMaxLen bounds a card summary to roughly two lines —
// long enough to say what a project IS, short enough to scan eight of them
// on one screen without reading prose.
const knowledgeHubRoleSummaryMaxLen = 220

// knowledgeHubRoleSummary extracts the one-line "what is this project"
// answer every CONTEXT.md in the hub already leads with: a "- Role:" bullet
// followed by an indented description line. Written for an agent audience,
// so it routinely trails off into inline source citations
// ("... Source: `C:\Dev\CRM\main.py`.") — those are cut at the first
// recognized citation marker, since they say WHERE the claim came from, not
// WHAT the project does, and a human card has no use for a file path.
func knowledgeHubRoleSummary(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	foundRoleLabel := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !foundRoleLabel {
			if line == "- Role:" {
				foundRoleLabel = true
			}
			continue
		}
		if line == "" {
			continue
		}
		summary := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		for _, marker := range []string{" Source:", " Sources:", " Source `", " Sources `"} {
			if idx := strings.Index(summary, marker); idx > 0 {
				summary = summary[:idx]
			}
		}
		summary = strings.TrimSpace(summary)
		if len(summary) > knowledgeHubRoleSummaryMaxLen {
			summary = strings.TrimSpace(summary[:knowledgeHubRoleSummaryMaxLen]) + "…"
		}
		return summary
	}
	return ""
}

// GetKnowledgeHubProjects is the Company Knowledge tab's landing view: one
// card per registered project (projects/registry.json) instead of the raw
// file tree GetKnowledgeHubTree serves. The tree is hundreds of
// agent-audit-trail files with dated, jargon-heavy names
// ("2026-07-15-crm-accounts-phase2-runtime-candidate.md") — real and
// intentional for the agents that write them, unusable as a human's first
// screen. A project list of "what does my company know about CRM" is the
// question an owner actually has.
func (h *Handler) GetKnowledgeHubProjects(w http.ResponseWriter, r *http.Request) {
	root := strings.TrimSpace(os.Getenv("KNOWLEDGE_HUB_PATH"))
	if root == "" {
		writeJSON(w, http.StatusOK, knowledgeHubProjectsResponse{Configured: false})
		return
	}

	registryPath := filepath.Join(root, "projects", "registry.json")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		writeJSON(w, http.StatusOK, knowledgeHubProjectsResponse{Configured: true})
		return
	}
	var registry knowledgeHubRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		writeJSON(w, http.StatusOK, knowledgeHubProjectsResponse{Configured: true})
		return
	}

	projects := make([]knowledgeHubProjectSummary, 0, len(registry.Projects))
	for _, p := range registry.Projects {
		summary := ""
		if p.Context != "" {
			summary = knowledgeHubRoleSummary(filepath.Join(root, filepath.FromSlash(p.Context)))
		}
		projects = append(projects, knowledgeHubProjectSummary{
			ID:        p.ID,
			Name:      p.Name,
			Summary:   summary,
			Context:   p.Context,
			Changelog: p.Changelog,
		})
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })

	writeJSON(w, http.StatusOK, knowledgeHubProjectsResponse{Configured: true, Projects: projects})
}

// knowledgeHubSkippedDirNames are directories that are debris wherever they
// appear in the hub, discovered by name rather than assumed exhaustive —
// see the comment at the GetKnowledgeHubTree call site for how agent-evals
// was found.
var knowledgeHubSkippedDirNames = map[string]bool{
	"agent-evals":   true,
	".pytest_cache": true,
	"__pycache__":   true,
	"node_modules":  true,
}

func isSkippedKnowledgeHubDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return knowledgeHubSkippedDirNames[name]
}

func firstOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// resolveKnowledgeHubFile validates a caller-supplied relative path against
// path traversal before it ever reaches the filesystem: joins it onto root,
// then re-derives the relative path from the ABSOLUTE join and rejects
// anything that climbs outside root (a leading ".." after Rel) or isn't a
// .md file. filepath.Join alone already collapses "..", but re-deriving and
// checking is what actually proves the result stayed under root rather than
// trusting that collapsing did what was intended.
func resolveKnowledgeHubFile(root, requestedPath string) (string, bool) {
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" || !strings.HasSuffix(strings.ToLower(requestedPath), ".md") {
		return "", false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	joined := filepath.Join(absRoot, requestedPath)
	rel, err := filepath.Rel(absRoot, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return joined, true
}

// GetKnowledgeHubFile returns one markdown file's raw content, so the
// frontend can render it through the same markdown pipeline chat/issues
// already use. ?path= is relative to the mounted hub root and validated by
// resolveKnowledgeHubFile — it does not have to be one of the tree's listed
// paths (a file added to the hub after the last tree fetch still resolves),
// only inside the mounted root and a .md file.
func (h *Handler) GetKnowledgeHubFile(w http.ResponseWriter, r *http.Request) {
	root := strings.TrimSpace(os.Getenv("KNOWLEDGE_HUB_PATH"))
	if root == "" {
		writeError(w, http.StatusServiceUnavailable, "knowledge hub is not configured")
		return
	}
	resolved, ok := resolveKnowledgeHubFile(root, r.URL.Query().Get("path"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	// 4 MiB cap: every real file in this hub is prose/markdown (KB-scale);
	// anything larger reads as a mistake, not a slow legitimate document.
	info, err := os.Stat(resolved)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if info.Size() > 4<<20 {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large to display")
		return
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":     r.URL.Query().Get("path"),
		"content":  string(content),
		"mod_time": info.ModTime().UTC().Format(time.RFC3339),
	})
}
