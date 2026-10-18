package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func workflowEventTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("reason", "", "")
	cmd.Flags().StringSlice("failed-criterion", nil, "")
	cmd.Flags().String("evidence", "", "")
	cmd.Flags().String("required-correction", "", "")
	cmd.Flags().String("candidate", "", "")
	cmd.Flags().String("artifact-evidence-file", "", "")
	cmd.Flags().String("stage-evidence-file", "", "")
	cmd.Flags().String("question", "", "")
	cmd.Flags().String("recommendation", "", "")
	cmd.Flags().String("consequence", "", "")
	cmd.Flags().StringSlice("option", nil, "")
	return cmd
}

func TestOwnerRuntimeRouteCLIStageEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stage.json")
	packet := `{"candidate_sha256":"candidate-digest","gates":{"review":{"state":"passed","refs":["actual-reference"]}}}`
	if err := os.WriteFile(path, []byte(packet), 0600); err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"review_passed", "deployment_succeeded", "production_verified", "rollback_succeeded"} {
		t.Run(event, func(t *testing.T) {
			cmd := workflowEventTestCommand()
			_ = cmd.Flags().Set("reason", "actual evidence")
			_ = cmd.Flags().Set("candidate", "candidate-digest")
			_ = cmd.Flags().Set("stage-evidence-file", path)
			body, err := workflowEventBody(cmd, event)
			if err != nil {
				t.Fatal(err)
			}
			if body["candidate"] != "candidate-digest" || string(body["stage_evidence"].(json.RawMessage)) != packet {
				t.Fatalf("transport changed evidence: %#v", body)
			}
		})
	}
}

func TestWorkflowEventBodyRequiresArtifactEvidenceForImplementationReady(t *testing.T) {
	cmd := workflowEventTestCommand()
	cmd.Flags().Set("reason", "implementation complete")
	if _, err := workflowEventBody(cmd, "implementation_ready"); err == nil {
		t.Fatal("implementation_ready accepted without --artifact-evidence-file")
	}
}

func TestWorkflowEventBodyReadsArtifactEvidenceForImplementationReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"contract":"repository_worktree_inventory.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := workflowEventTestCommand()
	cmd.Flags().Set("reason", "implementation complete")
	cmd.Flags().Set("artifact-evidence-file", path)
	body, err := workflowEventBody(cmd, "implementation_ready")
	if err != nil {
		t.Fatal(err)
	}
	if string(body["artifact_evidence"].(json.RawMessage)) != `{"schema_version":1,"contract":"repository_worktree_inventory.v1"}` {
		t.Fatalf("artifact evidence = %s", body["artifact_evidence"])
	}
}

func TestWorkflowEventBodyRequiresStructuredOwnerDecision(t *testing.T) {
	cmd := workflowEventTestCommand()
	cmd.Flags().Set("reason", "owner input required")
	if _, err := workflowEventBody(cmd, "owner_decision_required"); err == nil {
		t.Fatal("owner_decision_required accepted without decision packet flags")
	}
}

func TestWorkflowEventBodyBuildsStructuredOwnerDecision(t *testing.T) {
	cmd := workflowEventTestCommand()
	cmd.Flags().Set("reason", "owner input required")
	cmd.Flags().Set("question", "Which option should ship?")
	cmd.Flags().Set("recommendation", "Choose A because it is reversible")
	cmd.Flags().Set("consequence", "A resumes implementation; B keeps the issue paused")
	cmd.Flags().Set("option", "A")
	body, err := workflowEventBody(cmd, "owner_decision_required")
	if err != nil {
		t.Fatalf("build owner decision event: %v", err)
	}
	packet, ok := body["owner_decision"].(map[string]any)
	if !ok || packet["question"] != "Which option should ship?" {
		t.Fatalf("owner decision body = %#v", body["owner_decision"])
	}
}

func TestWorkflowEventBodyRequiresStructuredReviewFailure(t *testing.T) {
	cmd := workflowEventTestCommand()
	cmd.Flags().Set("reason", "failed")
	if _, err := workflowEventBody(cmd, "review_failed"); err == nil {
		t.Fatal("review_failed accepted without correction packet flags")
	}
}

func TestWorkflowEventBodyBuildsStructuredReviewFailure(t *testing.T) {
	cmd := workflowEventTestCommand()
	cmd.Flags().Set("reason", "failed")
	cmd.Flags().Set("failed-criterion", "exact output")
	cmd.Flags().Set("evidence", "received DRAFT")
	cmd.Flags().Set("required-correction", "return APPROVED")
	cmd.Flags().Set("candidate", "candidate-1")
	body, err := workflowEventBody(cmd, "review_failed")
	if err != nil {
		t.Fatalf("build workflow event: %v", err)
	}
	correction, ok := body["correction"].(map[string]any)
	if !ok || correction["required_correction"] != "return APPROVED" {
		t.Fatalf("correction body = %#v", body["correction"])
	}
}
