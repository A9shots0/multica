package agentqualification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRealisticBrowserBridgeAcceptsBoundInAppArtifactsAndDerivesFacts(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R02")
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := filepath.Join(parent, "browser-proof")
	collector := FileRealisticBrowserCollector{RunFamilyID: "browser-family", ArtifactRoot: artifactRoot}
	if _, err := collector.Capture(context.Background(), workspace.Root, "report/index.html"); !errors.Is(err, ErrRealisticBrowserEvidencePending) {
		t.Fatal(err)
	}
	requestRaw, _ := os.ReadFile(filepath.Join(artifactRoot, "browser-request.json"))
	var request RealisticBrowserRequest
	if err := json.Unmarshal(requestRaw, &request); err != nil {
		t.Fatal(err)
	}
	requestHash := sha256.Sum256(requestRaw)
	requestSHA := hex.EncodeToString(requestHash[:])
	preview := RealisticBrowserPreviewReceipt{SchemaVersion: RealisticBrowserPreviewSchemaVersion, RunFamilyID: request.RunFamilyID, WorkspaceTreeSHA256: request.WorkspaceTreeSHA256, RequestSHA256: requestSHA, URL: "http://127.0.0.1:43123/", NetworkBoundary: "127.0.0.1_ephemeral_report_directory_only"}
	previewRaw, _ := json.Marshal(preview)
	if err := os.WriteFile(filepath.Join(artifactRoot, "browser-preview.json"), previewRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	previewHash := sha256.Sum256(previewRaw)
	evidence := RealisticBrowserEvidence{SchemaVersion: RealisticBrowserEvidenceSchemaVersion, RequestSHA256: requestSHA, PreviewSHA256: hex.EncodeToString(previewHash[:])}
	document := `<html><body><span id="total">47</span><article class="card"></article><article class="card"></article><article class="card"></article><article class="card"></article></body></html>`
	for _, item := range []struct {
		viewport string
		width    int
	}{{"narrow", 390}, {"medium", 900}, {"wide", 1440}} {
		screenshotPath := item.viewport + ".png"
		domPath := item.viewport + ".json"
		screenshotRaw := []byte("trusted-in-app-screenshot-" + item.viewport)
		snapshotRaw, _ := json.Marshal(RealisticBrowserDOMSnapshot{SchemaVersion: RealisticBrowserDOMSnapshotSchemaVersion, RequestSHA256: requestSHA, URL: preview.URL, Viewport: item.viewport, DocumentHTML: document, DocumentScrollWidth: item.width, DocumentClientWidth: item.width, ActualViewportWidth: item.width})
		if err := os.WriteFile(filepath.Join(artifactRoot, screenshotPath), screenshotRaw, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(artifactRoot, domPath), snapshotRaw, 0o600); err != nil {
			t.Fatal(err)
		}
		screenshotHash := sha256.Sum256(screenshotRaw)
		domHash := sha256.Sum256(snapshotRaw)
		evidence.Captures = append(evidence.Captures, RealisticBrowserCapture{Viewport: item.viewport, ScreenshotArtifactPath: screenshotPath, ScreenshotSHA256: hex.EncodeToString(screenshotHash[:]), DOMSnapshotArtifactPath: domPath, DOMSnapshotSHA256: hex.EncodeToString(domHash[:])})
	}
	evidenceRaw, _ := json.Marshal(evidence)
	if err := os.WriteFile(filepath.Join(artifactRoot, "browser-evidence.json"), evidenceRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	proofs, err := collector.Capture(context.Background(), workspace.Root, "report/index.html")
	if err != nil || len(proofs) != 3 {
		t.Fatalf("bound Browser evidence was not accepted: proofs=%+v err=%v", proofs, err)
	}
	for _, proof := range proofs {
		if proof.RenderedTotal != 47 || proof.RenderedCardCount != 4 || proof.HorizontalOverflow {
			t.Fatalf("Browser facts were not independently derived: %+v", proof)
		}
	}
}

func TestRealisticBrowserBridgeFailsClosedWithoutInAppAttestation(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R02")
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := filepath.Join(parent, "browser-proof")
	collector := FileRealisticBrowserCollector{RunFamilyID: "browser-family", ArtifactRoot: artifactRoot}
	if _, err := collector.Capture(context.Background(), workspace.Root, "report/index.html"); !errors.Is(err, ErrRealisticBrowserEvidencePending) {
		t.Fatalf("first capture should create a pending handoff: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactRoot, "browser-request.json")); err != nil {
		t.Fatalf("Browser handoff request missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "browser-evidence.json"), []byte(`{"proofs":[{"viewport":"narrow"},{"viewport":"medium"},{"viewport":"wide"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Capture(context.Background(), workspace.Root, "report/index.html"); !errors.Is(err, ErrRealisticBrowserAttestationUnavailable) {
		t.Fatalf("caller-authored Browser evidence must fail closed: %v", err)
	}
}

func TestRealisticBrowserBridgeArchivesPriorCaptureWhenWorkspaceChanges(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareRealisticCaseWorkspace(parent, "R02")
	if err != nil {
		t.Fatal(err)
	}
	artifactRoot := filepath.Join(parent, "browser-proof")
	collector := FileRealisticBrowserCollector{RunFamilyID: "browser-family", ArtifactRoot: artifactRoot}
	if _, err := collector.Capture(context.Background(), workspace.Root, "report/index.html"); !errors.Is(err, ErrRealisticBrowserEvidencePending) {
		t.Fatal(err)
	}
	firstRequest, err := os.ReadFile(filepath.Join(artifactRoot, "browser-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Edit("report/styles.css", []byte("main { width: 100%; }")); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Capture(context.Background(), workspace.Root, "report/index.html"); !errors.Is(err, ErrRealisticBrowserEvidencePending) {
		t.Fatalf("changed workspace did not open a fresh Browser capture cycle: %v", err)
	}
	firstHash := sha256.Sum256(firstRequest)
	archived := filepath.Join(artifactRoot, "browser-cycles", hex.EncodeToString(firstHash[:]), "browser-request.json")
	if archivedRaw, err := os.ReadFile(archived); err != nil || !stringEqualBytes(archivedRaw, firstRequest) {
		t.Fatalf("prior Browser request was not preserved: err=%v", err)
	}
	secondRequest, err := os.ReadFile(filepath.Join(artifactRoot, "browser-request.json"))
	if err != nil || stringEqualBytes(secondRequest, firstRequest) {
		t.Fatalf("fresh Browser request was not bound to the corrected workspace: err=%v", err)
	}
}

func stringEqualBytes(left, right []byte) bool { return string(left) == string(right) }

func TestRealisticBrowserDOMFactsAreDerivedOfflineAtEveryViewport(t *testing.T) {
	const requestSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const previewURL = "http://127.0.0.1:43123/"
	document := `<html><body><span id="total">47</span><section><article class="card"></article><article class="card featured"></article><article class="card"></article><article class="card"></article></section></body></html>`
	for _, item := range []struct {
		viewport string
		width    int
	}{{"narrow", 390}, {"medium", 900}, {"wide", 1440}} {
		snapshot := RealisticBrowserDOMSnapshot{SchemaVersion: RealisticBrowserDOMSnapshotSchemaVersion, RequestSHA256: requestSHA, URL: previewURL, Viewport: item.viewport, DocumentHTML: document, DocumentScrollWidth: item.width, DocumentClientWidth: item.width, ActualViewportWidth: item.width}
		total, cards, overflow, err := deriveRealisticBrowserFactsFromSnapshot(snapshot, requestSHA, previewURL, item.viewport)
		if err != nil || total != 47 || cards != 4 || overflow {
			t.Fatalf("%s offline DOM derivation failed: total=%d cards=%d overflow=%t err=%v", item.viewport, total, cards, overflow, err)
		}
	}
}

func TestRealisticBrowserDOMFactsRejectWrongViewportAndDeriveLayout(t *testing.T) {
	const requestSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const previewURL = "http://127.0.0.1:43123/"
	snapshot := RealisticBrowserDOMSnapshot{SchemaVersion: RealisticBrowserDOMSnapshotSchemaVersion, RequestSHA256: requestSHA, URL: previewURL, Viewport: "narrow", DocumentHTML: `<span id="total">999</span><article class="card"></article>`, DocumentScrollWidth: 920, DocumentClientWidth: 900, ActualViewportWidth: 900}
	if _, _, _, err := deriveRealisticBrowserFactsFromSnapshot(snapshot, requestSHA, previewURL, "narrow"); err == nil {
		t.Fatal("a narrow proof with a medium actual viewport was accepted")
	}
	snapshot.ActualViewportWidth = 390
	snapshot.DocumentClientWidth = 390
	snapshot.DocumentScrollWidth = 420
	total, cards, overflow, err := deriveRealisticBrowserFactsFromSnapshot(snapshot, requestSHA, previewURL, "narrow")
	if err != nil || total != 999 || cards != 1 || !overflow {
		t.Fatalf("DOM facts were not derived: total=%d cards=%d overflow=%t err=%v", total, cards, overflow, err)
	}
}
