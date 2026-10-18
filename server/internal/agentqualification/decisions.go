package agentqualification

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/multica-ai/multica/server/internal/systemgraph"
)

const (
	StatusQualifiedBounded  = "qualified_bounded"
	StatusProvisionalShadow = "provisional_shadow_only"
	StatusHeldPending       = "held_pending_qualification"
	StatusHeldExternal      = "held_external_qualification"
)

type ConfigurationDecision struct {
	Configuration           Configuration `json:"configuration"`
	Status                  string        `json:"status"`
	QualificationScope      string        `json:"qualification_scope"`
	EvidenceIssues          []string      `json:"evidence_issues"`
	AutomaticHoldConditions []string      `json:"automatic_hold_conditions"`
}

type DecisionCatalog struct {
	Version   string                  `json:"version"`
	Decisions []ConfigurationDecision `json:"decisions"`
}

//go:embed accepted_configurations.json
var decisionsJSON []byte

var (
	decisionsOnce sync.Once
	decisionSet   DecisionCatalog
	decisionsErr  error
)

func AcceptedConfigurations() (DecisionCatalog, error) {
	decisionsOnce.Do(func() {
		decoder := json.NewDecoder(bytes.NewReader(decisionsJSON))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decisionSet); err != nil {
			decisionsErr = fmt.Errorf("decode accepted configurations: %w", err)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			decisionsErr = fmt.Errorf("decode accepted configurations: trailing JSON")
			return
		}
		decisionsErr = decisionSet.Validate()
	})
	return decisionSet, decisionsErr
}

func (c DecisionCatalog) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("configuration decision version is required")
	}
	seen := make(map[string]struct{}, len(c.Decisions))
	for _, decision := range c.Decisions {
		if err := ValidateConfiguration(decision.Configuration); err != nil {
			return fmt.Errorf("configuration %s: %w", decision.Configuration.ID, err)
		}
		if _, exists := seen[decision.Configuration.ID]; exists {
			return fmt.Errorf("duplicate configuration decision %s", decision.Configuration.ID)
		}
		seen[decision.Configuration.ID] = struct{}{}
		if decision.QualificationScope == "" || len(decision.AutomaticHoldConditions) == 0 {
			return fmt.Errorf("configuration %s lacks qualification scope or hold conditions", decision.Configuration.ID)
		}
		switch decision.Status {
		case StatusQualifiedBounded, StatusProvisionalShadow:
			if len(decision.EvidenceIssues) == 0 {
				return fmt.Errorf("configuration %s lacks qualification evidence", decision.Configuration.ID)
			}
		case StatusHeldPending:
			// A selected role/configuration pair may be recorded before it has
			// competence evidence. It remains ineligible for execution.
		case StatusHeldExternal:
			if decision.Configuration.RuntimeProvider != "mac_studio_separate_subscription" {
				return fmt.Errorf("external qualification %s must remain on Mac Studio", decision.Configuration.ID)
			}
		default:
			return fmt.Errorf("configuration %s has unknown decision status %s", decision.Configuration.ID, decision.Status)
		}
	}
	return nil
}

func (c DecisionCatalog) Decision(configurationID string) (ConfigurationDecision, bool) {
	for _, decision := range c.Decisions {
		if decision.Configuration.ID == configurationID {
			return decision, true
		}
	}
	return ConfigurationDecision{}, false
}

func (c DecisionCatalog) StatusFor(configurationID string) string {
	decision, ok := c.Decision(configurationID)
	if !ok {
		return "missing"
	}
	return decision.Status
}

func (c DecisionCatalog) BuildExecutionPlan(roles systemgraph.RoleCatalog, route systemgraph.RoutePlan) systemgraph.ExecutionPlan {
	return roles.BuildExecutionPlan(route, c.StatusFor)
}

// AllowsExecution is deliberately narrower than selection: Item 14 proves
// bounded local configurations but grants no production execution authority.
func (d ConfigurationDecision) AllowsExecution(production bool) bool {
	if production {
		return false
	}
	return d.Status == StatusQualifiedBounded
}
