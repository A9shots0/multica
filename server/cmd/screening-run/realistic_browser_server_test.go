package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestAcceptRealisticBrowserCapturePersistsBoundEvidence(t *testing.T) {
	root := t.TempDir()
	request := agentqualification.RealisticBrowserRequest{SchemaVersion: agentqualification.RealisticBrowserBridgeSchemaVersion, CaseID: "R02", RunFamilyID: "run-1", WorkspaceTreeSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", EntryPath: "report/index.html", Viewports: []string{"narrow", "medium", "wide"}, BrowserSurface: "chatgpt_in_app_browser_only"}
	requestRaw, _ := json.MarshalIndent(request, "", "  ")
	requestHash := sha256.Sum256(requestRaw)
	preview := agentqualification.RealisticBrowserPreviewReceipt{SchemaVersion: agentqualification.RealisticBrowserPreviewSchemaVersion, RunFamilyID: request.RunFamilyID, WorkspaceTreeSHA256: request.WorkspaceTreeSHA256, RequestSHA256: stringHex(requestHash[:]), URL: "http://127.0.0.1:12345/", NetworkBoundary: "127.0.0.1_ephemeral_report_directory_only"}
	if err := os.WriteFile(filepath.Join(root, "browser-request.json"), requestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(filepath.Join(root, "browser-preview.json"), preview); err != nil {
		t.Fatal(err)
	}
	document := `<html><body><span id="total">47</span><article class="card"></article><article class="card"></article><article class="card"></article><article class="card"></article></body></html>`
	payload, _ := json.Marshal(realisticBrowserCaptureUpload{Viewport: "narrow", ScreenshotBase64: base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0xd9}), DocumentHTML: document, DocumentScrollWidth: 980, DocumentClientWidth: 390, ActualViewportWidth: 390})
	httpRequest := httptest.NewRequest("POST", "http://127.0.0.1:12345/__benchmark_capture", bytes.NewReader(payload))
	httpRequest.RemoteAddr = "127.0.0.1:54321"
	httpRequest.Header.Set("X-Benchmark-Capture-Token", "secret")
	if err := acceptRealisticBrowserCapture(httpRequest, root, "secret", request, preview); err != nil {
		t.Fatal(err)
	}
	var evidence agentqualification.RealisticBrowserEvidence
	raw, err := os.ReadFile(filepath.Join(root, "browser-evidence.json"))
	if err != nil || json.Unmarshal(raw, &evidence) != nil || len(evidence.Captures) != 1 || evidence.Captures[0].Viewport != "narrow" {
		t.Fatalf("capture evidence was not persisted: err=%v evidence=%+v", err, evidence)
	}
}

func TestAcceptRealisticBrowserCaptureRejectsMissingToken(t *testing.T) {
	httpRequest := httptest.NewRequest("POST", "http://127.0.0.1/__benchmark_capture", bytes.NewReader([]byte(`{}`)))
	httpRequest.RemoteAddr = "127.0.0.1:54321"
	if err := acceptRealisticBrowserCapture(httpRequest, t.TempDir(), "secret", agentqualification.RealisticBrowserRequest{}, agentqualification.RealisticBrowserPreviewReceipt{}); err == nil {
		t.Fatal("capture without the out-of-page token was accepted")
	}
}

func stringHex(raw []byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, len(raw)*2)
	for index, value := range raw {
		encoded[index*2] = digits[value>>4]
		encoded[index*2+1] = digits[value&0x0f]
	}
	return string(encoded)
}
