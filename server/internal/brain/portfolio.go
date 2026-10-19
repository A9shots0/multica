package brain

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/multica-ai/multica/server/internal/service"
)

const canonicalKnowledgeRepository = "A9shots0/Silver-Mediums-Knowledge"

type portfolioClassification struct {
	EvidenceStage    string `json:"evidenceStage"`
	ObservedAt       string `json:"observedAt"`
	ProgramID        string `json:"programId"`
	SourceReference  string `json:"sourceReference"`
	SourceRepository string `json:"sourceRepository"`
	SourceRevision   string `json:"sourceRevision"`
	SourceSHA256     string `json:"sourceSha256"`
}

type portfolioProject struct {
	ID                      string                  `json:"id"`
	Revision                string                  `json:"revision"`
	PortfolioClass          string                  `json:"portfolioClass"`
	PortfolioClassification portfolioClassification `json:"portfolioClassification"`
}

type portfolioProjectsResponse struct {
	Projects []portfolioProject `json:"projects"`
	ServedAt string             `json:"servedAt"`
}

// PortfolioResolver resolves only administrator-configured Multica UUIDs to
// exact Brain project IDs. Brain supplies a projection of Knowledge provenance;
// this adapter never infers identity from project names or repositories.
type PortfolioResolver struct {
	Client *Client
}

func NewPortfolioResolver(client *Client) *PortfolioResolver {
	return &PortfolioResolver{Client: client}
}

func (r *PortfolioResolver) ResolveCompanyProjects(ctx context.Context, workspaceID string, localProjectIDs []string) (map[string]service.CompanyPortfolioBinding, error) {
	bindings := make(map[string]service.CompanyPortfolioBinding)
	if r == nil || r.Client == nil {
		return bindings, nil
	}
	canonicalToLocal := make(map[string][]string)
	for _, localID := range localProjectIDs {
		canonicalID, ok := r.Client.Project(workspaceID, localID)
		if !ok {
			continue
		}
		canonicalID = strings.TrimSpace(canonicalID)
		canonicalToLocal[canonicalID] = append(canonicalToLocal[canonicalID], localID)
	}
	if len(canonicalToLocal) == 0 {
		return bindings, nil
	}
	raw, err := r.Client.request(ctx, http.MethodGet, "projects", nil)
	if err != nil {
		return nil, err
	}
	var response portfolioProjectsResponse
	if json.Unmarshal(raw, &response) != nil || strings.TrimSpace(response.ServedAt) == "" {
		return nil, errors.New("invalid brain portfolio projection")
	}
	seen := make(map[string]struct{}, len(response.Projects))
	for _, project := range response.Projects {
		if _, duplicate := seen[project.ID]; duplicate {
			return nil, errors.New("duplicate brain project identity")
		}
		seen[project.ID] = struct{}{}
		localIDs, wanted := canonicalToLocal[project.ID]
		if !wanted {
			continue
		}
		binding, ok := verifiedPortfolioBinding(project, response.ServedAt)
		if !ok {
			continue
		}
		for _, localID := range localIDs {
			bindings[localID] = binding
		}
	}
	return bindings, nil
}

func verifiedPortfolioBinding(project portfolioProject, projectedAt string) (service.CompanyPortfolioBinding, bool) {
	classification := strings.TrimSpace(project.PortfolioClass)
	membership := "unknown"
	switch classification {
	case "business_application", "company_infrastructure":
		membership = "company"
	case "personal":
		membership = "personal"
	case "experimental", "archived", "unknown":
	default:
		return service.CompanyPortfolioBinding{}, false
	}
	p := project.PortfolioClassification
	if strings.TrimSpace(project.ID) == "" || strings.TrimSpace(project.Revision) == "" ||
		p.SourceRepository != canonicalKnowledgeRepository || strings.TrimSpace(p.SourceReference) == "" ||
		strings.TrimSpace(p.SourceRevision) == "" || strings.TrimSpace(p.ObservedAt) == "" ||
		strings.TrimSpace(p.EvidenceStage) == "" {
		return service.CompanyPortfolioBinding{}, false
	}
	return service.CompanyPortfolioBinding{
		CanonicalProjectID:      project.ID,
		CompanyMembership:       membership,
		PortfolioClassification: classification,
		VerificationState:       "verified",
		Provenance: &service.CompanyPortfolioProvenance{
			SourceSystem:       "silver-mediums-knowledge",
			SourceRepository:   p.SourceRepository,
			SourceReference:    p.SourceReference,
			SourceRevision:     p.SourceRevision,
			SourceSHA256:       p.SourceSHA256,
			SourceObservedAt:   p.ObservedAt,
			EvidenceStage:      p.EvidenceStage,
			ProgramID:          p.ProgramID,
			ProjectionSystem:   "silver-mediums-brain",
			ProjectionRevision: project.Revision,
			ProjectedAt:        projectedAt,
		},
	}, true
}
