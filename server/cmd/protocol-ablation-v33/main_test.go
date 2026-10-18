package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestV33PlanIsFrozenProviderFreeFourCells(t *testing.T) {
	p, err := v33Plan()
	if err != nil {
		t.Fatal(err)
	}
	if p["provider_called"] != false || len(p["cells"].([]agentqualification.OpenCodeStructuredV33Capsule)) != 4 || p["max_combined_cost_micros"].(int64) != 1_805_000 {
		t.Fatalf("bad v33 plan: %+v", p)
	}
}

func TestV33PendingClaimHaltsBeforeRuntimePreparation(t *testing.T) {
	old := os.Getenv("MULTICA_V33_EXECUTE_PAID")
	defer os.Setenv("MULTICA_V33_EXECUTE_PAID", old)
	os.Setenv("MULTICA_V33_EXECUTE_PAID", v33PaidConfirmation)
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		t.Fatal(err)
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil || len(order) != 4 {
		t.Fatalf("execution order: %v", err)
	}
	b := order[0]
	ledger := agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{b.BindingID: {BindingID: b.BindingID, Status: "pending"}}}
	root := t.TempDir()
	if err := writeV33Atomic(filepath.Join(root, "ledger.json"), ledger); err != nil {
		t.Fatal(err)
	}
	got, err := executeV33(context.Background(), root)
	if err == nil || !got.GlobalHalt || got.HaltReason != "ambiguous_transport" {
		t.Fatalf("pending recovery did not halt safely: %+v %v", got, err)
	}
}

func TestV33UsageMathRejectsOverflow(t *testing.T) {
	u := agentqualification.ProtocolAblationV32Usage{ProviderTurns: math.MaxInt, InputTokens: math.MaxInt64}
	if addUsageV33(&u, agentqualification.ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 1}) {
		t.Fatal("overflow accepted")
	}
	if _, ok := safeTotalV33(agentqualification.ProtocolAblationV32Usage{InputTokens: math.MaxInt64, OutputTokens: 1}); ok {
		t.Fatal("total overflow accepted")
	}
}

func TestV33LedgerJSONRemainsDurableShape(t *testing.T) {
	raw := mustV33(agentqualification.ProtocolAblationV32Ledger{Claims: map[string]agentqualification.ProtocolAblationV32Claim{}})
	var decoded agentqualification.ProtocolAblationV32Ledger
	if json.Unmarshal(raw, &decoded) != nil || decoded.Claims == nil {
		t.Fatal("ledger durability encoding failed")
	}
}

func TestV33WorkspacePreparationCreatesMissingParent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing", "artifact-root")
	workspace, err := prepareV33Workspace(root, "candidate/protocol")
	if err != nil {
		t.Fatal(err)
	}
	wantParent := filepath.Join(root, "workspaces", "candidate_protocol")
	if info, err := os.Stat(wantParent); err != nil || !info.IsDir() {
		t.Fatalf("workspace parent missing after preparation: %v", err)
	}
	if workspace.Root == "" || workspace.InitialCopy == "" {
		t.Fatalf("fixture copies missing: %+v", workspace)
	}
}
func TestV33ExecuteRequiresExplicitPaidConfirmation(t *testing.T) {
	old := os.Getenv("MULTICA_V33_EXECUTE_PAID")
	defer os.Setenv("MULTICA_V33_EXECUTE_PAID", old)
	os.Unsetenv("MULTICA_V33_EXECUTE_PAID")
	if _, err := executeV33(context.Background(), t.TempDir()); err == nil {
		t.Fatal("paid execution unlocked")
	}
}

func TestV33RuntimeHTTPClientUsesFrozenBoundedTimeout(t *testing.T) {
	client := newV33HTTPClient()
	want := time.Duration(agentqualification.OpenCodeStructuredV33HTTPTimeoutSeconds) * time.Second
	if client.Timeout != want || want != 120*time.Second {
		t.Fatalf("timeout=%s want=%s", client.Timeout, want)
	}
}

func TestV33ReadinessUsesSeparateBoundedRequestAndPreservesObservedHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/doc" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte("exact-openapi-bytes"))
	}))
	defer server.Close()
	raw, observed, err := fetchV33OpenAPI(context.Background(), server.URL)
	if err != nil || string(raw) != "exact-openapi-bytes" || len(observed) != 64 {
		t.Fatalf("raw=%q hash=%q err=%v", raw, observed, err)
	}
	_, want := agentqualification.HashOpenCodeStructuredV33Runtime(nil, raw)
	if observed != want {
		t.Fatalf("observed=%s want=%s", observed, want)
	}
}

func TestV33ReadinessNeverLabelsEmptyOrFailedTransportAsHashMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	raw, observed, err := fetchV33OpenAPI(context.Background(), server.URL)
	if err == nil || raw != nil || observed != "" || strings.Contains(strings.ToLower(err.Error()), "hash mismatch") {
		t.Fatalf("raw=%q hash=%q err=%v", raw, observed, err)
	}

	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer blocked.Close()
	started := time.Now()
	raw, observed, err = fetchV33OpenAPI(context.Background(), blocked.URL)
	if err == nil || raw != nil || observed != "" || time.Since(started) > 3*time.Second || strings.Contains(strings.ToLower(err.Error()), "hash mismatch") {
		t.Fatalf("unbounded/misclassified readiness: elapsed=%s raw=%q hash=%q err=%v", time.Since(started), raw, observed, err)
	}
}

func TestV33HaltArtifactPreservesExactObservedHashError(t *testing.T) {
	root := t.TempDir()
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger := agentqualification.ProtocolAblationV32Ledger{}
	cause := context.DeadlineExceeded
	_ = sealV33Halt(root, ledgerPath, &ledger, "binding", "runtime_failure", cause)
	raw, err := os.ReadFile(filepath.Join(root, "halts", "binding.json"))
	if err != nil || !strings.Contains(string(raw), cause.Error()) || !strings.Contains(string(raw), "error_sha256") {
		t.Fatalf("halt evidence missing: %s %v", raw, err)
	}
}
