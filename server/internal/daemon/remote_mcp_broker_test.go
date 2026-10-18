package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/systemgraph"
	"github.com/multica-ai/multica/server/pkg/remotemcp"
	"github.com/multica-ai/multica/server/pkg/remotemcp/remotemcptest"
)

func TestRemoteMCPProxyFiltersToolsAndInjectsCredential(t *testing.T) {
	fixture := remotemcptest.NewServer()
	defer fixture.Close()
	schema := json.RawMessage(`{"type":"object","properties":{}}`)
	canonical, err := canonicalRemoteMCPJSON(schema)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(fixture.URL)
	proxy := &remoteMCPProxy{
		taskID: "task", endpoint: endpoint, client: fixture.Client(), path: "/capability",
		credentialHeaders: http.Header{"Authorization": []string{"Bearer " + remotemcptest.Credential}},
		semaphore:         make(chan struct{}, remoteMCPMaxConcurrency), logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		connection: remotemcp.Connection{
			InstallationID: "installation", ContributionKey: "fixture",
			ApprovedTools: []remotemcp.Tool{{
				Name: "fixture.read", Description: "pinned", InputSchema: canonical,
				SchemaDigest: remotemcp.DigestBytes(canonical), Risk: "read",
			}},
		},
	}

	request := httptest.NewRequest(http.MethodPost, "/capability", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "fixture.write") || !strings.Contains(response.Body.String(), "fixture.read") {
		t.Fatalf("filtered tools/list = %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "pinned") {
		t.Fatalf("tools/list did not preserve reviewed description: %s", response.Body.String())
	}

	writeSchema, err := canonicalRemoteMCPJSON(json.RawMessage(`{
		"type":"object",
		"required":["value"],
		"properties":{"value":{"type":"string"}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	proxy.connection.ApprovedTools = append(proxy.connection.ApprovedTools, remotemcp.Tool{
		Name: "fixture.write", InputSchema: writeSchema, SchemaDigest: remotemcp.DigestBytes(writeSchema), Risk: "write",
	})
	writeRequest := httptest.NewRequest(http.MethodPost, "/capability", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"fixture.write","arguments":{"value":"broker-write"}}}`))
	writeResponse := httptest.NewRecorder()
	proxy.ServeHTTP(writeResponse, writeRequest)
	if writes := fixture.Writes(); len(writes) != 1 || writes[0] != "broker-write" {
		t.Fatalf("fixture writes = %#v, response = %s", writes, writeResponse.Body.String())
	}
}

func TestRemoteMCPProxyRejectsUnapprovedToolWithoutCallingUpstream(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer upstream.Close()
	endpoint, _ := url.Parse(upstream.URL)
	proxy := &remoteMCPProxy{
		endpoint: endpoint, client: upstream.Client(), path: "/capability",
		semaphore:  make(chan struct{}, remoteMCPMaxConcurrency),
		connection: remotemcp.Connection{ApprovedTools: []remotemcp.Tool{{Name: "allowed"}}},
	}
	request := httptest.NewRequest(http.MethodPost, "/capability", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"denied","arguments":{"secret":"not logged"}}}`))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if called || !strings.Contains(response.Body.String(), "not approved") {
		t.Fatalf("called=%v response=%s", called, response.Body.String())
	}
}

func TestRemoteMCPProxyPreservesAcceptedNotificationStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("upstream method = %s", request.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()
	endpoint, _ := url.Parse(upstream.URL)
	proxy := &remoteMCPProxy{
		endpoint: endpoint, client: upstream.Client(), path: "/capability",
		semaphore: make(chan struct{}, remoteMCPMaxConcurrency),
	}

	request := httptest.NewRequest(http.MethodPost, "/capability", bytes.NewBufferString(
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
	))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("notification response body = %q, want empty", response.Body.String())
	}
}

func TestRemoteMCPProxyRechecksCredentialBeforeUpstreamCall(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer upstream.Close()
	endpoint, _ := url.Parse(upstream.URL)
	proxy := &remoteMCPProxy{
		endpoint: endpoint, client: upstream.Client(), path: "/capability",
		semaphore: make(chan struct{}, remoteMCPMaxConcurrency),
		connection: remotemcp.Connection{
			ContributionID: "contribution", CredentialHeader: "Authorization",
			ApprovedTools: []remotemcp.Tool{{Name: "allowed"}},
		},
		resolveCredential: func(context.Context, string) (http.Header, error) {
			return nil, errors.New("revoked")
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/capability", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"allowed","arguments":{}}}`))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if called || !strings.Contains(response.Body.String(), "revoked or unavailable") {
		t.Fatalf("called=%v response=%s", called, response.Body.String())
	}
}

func TestRemoteMCPGuardRejectsBeforeCredentialResolutionOrDiscovery(t *testing.T) {
	credentialResolved := false
	connection := remotemcp.Connection{
		ContributionID: "contribution", ContributionKey: "guarded", Endpoint: "https://example.invalid/mcp",
		CredentialHeader: "Authorization", FailurePolicy: "optional",
		ApprovedTools: []remotemcp.Tool{{Name: "allowed"}},
	}
	guard := remoteMCPGuardFixture(t, time.Now().UTC(), "allowed", "contribution")
	guard.Scope.Receipt.CredentialPurposes = []string{"remote_mcp:other"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, brokers, err := startTaskRemoteMCPBrokers(
		ctx, ctx, "guarded-task", "claude", []remotemcp.Connection{connection},
		func(context.Context, string) (http.Header, error) {
			credentialResolved = true
			return http.Header{"Authorization": []string{"secret"}}, nil
		}, nil, guard,
	)
	if err == nil || !strings.Contains(err.Error(), "guarded capability denied") {
		t.Fatalf("error = %v, want guarded capability denial", err)
	}
	if credentialResolved || brokers != nil {
		t.Fatalf("guard denial resolved credential or started broker: resolved=%v brokers=%v", credentialResolved, brokers)
	}
}

func TestRemoteMCPGuardRejectsToolBeforeCredentialOrUpstream(t *testing.T) {
	upstreamCalled := false
	credentialResolved := false
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { upstreamCalled = true }))
	defer upstream.Close()
	endpoint, _ := url.Parse(upstream.URL)
	connection := remotemcp.Connection{
		ContributionID: "contribution", CredentialHeader: "Authorization",
		ApprovedTools: []remotemcp.Tool{{Name: "approved"}},
	}
	proxy := &remoteMCPProxy{
		endpoint: endpoint, client: upstream.Client(), path: "/capability",
		semaphore: make(chan struct{}, remoteMCPMaxConcurrency), connection: connection,
		capabilityGuard: remoteMCPGuardFixture(t, time.Now().UTC(), "different", "contribution"),
		resolveCredential: func(context.Context, string) (http.Header, error) {
			credentialResolved = true
			return http.Header{"Authorization": []string{"secret"}}, nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/capability", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"approved","arguments":{}}}`,
	))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if upstreamCalled || credentialResolved || !strings.Contains(response.Body.String(), "not authorized for this task") {
		t.Fatalf("upstream=%v credential=%v response=%s", upstreamCalled, credentialResolved, response.Body.String())
	}
}

func TestRemoteMCPGuardAllowsExactToolAndCredentialPurpose(t *testing.T) {
	fixture := remotemcptest.NewServer()
	defer fixture.Close()
	endpoint, _ := url.Parse(fixture.URL)
	connection := remotemcp.Connection{
		ContributionID: "contribution", CredentialHeader: "Authorization",
		ApprovedTools: []remotemcp.Tool{{Name: "fixture.read"}},
	}
	credentialResolved := false
	proxy := &remoteMCPProxy{
		endpoint: endpoint, client: fixture.Client(), path: "/capability",
		semaphore: make(chan struct{}, remoteMCPMaxConcurrency), connection: connection,
		capabilityGuard: remoteMCPGuardFixture(t, time.Now().UTC(), "fixture.read", "contribution"),
		resolveCredential: func(context.Context, string) (http.Header, error) {
			credentialResolved = true
			return http.Header{"Authorization": []string{"Bearer " + remotemcptest.Credential}}, nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/capability", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"fixture.read","arguments":{}}}`,
	))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if !credentialResolved || !strings.Contains(response.Body.String(), `"result"`) {
		t.Fatalf("credential=%v response=%s", credentialResolved, response.Body.String())
	}
}

func remoteMCPGuardFixture(t *testing.T, now time.Time, toolName, contributionID string) *remoteMCPTaskCapabilityGuard {
	t.Helper()
	route := systemgraph.RoutePlan{
		Mode:            "execute",
		AllowedHosts:    []string{"alienware_windows"},
		ScopedNodeIDs:   []string{"crm_product"},
		CoverageGate:    "expand",
		RiskClass:       "consequential",
		RequestedAction: "edit_source",
	}
	receipt := systemgraph.CapabilityReceipt{
		ID: "cap-remote-mcp", ExecutorAgentID: "agent-1", IssueID: "issue-1", AuthorityRevision: 2,
		RouteVersion: "route-1", RoleID: "shared_windows_executor", ConfigurationID: "future.windows.runtime.v1",
		Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, HostAttestation: "alienware_windows",
		AllowedTools: []string{toolName}, CredentialPurposes: []string{"remote_mcp:" + contributionID},
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), CoverageReceiptID: "assure-1",
		OwnerAuthorizationReceiptID: "owner-1", CandidateID: "candidate-1", AuthorizedScope: "crm-only",
	}
	assurance := &systemgraph.ArchitectureAssuranceReceipt{
		ID: "assure-1", IssueID: "issue-1", RouteVersion: "route-1", CoverageGate: "expand",
		IssuerAgentID: "architect-1", IssuerRoleID: "architecture_assurance",
		IssuerConfigurationID: "future.architecture.runtime.v1", Source: "controller_record",
		Passed: true, Immutable: true, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
	}
	owner := &systemgraph.OwnerAuthorizationReceipt{
		ID: "owner-1", IssueID: "issue-1", AuthorityRevision: 2, CandidateID: "candidate-1",
		Action: "edit_source", TargetNodeIDs: []string{"crm_product"}, AuthorizedScope: "crm-only",
		IssuerRoleID: "owner", Source: "controller_record", Immutable: true, RecordedAt: now.Add(-time.Minute),
	}
	scope := &agentqualification.TaskCapabilityScopeV1{
		Version: agentqualification.TaskCapabilityScopeVersionV1, WorkspaceID: "workspace-1", IssueID: "issue-1",
		ExecutorAgentID: "agent-1", RuntimeID: "runtime-1", RoleID: "shared_windows_executor",
		ConfigurationID: "future.windows.runtime.v1", RuntimeBindingReceiptID: "runtime-binding-1", RouteVersion: "route-1", HostAttestation: "alienware_windows",
		Action: "edit_source", Route: route, Receipt: receipt, Assurance: assurance, OwnerAuthorization: owner,
	}
	actual := agentqualification.TaskCapabilityGatewayCallV1{
		WorkspaceID: "workspace-1", IssueID: "issue-1", ExecutorAgentID: "agent-1", RuntimeID: "runtime-1",
		RoleID: "shared_windows_executor", ConfigurationID: "future.windows.runtime.v1", RouteVersion: "route-1",
		HostAttestation: "alienware_windows", Action: "edit_source", TargetNodeIDs: []string{"crm_product"},
		CandidateID: "candidate-1", AuthorizedScope: "crm-only", AssuranceConfigurationID: "future.architecture.runtime.v1",
	}
	return &remoteMCPTaskCapabilityGuard{Scope: scope, Actual: actual, Now: func() time.Time { return now }}
}

func TestRemoteMCPProviderMatrixAndConfigMerge(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "hermes", "qoder", "mcode"} {
		if !providerSupportsRemoteMCPBroker(provider) {
			t.Fatalf("provider %s must support Remote MCP", provider)
		}
	}
	if providerSupportsRemoteMCPBroker("deveco") {
		t.Fatal("unsupported provider was accepted")
	}
	merged, err := mergeTaskRemoteMCPConfig(
		json.RawMessage(`{"mcpServers":{"agent":{"command":"agent"}}}`),
		json.RawMessage(`{"mcpServers":{"plugin":{"type":"http","url":"http://127.0.0.1/mcp"}}}`),
	)
	if err != nil || !strings.Contains(string(merged), `"agent"`) || !strings.Contains(string(merged), `"plugin"`) {
		t.Fatalf("merge = %s, %v", merged, err)
	}
}

func TestDecodeRemoteMCPSSEData(t *testing.T) {
	raw, err := decodeRemoteMCPSSEData("text/event-stream; charset=utf-8", []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1}\n\n"))
	if err != nil || string(raw) != `{"jsonrpc":"2.0","id":1}` {
		t.Fatalf("decodeRemoteMCPSSEData = %q, %v", raw, err)
	}
}
