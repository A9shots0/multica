package service

import (
	"context"
	"errors"
	"testing"
)

type portfolioResolverFunc func(context.Context, string, []string) (map[string]CompanyPortfolioBinding, error)

func (f portfolioResolverFunc) ResolveCompanyProjects(ctx context.Context, workspaceID string, localProjectIDs []string) (map[string]CompanyPortfolioBinding, error) {
	return f(ctx, workspaceID, localProjectIDs)
}

func verifiedMulticaPortfolioBinding() CompanyPortfolioBinding {
	return CompanyPortfolioBinding{
		CanonicalProjectID:      "multica",
		CompanyMembership:       "company",
		PortfolioClassification: "company_infrastructure",
		VerificationState:       "verified",
		Provenance: &CompanyPortfolioProvenance{
			SourceSystem:       "silver-mediums-knowledge",
			SourceRepository:   "A9shots0/Silver-Mediums-Knowledge",
			SourceReference:    "initiatives/company-improvement/portfolio.json",
			SourceRevision:     "e3f7669146740cbeb3069489349931a63f5890d1",
			SourceObservedAt:   "2026-10-18",
			EvidenceStage:      "business-verified",
			ProgramID:          "COMP-REA",
			ProjectionSystem:   "silver-mediums-brain",
			ProjectionRevision: "9585421724da1c797fd073f9340a0a4aa001df06573dd251f7800800740012f5",
			ProjectedAt:        "2026-10-19T00:53:42Z",
		},
	}
}

func TestAttachCompanyPortfolioBindingsPreservesLocalProjectIdentity(t *testing.T) {
	const localID = "7d4de3db-8a78-4b01-8cb3-664e95d23d22"
	service := CompanyOperationsService{PortfolioResolver: portfolioResolverFunc(func(_ context.Context, workspaceID string, localIDs []string) (map[string]CompanyPortfolioBinding, error) {
		if workspaceID != "11111111-1111-4111-8111-111111111111" || len(localIDs) != 1 || localIDs[0] != localID {
			t.Fatalf("unexpected resolver scope: %s %v", workspaceID, localIDs)
		}
		return map[string]CompanyPortfolioBinding{localID: verifiedMulticaPortfolioBinding()}, nil
	})}
	projections := []CompanyServiceProjection{{ProjectID: stringPointer(localID)}}
	service.attachCompanyPortfolioBindings(context.Background(), "11111111-1111-4111-8111-111111111111", projections)

	if projections[0].ProjectID == nil || *projections[0].ProjectID != localID {
		t.Fatal("Multica local project identity changed")
	}
	if projections[0].PortfolioBinding == nil || projections[0].PortfolioBinding.CanonicalProjectID != "multica" {
		t.Fatalf("canonical binding missing: %+v", projections[0].PortfolioBinding)
	}
}

func TestAttachCompanyPortfolioBindingsFailsClosed(t *testing.T) {
	const localID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for _, test := range []struct {
		name     string
		resolver CompanyPortfolioResolver
	}{
		{name: "missing mapping", resolver: portfolioResolverFunc(func(context.Context, string, []string) (map[string]CompanyPortfolioBinding, error) {
			return map[string]CompanyPortfolioBinding{}, nil
		})},
		{name: "resolver unavailable", resolver: portfolioResolverFunc(func(context.Context, string, []string) (map[string]CompanyPortfolioBinding, error) {
			return nil, errors.New("unavailable")
		})},
		{name: "invalid provenance", resolver: portfolioResolverFunc(func(context.Context, string, []string) (map[string]CompanyPortfolioBinding, error) {
			return map[string]CompanyPortfolioBinding{localID: {CanonicalProjectID: "multica", CompanyMembership: "company", PortfolioClassification: "company_infrastructure", VerificationState: "verified"}}, nil
		})},
		{name: "classification mismatch", resolver: portfolioResolverFunc(func(context.Context, string, []string) (map[string]CompanyPortfolioBinding, error) {
			binding := verifiedMulticaPortfolioBinding()
			binding.CompanyMembership = "personal"
			return map[string]CompanyPortfolioBinding{localID: binding}, nil
		})},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := CompanyOperationsService{PortfolioResolver: test.resolver}
			projections := []CompanyServiceProjection{{ProjectID: stringPointer(localID)}}
			service.attachCompanyPortfolioBindings(context.Background(), "11111111-1111-4111-8111-111111111111", projections)
			binding := projections[0].PortfolioBinding
			if binding == nil || binding.CompanyMembership != "unknown" || binding.PortfolioClassification != "unknown" || binding.VerificationState != "unverified" || binding.CanonicalProjectID != "" {
				t.Fatalf("fail-closed binding = %+v", binding)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }
