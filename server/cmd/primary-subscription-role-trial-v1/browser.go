package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

type trialBrowserUpload struct {
	Viewport            string `json:"viewport"`
	ScreenshotBase64    string `json:"screenshot_base64"`
	DocumentHTML        string `json:"document_html"`
	DocumentScrollWidth int    `json:"document_scroll_width"`
	DocumentClientWidth int    `json:"document_client_width"`
	ActualViewportWidth int    `json:"actual_viewport_width"`
}

func (protocol trialProtocol) serveTrialBrowser(ctx context.Context, root, observationID string) error {
	definition, err := protocol.activeTrialDefinition()
	if err != nil {
		return err
	}
	ledger, err := loadTrialLedger(root)
	if err != nil {
		return err
	}
	if err := protocol.validateTrialLedger(definition, ledger, false); err != nil {
		return err
	}
	var observation *trialObservation
	for index := range ledger.Observations {
		if ledger.Observations[index].ObservationID == observationID {
			observation = &ledger.Observations[index]
		}
	}
	if observation == nil || observation.Status != "awaiting_browser_evidence" || observation.CaseID != "R02" || observation.ExecutionState.Controller.PendingAction == nil {
		return fmt.Errorf("R02 observation is not awaiting Browser evidence")
	}
	collector := agentqualification.FileRealisticBrowserCollector{RunFamilyID: observation.RunFamilyID, ArtifactRoot: observation.BrowserEvidenceRoot}
	if _, err := collector.Capture(ctx, observation.Workspace.Root, "report/index.html"); err != nil && !errorsIsBrowserPending(err) {
		return err
	}
	requestPath := filepath.Join(observation.BrowserEvidenceRoot, "browser-request.json")
	requestRaw, err := os.ReadFile(requestPath)
	if err != nil {
		return err
	}
	var request agentqualification.RealisticBrowserRequest
	if json.Unmarshal(requestRaw, &request) != nil {
		return fmt.Errorf("Browser request is invalid")
	}
	treeHash, err := agentqualification.RealisticFixtureTreeSHA256(os.DirFS(observation.Workspace.Root), ".")
	if err != nil || treeHash != request.WorkspaceTreeSHA256 || request.RunFamilyID != observation.RunFamilyID {
		return fmt.Errorf("Browser request no longer binds the disposable fixture")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	requestSHA := hashBytes(requestRaw)
	url := "http://" + listener.Addr().String() + "/"
	preview := agentqualification.RealisticBrowserPreviewReceipt{SchemaVersion: agentqualification.RealisticBrowserPreviewSchemaVersion, RunFamilyID: observation.RunFamilyID, WorkspaceTreeSHA256: treeHash, RequestSHA256: requestSHA, URL: url, NetworkBoundary: "127.0.0.1_ephemeral_report_directory_only"}
	previewRaw, _ := json.MarshalIndent(preview, "", "  ")
	if err := writeOrEqual(filepath.Join(observation.BrowserEvidenceRoot, "browser-preview.json"), previewRaw); err != nil {
		return err
	}
	tokenRaw := make([]byte, 32)
	if _, err := rand.Read(tokenRaw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenRaw)
	if err := os.WriteFile(filepath.Join(observation.BrowserEvidenceRoot, "browser-capture-token.txt"), []byte(token), 0o600); err != nil {
		return err
	}
	ready, _ := json.Marshal(map[string]any{"status": "serving_r02_preview", "url": url, "observation_id": observationID, "capture_token_path": filepath.Join(observation.BrowserEvidenceRoot, "browser-capture-token.txt"), "stop": "interrupt after three captures"})
	fmt.Println(string(ready))
	reportRoot := filepath.Join(observation.Workspace.Root, "report")
	mux := http.NewServeMux()
	mux.HandleFunc("/__trial_capture", func(response http.ResponseWriter, httpRequest *http.Request) {
		if err := acceptTrialBrowserCapture(httpRequest, observation.BrowserEvidenceRoot, token, request, preview); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"captured"}`))
	})
	mux.Handle("/", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead || strings.Contains(request.URL.Path, "..") {
			http.Error(response, "invalid request", http.StatusBadRequest)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		http.FileServer(http.Dir(reportRoot)).ServeHTTP(response, request)
	}))
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func errorsIsBrowserPending(err error) bool {
	return err == agentqualification.ErrRealisticBrowserEvidencePending || strings.Contains(err.Error(), agentqualification.ErrRealisticBrowserEvidencePending.Error())
}

func acceptTrialBrowserCapture(httpRequest *http.Request, evidenceRoot, token string, request agentqualification.RealisticBrowserRequest, preview agentqualification.RealisticBrowserPreviewReceipt) error {
	if httpRequest.Method != http.MethodPost || httpRequest.Header.Get("X-Benchmark-Capture-Token") != token {
		return fmt.Errorf("Browser capture identity invalid")
	}
	host, _, err := net.SplitHostPort(httpRequest.RemoteAddr)
	if err != nil || host != "127.0.0.1" {
		return fmt.Errorf("Browser capture must be loopback")
	}
	defer httpRequest.Body.Close()
	var upload trialBrowserUpload
	decoder := json.NewDecoder(io.LimitReader(httpRequest.Body, 5<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&upload); err != nil {
		return err
	}
	ranges := map[string][2]int{"narrow": {320, 480}, "medium": {700, 1024}, "wide": {1200, 1920}}
	allowed, ok := ranges[upload.Viewport]
	if !ok || upload.ActualViewportWidth < allowed[0] || upload.ActualViewportWidth > allowed[1] || upload.DocumentHTML == "" || upload.DocumentClientWidth <= 0 || upload.DocumentScrollWidth <= 0 || upload.DocumentClientWidth > upload.ActualViewportWidth || upload.ActualViewportWidth-upload.DocumentClientWidth > 40 {
		return fmt.Errorf("Browser viewport evidence invalid")
	}
	screenshot, err := base64.StdEncoding.DecodeString(upload.ScreenshotBase64)
	if err != nil || len(screenshot) < 4 || len(screenshot) > 4<<20 || !((screenshot[0] == 0xff && screenshot[1] == 0xd8) || (screenshot[0] == 0x89 && screenshot[1] == 0x50 && screenshot[2] == 0x4e && screenshot[3] == 0x47)) {
		return fmt.Errorf("Browser screenshot invalid")
	}
	requestRaw, err := os.ReadFile(filepath.Join(evidenceRoot, "browser-request.json"))
	if err != nil || hashBytes(requestRaw) != preview.RequestSHA256 || request.RunFamilyID != preview.RunFamilyID {
		return fmt.Errorf("Browser request drifted")
	}
	previewRaw, err := os.ReadFile(filepath.Join(evidenceRoot, "browser-preview.json"))
	if err != nil {
		return err
	}
	dom := agentqualification.RealisticBrowserDOMSnapshot{SchemaVersion: agentqualification.RealisticBrowserDOMSnapshotSchemaVersion, RequestSHA256: preview.RequestSHA256, URL: preview.URL, Viewport: upload.Viewport, DocumentHTML: upload.DocumentHTML, DocumentScrollWidth: upload.DocumentScrollWidth, DocumentClientWidth: upload.DocumentClientWidth, ActualViewportWidth: upload.ActualViewportWidth}
	domRaw, _ := json.MarshalIndent(dom, "", "  ")
	screenshotName, domName := upload.Viewport+".jpg", upload.Viewport+".json"
	if err := writeOrEqual(filepath.Join(evidenceRoot, screenshotName), screenshot); err != nil {
		return err
	}
	if err := writeOrEqual(filepath.Join(evidenceRoot, domName), domRaw); err != nil {
		return err
	}
	capture := agentqualification.RealisticBrowserCapture{Viewport: upload.Viewport, ScreenshotArtifactPath: screenshotName, ScreenshotSHA256: hashBytes(screenshot), DOMSnapshotArtifactPath: domName, DOMSnapshotSHA256: hashBytes(domRaw)}
	evidencePath := filepath.Join(evidenceRoot, "browser-evidence.json")
	evidence := agentqualification.RealisticBrowserEvidence{SchemaVersion: agentqualification.RealisticBrowserEvidenceSchemaVersion, RequestSHA256: preview.RequestSHA256, PreviewSHA256: hashBytes(previewRaw)}
	if existing, readErr := os.ReadFile(evidencePath); readErr == nil {
		if json.Unmarshal(existing, &evidence) != nil || evidence.RequestSHA256 != preview.RequestSHA256 || evidence.PreviewSHA256 != hashBytes(previewRaw) {
			return fmt.Errorf("existing Browser evidence invalid")
		}
	}
	replaced := false
	for index := range evidence.Captures {
		if evidence.Captures[index].Viewport == capture.Viewport {
			if evidence.Captures[index] != capture {
				return fmt.Errorf("Browser viewport replay drifted")
			}
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
	evidenceRaw, _ := json.MarshalIndent(evidence, "", "  ")
	return writeAtomic(evidencePath, evidenceRaw)
}

func sha256String(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum)
}
