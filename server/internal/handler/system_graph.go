package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/internal/systemgraph"
)

type systemGraphQueryRequest struct {
	Query       string                   `json:"query"`
	JourneyID   string                   `json:"journey_id,omitempty"`
	SeedNodeIDs []string                 `json:"seed_node_ids,omitempty"`
	SeedEdgeIDs []string                 `json:"seed_edge_ids,omitempty"`
	TargetID    string                   `json:"target_id,omitempty"`
	Route       systemgraph.RouteRequest `json:"route,omitempty"`
}

// GetSystemGraph returns the canonical, secret-free Silver Mediums graph. The
// route is owner-only because even redacted topology is company-sensitive.
func (h *Handler) GetSystemGraph(w http.ResponseWriter, r *http.Request) {
	catalog, err := systemgraph.Default()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "system graph is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

// GetRoleArchitecture returns role functions and squad templates without model
// assignments. Model choice remains an evidence-backed qualification result,
// not an architectural default hidden in an agent prompt.
func (h *Handler) GetRoleArchitecture(w http.ResponseWriter, r *http.Request) {
	catalog, err := systemgraph.DefaultRoles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "role architecture is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

// GetAgentConfigurations returns the explicit, evidence-scoped configuration
// decisions. A selected model is never equivalent to production authority.
func (h *Handler) GetAgentConfigurations(w http.ResponseWriter, r *http.Request) {
	catalog, err := agentqualification.AcceptedConfigurations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent configurations are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

// QuerySystemGraph exposes bounded graph queries to owner-facing tools. It
// never accepts arbitrary graph traversal code and never returns credentials.
func (h *Handler) QuerySystemGraph(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var request systemGraphQueryRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid system graph query")
		return
	}
	catalog, err := systemgraph.Default()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "system graph is unavailable")
		return
	}
	switch request.Query {
	case "journey_subgraph":
		result, err := catalog.JourneySubgraph(request.JourneyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "blast_radius":
		result, err := catalog.BlastRadiusFrom(request.SeedNodeIDs, request.SeedEdgeIDs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "stale_evidence":
		writeJSON(w, http.StatusOK, catalog.StaleEvidence(time.Now().UTC()))
	case "authority_for":
		result, err := catalog.AuthorityFor(request.TargetID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "release_requirements":
		result, err := catalog.RequirementsForRelease(request.JourneyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "conflicting_claims":
		writeJSON(w, http.StatusOK, catalog.ConflictingClaims())
	case "related_outcomes":
		result, err := catalog.RelatedOutcomes(request.SeedNodeIDs, request.SeedEdgeIDs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "issue_route":
		result, err := catalog.RouteIssue(request.Route)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, http.StatusBadRequest, "unsupported system graph query")
	}
}
