package agentqualification

import (
	"strings"
	"time"
)

// APIBudget is intentionally separate from subscription percentage capacity.
// API spend and ChatGPT/Codex subscription limits are different billing lanes
// and must never be pooled or converted into one another.
type APIBudget struct {
	Provider                string `json:"provider"`
	ProjectLane             string `json:"project_lane"`
	Currency                string `json:"currency"`
	WindowID                string `json:"window_id"`
	MaxSpendMicros          int64  `json:"max_spend_micros"`
	PerRequestReserveMicros int64  `json:"per_request_reserve_micros"`
	MaxSnapshotAgeSeconds   int    `json:"max_snapshot_age_seconds"`
}

type APISpendSnapshot struct {
	Provider    string `json:"provider"`
	ProjectLane string `json:"project_lane"`
	Currency    string `json:"currency"`
	WindowID    string `json:"window_id"`
	SpentMicros int64  `json:"spent_micros"`
	ObservedAt  string `json:"observed_at"`
	Source      string `json:"source"`
}

type APIBudgetAdmission struct {
	CanLaunch bool     `json:"can_launch"`
	Reasons   []string `json:"reasons,omitempty"`
}

func EvaluateAPIBudgetAdmission(budget APIBudget, snapshot APISpendSnapshot, evaluatedAt string) APIBudgetAdmission {
	result := APIBudgetAdmission{}
	add := func(reason string) { result.Reasons = append(result.Reasons, reason) }
	if strings.TrimSpace(budget.Provider) == "" || strings.TrimSpace(budget.ProjectLane) == "" || budget.Currency != "USD" || strings.TrimSpace(budget.WindowID) == "" || budget.MaxSpendMicros <= 0 || budget.PerRequestReserveMicros <= 0 || budget.MaxSnapshotAgeSeconds <= 0 {
		add("api_budget_invalid")
	}
	if snapshot.Provider != budget.Provider || snapshot.ProjectLane != budget.ProjectLane || snapshot.Currency != budget.Currency || snapshot.WindowID != budget.WindowID || snapshot.Source != "authoritative_api_usage" {
		add("api_snapshot_invalid")
	}
	if snapshot.SpentMicros < 0 {
		add("api_spend_invalid")
	}
	if snapshot.SpentMicros+budget.PerRequestReserveMicros > budget.MaxSpendMicros {
		add("api_spend_ceiling_reached")
	}
	evaluated, evaluationErr := time.Parse(time.RFC3339Nano, evaluatedAt)
	observed, observedErr := time.Parse(time.RFC3339Nano, snapshot.ObservedAt)
	if evaluationErr != nil || observedErr != nil || observed.After(evaluated) || evaluated.Sub(observed) > time.Duration(budget.MaxSnapshotAgeSeconds)*time.Second {
		add("api_snapshot_stale")
	}
	result.Reasons = unique(result.Reasons)
	result.CanLaunch = len(result.Reasons) == 0
	return result
}
