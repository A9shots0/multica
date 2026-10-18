package agentqualification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const RealisticBrowserBridgeSchemaVersion = "silver-mediums.realistic-browser-bridge.v1"
const RealisticBrowserPreviewSchemaVersion = "silver-mediums.realistic-browser-preview.v1"
const RealisticBrowserDOMSnapshotSchemaVersion = "silver-mediums.realistic-browser-dom.v1"
const RealisticBrowserEvidenceSchemaVersion = "silver-mediums.realistic-browser-evidence.v1"

var ErrRealisticBrowserAttestationUnavailable = fmt.Errorf("trusted in-app Browser capture attestation is unavailable in the CLI harness")

type RealisticBrowserRequest struct {
	SchemaVersion       string   `json:"schema_version"`
	CaseID              string   `json:"case_id"`
	RunFamilyID         string   `json:"run_family_id"`
	WorkspaceTreeSHA256 string   `json:"workspace_tree_sha256"`
	EntryPath           string   `json:"entry_path"`
	Viewports           []string `json:"viewports"`
	BrowserSurface      string   `json:"browser_surface"`
}

type RealisticBrowserPreviewReceipt struct {
	SchemaVersion       string `json:"schema_version"`
	RunFamilyID         string `json:"run_family_id"`
	WorkspaceTreeSHA256 string `json:"workspace_tree_sha256"`
	RequestSHA256       string `json:"request_sha256"`
	URL                 string `json:"url"`
	NetworkBoundary     string `json:"network_boundary"`
}

type RealisticBrowserDOMSnapshot struct {
	SchemaVersion       string `json:"schema_version"`
	RequestSHA256       string `json:"request_sha256"`
	URL                 string `json:"url"`
	Viewport            string `json:"viewport"`
	DocumentHTML        string `json:"document_html"`
	DocumentScrollWidth int    `json:"document_scroll_width"`
	DocumentClientWidth int    `json:"document_client_width"`
	ActualViewportWidth int    `json:"actual_viewport_width"`
}

type RealisticBrowserCapture struct {
	Viewport                string `json:"viewport"`
	ScreenshotArtifactPath  string `json:"screenshot_artifact_path"`
	ScreenshotSHA256        string `json:"screenshot_sha256"`
	DOMSnapshotArtifactPath string `json:"dom_snapshot_artifact_path"`
	DOMSnapshotSHA256       string `json:"dom_snapshot_sha256"`
}

type RealisticBrowserEvidence struct {
	SchemaVersion string                    `json:"schema_version"`
	RequestSHA256 string                    `json:"request_sha256"`
	PreviewSHA256 string                    `json:"preview_sha256"`
	Captures      []RealisticBrowserCapture `json:"captures"`
}

func validRealisticViewportWidth(viewport string, width int) bool {
	ranges := map[string][2]int{"narrow": {320, 480}, "medium": {700, 1024}, "wide": {1200, 1920}}
	allowed, ok := ranges[viewport]
	return ok && width >= allowed[0] && width <= allowed[1]
}

// FileRealisticBrowserCollector creates a sealed handoff for the app's built-in
// Browser. It never launches a browser or accepts candidate-authored evidence.
type FileRealisticBrowserCollector struct {
	RunFamilyID  string
	ArtifactRoot string
}

func (collector FileRealisticBrowserCollector) Capture(_ context.Context, workspaceRoot, entryPath string) ([]RealisticBrowserProof, error) {
	if collector.RunFamilyID == "" || !filepath.IsAbs(collector.ArtifactRoot) || !filepath.IsAbs(workspaceRoot) || !safeFixtureRelativePath(entryPath) {
		return nil, fmt.Errorf("Browser bridge identity or path is invalid")
	}
	treeHash, err := RealisticFixtureTreeSHA256(os.DirFS(workspaceRoot), ".")
	if err != nil {
		return nil, err
	}
	request := RealisticBrowserRequest{SchemaVersion: RealisticBrowserBridgeSchemaVersion, CaseID: "R02", RunFamilyID: collector.RunFamilyID, WorkspaceTreeSHA256: treeHash, EntryPath: filepath.ToSlash(entryPath), Viewports: []string{"narrow", "medium", "wide"}, BrowserSurface: "chatgpt_in_app_browser_only"}
	requestRaw, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(collector.ArtifactRoot, 0o700); err != nil {
		return nil, err
	}
	requestPath := filepath.Join(collector.ArtifactRoot, "browser-request.json")
	evidencePath := filepath.Join(collector.ArtifactRoot, "browser-evidence.json")
	if existing, readErr := os.ReadFile(requestPath); readErr == nil && !bytes.Equal(existing, requestRaw) {
		if err := archiveRealisticBrowserCycle(collector.ArtifactRoot, existing); err != nil {
			return nil, err
		}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}
	if err := writeImmutableOrEqual(requestPath, requestRaw); err != nil {
		return nil, err
	}
	_, err = os.Stat(evidencePath)
	if os.IsNotExist(err) {
		return nil, ErrRealisticBrowserEvidencePending
	}
	if err != nil {
		return nil, err
	}
	requestHash := sha256.Sum256(requestRaw)
	requestSHA := hex.EncodeToString(requestHash[:])
	previewPath := filepath.Join(collector.ArtifactRoot, "browser-preview.json")
	previewRaw, err := os.ReadFile(previewPath)
	if err != nil {
		return nil, ErrRealisticBrowserAttestationUnavailable
	}
	var preview RealisticBrowserPreviewReceipt
	if json.Unmarshal(previewRaw, &preview) != nil || !validRealisticLoopbackPreview(preview, request, requestSHA) {
		return nil, fmt.Errorf("trusted Browser preview receipt is invalid")
	}
	previewHash := sha256.Sum256(previewRaw)
	var evidence RealisticBrowserEvidence
	evidenceRaw, err := os.ReadFile(evidencePath)
	if err != nil || json.Unmarshal(evidenceRaw, &evidence) != nil || evidence.SchemaVersion != RealisticBrowserEvidenceSchemaVersion || evidence.RequestSHA256 != requestSHA || evidence.PreviewSHA256 != hex.EncodeToString(previewHash[:]) || len(evidence.Captures) != 3 {
		return nil, ErrRealisticBrowserAttestationUnavailable
	}
	proofs := make([]RealisticBrowserProof, 0, 3)
	seen := map[string]bool{}
	for _, capture := range evidence.Captures {
		if seen[capture.Viewport] || !contains(request.Viewports, capture.Viewport) {
			return nil, ErrRealisticBrowserAttestationUnavailable
		}
		seen[capture.Viewport] = true
		if err := verifyBrowserArtifact(collector.ArtifactRoot, capture.ScreenshotArtifactPath, capture.ScreenshotSHA256); err != nil {
			return nil, ErrRealisticBrowserAttestationUnavailable
		}
		if err := verifyBrowserArtifact(collector.ArtifactRoot, capture.DOMSnapshotArtifactPath, capture.DOMSnapshotSHA256); err != nil {
			return nil, ErrRealisticBrowserAttestationUnavailable
		}
		total, cards, overflow, err := deriveRealisticBrowserFacts(collector.ArtifactRoot, capture.DOMSnapshotArtifactPath, requestSHA, preview.URL, capture.Viewport)
		if err != nil {
			return nil, ErrRealisticBrowserAttestationUnavailable
		}
		proofs = append(proofs, RealisticBrowserProof{Viewport: capture.Viewport, CaseID: request.CaseID, RunFamilyID: request.RunFamilyID, WorkspaceTreeSHA256: request.WorkspaceTreeSHA256, RequestSHA256: requestSHA, ScreenshotArtifactPath: capture.ScreenshotArtifactPath, ScreenshotSHA256: capture.ScreenshotSHA256, DOMSnapshotArtifactPath: capture.DOMSnapshotArtifactPath, DOMSnapshotSHA256: capture.DOMSnapshotSHA256, RenderedTotal: total, RenderedCardCount: cards, HorizontalOverflow: overflow})
	}
	if !exactSet(mapKeys(seen), request.Viewports...) {
		return nil, ErrRealisticBrowserAttestationUnavailable
	}
	return proofs, nil
}

func archiveRealisticBrowserCycle(root string, requestRaw []byte) error {
	hash := sha256.Sum256(requestRaw)
	archiveRoot := filepath.Join(root, "browser-cycles", hex.EncodeToString(hash[:]))
	if err := os.MkdirAll(archiveRoot, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"browser-request.json", "browser-preview.json", "browser-evidence.json", "browser-capture-token.txt", "narrow.jpg", "narrow.json", "medium.jpg", "medium.json", "wide.jpg", "wide.json"} {
		source := filepath.Join(root, name)
		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		destination := filepath.Join(archiveRoot, name)
		if _, err := os.Stat(destination); err == nil {
			return fmt.Errorf("Browser capture cycle archive already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(source, destination); err != nil {
			return fmt.Errorf("archive Browser capture cycle: %w", err)
		}
	}
	return nil
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func validRealisticLoopbackPreview(preview RealisticBrowserPreviewReceipt, request RealisticBrowserRequest, requestSHA string) bool {
	parsed, err := url.Parse(preview.URL)
	return err == nil && preview.SchemaVersion == RealisticBrowserPreviewSchemaVersion && preview.RunFamilyID == request.RunFamilyID && preview.WorkspaceTreeSHA256 == request.WorkspaceTreeSHA256 && preview.RequestSHA256 == requestSHA && preview.NetworkBoundary == "127.0.0.1_ephemeral_report_directory_only" && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.Port() != "" && parsed.Path == "/" && parsed.RawQuery == "" && parsed.Fragment == ""
}

func deriveRealisticBrowserFacts(root, relative, requestSHA, previewURL, viewport string) (int, int, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return 0, 0, false, err
	}
	var snapshot RealisticBrowserDOMSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return 0, 0, false, fmt.Errorf("Browser DOM snapshot is invalid or not bound to the loopback handoff")
	}
	return deriveRealisticBrowserFactsFromSnapshot(snapshot, requestSHA, previewURL, viewport)
}

func deriveRealisticBrowserFactsFromSnapshot(snapshot RealisticBrowserDOMSnapshot, requestSHA, previewURL, viewport string) (int, int, bool, error) {
	if snapshot.SchemaVersion != RealisticBrowserDOMSnapshotSchemaVersion || snapshot.RequestSHA256 != requestSHA || snapshot.URL != previewURL || snapshot.Viewport != viewport || !validRealisticViewportWidth(viewport, snapshot.ActualViewportWidth) || snapshot.DocumentClientWidth <= 0 || snapshot.DocumentScrollWidth <= 0 || snapshot.DocumentClientWidth > snapshot.ActualViewportWidth || snapshot.ActualViewportWidth-snapshot.DocumentClientWidth > 40 {
		return 0, 0, false, fmt.Errorf("Browser DOM snapshot is invalid or not bound to the loopback handoff")
	}
	document, err := html.Parse(strings.NewReader(snapshot.DocumentHTML))
	if err != nil {
		return 0, 0, false, fmt.Errorf("parse Browser DOM snapshot: %w", err)
	}
	totalText := ""
	cardCount := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			id, classes := "", ""
			for _, attr := range node.Attr {
				if attr.Key == "id" {
					id = attr.Val
				}
				if attr.Key == "class" {
					classes = attr.Val
				}
			}
			if id == "total" {
				totalText = strings.TrimSpace(nodeText(node))
			}
			if node.Data == "article" && contains(strings.Fields(classes), "card") {
				cardCount++
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	total, err := strconv.Atoi(totalText)
	if err != nil {
		return 0, 0, false, fmt.Errorf("Browser DOM total is not an integer")
	}
	return total, cardCount, snapshot.DocumentScrollWidth > snapshot.DocumentClientWidth, nil
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func verifyBrowserArtifact(root, relative, expected string) error {
	if !safeFixtureRelativePath(relative) || !validSHA256(expected) {
		return fmt.Errorf("Browser artifact identity invalid")
	}
	rootAbs, _ := filepath.Abs(root)
	pathAbs, _ := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(relative)))
	if pathAbs == rootAbs || !strings.HasPrefix(strings.ToLower(pathAbs), strings.ToLower(rootAbs)+string(os.PathSeparator)) {
		return fmt.Errorf("Browser artifact escapes evidence root")
	}
	raw, err := os.ReadFile(pathAbs)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != expected {
		return fmt.Errorf("Browser artifact hash mismatch")
	}
	return nil
}

func writeImmutableOrEqual(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err == nil {
		if string(existing) != string(contents) {
			return fmt.Errorf("immutable Browser request drifted")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, contents, 0o600)
}
