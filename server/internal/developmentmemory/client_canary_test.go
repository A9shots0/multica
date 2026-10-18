package developmentmemory

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestDevelopmentMemoryRealStackCanary is opt-in because it writes only to the
// explicitly supplied disposable Intelligence database. Release qualification
// uses it to prove the real HTTP client, idempotent writer, and cross-issue
// project baseline together instead of inferring integration from unit tests.
func TestDevelopmentMemoryRealStackCanary(t *testing.T) {
	baseURL := os.Getenv("MULTICA_DEVELOPMENT_MEMORY_CANARY_URL")
	writer := os.Getenv("MULTICA_DEVELOPMENT_MEMORY_CANARY_WRITER_TOKEN")
	reader := os.Getenv("MULTICA_DEVELOPMENT_MEMORY_CANARY_READER_TOKEN")
	if baseURL == "" || writer == "" || reader == "" {
		t.Skip("disposable Intelligence canary is not configured")
	}
	client, err := New(Config{BaseURL: baseURL, WriterToken: writer, ReaderToken: reader})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	firstTask := Task{
		IssueID: "10000000-0000-7000-8000-000000000001", IssueIdentifier: "CANARY-1",
		Title: "Coordinate CRM and Viral Queue", Goal: "Keep frontend controls and ranking contracts aligned.",
		ProjectID: "canary-viral-operations", ProjectName: "Canary viral operations",
		WorkspaceID: "canary-workspace", WorkspaceSlug: "canary",
		Repositories: []Repository{
			{URL: "https://github.com/A9shots0/SMCrm.git", Ref: "main"},
			{URL: "https://github.com/A9shots0/viralcontentpuller.git", Ref: "main"},
		},
	}
	if _, err := client.PrepareContext(ctx, firstTask); err != nil {
		t.Fatalf("prepare first issue: %v", err)
	}
	crm := "a9shots0/smcrm"
	started := time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC)
	if err := client.Deliver(ctx, Delivery{
		ProposalID:   "canary-proposal-project-memory",
		TaskID:       "canary-task-project-memory",
		WorkstreamID: "multica-issue-" + firstTask.IssueID,
		AgentName:    "Codex", TaskStartedAt: started,
		TaskCompletedAt: started.Add(30 * time.Second), TaskStatus: "complete",
		Repositories: []BoundRepository{
			{CanonicalIdentity: crm},
			{CanonicalIdentity: "a9shots0/viralcontentpuller"},
		},
		Payload: ProposalDraft{
			Summary:    "CRM and Viral Queue use one reviewed control contract.",
			NextAction: "Verify a separate issue receives the approved project rule.",
			MemoryItems: []MemoryItemDraft{{
				RepositoryIdentity: &crm, Scope: "project", Kind: "decision",
				Statement:    "CRM owns operator-facing Viral Queue controls.",
				EvidenceRefs: []string{"canary:reviewed-project-rule"},
			}},
			RepositoryEvidence: []RepositoryEvidence{
				{CanonicalIdentity: crm, EvidenceRefs: []string{}},
				{CanonicalIdentity: "a9shots0/viralcontentpuller", EvidenceRefs: []string{}},
			},
			RemainingUncertainty: []string{},
		},
	}); err != nil {
		t.Fatalf("deliver reviewed first issue: %v", err)
	}

	secondTask := firstTask
	secondTask.IssueID = "10000000-0000-7000-8000-000000000002"
	secondTask.IssueIdentifier = "CANARY-2"
	secondTask.Title = "Start a separate CRM control issue"
	packet, err := client.PrepareContext(ctx, secondTask)
	if err != nil {
		t.Fatalf("prepare second issue: %v", err)
	}
	if !strings.Contains(string(packet), "CRM owns operator-facing Viral Queue controls.") ||
		!strings.Contains(string(packet), `"project_baselines"`) {
		t.Fatalf("second issue did not receive reviewed project memory: %s", packet)
	}
}
