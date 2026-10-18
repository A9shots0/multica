package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/previewpublication"
	"github.com/multica-ai/multica/server/internal/workflowguard"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func testStaticPreviewSpec(t *testing.T, expires time.Time) (staticPreviewSpec, string) {
	t.Helper()
	worktree := t.TempDir()
	root := filepath.Join(worktree, ".multica", "preview")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<!doctype html><p>synthetic</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return staticPreviewSpec{
		TaskID: "task-1",
		Binding: previewpublication.Binding{
			WorkspaceID:       "11111111-1111-1111-1111-111111111111",
			IssueID:           "22222222-2222-2222-2222-222222222222",
			AgentID:           "33333333-3333-3333-3333-333333333333",
			AgentRunID:        "44444444-4444-4444-4444-444444444444",
			CandidateID:       "55555555-5555-5555-5555-555555555555",
			CandidateRevision: 1,
			SourceRevision:    "preview-source:55555555-5555-5555-5555-555555555555",
			Worktree: previewpublication.WorktreeProvenance{
				DaemonID:  "daemon-1",
				RuntimeID: "66666666-6666-6666-6666-666666666666",
				Path:      worktree,
				Branch:    "agent/correction/1",
			},
		},
		ArtifactRoot: ".multica/preview",
		ExpiresAt:    expires,
	}, worktree
}

func newTestStaticPreview(t *testing.T, spec staticPreviewSpec) *staticPreview {
	t.Helper()
	snapshot, err := snapshotStaticPreview(filepath.Join(spec.Binding.Worktree.Path, ".multica", "preview"))
	if err != nil {
		t.Fatal(err)
	}
	producer, err := newStaticPreview(context.Background(), spec, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return producer
}

func TestStaticPreviewServesIdentityReadOnlyWithIsolationHeaders(t *testing.T) {
	spec, _ := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	producer := newTestStaticPreview(t, spec)
	defer producer.stop()

	req, err := http.NewRequest(http.MethodGet, producer.identityURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "session=secret")
	resp, err := (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/json" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("identity response = %d, content-type=%q cache=%q", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Cache-Control"))
	}
	var identity previewpublication.Identity
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	if identity.SchemaVersion != previewpublication.IdentitySchemaVersion || identity.Binding != spec.Binding {
		t.Fatalf("identity = %+v", identity)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'none'") || !strings.Contains(csp, "form-action 'none'") {
		t.Fatalf("restrictive CSP = %q", csp)
	}
	if resp.Header.Get("Set-Cookie") != "" || resp.Header.Get("Authorization") != "" {
		t.Fatal("preview response exposed credentials")
	}

	req, _ = http.NewRequest(http.MethodPost, producer.previewURL, nil)
	resp, err = (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != "GET, HEAD" {
		t.Fatalf("mutation response = %d allow=%q", resp.StatusCode, resp.Header.Get("Allow"))
	}
}

func TestStaticPreviewRejectsTraversalAndSymlinkEscape(t *testing.T) {
	_, worktree := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	root := filepath.Join(worktree, ".multica", "preview")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "secret.txt")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation is unavailable")
		}
		t.Fatal(err)
	}
	if _, err := staticPreviewRequestPath("/../secret.txt"); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if _, err := snapshotStaticPreview(root); err == nil {
		t.Fatal("symlink escape was accepted")
	}
	if _, err := resolveStaticPreviewRoot(worktree, ".multica/preview"); err != nil {
		t.Fatalf("valid preview root rejected: %v", err)
	}
}

func TestStaticPreviewSnapshotAllowlistAndImmutability(t *testing.T) {
	spec, worktree := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	root := filepath.Join(worktree, ".multica", "preview")
	if err := os.WriteFile(filepath.Join(root, "styles.css"), []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.js"), []byte("console.log('safe')"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotStaticPreview(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Assets) != 3 || len(snapshot.Manifest) != 3 {
		t.Fatalf("snapshot assets = %#v manifest = %#v", snapshot.Assets, snapshot.Manifest)
	}
	original := append([]byte(nil), snapshot.Assets["index.html"].Content...)
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	producer, err := newStaticPreview(context.Background(), spec, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.stop()
	resp, err := (&http.Client{Timeout: time.Second}).Get(producer.previewURL)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil || string(body) != string(original) {
		t.Fatalf("served snapshot changed: status=%d err=%v body=%q", resp.StatusCode, readErr, body)
	}
}

func TestStaticPreviewSnapshotRejectsSecretsAndUnlistedFiles(t *testing.T) {
	_, worktree := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	root := filepath.Join(worktree, ".multica", "preview")
	for _, name := range []string{".env", "secret.txt", "build.json", "unknown.bin"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := snapshotStaticPreview(root); err == nil {
			t.Fatalf("unlisted file %q was admitted", name)
		}
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "app.js"), []byte("const token='msr_secret'"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotStaticPreview(root); err == nil {
		t.Fatal("credential-like content was admitted")
	}
}

func TestStaticPreviewRegistryFetchRequiresExactBinding(t *testing.T) {
	spec, _ := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	spec.TaskID = spec.Binding.AgentRunID
	producer := newTestStaticPreview(t, spec)
	registry := newStaticPreviewRegistry()
	key := spec.TaskID + "\x00" + spec.Binding.CandidateID
	registry.entries[key] = producer
	t.Cleanup(registry.stopAll)
	request := protocol.PreviewFetchRequest{
		WorkspaceID: spec.Binding.WorkspaceID, IssueID: spec.Binding.IssueID,
		AgentID: spec.Binding.AgentID, AgentRunID: spec.Binding.AgentRunID, TaskID: spec.TaskID,
		CandidateID: spec.Binding.CandidateID, CandidateRevision: spec.Binding.CandidateRevision,
		SourceRevision: spec.Binding.SourceRevision, DaemonID: spec.Binding.Worktree.DaemonID,
		RuntimeID: spec.Binding.Worktree.RuntimeID, WorktreePath: spec.Binding.Worktree.Path,
		Branch: spec.Binding.Worktree.Branch, Method: http.MethodGet, Path: "/",
	}
	response := registry.fetch(request)
	if response.Status != http.StatusOK || string(response.Body) != "<!doctype html><p>synthetic</p>" ||
		response.Headers["Content-Security-Policy"] == "" {
		t.Fatalf("fetch response = %#v", response)
	}
	request.SourceRevision = "wrong"
	if response := registry.fetch(request); response.Status != http.StatusNotFound {
		t.Fatalf("mismatched binding status = %d", response.Status)
	}
	request.SourceRevision = spec.Binding.SourceRevision
	request.Method = http.MethodPost
	if response := registry.fetch(request); response.Status != http.StatusBadRequest {
		t.Fatalf("mutation status = %d", response.Status)
	}
}

func TestHandleServerRPCPreviewFetch(t *testing.T) {
	spec, _ := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	spec.TaskID = spec.Binding.AgentRunID
	producer := newTestStaticPreview(t, spec)
	registry := newStaticPreviewRegistry()
	registry.entries[spec.TaskID+"\x00"+spec.Binding.CandidateID] = producer
	t.Cleanup(registry.stopAll)
	responses := make(chan []byte, 1)
	rpc := newWSRPCClient(time.Second)
	generation := rpc.attach(func(frame []byte) (*wsOutbound, error) {
		responses <- frame
		return &wsOutbound{data: frame}, nil
	})
	daemon := &Daemon{previewRegistry: registry, wsRPC: rpc}
	fetch := protocol.PreviewFetchRequest{
		WorkspaceID: spec.Binding.WorkspaceID, IssueID: spec.Binding.IssueID,
		AgentID: spec.Binding.AgentID, AgentRunID: spec.Binding.AgentRunID, TaskID: spec.TaskID,
		CandidateID: spec.Binding.CandidateID, CandidateRevision: spec.Binding.CandidateRevision,
		SourceRevision: spec.Binding.SourceRevision, DaemonID: spec.Binding.Worktree.DaemonID,
		RuntimeID: spec.Binding.Worktree.RuntimeID, WorktreePath: spec.Binding.Worktree.Path,
		Branch: spec.Binding.Worktree.Branch, Method: http.MethodHead, Path: previewpublication.IdentityPath,
	}
	body, err := json.Marshal(fetch)
	if err != nil {
		t.Fatal(err)
	}
	daemon.handleServerRPCRequest(generation, protocol.RPCRequestPayload{
		RequestID: "request-1", Method: "preview.fetch", Body: body,
	})
	var message protocol.Message
	if err := json.Unmarshal(<-responses, &message); err != nil {
		t.Fatal(err)
	}
	var response protocol.RPCResponsePayload
	if err := json.Unmarshal(message.Payload, &response); err != nil {
		t.Fatal(err)
	}
	var result protocol.PreviewFetchResponse
	if err := json.Unmarshal(response.Body, &result); err != nil {
		t.Fatal(err)
	}
	if response.RequestID != "request-1" || response.Status != http.StatusOK ||
		result.Status != http.StatusOK || len(result.Body) != 0 ||
		result.Headers["Content-Type"] != "application/json" {
		t.Fatalf("server RPC response = %#v result=%#v", response, result)
	}
}

func TestStaticPreviewRegistryPublishesAndStopsOnEligibilityLoss(t *testing.T) {
	spec, _ := testStaticPreviewSpec(t, time.Now().UTC().Add(time.Minute))
	registry := newStaticPreviewRegistry()
	published := 0
	publish := func(_ context.Context, _ string, claim previewpublication.ProducerClaim) (previewpublication.Publication, error) {
		published++
		return previewpublication.Publication{
			SchemaVersion: previewpublication.SchemaVersion,
			Binding:       spec.Binding,
			PreviewURL:    claim.PreviewURL,
			IdentityURL:   claim.IdentityURL,
			DataPolicy:    claim.DataPolicy,
			PublishedAt:   time.Now().UTC(),
			ExpiresAt:     claim.ExpiresAt,
		}, nil
	}
	if err := registry.start(context.Background(), spec, publish); err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("initial publication count = %d", published)
	}
	key := spec.TaskID + "\x00" + spec.Binding.CandidateID
	registry.mu.Lock()
	url := registry.entries[key].previewURL
	registry.mu.Unlock()
	registry.stopTask(spec.TaskID)
	if _, err := (&http.Client{Timeout: 200 * time.Millisecond}).Get(url); err == nil {
		t.Fatal("invalid probe unexpectedly succeeded")
	}
}

func TestStaticPreviewExpiresAndBecomesUnreachable(t *testing.T) {
	spec, _ := testStaticPreviewSpec(t, time.Now().UTC().Add(80*time.Millisecond))
	producer := newTestStaticPreview(t, spec)

	defer producer.stop()
	time.Sleep(150 * time.Millisecond)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	_, err := client.Get(producer.previewURL)
	if err == nil {
		t.Fatal("expired preview remained reachable")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "server closed") {
		t.Fatalf("unexpected expiry error: %v", err)
	}
}

func TestStaticPreviewSpecRequiresControllerPacket(t *testing.T) {
	assignment := workflowguard.LegacyAssignmentV1{
		ContractVersion:      workflowguard.LegacyAssignmentVersion,
		TaskKind:             "correction",
		IssueID:              "22222222-2222-2222-2222-222222222222",
		IssueTitle:           "Correction",
		SourceTaskID:         "77777777-7777-7777-7777-777777777777",
		WorkflowEventID:      "88888888-8888-8888-8888-888888888888",
		WorkflowEventType:    "correction_requested",
		WorkflowEventReason:  "review gap",
		TargetAgentID:        "33333333-3333-3333-3333-333333333333",
		SquadID:              "99999999-9999-9999-9999-999999999999",
		RuntimeID:            "66666666-6666-6666-6666-666666666666",
		RuntimeProvider:      "opencode",
		Model:                "model-1",
		ReasoningEffort:      "medium",
		AcceptanceCriteria:   json.RawMessage(`[]`),
		WorkflowEventPayload: json.RawMessage(`{}`),
		ExecutionPolicy:      workflowguard.LegacyExecutionPolicyForAcceptance(nil),
		RequiredSkills: []workflowguard.RequiredSkill{{
			ID: "skill-1", Source: "catalog", SHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		}},
		Preview: &workflowguard.StaticPreviewDeclaration{
			Kind: workflowguard.StaticPreviewKind, ArtifactRoot: workflowguard.StaticPreviewArtifactRoot,
			CandidateID: "55555555-5555-5555-5555-555555555555", CandidateRevision: 1,
			SourceRevision: "source-1", PolicyID: workflowguard.StaticPreviewPolicyID,
		},
	}
	packet, err := json.Marshal(assignment)
	if err != nil {
		t.Fatal(err)
	}
	task := Task{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    "11111111-1111-1111-1111-111111111111",
		IssueID:        "22222222-2222-2222-2222-222222222222",
		AgentID:        "33333333-3333-3333-3333-333333333333",
		RuntimeID:      "66666666-6666-6666-6666-666666666666",
		WorkflowPacket: packet,
	}
	spec, declared, err := staticPreviewSpecForTask(task, "daemon-1", "/tmp/worktree", "agent/correction/1", time.Now())
	if err != nil || !declared || spec.Binding.CandidateID != "55555555-5555-5555-5555-555555555555" {
		t.Fatalf("static preview spec = %+v declared=%v err=%v", spec, declared, err)
	}
	assignment.Preview.ArtifactRoot = "/tmp"
	task.WorkflowPacket, err = json.Marshal(assignment)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := staticPreviewSpecForTask(task, "daemon-1", "/tmp/worktree", "agent/correction/1", time.Now()); err == nil {
		t.Fatal("unapproved artifact root was accepted")
	}
}
