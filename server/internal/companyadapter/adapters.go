package companyadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
)

// WatcherIncident is the privacy-safe subset exported by the 8 GB Railway
// watcher. Raw log samples remain in the watcher and are addressed by evidence
// references instead of copied into Multica.
type WatcherIncident struct {
	EventID          string    `json:"event_id"`
	SourceInstance   string    `json:"source_instance"`
	IncidentID       string    `json:"incident_id"`
	Revision         string    `json:"revision"`
	PredecessorEvent string    `json:"predecessor_event_id,omitempty"`
	ServiceKey       string    `json:"service_ref"`
	Environment      string    `json:"environment_ref"`
	FailureFamily    string    `json:"failure_family"`
	Generation       int64     `json:"generation"`
	LifecycleState   string    `json:"lifecycle_state"`
	Severity         string    `json:"severity"`
	ActionRequired   bool      `json:"action_required"`
	Summary          string    `json:"summary"`
	NextStep         string    `json:"next_step"`
	EvidenceRefs     []string  `json:"evidence_refs"`
	RawPayloadHash   string    `json:"raw_payload_hash"`
	ObservedAt       time.Time `json:"observed_at"`
}

// PhoneFarmSupportIncident is the bounded projection exported by Phone Farm.
// It deliberately has no employee name, phone content, attachment, credential,
// or raw device identifier.
type PhoneFarmSupportIncident struct {
	EventID           string    `json:"event_id"`
	SourceInstance    string    `json:"source_instance"`
	SupportIncidentID string    `json:"support_incident_id"`
	Revision          string    `json:"revision"`
	PredecessorEvent  string    `json:"predecessor_event_id,omitempty"`
	ServiceKey        string    `json:"service_ref"`
	Environment       string    `json:"environment_ref"`
	FailureFamily     string    `json:"failure_family"`
	Generation        int64     `json:"generation"`
	LifecycleState    string    `json:"lifecycle_state"`
	Severity          string    `json:"severity"`
	ActionRequired    bool      `json:"action_required"`
	Summary           string    `json:"summary"`
	NextStep          string    `json:"next_step"`
	AffectedRefs      []string  `json:"affected_refs"`
	EvidenceRefs      []string  `json:"evidence_refs"`
	RawPayloadHash    string    `json:"raw_payload_hash"`
	ObservedAt        time.Time `json:"observed_at"`
}

func WatcherSignal(in WatcherIncident) service.CompanySignalInput {
	return service.CompanySignalInput{
		ContractVersion: 1, EventID: in.EventID, SourceSystem: "railway-ollama-alerts",
		SourceInstance: in.SourceInstance, SourceNativeID: in.IncidentID, SourceRevision: in.Revision,
		EventKind: "application_log", SourceLane: "application_logs", LifecycleState: in.LifecycleState,
		Severity: in.Severity, ActionRequired: in.ActionRequired, Summary: in.Summary, NextStep: in.NextStep,
		ServiceKey: in.ServiceKey, Environment: in.Environment, EvidenceRefs: in.EvidenceRefs,
		PredecessorEventID: in.PredecessorEvent, PrivacyClass: "confidential", RawPayloadHash: in.RawPayloadHash,
		CorrelationKey:     CorrelationKey(in.ServiceKey, in.Environment, in.FailureFamily),
		IncidentGeneration: in.Generation, ObservedAt: in.ObservedAt.UTC(),
	}
}

func PhoneFarmSupportSignal(in PhoneFarmSupportIncident) service.CompanySignalInput {
	return service.CompanySignalInput{
		ContractVersion: 1, EventID: in.EventID, SourceSystem: "phonefarm-support",
		SourceInstance: in.SourceInstance, SourceNativeID: in.SupportIncidentID, SourceRevision: in.Revision,
		EventKind: "support_report", SourceLane: "support_impact", LifecycleState: in.LifecycleState,
		Severity: in.Severity, ActionRequired: in.ActionRequired, Summary: in.Summary, NextStep: in.NextStep,
		AffectedRefs: in.AffectedRefs, ServiceKey: in.ServiceKey, Environment: in.Environment,
		EvidenceRefs: in.EvidenceRefs, PredecessorEventID: in.PredecessorEvent, PrivacyClass: "restricted",
		RawPayloadHash:     in.RawPayloadHash,
		CorrelationKey:     CorrelationKey(in.ServiceKey, in.Environment, in.FailureFamily),
		IncidentGeneration: in.Generation, ObservedAt: in.ObservedAt.UTC(),
	}
}

func CorrelationKey(serviceKey, environment, failureFamily string) string {
	canonical := strings.Join([]string{
		"company-incident-v1", strings.ToLower(strings.TrimSpace(serviceKey)),
		strings.ToLower(strings.TrimSpace(environment)), strings.ToLower(strings.TrimSpace(failureFamily)),
	}, "|")
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

func ValidateCorrelationInputs(serviceKey, environment, failureFamily string) error {
	if strings.TrimSpace(serviceKey) == "" || strings.TrimSpace(environment) == "" || strings.TrimSpace(failureFamily) == "" {
		return fmt.Errorf("service, environment, and failure family are required")
	}
	return nil
}
