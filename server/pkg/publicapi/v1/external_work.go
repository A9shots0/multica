package publicapiv1

import (
	"fmt"
	"strings"
	"time"
)

const OutcomeReceiptContractVersion = "0.2"

const (
	maxExternalIdentifierLength = 512
	maxExternalTextLength       = 16 * 1024
	maxExternalListItems        = 256
)

type OutcomeReceiptV02 struct {
	WorkID                    string                      `json:"work_id"`
	RunID                     string                      `json:"run_id"`
	ReceiptRevision           int64                       `json:"receipt_revision"`
	SupersedesReceiptRevision *int64                      `json:"supersedes_receipt_revision,omitempty"`
	Provider                  string                      `json:"provider"`
	Objective                 string                      `json:"objective"`
	Scope                     string                      `json:"scope"`
	RequestedMode             string                      `json:"requested_mode"`
	Authority                 OutcomeReceiptAuthority     `json:"authority"`
	State                     string                      `json:"state"`
	CurrentMilestone          string                      `json:"current_milestone"`
	ChangeSinceLastReceipt    string                      `json:"change_since_last_receipt"`
	Evidence                  []OutcomeReceiptEvidence    `json:"evidence"`
	Dependencies              []string                    `json:"dependencies"`
	NextAction                string                      `json:"next_action"`
	OwnerDecision             OutcomeReceiptOwnerDecision `json:"owner_decision"`
	Risk                      string                      `json:"risk"`
	ResourceUsage             OutcomeReceiptResourceUsage `json:"resource_usage"`
	Effect                    OutcomeReceiptEffect        `json:"effect"`
	Freshness                 OutcomeReceiptFreshness     `json:"freshness"`
	Acceptance                OutcomeReceiptAcceptance    `json:"acceptance"`
}

type OutcomeReceiptAuthority struct {
	Reference      string   `json:"reference"`
	AllowedEffects []string `json:"allowed_effects"`
}

type OutcomeReceiptEvidence struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
}

type OutcomeReceiptOwnerDecision struct {
	Required      bool     `json:"required"`
	Category      *string  `json:"category"`
	Question      *string  `json:"question"`
	Options       []string `json:"options"`
	WhyNow        *string  `json:"why_now"`
	InvalidatedBy []string `json:"invalidated_by"`
}

type OutcomeReceiptResourceUsage struct {
	Source               string   `json:"source"`
	Attempts             *int64   `json:"attempts"`
	WallClockSeconds     *int64   `json:"wall_clock_seconds"`
	ActiveComputeSeconds *int64   `json:"active_compute_seconds"`
	InputTokens          *int64   `json:"input_tokens"`
	OutputTokens         *int64   `json:"output_tokens"`
	CostUSD              *float64 `json:"cost_usd"`
	CapacityWindowRef    *string  `json:"capacity_window_ref"`
}

type OutcomeReceiptEffect struct {
	UserOutcome       string `json:"user_outcome"`
	ProductionChanged bool   `json:"production_changed"`
}

type OutcomeReceiptFreshness struct {
	ObservedAt string  `json:"observed_at"`
	ExpiresAt  *string `json:"expires_at"`
}

type OutcomeReceiptAcceptance struct {
	Stage             string   `json:"stage"`
	Status            string   `json:"status"`
	CriteriaRemaining []string `json:"criteria_remaining"`
}

type ExternalReceiptAdmissionResponse struct {
	ReceiptID              string `json:"receipt_id"`
	EvidenceID             string `json:"evidence_id"`
	ExecutionID            string `json:"execution_id"`
	ExecutionRevision      int64  `json:"execution_revision"`
	CurrentPictureRevision int64  `json:"current_picture_revision"`
	AuthorityClaimState    string `json:"authority_claim_state"`
	DecisionProposalState  string `json:"decision_proposal_state"`
	Replay                 bool   `json:"replay"`
}

func (r OutcomeReceiptV02) Validate() error {
	for name, value := range map[string]string{
		"work_id": r.WorkID, "run_id": r.RunID, "objective": r.Objective,
		"scope": r.Scope, "authority.reference": r.Authority.Reference,
		"current_milestone":         r.CurrentMilestone,
		"change_since_last_receipt": r.ChangeSinceLastReceipt,
		"next_action":               r.NextAction,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		limit := maxExternalTextLength
		if name == "work_id" || name == "run_id" || name == "authority.reference" {
			limit = maxExternalIdentifierLength
		}
		if len(value) > limit {
			return fmt.Errorf("%s exceeds %d bytes", name, limit)
		}
	}
	if len(r.Authority.AllowedEffects) > maxExternalListItems || len(r.Evidence) > maxExternalListItems ||
		len(r.Dependencies) > maxExternalListItems || len(r.Acceptance.CriteriaRemaining) > maxExternalListItems ||
		len(r.OwnerDecision.Options) > maxExternalListItems || len(r.OwnerDecision.InvalidatedBy) > maxExternalListItems {
		return fmt.Errorf("receipt list exceeds %d items", maxExternalListItems)
	}
	if len(r.Evidence) == 0 {
		return fmt.Errorf("at least one evidence reference is required")
	}
	if r.ReceiptRevision < 1 {
		return fmt.Errorf("receipt_revision must be positive")
	}
	if r.ReceiptRevision == 1 && r.SupersedesReceiptRevision != nil {
		return fmt.Errorf("receipt revision 1 cannot supersede another revision")
	}
	if r.ReceiptRevision > 1 && (r.SupersedesReceiptRevision == nil || *r.SupersedesReceiptRevision != r.ReceiptRevision-1) {
		return fmt.Errorf("receipt revision %d must supersede revision %d", r.ReceiptRevision, r.ReceiptRevision-1)
	}
	if !oneOf(r.Provider, "codex", "claude", "chatgpt", "copilot", "evo", "other") {
		return fmt.Errorf("unsupported provider")
	}
	if !oneOf(r.RequestedMode, "explain", "review", "diagnose", "change", "deploy", "monitor") {
		return fmt.Errorf("unsupported requested_mode")
	}
	if (r.RequestedMode == "change" || r.RequestedMode == "deploy" || r.Effect.ProductionChanged) && len(r.Authority.AllowedEffects) == 0 {
		return fmt.Errorf("effectful receipts require explicit authority.allowed_effects")
	}
	if !oneOf(r.State, "working", "waiting_external", "waiting_owner", "blocked", "complete", "stopped") {
		return fmt.Errorf("unsupported state")
	}
	if !oneOf(r.Risk, "low", "medium", "high") {
		return fmt.Errorf("unsupported risk")
	}
	if !oneOf(r.ResourceUsage.Source, "provider_reported", "measured", "unavailable") {
		return fmt.Errorf("unsupported resource_usage.source")
	}
	if anyNegativeInt(r.ResourceUsage.Attempts, r.ResourceUsage.WallClockSeconds, r.ResourceUsage.ActiveComputeSeconds, r.ResourceUsage.InputTokens, r.ResourceUsage.OutputTokens) || r.ResourceUsage.CostUSD != nil && *r.ResourceUsage.CostUSD < 0 {
		return fmt.Errorf("resource usage values must not be negative")
	}
	if r.ResourceUsage.WallClockSeconds != nil && r.ResourceUsage.ActiveComputeSeconds != nil && *r.ResourceUsage.ActiveComputeSeconds > *r.ResourceUsage.WallClockSeconds {
		return fmt.Errorf("active_compute_seconds cannot exceed wall_clock_seconds")
	}
	if !oneOf(r.Effect.UserOutcome, "unavailable", "degraded", "temporarily_restored", "restored", "unchanged") {
		return fmt.Errorf("unsupported effect.user_outcome")
	}
	if !oneOf(r.Acceptance.Stage, "local", "source_ready", "merged", "release_ready", "deployed", "production_verified") ||
		!oneOf(r.Acceptance.Status, "not_tested", "partial", "accepted", "rejected") {
		return fmt.Errorf("unsupported acceptance state")
	}
	if r.State == "complete" && r.Acceptance.Status != "accepted" {
		return fmt.Errorf("complete receipts require accepted acceptance status")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Freshness.ObservedAt); err != nil {
		return fmt.Errorf("freshness.observed_at must be RFC3339: %w", err)
	}
	if r.Freshness.ExpiresAt != nil && strings.TrimSpace(*r.Freshness.ExpiresAt) == "" {
		return fmt.Errorf("freshness.expires_at must be null, an RFC3339 time, or a nonblank invalidation condition")
	}
	if r.State == "waiting_owner" && !r.OwnerDecision.Required {
		return fmt.Errorf("waiting_owner requires an owner decision proposal")
	}
	if r.OwnerDecision.Required {
		if r.OwnerDecision.Category == nil || !oneOf(*r.OwnerDecision.Category, "business", "ui", "budget", "policy_exception") ||
			r.OwnerDecision.Question == nil || strings.TrimSpace(*r.OwnerDecision.Question) == "" ||
			r.OwnerDecision.WhyNow == nil || strings.TrimSpace(*r.OwnerDecision.WhyNow) == "" || len(r.OwnerDecision.Options) < 2 {
			return fmt.Errorf("owner decision proposals require category, question, why_now, and at least two options")
		}
	}
	for i, option := range r.OwnerDecision.Options {
		if strings.TrimSpace(option) == "" || len(option) > maxExternalTextLength {
			return fmt.Errorf("owner_decision.options[%d] is invalid", i)
		}
	}
	for i, invalidator := range r.OwnerDecision.InvalidatedBy {
		if strings.TrimSpace(invalidator) == "" || len(invalidator) > maxExternalTextLength {
			return fmt.Errorf("owner_decision.invalidated_by[%d] is invalid", i)
		}
	}
	for i, value := range r.Authority.AllowedEffects {
		if strings.TrimSpace(value) == "" || len(value) > maxExternalTextLength {
			return fmt.Errorf("authority.allowed_effects[%d] is blank", i)
		}
	}
	for i, item := range r.Evidence {
		if !oneOf(item.Type, "commit", "pr", "test", "deployment", "runtime", "artifact", "owner_ui", "owner_report", "controller") || strings.TrimSpace(item.Reference) == "" || len(item.Reference) > maxExternalTextLength {
			return fmt.Errorf("evidence[%d] is invalid", i)
		}
	}
	for i, dependency := range r.Dependencies {
		if strings.TrimSpace(dependency) == "" || len(dependency) > maxExternalTextLength {
			return fmt.Errorf("dependencies[%d] is invalid", i)
		}
	}
	for i, criterion := range r.Acceptance.CriteriaRemaining {
		if strings.TrimSpace(criterion) == "" || len(criterion) > maxExternalTextLength {
			return fmt.Errorf("acceptance.criteria_remaining[%d] is invalid", i)
		}
	}
	if r.ResourceUsage.CapacityWindowRef != nil && len(*r.ResourceUsage.CapacityWindowRef) > maxExternalTextLength {
		return fmt.Errorf("resource_usage.capacity_window_ref is too large")
	}
	return nil
}

type ContextRequestV02 struct {
	RequestID             string                  `json:"request_id"`
	WorkID                string                  `json:"work_id"`
	RequestType           string                  `json:"request_type"`
	Need                  string                  `json:"need"`
	Reason                string                  `json:"reason"`
	Scope                 string                  `json:"scope"`
	FreshnessRequired     string                  `json:"freshness_required"`
	MaximumPayload        string                  `json:"maximum_payload"`
	ResponseContract      ContextResponseContract `json:"response_contract"`
	AuthorizationBoundary string                  `json:"authorization_boundary"`
	CachePolicy           string                  `json:"cache_policy"`
}

type ContextResponseContract struct {
	RequiredFields []string `json:"required_fields"`
	EvidenceLevel  string   `json:"evidence_level"`
}

func (r ContextRequestV02) Validate() error {
	if blank(r.RequestID, r.WorkID, r.Need, r.Reason, r.Scope, r.FreshnessRequired, r.CachePolicy) || len(r.ResponseContract.RequiredFields) == 0 {
		return fmt.Errorf("context request is missing required fields")
	}
	for name, value := range map[string]string{
		"request_id": r.RequestID, "work_id": r.WorkID, "need": r.Need,
		"reason": r.Reason, "scope": r.Scope, "freshness_required": r.FreshnessRequired,
		"cache_policy": r.CachePolicy,
	} {
		limit := maxExternalTextLength
		if name == "request_id" || name == "work_id" {
			limit = maxExternalIdentifierLength
		}
		if len(value) > limit {
			return fmt.Errorf("%s exceeds %d bytes", name, limit)
		}
	}
	if len(r.ResponseContract.RequiredFields) > 32 {
		return fmt.Errorf("response_contract.required_fields exceeds 32 items")
	}
	seen := make(map[string]bool, len(r.ResponseContract.RequiredFields))
	for i, field := range r.ResponseContract.RequiredFields {
		field = strings.TrimSpace(field)
		if field == "" || len(field) > 128 || seen[field] {
			return fmt.Errorf("response_contract.required_fields[%d] is invalid", i)
		}
		seen[field] = true
	}
	if !oneOf(r.RequestType, "decision", "source", "runtime", "artifact", "history", "policy", "dependency") ||
		!oneOf(r.MaximumPayload, "compact", "document", "exact_artifact") ||
		!oneOf(r.ResponseContract.EvidenceLevel, "observed", "derived", "authoritative") ||
		!oneOf(r.AuthorizationBoundary, "read_only", "bounded_write", "exact_action") {
		return fmt.Errorf("context request contains an unsupported enum value")
	}
	return nil
}

type ContextResponseV02 struct {
	RequestID     string                `json:"request_id"`
	WorkID        string                `json:"work_id"`
	Status        string                `json:"status"`
	Source        ContextResponseSource `json:"source"`
	EvidenceLevel string                `json:"evidence_level"`
	ObservedAt    string                `json:"observed_at"`
	ExpiresAt     *string               `json:"expires_at"`
	Payload       map[string]any        `json:"payload"`
	Limitations   []string              `json:"limitations"`
}

type ContextResponseSource struct {
	System    string  `json:"system"`
	Reference *string `json:"reference"`
}

func (r ContextResponseV02) Validate() error {
	if blank(r.RequestID, r.WorkID) || r.Payload == nil || r.Limitations == nil {
		return fmt.Errorf("context response is missing required fields")
	}
	if !oneOf(r.Status, "found", "unknown", "stale", "unauthorized", "too_large", "error") ||
		!oneOf(r.Source.System, "multica", "brain", "repository", "runtime", "artifact", "dependency", "provider") ||
		!oneOf(r.EvidenceLevel, "observed", "derived", "authoritative") {
		return fmt.Errorf("context response contains an unsupported enum value")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.ObservedAt); err != nil {
		return fmt.Errorf("observed_at must be RFC3339: %w", err)
	}
	if r.Status == "found" && (r.Source.Reference == nil || strings.TrimSpace(*r.Source.Reference) == "") {
		return fmt.Errorf("found responses require a source reference")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func blank(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func anyNegativeInt(values ...*int64) bool {
	for _, value := range values {
		if value != nil && *value < 0 {
			return true
		}
	}
	return false
}
