package brain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const multicaPlatformProjectID = "7d4de3db-8a78-4b01-8cb3-664e95d23d22"

func portfolioClient(t *testing.T, body string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/knowledge/projects" {
			t.Fatalf("unexpected portfolio request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Client{
		config: Config{URL: server.URL, Workspaces: map[string]Workspace{
			workspaceID: {Projects: map[string]string{multicaPlatformProjectID: "multica"}},
		}},
		token: "test-credential",
		http:  server.Client(),
	}
}

func TestPortfolioResolverBindsRealMulticaProjectToKnowledgeIdentity(t *testing.T) {
	client := portfolioClient(t, `{
		"schemaVersion":"silvermediums.brain.v2",
		"servedAt":"2026-10-19T00:53:42Z",
		"projects":[{
			"id":"multica",
			"revision":"9585421724da1c797fd073f9340a0a4aa001df06573dd251f7800800740012f5",
			"portfolioClass":"company_infrastructure",
			"portfolioClassification":{
				"evidenceStage":"business-verified",
				"observedAt":"2026-10-18",
				"programId":"COMP-REA",
				"sourceReference":"initiatives/company-improvement/portfolio.json",
				"sourceRepository":"A9shots0/Silver-Mediums-Knowledge",
				"sourceRevision":"e3f7669146740cbeb3069489349931a63f5890d1",
				"sourceSha256":"08ca036de90092ec2553ca34a06ea58fd7330649e7fc2f387d0edf2536f30a34"
			}
		}]
	}`)

	bindings, err := NewPortfolioResolver(client).ResolveCompanyProjects(context.Background(), workspaceID, []string{multicaPlatformProjectID})
	if err != nil {
		t.Fatal(err)
	}
	binding, ok := bindings[multicaPlatformProjectID]
	if !ok {
		t.Fatal("verified Multica portfolio binding missing")
	}
	if binding.CanonicalProjectID != "multica" || binding.CompanyMembership != "company" || binding.PortfolioClassification != "company_infrastructure" {
		t.Fatalf("unexpected binding: %+v", binding)
	}
	if binding.Provenance == nil || binding.Provenance.SourceRevision != "e3f7669146740cbeb3069489349931a63f5890d1" || binding.Provenance.ProjectionRevision == "" {
		t.Fatalf("usable provenance missing: %+v", binding.Provenance)
	}
}

func TestPortfolioResolverLeavesUnknownOrMismatchedProjectsUnverified(t *testing.T) {
	t.Run("unknown local project", func(t *testing.T) {
		client := portfolioClient(t, `{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","projects":[]}`)
		bindings, err := NewPortfolioResolver(client).ResolveCompanyProjects(context.Background(), workspaceID, []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"})
		if err != nil || len(bindings) != 0 {
			t.Fatalf("unknown project produced a binding: %+v, %v", bindings, err)
		}
	})

	t.Run("brain identity mismatch", func(t *testing.T) {
		client := portfolioClient(t, `{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","projects":[{"id":"brain","revision":"revision","portfolioClass":"company_infrastructure","portfolioClassification":{"evidenceStage":"business-verified","observedAt":"2026-10-18","sourceReference":"initiatives/company-improvement/portfolio.json","sourceRepository":"A9shots0/Silver-Mediums-Knowledge","sourceRevision":"commit"}}]}`)
		bindings, err := NewPortfolioResolver(client).ResolveCompanyProjects(context.Background(), workspaceID, []string{multicaPlatformProjectID})
		if err != nil || len(bindings) != 0 {
			t.Fatalf("mismatched Brain project produced a binding: %+v, %v", bindings, err)
		}
	})
}

func TestPortfolioResolverRejectsUnsupportedProvenance(t *testing.T) {
	client := portfolioClient(t, `{"schemaVersion":"silvermediums.brain.v2","servedAt":"now","projects":[{"id":"multica","revision":"revision","portfolioClass":"company_infrastructure","portfolioClassification":{"evidenceStage":"business-verified","observedAt":"2026-10-18","sourceReference":"other.json","sourceRepository":"A9shots0/another-registry","sourceRevision":"commit"}}]}`)
	bindings, err := NewPortfolioResolver(client).ResolveCompanyProjects(context.Background(), workspaceID, []string{multicaPlatformProjectID})
	if err != nil || len(bindings) != 0 {
		t.Fatalf("unsupported provenance produced a verified binding: %+v, %v", bindings, err)
	}
}
