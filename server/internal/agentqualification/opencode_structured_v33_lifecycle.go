package agentqualification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
)

var ErrOpenCodeStructuredV33CandidateResourceExhaustion = errors.New("v33 candidate resource exhaustion")

const (
	OpenCodeStructuredV33ExecutableSHA256 = "b6efa9eb3ee1d5c25438f3cba03c471e5d8661a6121abc47f209db3b3f05f415"
	OpenCodeStructuredV33OpenAPISHA256    = "c3a9f94af0c3324d97b482b14c692e810ce7ccac3136319ba46334de972b4cf1"
)

type OpenCodeStructuredV33SessionReceipt struct {
	ID, Slug, ProjectID, Directory, Title, Version string
	Created, Updated                               int64
	HTTPStatus                                     int
	ContentType, RawSHA256                         string
	Raw                                            json.RawMessage
}

func ParseOpenCodeStructuredV33CreatedSession(status int, contentType string, raw []byte) (OpenCodeStructuredV33SessionReceipt, error) {
	var z OpenCodeStructuredV33SessionReceipt
	if status != http.StatusOK || !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		return z, fmt.Errorf("v33 invalid create-session HTTP envelope")
	}
	var s struct {
		ID          string          `json:"id"`
		Slug        string          `json:"slug"`
		ProjectID   string          `json:"projectID"`
		WorkspaceID string          `json:"workspaceID,omitempty"`
		Directory   string          `json:"directory"`
		Path        json.RawMessage `json:"path,omitempty"`
		ParentID    string          `json:"parentID,omitempty"`
		Summary     json.RawMessage `json:"summary,omitempty"`
		Cost        json.RawMessage `json:"cost,omitempty"`
		Tokens      json.RawMessage `json:"tokens,omitempty"`
		Share       json.RawMessage `json:"share,omitempty"`
		Title       string          `json:"title"`
		Agent       json.RawMessage `json:"agent,omitempty"`
		Model       json.RawMessage `json:"model,omitempty"`
		Version     string          `json:"version"`
		Metadata    json.RawMessage `json:"metadata,omitempty"`
		Permission  json.RawMessage `json:"permission,omitempty"`
		Revert      json.RawMessage `json:"revert,omitempty"`
		Time        struct {
			Created int64 `json:"created"`
			Updated int64 `json:"updated"`
		} `json:"time"`
	}
	if err := decodeOpenCodeStructuredV33Exact(raw, &s); err != nil {
		return z, err
	}
	if !strings.HasPrefix(s.ID, "ses_") || s.Slug == "" || s.ProjectID == "" || s.Directory == "" || s.ParentID != "" || s.Title == "" || s.Version != OpenCodeStructuredV33Version || s.Time.Created <= 0 || s.Time.Updated < s.Time.Created {
		return z, fmt.Errorf("v33 create-session body incomplete, parented, or wrong version")
	}
	return OpenCodeStructuredV33SessionReceipt{ID: s.ID, Slug: s.Slug, ProjectID: s.ProjectID, Directory: s.Directory, Title: s.Title, Version: s.Version, Created: s.Time.Created, Updated: s.Time.Updated, HTTPStatus: status, ContentType: "application/json", RawSHA256: hashRealisticNextBytes(raw), Raw: append(json.RawMessage(nil), raw...)}, nil
}

type OpenCodeStructuredV33Preflight struct {
	Version, ExecutableSHA256, OpenAPISHA256, InlineConfigSHA256, ResolvedConfigSHA256, BaseURL string
	Ready, ShutdownObserved                                                                     bool
}

func ValidateOpenCodeStructuredV33Preflight(p OpenCodeStructuredV33Preflight) error {
	if err := ValidateOpenCodeStructuredV33Ready(p); err != nil {
		return err
	}
	if !p.ShutdownObserved {
		return fmt.Errorf("v33 shutdown evidence missing")
	}
	return nil
}

func ValidateOpenCodeStructuredV33Ready(p OpenCodeStructuredV33Preflight) error {
	if p.Version != OpenCodeStructuredV33Version || strings.ToLower(p.ExecutableSHA256) != OpenCodeStructuredV33ExecutableSHA256 || strings.ToLower(p.OpenAPISHA256) != OpenCodeStructuredV33OpenAPISHA256 {
		return fmt.Errorf("v33 executable/OpenAPI identity mismatch")
	}
	if p.InlineConfigSHA256 == "" || p.ResolvedConfigSHA256 == "" || !p.Ready {
		return fmt.Errorf("v33 preflight/config incomplete")
	}
	if u := strings.TrimPrefix(p.BaseURL, "http://"); u == p.BaseURL || (!strings.HasPrefix(u, "127.0.0.1:") && !strings.HasPrefix(u, "localhost:") && !strings.HasPrefix(u, "[::1]:")) {
		return fmt.Errorf("v33 server not loopback")
	}
	return nil
}

type OpenCodeStructuredV33ServerLifecycle interface {
	Start(context.Context, []string, []string) error
	Ready(context.Context) (version, executableSHA, openAPISHA, resolvedConfigSHA, baseURL string, err error)
	Shutdown(context.Context) error
}

func RunOpenCodeStructuredV33ProviderFreePreflight(ctx context.Context, c OpenCodeStructuredV33Capsule, s OpenCodeStructuredV33ServerLifecycle) (p OpenCodeStructuredV33Preflight, err error) {
	if err = s.Start(ctx, c.ServerArgs, c.EnvironmentTemplate); err != nil {
		return p, err
	}
	defer func() {
		shutdownErr := s.Shutdown(ctx)
		p.ShutdownObserved = shutdownErr == nil
		if err == nil && shutdownErr != nil {
			err = shutdownErr
		}
	}()
	p.Version, p.ExecutableSHA256, p.OpenAPISHA256, p.ResolvedConfigSHA256, p.BaseURL, err = s.Ready(ctx)
	if err != nil {
		return p, err
	}
	p.Ready = true
	p.InlineConfigSHA256 = c.InlineConfigSHA256
	if p.ResolvedConfigSHA256 != c.ResolvedConfigSHA256 {
		return p, fmt.Errorf("v33 resolved config differs from capsule")
	}
	return p, nil
}

type OpenCodeStructuredV33Aggregate struct {
	Tokens, CostMicros int64
	ProviderTurns      int
}

// PrepareOpenCodeStructuredV33Step shrinks the server-side output allowance
// before every provider step. conservativeNextInput is a separately evidenced
// upper bound for the exact next request.
func PrepareOpenCodeStructuredV33Step(d ProtocolAblationV32Definition, c OpenCodeStructuredV33Capsule, used ProtocolAblationV32Usage, conservativeNextInput int64) (OpenCodeStructuredV33Capsule, error) {
	usedTotal, ok := safeAddV33(used.InputTokens, used.OutputTokens, used.ReasoningTokens)
	if !ok || conservativeNextInput <= 0 || usedTotal > d.PerObservationTotalTokenReservation {
		return c, fmt.Errorf("v33 invalid step accounting/input bound")
	}
	usedGenerated, ok := safeAddV33(used.OutputTokens, used.ReasoningTokens)
	if !ok || usedGenerated > d.PerObservationGeneratedTokenCeiling {
		return c, fmt.Errorf("v33 generated accounting invalid")
	}
	remainingTotal := d.PerObservationTotalTokenReservation - usedTotal
	remainingGenerated := d.PerObservationGeneratedTokenCeiling - usedGenerated
	if conservativeNextInput > remainingTotal {
		return c, ErrOpenCodeStructuredV33CandidateResourceExhaustion
	}
	ceiling := remainingTotal - conservativeNextInput
	if ceiling > remainingGenerated {
		ceiling = remainingGenerated
	}
	if ceiling <= 0 {
		return c, ErrOpenCodeStructuredV33CandidateResourceExhaustion
	}
	prefix, found := "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=", false
	for i, entry := range c.EnvironmentTemplate {
		if strings.HasPrefix(entry, prefix) {
			c.EnvironmentTemplate[i] = fmt.Sprintf("%s%d", prefix, ceiling)
			found = true
		}
	}
	if !found {
		return c, fmt.Errorf("v33 output limit environment missing")
	}
	c.GeneratedTokenCeiling = ceiling
	return c, nil
}

// CompleteOpenCodeStructuredV33ResourceExhaustion records a candidate-local
// exhaustion after at least one fully receipted provider turn. It deliberately
// reuses v32's strict identity, receipt, usage, claim, and evidence validation;
// only the final scientific classification is v33-specific.
func CompleteOpenCodeStructuredV33ResourceExhaustion(d ProtocolAblationV32Definition, ledger *ProtocolAblationV32Ledger, binding ProtocolAblationV32Binding, observation ProtocolAblationV32Observation) error {
	if observation.Outcome != "failed" || observation.FailureClass != "candidate_resource_exhaustion" || observation.FailureScope != "observation" {
		return fmt.Errorf("v33 resource exhaustion classification invalid")
	}
	normalized := observation
	normalized.FailureClass = "response_or_tool_protocol_failure"
	normalized.FailureScope = ""
	if err := CompleteProtocolAblationV32Observation(d, ledger, binding, normalized); err != nil {
		return err
	}
	claim := ledger.Claims[binding.BindingID]
	copy := observation
	claim.Completed = &copy
	ledger.Claims[binding.BindingID] = claim
	return nil
}

// CompleteOpenCodeStructuredV33TransportTimeout closes one already-claimed
// observation as invalid/unscored when OpenCode durably exported an aborted
// assistant envelope before recording any provider usage. It is deliberately
// observation-local: the source campaign halt remains immutable evidence, while
// a new continuation may proceed without treating zero recorded usage as model
// competence evidence.
func CompleteOpenCodeStructuredV33TransportTimeout(d ProtocolAblationV32Definition, ledger *ProtocolAblationV32Ledger, binding ProtocolAblationV32Binding, observation ProtocolAblationV32Observation) error {
	if !protocolAblationV32BindingMember(d, binding) {
		return fmt.Errorf("v33 timeout binding is not an exact matrix member")
	}
	claim, ok := ledger.Claims[binding.BindingID]
	if !ok || claim.Status != "pending" || claim.Completed != nil {
		return fmt.Errorf("v33 timeout completion lacks exclusive pending claim")
	}
	if observation.BindingID != binding.BindingID || observation.Outcome != "invalid" || observation.FailureClass != "transport_timeout" || observation.FailureScope != "observation" || observation.Usage != (ProtocolAblationV32Usage{}) || len(observation.Actions) != 0 || observation.GraderResult.CaseID != d.Case.CaseID || observation.GraderResult.Verdict != "" || len(observation.RawReceipts) != 1 {
		return fmt.Errorf("v33 timeout observation classification/accounting invalid")
	}
	identity := observation.ProviderIdentity
	if identity.DirectProvider != "openrouter" || identity.ReportedModel != strings.TrimPrefix(binding.Candidate.Model, "openrouter/") || identity.ReportedEffort != binding.Candidate.Effort || !strings.EqualFold(identity.ReportedServingProvider, binding.Candidate.ServingProvider) || len(identity.ResolvedConfigSHA256) != 64 || identity.ServingProviderEvidence != openCodeV33RoutePolicy {
		return fmt.Errorf("v33 timeout provider identity invalid")
	}
	if !json.Valid(observation.RawReceipts[0]) || observation.EvidenceSHA256 != ProtocolAblationV32ObservationEvidenceSHA(observation) {
		return fmt.Errorf("v33 timeout evidence invalid")
	}
	copy := observation
	claim.Status, claim.Completed = "completed", &copy
	ledger.Claims[binding.BindingID] = claim
	return nil
}

func AdmitOpenCodeStructuredV33Observation(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, a OpenCodeStructuredV33Aggregate) error {
	t, ok := safeAddV33(a.Tokens, d.PerObservationTotalTokenReservation)
	if !ok {
		return fmt.Errorf("v33 aggregate token overflow")
	}
	cost, ok := OpenCodeStructuredV33CostReservation(d, b.Candidate)
	if !ok {
		return fmt.Errorf("v33 price-derived reservation overflow or invalid")
	}
	c, ok := safeAddV33(a.CostMicros, cost)
	if !ok {
		return fmt.Errorf("v33 aggregate cost overflow")
	}
	if a.ProviderTurns < 0 || b.Protocol.MaxProviderTurns > math.MaxInt-a.ProviderTurns {
		return fmt.Errorf("v33 aggregate turn overflow")
	}
	turns := a.ProviderTurns + b.Protocol.MaxProviderTurns
	if t > d.MaxCombinedTokens || c > d.MaxCombinedCostMicros || turns > d.MaxCombinedProviderTurns {
		return fmt.Errorf("v33 aggregate admission denied")
	}
	return nil
}

func OpenCodeStructuredV33CostReservation(d ProtocolAblationV32Definition, candidate ProtocolAblationV32Candidate) (int64, bool) {
	generated := d.PerObservationGeneratedTokenCeiling
	input := d.PerObservationTotalTokenReservation - generated
	if input < 0 || generated < 0 || candidate.InputMicrosPerMillion < 0 || candidate.GeneratedMicrosPerMillion < 0 {
		return 0, false
	}
	inCost, ok := safeMulV33(input, candidate.InputMicrosPerMillion)
	if !ok {
		return 0, false
	}
	outCost, ok := safeMulV33(generated, candidate.GeneratedMicrosPerMillion)
	if !ok {
		return 0, false
	}
	n, ok := safeAddV33(inCost, outCost)
	if !ok {
		return 0, false
	}
	q, r := n/int64(1_000_000), n%int64(1_000_000)
	if r > 0 {
		return safeAddV33(q, 1)
	}
	return q, true
}

func safeMulV33(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || (a != 0 && b > math.MaxInt64/a) {
		return 0, false
	}
	return a * b, true
}

type OpenCodeStructuredV33DurableEvent struct{ Kind, BindingID, EvidenceSHA256 string }

func ValidateOpenCodeStructuredV33DurableOrder(events []OpenCodeStructuredV33DurableEvent) error {
	want := []string{"preflight_sealed", "claim_sealed", "raw_response_sealed", "observation_sealed", "ledger_sealed"}
	if len(events) != len(want) {
		return fmt.Errorf("v33 durable event count")
	}
	binding := ""
	for i, e := range events {
		if e.Kind != want[i] || e.EvidenceSHA256 == "" {
			return fmt.Errorf("v33 durable write order")
		}
		if i == 1 {
			binding = e.BindingID
		}
		if i >= 1 && e.BindingID != binding {
			return fmt.Errorf("v33 durable binding drift")
		}
	}
	return nil
}
func RecoverOpenCodeStructuredV33Claim(events []OpenCodeStructuredV33DurableEvent) string {
	claim, raw, observation := false, false, false
	for _, e := range events {
		switch e.Kind {
		case "claim_sealed":
			claim = true
		case "raw_response_sealed":
			raw = true
		case "observation_sealed":
			observation = true
		}
	}
	if claim && !raw {
		return "global_halt_ambiguous_transport_no_retry"
	}
	if raw && !observation {
		return "reparse_raw_then_seal_observation"
	}
	if observation {
		return "resume_ledger_seal"
	}
	return "no_claim"
}

// ClassifyOpenCodeStructuredV33Failure makes campaign stop semantics explicit.
// Only candidate competence/protocol/resource exhaustion is local. Failures in
// credentials, runtime/config/credits, transport/evidence, unsafe action, or
// actual cumulative campaign limits stop the campaign and are durably recorded.
func ClassifyOpenCodeStructuredV33Failure(class string) string {
	switch strings.TrimSpace(class) {
	case "competence_failure", "response_or_tool_protocol_failure", "candidate_resource_exhaustion":
		return "candidate_local"
	case "credential_failure", "runtime_failure", "config_failure", "credits_failure", "transport_failure", "evidence_integrity_failure", "unsafe_execution", "campaign_budget_breach":
		return "global_halt"
	default:
		return "global_halt_unclassified"
	}
}

func SealOpenCodeStructuredV33GlobalHalt(class, evidenceSHA256 string) (OpenCodeStructuredV33DurableEvent, error) {
	scope := ClassifyOpenCodeStructuredV33Failure(class)
	if !strings.HasPrefix(scope, "global_halt") || strings.TrimSpace(evidenceSHA256) == "" {
		return OpenCodeStructuredV33DurableEvent{}, fmt.Errorf("v33 global halt requires global class and durable evidence")
	}
	return OpenCodeStructuredV33DurableEvent{Kind: "campaign_global_halt_sealed", BindingID: class, EvidenceSHA256: evidenceSHA256}, nil
}
func HashOpenCodeStructuredV33Runtime(executable, openAPI []byte) (string, string) {
	return hashRealisticNextBytes(executable), hashRealisticNextBytes(openAPI)
}
func EqualOpenCodeStructuredV33Raw(a, b []byte) bool { return bytes.Equal(a, b) }
