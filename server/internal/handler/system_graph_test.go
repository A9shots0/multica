package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/systemgraph"
)

func TestGetSystemGraphReturnsValidatedRedactedCatalog(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Handler{}).GetSystemGraph(recorder, httptest.NewRequest(http.MethodGet, "/api/workspaces/test/system-graph", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var catalog systemgraph.Catalog
	if err := json.Unmarshal(recorder.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), "api_key") || strings.Contains(strings.ToLower(recorder.Body.String()), "secret") {
		t.Fatal("system graph response contains secret-shaped fields")
	}
}

func TestQuerySystemGraphRoutesPhoneFarmToMacOnly(t *testing.T) {
	body := `{"query":"issue_route","route":{"observed_node_ids":["phonefarm_app"],"risk_flags":["production"]}}`
	recorder := httptest.NewRecorder()
	(&Handler{}).QuerySystemGraph(recorder, httptest.NewRequest(http.MethodPost, "/api/workspaces/test/system-graph/query", strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"mac_studio_only"`) || !strings.Contains(recorder.Body.String(), `"windows"`) {
		t.Fatalf("host boundary absent: %s", recorder.Body.String())
	}
}

func TestQuerySystemGraphRejectsArbitraryOrMalformedQueries(t *testing.T) {
	for _, body := range []string{`{"query":"sql","seed_node_ids":["crm"]}`, `{"query":"blast_radius","surprise":true}`} {
		recorder := httptest.NewRecorder()
		(&Handler{}).QuerySystemGraph(recorder, httptest.NewRequest(http.MethodPost, "/api/workspaces/test/system-graph/query", strings.NewReader(body)))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status=%d response=%s", body, recorder.Code, recorder.Body.String())
		}
	}
}

func TestQuerySystemGraphExposesBoundedEdgeBlastAndAuthority(t *testing.T) {
	for _, body := range []string{
		`{"query":"blast_radius","seed_edge_ids":["e_identity_linkhub"]}`,
		`{"query":"authority_for","target_id":"canopy_media_pipeline"}`,
		`{"query":"stale_evidence"}`,
	} {
		recorder := httptest.NewRecorder()
		(&Handler{}).QuerySystemGraph(recorder, httptest.NewRequest(http.MethodPost, "/api/workspaces/test/system-graph/query", strings.NewReader(body)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("body %s status=%d response=%s", body, recorder.Code, recorder.Body.String())
		}
		if recorder.Body.Len() == 0 {
			t.Fatalf("body %s returned no evidence", body)
		}
	}
}

func TestGetRoleArchitectureReturnsControllerOwnedBoundedSquads(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Handler{}).GetRoleArchitecture(recorder, httptest.NewRequest(http.MethodGet, "/api/workspaces/test/role-architecture", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var roles systemgraph.RoleCatalog
	if err := json.Unmarshal(recorder.Body.Bytes(), &roles); err != nil {
		t.Fatal(err)
	}
	graph, err := systemgraph.Default()
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Validate(graph); err != nil {
		t.Fatal(err)
	}
}

func TestGetAgentConfigurationsReturnsEvidenceScopedFailClosedDecisions(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Handler{}).GetAgentConfigurations(recorder, httptest.NewRequest(http.MethodGet, "/api/workspaces/test/agent-configurations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var decisions agentqualification.DecisionCatalog
	if err := json.Unmarshal(recorder.Body.Bytes(), &decisions); err != nil {
		t.Fatal(err)
	}
	if err := decisions.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, decision := range decisions.Decisions {
		if decision.AllowsExecution(true) {
			t.Fatalf("configuration %s grants production execution", decision.Configuration.ID)
		}
	}
}
