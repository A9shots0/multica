package workflowprojection

import (
	"encoding/json"
	"sort"
	"strings"
)

const ReleaseStatusSchemaVersion int32 = 1

type ReleaseEvidenceGate struct {
	State       string   `json:"state"`
	Refs        []string `json:"refs"`
	ObservedAt  *string  `json:"observed_at"`
	Limitations []string `json:"limitations"`
}

type ReleaseGates struct {
	Review      ReleaseEvidenceGate `json:"review"`
	Build       ReleaseEvidenceGate `json:"build"`
	Deployment  ReleaseEvidenceGate `json:"deployment"`
	Exposure    ReleaseEvidenceGate `json:"exposure"`
	Runtime     ReleaseEvidenceGate `json:"runtime"`
	Business    ReleaseEvidenceGate `json:"business"`
	Regression  ReleaseEvidenceGate `json:"regression"`
	Rollback    ReleaseEvidenceGate `json:"rollback"`
	Restoration ReleaseEvidenceGate `json:"restoration"`
}

type ReleaseBinding struct {
	ReleaseID           string  `json:"release_id"`
	ReleaseRevision     int64   `json:"release_revision"`
	CandidateID         string  `json:"candidate_id"`
	CandidateRevision   int64   `json:"candidate_revision"`
	SourceRevision      string  `json:"source_revision"`
	ArtifactDigest      string  `json:"artifact_digest"`
	BuildManifestDigest string  `json:"build_manifest_digest"`
	ScopeRevision       int64   `json:"scope_revision"`
	AcceptanceVersion   int64   `json:"acceptance_version"`
	AuthorityRevision   int64   `json:"authority_revision"`
	TargetKey           string  `json:"target_key"`
	TargetGeneration    int64   `json:"target_generation"`
	DeployedRevision    *string `json:"deployed_revision"`
}

type ReleaseNextAction struct {
	Kind      string  `json:"kind"`
	Automatic bool    `json:"automatic"`
	ActorType *string `json:"actor_type"`
	ActionID  *string `json:"action_id"`
	DueAt     *string `json:"due_at"`
}

type ReleaseStop struct {
	Code      string `json:"code"`
	Reason    string `json:"reason"`
	Retryable bool   `json:"retryable"`
}

type ReleaseHistory struct {
	WasVerifiedLive    bool    `json:"was_verified_live"`
	LastVerifiedLiveAt *string `json:"last_verified_live_at"`
}

type ReleaseStatus struct {
	SchemaVersion int32              `json:"schema_version"`
	Phase         string             `json:"phase"`
	Binding       ReleaseBinding     `json:"binding"`
	Gates         ReleaseGates       `json:"gates"`
	Historical    ReleaseHistory     `json:"historical"`
	NextAction    *ReleaseNextAction `json:"next_action"`
	Stop          *ReleaseStop       `json:"stop"`
}

// ValidReleaseStatusDocument validates the v3 release status against the raw
// authoritative release and health fields in the same semantic document.
func ValidReleaseStatusDocument(raw []byte) bool {
	var document struct {
		ContractVersion int32          `json:"contract_version"`
		Release         map[string]any `json:"release"`
		Health          string         `json:"health"`
		EvidenceSummary map[string]any `json:"evidence_summary"`
		ReleaseStatus   *ReleaseStatus `json:"release_status"`
	}
	if json.Unmarshal(raw, &document) != nil || document.ContractVersion != 3 ||
		!validReleaseStatus(document.ReleaseStatus, document.Release, document.Health) {
		return false
	}
	return equalOptionalString(document.ReleaseStatus.Binding.DeployedRevision, exactDeploymentReceipt(document.EvidenceSummary))
}

func buildReleaseStatus(release, candidate, scope, target, evidence, execution map[string]any, historical map[string]any, health, terminalReason string) *ReleaseStatus {
	if intValue(release["attempt_contract_version"]) != 3 {
		return nil
	}
	binding := ReleaseBinding{
		ReleaseID: optionalStringValue(release, "id"), ReleaseRevision: intValue(release["revision"]),
		CandidateID: optionalStringValue(release, "candidate_id"), CandidateRevision: intValue(release["candidate_revision"]),
		SourceRevision: optionalStringValue(candidate, "source_revision"), ArtifactDigest: optionalStringValue(release, "artifact_digest"),
		BuildManifestDigest: optionalStringValue(release, "build_manifest_digest"), ScopeRevision: intValue(release["scope_revision"]),
		AcceptanceVersion: intValue(release["acceptance_version"]), AuthorityRevision: intValue(release["authority_revision"]),
		TargetKey: optionalStringValue(release, "target_key"), TargetGeneration: intValue(release["target_generation"]),
	}
	if binding.ReleaseID == "" || binding.ReleaseRevision < 1 || binding.CandidateID == "" || binding.CandidateRevision < 1 ||
		binding.SourceRevision == "" || binding.ArtifactDigest == "" || binding.BuildManifestDigest == "" ||
		binding.ScopeRevision < 1 || binding.AcceptanceVersion < 1 || binding.AuthorityRevision < 1 ||
		binding.TargetKey == "" || binding.TargetGeneration < 1 || intValue(scope["revision"]) != binding.ScopeRevision {
		return nil
	}
	if exactTargetRelease(release, target) {
		binding.DeployedRevision = exactDeploymentReceipt(evidence)
	}

	status := &ReleaseStatus{
		SchemaVersion: ReleaseStatusSchemaVersion, Binding: binding,
		Gates: ReleaseGates{
			Review: evidenceGate(evidence, "technical_review"), Build: evidenceGate(evidence, "reproducible_build"),
			Deployment: evidenceGate(evidence, "deployment"), Exposure: evidenceGate(evidence, "exposure"),
			Runtime: evidenceGate(evidence, "runtime"), Business: evidenceGate(evidence, "business"),
			Regression: evidenceGate(evidence, "regression"), Rollback: evidenceGate(evidence, "rollback"),
			Restoration: evidenceGate(evidence, "restoration"),
		},
		NextAction: releaseNextAction(execution),
	}
	state := optionalStringValue(release, "state")
	switch state {
	case "pending", "built", "reviewed":
		status.Phase = "reviewed"
	case "authorized", "deploying":
		status.Phase = "releasing"
	case "deployed", "exposed", "production_verifying":
		status.Phase = "deployed_checking"
	case "live":
		if health != "healthy" {
			return nil
		}
		status.Phase = "verified_live"
	case "degraded":
		status.Phase = "regressed"
	case "rolling_back", "rolled_back":
		status.Phase = "rolling_back"
	case "restored":
		if health != "restored" {
			return nil
		}
		status.Phase = "rolled_back"
	case "failed":
		status.Phase = "technical_stop"
		status.Stop = &ReleaseStop{Code: "release_failed", Reason: strings.TrimSpace(terminalReason), Retryable: false}
		if status.Stop.Reason == "" {
			status.Stop.Reason = "release execution stopped"
		}
	default:
		return nil
	}
	if status.Phase == "verified_live" || status.Phase == "regressed" || status.Phase == "rolling_back" || status.Phase == "rolled_back" {
		status.Historical.WasVerifiedLive = historical["satisfied"] == true
		status.Historical.LastVerifiedLiveAt = latestObserved(status.Gates.Business)
		if status.Historical.WasVerifiedLive && status.Historical.LastVerifiedLiveAt == nil {
			return nil
		}
	}
	if status.Phase == "verified_live" && (!allPassed(status.Gates.Review, status.Gates.Build, status.Gates.Deployment, status.Gates.Exposure, status.Gates.Runtime, status.Gates.Business, status.Gates.Regression) || binding.DeployedRevision == nil) {
		return nil
	}
	if status.Phase == "deployed_checking" && (status.Gates.Deployment.State != "passed" || binding.DeployedRevision == nil) {
		return nil
	}
	if status.Phase == "rolled_back" && (!allPassed(status.Gates.Rollback, status.Gates.Restoration) || binding.DeployedRevision == nil) {
		return nil
	}
	if !validReleaseStatus(status, release, health) {
		return nil
	}
	return status
}

// exactDeploymentReceipt returns the one immutable provider readback identity
// bound to current, applicable deployment evidence. An observation timestamp
// is ordering metadata, not proof of which effect reached the target.
func exactDeploymentReceipt(summary map[string]any) *string {
	items := objectSlice(summary["items"])
	var receipt string
	for _, item := range items {
		if optionalStringValue(item, "evidence_type") != "deployment" || item["applies"] != true {
			continue
		}
		candidate := strings.TrimSpace(optionalStringValue(item, "provider_receipt_id"))
		if candidate == "" || (receipt != "" && receipt != candidate) {
			return nil
		}
		receipt = candidate
	}
	if receipt == "" {
		return nil
	}
	return &receipt
}

func objectSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			object, ok := item.(map[string]any)
			if !ok {
				return nil
			}
			result = append(result, object)
		}
		return result
	default:
		return nil
	}
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func exactTargetRelease(release, target map[string]any) bool {
	return optionalStringValue(target, "release_id") == optionalStringValue(release, "id") &&
		optionalStringValue(target, "candidate_id") == optionalStringValue(release, "candidate_id") &&
		optionalStringValue(target, "target_key") == optionalStringValue(release, "target_key") &&
		intValue(target["generation"]) == intValue(release["target_generation"])
}

func evidenceGate(summary map[string]any, evidenceType string) ReleaseEvidenceGate {
	gate := ReleaseEvidenceGate{State: "missing", Refs: []string{}, Limitations: []string{}}
	items, _ := summary["items"].([]map[string]any)
	for _, item := range items {
		if optionalStringValue(item, "evidence_type") != evidenceType {
			continue
		}
		id := optionalStringValue(item, "id")
		if id != "" {
			gate.Refs = append(gate.Refs, id)
		}
		for _, limitation := range stringSlice(item["limitations"]) {
			gate.Limitations = append(gate.Limitations, limitation)
		}
		freshness, _ := item["freshness"].(map[string]any)
		observed := optionalStringValue(item, "observed_at")
		if freshness["contradicted"] == true {
			gate.State = "contradicted"
		} else if freshness["superseded"] == true || freshness["current"] != true {
			if gate.State == "missing" {
				gate.State = "stale"
			}
		} else {
			gate.State = "passed"
			if observed != "" {
				gate.ObservedAt = &observed
			}
		}
	}
	if evidenceType == "technical_review" {
		reviews, _ := summary["technical_reviews"].([]map[string]any)
		for _, review := range reviews {
			id := optionalStringValue(review, "id")
			if id != "" {
				gate.Refs = append(gate.Refs, id)
			}
			if optionalStringValue(review, "outcome") == "passed" {
				gate.State = "passed"
				if observed := optionalStringValue(review, "completed_at"); observed != "" {
					gate.ObservedAt = &observed
				}
			}
		}
	}
	sort.Strings(gate.Refs)
	sort.Strings(gate.Limitations)
	return gate
}

func stringSlice(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func releaseNextAction(execution map[string]any) *ReleaseNextAction {
	expected, _ := execution["expected_event"].(map[string]any)
	kind := optionalStringValue(expected, "type")
	if kind == "" {
		return nil
	}
	var actorType *string
	if actor := optionalStringValue(execution, "actor_type"); actor != "" {
		actorType = &actor
	}
	var dueAt *string
	if due := optionalStringValue(execution, "next_action_at"); due != "" {
		dueAt = &due
	}
	return &ReleaseNextAction{Kind: kind, Automatic: true, ActorType: actorType, DueAt: dueAt}
}

func latestObserved(gate ReleaseEvidenceGate) *string { return gate.ObservedAt }

func allPassed(gates ...ReleaseEvidenceGate) bool {
	for _, gate := range gates {
		if gate.State != "passed" {
			return false
		}
	}
	return true
}

func validReleaseStatus(status *ReleaseStatus, release map[string]any, health string) bool {
	if status == nil || status.SchemaVersion != ReleaseStatusSchemaVersion ||
		status.Binding.ReleaseID != optionalStringValue(release, "id") ||
		status.Binding.ReleaseRevision != intValue(release["revision"]) ||
		status.Binding.CandidateID != optionalStringValue(release, "candidate_id") ||
		status.Binding.CandidateRevision != intValue(release["candidate_revision"]) ||
		status.Binding.ScopeRevision != intValue(release["scope_revision"]) ||
		status.Binding.AcceptanceVersion != intValue(release["acceptance_version"]) ||
		status.Binding.AuthorityRevision != intValue(release["authority_revision"]) ||
		status.Binding.TargetKey != optionalStringValue(release, "target_key") ||
		status.Binding.TargetGeneration != intValue(release["target_generation"]) ||
		strings.TrimSpace(status.Binding.SourceRevision) == "" || strings.TrimSpace(status.Binding.ArtifactDigest) == "" ||
		strings.TrimSpace(status.Binding.BuildManifestDigest) == "" || status.Binding.ReleaseRevision < 1 ||
		status.Binding.CandidateRevision < 1 || status.Binding.ScopeRevision < 1 || status.Binding.AcceptanceVersion < 1 ||
		status.Binding.AuthorityRevision < 1 || status.Binding.TargetGeneration < 1 ||
		status.Historical.WasVerifiedLive != (status.Historical.LastVerifiedLiveAt != nil) {
		return false
	}
	for _, gate := range []ReleaseEvidenceGate{status.Gates.Review, status.Gates.Build, status.Gates.Deployment, status.Gates.Exposure, status.Gates.Runtime, status.Gates.Business, status.Gates.Regression, status.Gates.Rollback, status.Gates.Restoration} {
		if !oneOfString(gate.State, "missing", "running", "passed", "failed", "stale", "contradicted") || gate.Refs == nil || gate.Limitations == nil ||
			(gate.State == "passed" && (len(gate.Refs) == 0 || gate.ObservedAt == nil)) {
			return false
		}
	}
	state := optionalStringValue(release, "state")
	switch status.Phase {
	case "reviewed":
		return oneOfString(state, "pending", "built", "reviewed") && status.Stop == nil
	case "releasing":
		return oneOfString(state, "authorized", "deploying") && status.Stop == nil
	case "deployed_checking":
		return oneOfString(state, "deployed", "exposed", "production_verifying") && status.Gates.Deployment.State == "passed" && status.Binding.DeployedRevision != nil && status.Stop == nil
	case "verified_live":
		return state == "live" && health == "healthy" && status.Binding.DeployedRevision != nil && status.Stop == nil &&
			allPassed(status.Gates.Review, status.Gates.Build, status.Gates.Deployment, status.Gates.Exposure, status.Gates.Runtime, status.Gates.Business, status.Gates.Regression)
	case "regressed":
		return state == "degraded" && oneOfString(status.Gates.Regression.State, "failed", "contradicted") && status.Stop == nil
	case "rolling_back":
		return oneOfString(state, "rolling_back", "rolled_back") && status.Stop == nil
	case "rolled_back":
		return state == "restored" && health == "restored" && status.Binding.DeployedRevision != nil && status.Stop == nil && allPassed(status.Gates.Rollback, status.Gates.Restoration)
	case "technical_stop":
		return status.Stop != nil && strings.TrimSpace(status.Stop.Code) != "" && strings.TrimSpace(status.Stop.Reason) != ""
	default:
		return false
	}
}
