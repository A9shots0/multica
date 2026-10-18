package companyadapter

import (
	"testing"
	"time"
)

func TestWatcherAndSupportUseSameDeterministicIncident(t *testing.T) {
	observed := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	watcher := WatcherSignal(WatcherIncident{
		EventID: "watcher:sidecar:41", SourceInstance: "mac-mini-8gb", IncidentID: "watcher-41", Revision: "7",
		ServiceKey: "phonefarm.sidecar", Environment: "production", FailureFamily: "sidecar-workflow-unavailable",
		Generation: 1, LifecycleState: "worsened", Severity: "critical", ActionRequired: true,
		Summary: "Sidecar requests are failing", NextStep: "Inspect the current release",
		EvidenceRefs: []string{"watcher:incident-41"}, RawPayloadHash: sixtyFour("a"), ObservedAt: observed,
	})
	support := PhoneFarmSupportSignal(PhoneFarmSupportIncident{
		EventID: "support:incident:9:report:3", SourceInstance: "phonefarm-production", SupportIncidentID: "support-9", Revision: "3",
		ServiceKey: "phonefarm.sidecar", Environment: "production", FailureFamily: "sidecar-workflow-unavailable",
		Generation: 1, LifecycleState: "new", Severity: "high", ActionRequired: false,
		Summary: "Three employees cannot complete the workflow", NextStep: "Keep reports linked to the operational incident",
		AffectedRefs: []string{"person:employee-1", "device:phone-2"}, EvidenceRefs: []string{"support:report-3"},
		RawPayloadHash: sixtyFour("b"), ObservedAt: observed.Add(time.Minute),
	})
	if watcher.CorrelationKey != support.CorrelationKey {
		t.Fatalf("correlation keys differ: watcher=%s support=%s", watcher.CorrelationKey, support.CorrelationKey)
	}
	if watcher.SourceLane == support.SourceLane {
		t.Fatalf("evidence lanes collapsed: %s", watcher.SourceLane)
	}
	if len(support.AffectedRefs) != 2 || support.PrivacyClass != "restricted" {
		t.Fatalf("support projection lost privacy-safe impact: %#v", support)
	}
}

func TestCorrelationKeyChangesWithServiceOrFailureFamily(t *testing.T) {
	base := CorrelationKey("phonefarm.sidecar", "production", "workflow-unavailable")
	if base == CorrelationKey("phonefarm.hub", "production", "workflow-unavailable") {
		t.Fatal("different services correlated")
	}
	if base == CorrelationKey("phonefarm.sidecar", "production", "authentication-failed") {
		t.Fatal("different failure families correlated")
	}
}

func sixtyFour(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result[:64]
}
