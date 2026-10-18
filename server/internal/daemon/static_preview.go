package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/previewpublication"
	"github.com/multica-ai/multica/server/internal/workflowguard"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	staticPreviewLifetime = 4 * time.Minute
	staticPreviewPoll     = 5 * time.Second
)

type staticPreviewSpec struct {
	TaskID       string
	Binding      previewpublication.Binding
	ArtifactRoot string
	ExpiresAt    time.Time
}

type staticPreviewPublisher func(context.Context, string, previewpublication.ProducerClaim) (previewpublication.Publication, error)

func staticPreviewSpecForTask(task Task, daemonID, worktreePath, branch string, now time.Time) (staticPreviewSpec, bool, error) {
	if len(task.WorkflowPacket) == 0 {
		return staticPreviewSpec{}, false, nil
	}
	var assignment workflowguard.LegacyAssignmentV1
	if err := json.Unmarshal(task.WorkflowPacket, &assignment); err != nil {
		return staticPreviewSpec{}, false, fmt.Errorf("decode workflow assignment: %w", err)
	}
	if err := assignment.Validate(); err != nil {
		return staticPreviewSpec{}, false, fmt.Errorf("validate workflow assignment: %w", err)
	}
	if assignment.Preview == nil {
		return staticPreviewSpec{}, false, nil
	}

	if strings.TrimSpace(daemonID) == "" || strings.TrimSpace(worktreePath) == "" || strings.TrimSpace(branch) == "" {
		return staticPreviewSpec{}, false, errors.New("static preview requires daemon worktree provenance")
	}
	return staticPreviewSpec{
		TaskID: task.ID,
		Binding: previewpublication.Binding{
			WorkspaceID:       task.WorkspaceID,
			IssueID:           task.IssueID,
			AgentID:           task.AgentID,
			AgentRunID:        task.ID,
			CandidateID:       assignment.Preview.CandidateID,
			CandidateRevision: assignment.Preview.CandidateRevision,
			SourceRevision:    assignment.Preview.SourceRevision,
			Worktree: previewpublication.WorktreeProvenance{
				DaemonID: daemonID, RuntimeID: task.RuntimeID,
				Path: worktreePath, Branch: branch,
			},
		},
		ArtifactRoot: workflowguard.StaticPreviewArtifactRoot,
		ExpiresAt:    now.UTC().Add(staticPreviewLifetime),
	}, true, nil
}

type staticPreviewRegistry struct {
	mu      sync.Mutex
	entries map[string]*staticPreview
}

const maxStaticPreviewAssetBytes = 8 << 20
const maxStaticPreviewRequestPath = 2 << 10

type staticPreviewAsset struct {
	Content     []byte
	ContentType string
	SHA256      string
}

type staticPreviewSnapshot struct {
	Assets   map[string]staticPreviewAsset
	Manifest map[string]string
}

var staticPreviewAssetAllowlist = map[string]string{
	"index.html": "text/html; charset=utf-8",
	"styles.css": "text/css; charset=utf-8",
	"app.js":     "text/javascript; charset=utf-8",
}

func newStaticPreviewRegistry() *staticPreviewRegistry {
	return &staticPreviewRegistry{entries: make(map[string]*staticPreview)}
}

func (r *staticPreviewRegistry) start(ctx context.Context, spec staticPreviewSpec, publish staticPreviewPublisher) error {
	if spec.TaskID == "" || spec.Binding.CandidateID == "" || publish == nil {
		return errors.New("static preview requires task, candidate, and publisher")
	}
	if _, err := resolveStaticPreviewRoot(spec.Binding.Worktree.Path, spec.ArtifactRoot); err != nil {
		return err
	}
	snapshot, err := snapshotStaticPreviewInWorktree(spec.Binding.Worktree.Path, spec.ArtifactRoot)
	if err != nil {
		return err
	}
	if !spec.ExpiresAt.After(time.Now().UTC()) || spec.ExpiresAt.Sub(time.Now().UTC()) > previewpublication.MaxLifetime {
		return errors.New("static preview expiry is invalid")
	}

	producer, err := newStaticPreview(ctx, spec, snapshot)
	if err != nil {
		return err
	}
	claim := previewpublication.ProducerClaim{
		PreviewURL:  producer.previewURL,
		IdentityURL: producer.identityURL,
		DataPolicy:  previewpublication.ReadOnlyDataPolicy(),
		ExpiresAt:   spec.ExpiresAt.UTC(),
	}
	if err := producer.selfProbe(claim); err != nil {
		_ = producer.close()
		return err
	}
	publication, err := publish(ctx, spec.TaskID, claim)
	if err != nil {
		_ = producer.close()
		return fmt.Errorf("publish static preview: %w", err)
	}
	if err := producer.verifyPublication(publication); err != nil {
		_ = producer.close()
		return err
	}

	key := spec.TaskID + "\x00" + spec.Binding.CandidateID
	r.mu.Lock()
	old := r.entries[key]
	r.entries[key] = producer
	r.mu.Unlock()
	if old != nil {
		_ = old.stop()
	}
	producer.startEligibilityPoll(publish)
	return nil
}

func (r *staticPreviewRegistry) stopTask(taskID string) {
	r.mu.Lock()
	var victims []*staticPreview
	for key, producer := range r.entries {
		if strings.HasPrefix(key, taskID+"\x00") {
			delete(r.entries, key)
			victims = append(victims, producer)
		}
	}
	r.mu.Unlock()
	for _, producer := range victims {
		_ = producer.stop()
	}
}

func (r *staticPreviewRegistry) stopAll() {
	r.mu.Lock()
	victims := make([]*staticPreview, 0, len(r.entries))
	for key, producer := range r.entries {
		delete(r.entries, key)
		victims = append(victims, producer)
	}
	r.mu.Unlock()
	for _, producer := range victims {
		_ = producer.stop()
	}
}

func (r *staticPreviewRegistry) fetch(request protocol.PreviewFetchRequest) protocol.PreviewFetchResponse {
	if request.TaskID == "" || request.AgentRunID != request.TaskID ||
		request.CandidateID == "" || request.CandidateRevision != 1 ||
		request.RuntimeID == "" || len(request.Path) == 0 ||
		len(request.Path) > maxStaticPreviewRequestPath ||
		(request.Method != http.MethodGet && request.Method != http.MethodHead) {
		return protocol.PreviewFetchResponse{Status: http.StatusBadRequest}
	}
	key := request.TaskID + "\x00" + request.CandidateID
	r.mu.Lock()
	producer := r.entries[key]
	r.mu.Unlock()
	if producer == nil {
		return protocol.PreviewFetchResponse{Status: http.StatusNotFound}
	}
	binding := producer.identity.Binding
	if binding.WorkspaceID != request.WorkspaceID || binding.IssueID != request.IssueID ||
		binding.AgentID != request.AgentID || binding.AgentRunID != request.AgentRunID ||
		binding.CandidateID != request.CandidateID ||
		binding.CandidateRevision != request.CandidateRevision ||
		binding.SourceRevision != request.SourceRevision ||
		binding.Worktree.DaemonID != request.DaemonID ||
		binding.Worktree.RuntimeID != request.RuntimeID ||
		binding.Worktree.Path != request.WorktreePath ||
		binding.Worktree.Branch != request.Branch {
		return protocol.PreviewFetchResponse{Status: http.StatusNotFound}
	}
	if !producer.expiresAt.After(time.Now().UTC()) {
		return protocol.PreviewFetchResponse{Status: http.StatusGone}
	}
	req, err := http.NewRequest(request.Method, "http://preview.local"+request.Path, nil)
	if err != nil {
		return protocol.PreviewFetchResponse{Status: http.StatusBadRequest}
	}
	recorder := httptest.NewRecorder()
	producer.staticHandler().ServeHTTP(recorder, req)
	result := recorder.Result()
	defer result.Body.Close()
	body, err := io.ReadAll(io.LimitReader(result.Body, maxStaticPreviewAssetBytes+1))
	if err != nil || len(body) > maxStaticPreviewAssetBytes {
		return protocol.PreviewFetchResponse{Status: http.StatusRequestEntityTooLarge}
	}
	headers := make(map[string]string)
	for _, name := range []string{
		"Content-Type", "Content-Length", "Content-Security-Policy",
		"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy",
		"Permissions-Policy", "Cross-Origin-Resource-Policy", "Allow",
	} {
		if value := result.Header.Get(name); value != "" {
			headers[name] = value
		}
	}
	return protocol.PreviewFetchResponse{Status: result.StatusCode, Headers: headers, Body: body}
}

type staticPreview struct {
	server      *http.Server
	listener    net.Listener
	previewURL  string
	identityURL string
	identity    previewpublication.Identity
	snapshot    staticPreviewSnapshot
	expiresAt   time.Time
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
	pollDone    chan struct{}
	pollStarted bool
}

func newStaticPreview(parent context.Context, spec staticPreviewSpec, snapshot staticPreviewSnapshot) (*staticPreview, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for static preview: %w", err)
	}
	ctx, cancel := context.WithCancel(parent)
	identity := previewpublication.Identity{
		SchemaVersion: previewpublication.IdentitySchemaVersion,
		Binding:       spec.Binding,
		DataPolicy:    previewpublication.ReadOnlyDataPolicy(),
		ExpiresAt:     spec.ExpiresAt.UTC(),
	}
	producer := &staticPreview{
		listener:    listener,
		previewURL:  "http://" + listener.Addr().String() + "/",
		identityURL: "http://" + listener.Addr().String() + previewpublication.IdentityPath,
		identity:    identity,
		snapshot:    snapshot,
		expiresAt:   spec.ExpiresAt.UTC(),
		ctx:         ctx,
		cancel:      cancel,
		pollDone:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", producer.staticHandler())
	producer.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       10 * time.Second,
	}
	go func() {
		if err := producer.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cancel()
		}
	}()
	go func() {
		timer := time.NewTimer(time.Until(producer.expiresAt))
		defer timer.Stop()
		select {
		case <-timer.C:
			_ = producer.close()
		case <-ctx.Done():
		}
	}()
	return producer, nil
}

func (p *staticPreview) staticHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setStaticPreviewHeaders(w)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == previewpublication.IdentityPath {
			data, err := json.Marshal(p.identity)
			if err != nil {
				http.Error(w, "identity unavailable", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			if r.Method == http.MethodGet {
				_, _ = w.Write(data)
			}
			return
		}
		if r.URL.Path == "" || r.URL.Path[0] != '/' {
			http.NotFound(w, r)
			return
		}
		relative, err := staticPreviewRequestPath(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		asset, ok := p.snapshot.Assets[relative]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", asset.ContentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(asset.Content)))
		http.ServeContent(w, r, relative, time.Time{}, bytes.NewReader(asset.Content))
	}
}

func setStaticPreviewHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'none'; form-action 'none'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; worker-src 'none'; child-src 'none'; manifest-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Cache-Control", "no-store")
}

func (p *staticPreview) selfProbe(claim previewpublication.ProducerClaim) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for _, rawURL := range []string{claim.PreviewURL, claim.IdentityURL} {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("static preview probe request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("static preview probe: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("static preview probe returned %d", resp.StatusCode)
		}
		if rawURL == claim.IdentityURL {
			var identity previewpublication.Identity
			if err := json.Unmarshal(body, &identity); err != nil || identity != p.identity {
				return errors.New("static preview identity probe mismatch")
			}
			if resp.Header.Get("Content-Type") != "application/json" || resp.Header.Get("Cache-Control") != "no-store" {
				return errors.New("static preview identity headers are invalid")
			}
		}
	}
	return nil
}

func (p *staticPreview) verifyPublication(publication previewpublication.Publication) error {
	if publication.PreviewURL != p.previewURL || publication.IdentityURL != p.identityURL ||
		publication.Binding != p.identity.Binding || publication.DataPolicy != p.identity.DataPolicy ||
		!publication.ExpiresAt.Equal(p.expiresAt) {
		return errors.New("controller publication does not match static preview identity")
	}
	return nil
}

func (p *staticPreview) startEligibilityPoll(publish staticPreviewPublisher) {
	p.pollStarted = true
	go func() {
		defer close(p.pollDone)
		timer := time.NewTicker(staticPreviewPoll)
		defer timer.Stop()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-timer.C:
				claim := previewpublication.ProducerClaim{
					PreviewURL: p.previewURL, IdentityURL: p.identityURL,
					DataPolicy: previewpublication.ReadOnlyDataPolicy(), ExpiresAt: p.expiresAt,
				}
				publication, err := publish(p.ctx, p.identity.Binding.AgentRunID, claim)
				if err != nil || p.verifyPublication(publication) != nil {
					_ = p.close()
					return
				}
			}
		}
	}()
}

func (p *staticPreview) close() error {
	var err error
	p.closeOnce.Do(func() {
		p.cancel()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = p.server.Shutdown(ctx)
	})
	return err
}

func (p *staticPreview) stop() error {
	err := p.close()
	if p.pollStarted {
		<-p.pollDone
	}
	return err
}

func resolveStaticPreviewRoot(worktree, artifactRoot string) (string, error) {
	if artifactRoot != workflowguard.StaticPreviewArtifactRoot {
		return "", errors.New("static preview artifact root is not controller-approved")
	}
	absoluteWorktree, err := filepath.Abs(worktree)
	if err != nil {
		return "", err
	}
	realWorktree, err := filepath.EvalSymlinks(absoluteWorktree)
	if err != nil {
		return "", fmt.Errorf("resolve preview worktree: %w", err)
	}
	root := filepath.Join(absoluteWorktree, filepath.FromSlash(artifactRoot))
	if !containedPath(absoluteWorktree, root) {
		return "", errors.New("static preview artifact root escapes worktree")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve static preview root: %w", err)
	}
	if !containedPath(realWorktree, realRoot) {
		return "", errors.New("static preview root escapes resolved worktree")
	}
	index, err := os.Lstat(filepath.Join(realRoot, "index.html"))
	if err != nil || !index.Mode().IsRegular() {
		return "", errors.New("static preview requires a regular index.html")
	}
	if _, err := filepath.EvalSymlinks(filepath.Join(realRoot, "index.html")); err != nil {
		return "", errors.New("resolve static preview index")
	}
	return realRoot, nil
}

func staticPreviewRequestPath(requestPath string) (string, error) {
	if requestPath == "/" {
		return "index.html", nil
	}
	clean := strings.TrimPrefix(path.Clean(requestPath), "/")
	if clean == "" || clean == "." || clean == ".." || strings.Contains(clean, "/") {
		return "", errors.New("static preview path is not allowlisted")
	}
	if _, ok := staticPreviewAssetAllowlist[clean]; !ok {
		return "", errors.New("static preview asset is not allowlisted")
	}
	return clean, nil
}

func snapshotStaticPreview(rootPath string) (staticPreviewSnapshot, error) {
	parent, name := filepath.Dir(rootPath), filepath.Base(rootPath)
	root, err := os.OpenRoot(parent)
	if err != nil {
		return staticPreviewSnapshot{}, fmt.Errorf("open static preview root: %w", err)
	}
	defer root.Close()
	return snapshotStaticPreviewFromRoot(root, filepath.ToSlash(name))
}

func snapshotStaticPreviewInWorktree(worktreePath, artifactRoot string) (staticPreviewSnapshot, error) {
	if artifactRoot != workflowguard.StaticPreviewArtifactRoot {
		return staticPreviewSnapshot{}, errors.New("static preview artifact root is not controller-approved")
	}
	root, err := os.OpenRoot(worktreePath)
	if err != nil {
		return staticPreviewSnapshot{}, fmt.Errorf("open static preview worktree: %w", err)
	}
	defer root.Close()
	return snapshotStaticPreviewFromRoot(root, artifactRoot)
}

func snapshotStaticPreviewFromRoot(root *os.Root, directoryPath string) (staticPreviewSnapshot, error) {
	beforeDirectory, err := root.Lstat(directoryPath)
	if err != nil || !beforeDirectory.IsDir() || beforeDirectory.Mode()&os.ModeSymlink != 0 {
		return staticPreviewSnapshot{}, errors.New("static preview root is not a regular directory")
	}
	directory, err := root.Open(directoryPath)
	if err != nil {
		return staticPreviewSnapshot{}, fmt.Errorf("open static preview directory: %w", err)
	}
	openedDirectory, statErr := directory.Stat()
	entries, err := directory.ReadDir(-1)
	directory.Close()
	if err != nil {
		return staticPreviewSnapshot{}, fmt.Errorf("read static preview directory: %w", err)
	}
	if statErr != nil || !os.SameFile(openedDirectory, beforeDirectory) {
		return staticPreviewSnapshot{}, errors.New("static preview directory changed while opening")
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if _, ok := staticPreviewAssetAllowlist[entry.Name()]; !ok || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return staticPreviewSnapshot{}, fmt.Errorf("static preview contains unapproved artifact %q", entry.Name())
		}
		seen[entry.Name()] = true
	}
	if !seen["index.html"] {
		return staticPreviewSnapshot{}, errors.New("static preview requires index.html")
	}
	snapshot := staticPreviewSnapshot{
		Assets:   make(map[string]staticPreviewAsset, len(seen)),
		Manifest: make(map[string]string, len(seen)),
	}
	for name := range seen {
		assetPath := path.Join(directoryPath, name)
		before, err := root.Lstat(assetPath)
		if err != nil || !before.Mode().IsRegular() {
			return staticPreviewSnapshot{}, fmt.Errorf("static preview asset %q is not regular", name)
		}
		file, err := root.Open(assetPath)
		if err != nil {
			return staticPreviewSnapshot{}, fmt.Errorf("open static preview asset %q: %w", name, err)
		}
		opened, statErr := file.Stat()
		after, lstatErr := root.Lstat(assetPath)
		if statErr != nil || lstatErr != nil || !after.Mode().IsRegular() || !os.SameFile(opened, before) || !os.SameFile(opened, after) {
			file.Close()
			return staticPreviewSnapshot{}, fmt.Errorf("static preview asset %q changed while snapshotting", name)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxStaticPreviewAssetBytes+1))
		file.Close()
		if readErr != nil {
			return staticPreviewSnapshot{}, fmt.Errorf("read static preview asset %q: %w", name, readErr)
		}
		if len(data) > maxStaticPreviewAssetBytes {
			return staticPreviewSnapshot{}, fmt.Errorf("static preview asset %q exceeds limit", name)
		}
		if containsPreviewCredential(data) {
			return staticPreviewSnapshot{}, fmt.Errorf("static preview asset %q contains credential-like data", name)
		}
		sum := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		snapshot.Assets[name] = staticPreviewAsset{
			Content: append([]byte(nil), data...), ContentType: staticPreviewAssetAllowlist[name], SHA256: sum,
		}
		snapshot.Manifest[name] = sum
	}
	afterDirectory, err := root.Lstat(directoryPath)
	if err != nil || !afterDirectory.IsDir() || !os.SameFile(openedDirectory, afterDirectory) {
		return staticPreviewSnapshot{}, errors.New("static preview directory changed while snapshotting")
	}
	return snapshot, nil
}

func containsPreviewCredential(data []byte) bool {
	lower := strings.ToLower(string(data))
	for _, marker := range []string{
		"-----begin private key-----", "authorization: bearer ", "msr_", "mul_",
		"password=", "password:", "secret=", "secret:", "api_key", "api-key",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func containedPath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
