package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed realistic_browser_server.go
var realisticBrowserServerSource []byte

func serveRealisticBrowserPreview(ctx context.Context, artifactRoot, candidateID string) error {
	requiredHash, err := agentqualification.RequiredRealisticBrowserServerSHA256()
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(realisticBrowserServerSource)) != requiredHash {
		return fmt.Errorf("realistic Browser preview server is not the frozen reviewed source")
	}
	observationRoot := filepath.Join(artifactRoot, "observations", safeArtifactName(candidateID), "r02")
	var state realisticRunState
	if err := readJSONFile(filepath.Join(observationRoot, "run-state.json"), &state); err != nil {
		return err
	}
	if state.Workspace.CaseID != "R02" || state.Controller.TerminalState != "awaiting_browser_evidence" || state.Controller.PendingAction == nil || state.Controller.PendingAction.Kind != "browser" {
		return fmt.Errorf("R02 is not awaiting Browser evidence")
	}
	requestRaw, err := os.ReadFile(filepath.Join(state.EvidenceRoot, "browser-request.json"))
	if err != nil {
		return err
	}
	var request agentqualification.RealisticBrowserRequest
	if err := json.Unmarshal(requestRaw, &request); err != nil {
		return err
	}
	treeHash, err := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(state.Workspace.Root), ".")
	if err != nil || treeHash != request.WorkspaceTreeSHA256 || request.RunFamilyID != state.Controller.RunFamilyID || request.EntryPath != "report/index.html" {
		return fmt.Errorf("Browser preview request no longer matches the candidate workspace")
	}
	reportRoot := filepath.Join(state.Workspace.Root, "report")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	requestHash := sha256.Sum256(requestRaw)
	url := "http://" + listener.Addr().String() + "/"
	receipt := agentqualification.RealisticBrowserPreviewReceipt{SchemaVersion: agentqualification.RealisticBrowserPreviewSchemaVersion, RunFamilyID: request.RunFamilyID, WorkspaceTreeSHA256: treeHash, RequestSHA256: fmt.Sprintf("%x", requestHash), URL: url, NetworkBoundary: "127.0.0.1_ephemeral_report_directory_only"}
	if err := writeJSONAtomic(filepath.Join(state.EvidenceRoot, "browser-preview.json"), receipt); err != nil {
		return err
	}
	captureTokenRaw := make([]byte, 32)
	if _, err := rand.Read(captureTokenRaw); err != nil {
		return fmt.Errorf("generate Browser capture token: %w", err)
	}
	captureToken := base64.RawURLEncoding.EncodeToString(captureTokenRaw)
	if err := os.WriteFile(filepath.Join(state.EvidenceRoot, "browser-capture-token.txt"), []byte(captureToken), 0o600); err != nil {
		return fmt.Errorf("write Browser capture token: %w", err)
	}
	raw, _ := json.Marshal(map[string]any{"status": "serving_r02_preview", "url": url, "candidate_id": candidateID, "browser_capture_status": "token_bound_in_app_capture_ready", "stop": "interrupt this command after Browser capture"})
	fmt.Println(string(raw))
	mux := http.NewServeMux()
	mux.HandleFunc("/__benchmark_capture", func(response http.ResponseWriter, captureRequest *http.Request) {
		if err := acceptRealisticBrowserCapture(captureRequest, state.EvidenceRoot, captureToken, request, receipt); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"captured"}`))
	})
	mux.Handle("/", noStore(http.FileServer(http.FS(os.DirFS(reportRoot)))))
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	err = server.Serve(listener)
	if err == http.ErrServerClosed || ctx.Err() != nil {
		return nil
	}
	return err
}

type realisticBrowserCaptureUpload struct {
	Viewport            string `json:"viewport"`
	ScreenshotBase64    string `json:"screenshot_base64"`
	DocumentHTML        string `json:"document_html"`
	DocumentScrollWidth int    `json:"document_scroll_width"`
	DocumentClientWidth int    `json:"document_client_width"`
	ActualViewportWidth int    `json:"actual_viewport_width"`
}

func acceptRealisticBrowserCapture(httpRequest *http.Request, evidenceRoot, captureToken string, request agentqualification.RealisticBrowserRequest, preview agentqualification.RealisticBrowserPreviewReceipt) error {
	if httpRequest.Method != http.MethodPost || httpRequest.RemoteAddr == "" {
		return fmt.Errorf("Browser capture requires a loopback POST")
	}
	host, _, err := net.SplitHostPort(httpRequest.RemoteAddr)
	if err != nil || host != "127.0.0.1" || httpRequest.Header.Get("X-Benchmark-Capture-Token") != captureToken {
		return fmt.Errorf("Browser capture identity is invalid")
	}
	defer httpRequest.Body.Close()
	var upload realisticBrowserCaptureUpload
	decoder := json.NewDecoder(io.LimitReader(httpRequest.Body, 5<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&upload); err != nil {
		return fmt.Errorf("decode Browser capture: %w", err)
	}
	validWidths := map[string][2]int{"narrow": {320, 480}, "medium": {700, 1024}, "wide": {1200, 1920}}
	widthRange, ok := validWidths[upload.Viewport]
	if !ok || upload.ActualViewportWidth < widthRange[0] || upload.ActualViewportWidth > widthRange[1] || upload.DocumentClientWidth <= 0 || upload.DocumentScrollWidth <= 0 || upload.DocumentClientWidth > upload.ActualViewportWidth || upload.ActualViewportWidth-upload.DocumentClientWidth > 40 || upload.DocumentHTML == "" {
		return fmt.Errorf("Browser viewport evidence is invalid")
	}
	screenshot, err := base64.StdEncoding.DecodeString(upload.ScreenshotBase64)
	if err != nil || len(screenshot) < 4 || len(screenshot) > 4<<20 || !((screenshot[0] == 0xff && screenshot[1] == 0xd8) || (screenshot[0] == 0x89 && screenshot[1] == 0x50 && screenshot[2] == 0x4e && screenshot[3] == 0x47)) {
		return fmt.Errorf("Browser screenshot is invalid")
	}
	requestRaw, err := os.ReadFile(filepath.Join(evidenceRoot, "browser-request.json"))
	if err != nil {
		return err
	}
	requestHash := sha256.Sum256(requestRaw)
	requestSHA := fmt.Sprintf("%x", requestHash)
	if requestSHA != preview.RequestSHA256 || request.RunFamilyID != preview.RunFamilyID {
		return fmt.Errorf("Browser capture no longer matches its request")
	}
	previewRaw, err := os.ReadFile(filepath.Join(evidenceRoot, "browser-preview.json"))
	if err != nil {
		return err
	}
	previewHash := sha256.Sum256(previewRaw)
	dom := agentqualification.RealisticBrowserDOMSnapshot{SchemaVersion: agentqualification.RealisticBrowserDOMSnapshotSchemaVersion, RequestSHA256: requestSHA, URL: preview.URL, Viewport: upload.Viewport, DocumentHTML: upload.DocumentHTML, DocumentScrollWidth: upload.DocumentScrollWidth, DocumentClientWidth: upload.DocumentClientWidth, ActualViewportWidth: upload.ActualViewportWidth}
	domRaw, err := json.MarshalIndent(dom, "", "  ")
	if err != nil {
		return err
	}
	screenshotName := upload.Viewport + ".jpg"
	domName := upload.Viewport + ".json"
	if err := os.WriteFile(filepath.Join(evidenceRoot, screenshotName), screenshot, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidenceRoot, domName), domRaw, 0o600); err != nil {
		return err
	}
	screenshotHash := sha256.Sum256(screenshot)
	domHash := sha256.Sum256(domRaw)
	capture := agentqualification.RealisticBrowserCapture{Viewport: upload.Viewport, ScreenshotArtifactPath: screenshotName, ScreenshotSHA256: fmt.Sprintf("%x", screenshotHash), DOMSnapshotArtifactPath: domName, DOMSnapshotSHA256: fmt.Sprintf("%x", domHash)}
	evidencePath := filepath.Join(evidenceRoot, "browser-evidence.json")
	evidence := agentqualification.RealisticBrowserEvidence{SchemaVersion: agentqualification.RealisticBrowserEvidenceSchemaVersion, RequestSHA256: requestSHA, PreviewSHA256: fmt.Sprintf("%x", previewHash)}
	if existing, readErr := os.ReadFile(evidencePath); readErr == nil {
		if json.Unmarshal(existing, &evidence) != nil || evidence.SchemaVersion != agentqualification.RealisticBrowserEvidenceSchemaVersion || evidence.RequestSHA256 != requestSHA || evidence.PreviewSHA256 != fmt.Sprintf("%x", previewHash) {
			return fmt.Errorf("existing Browser evidence is invalid")
		}
	}
	replaced := false
	for index := range evidence.Captures {
		if evidence.Captures[index].Viewport == capture.Viewport {
			evidence.Captures[index] = capture
			replaced = true
		}
	}
	if !replaced {
		evidence.Captures = append(evidence.Captures, capture)
	}
	order := map[string]int{"narrow": 0, "medium": 1, "wide": 2}
	sort.Slice(evidence.Captures, func(i, j int) bool {
		return order[evidence.Captures[i].Viewport] < order[evidence.Captures[j].Viewport]
	})
	return writeJSONAtomic(evidencePath, evidence)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.Contains(request.URL.Path, "..") {
			http.Error(response, "invalid path", http.StatusBadRequest)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(response, request)
	})
}
