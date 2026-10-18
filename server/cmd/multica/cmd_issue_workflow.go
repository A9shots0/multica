package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var issueWorkflowPlanCmd = &cobra.Command{
	Use:   "workflow-plan <issue-id>",
	Short: "Define the deterministic squad path for an issue",
	Args:  exactArgs(1),
	RunE:  runIssueWorkflowPlan,
}

var issueWorkflowShowCmd = &cobra.Command{
	Use:   "workflow-show <issue-id>",
	Short: "Show the deterministic squad workflow plan",
	Args:  exactArgs(1),
	RunE:  runIssueWorkflowShow,
}

var issueWorkflowEventCmd = &cobra.Command{
	Use:   "workflow-event <issue-id> <event>",
	Short: "Record a structured workflow result and route the next stage",
	Args:  exactArgs(2),
	RunE:  runIssueWorkflowEvent,
}

var issueWorkflowDecideCmd = &cobra.Command{
	Use: "workflow-decide <issue-id>", Short: "Answer the current structured owner decision", Args: exactArgs(1), RunE: runIssueWorkflowDecide,
}

func runIssueWorkflowPlan(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	issueRef, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}

	squadID, _ := cmd.Flags().GetString("squad-id")
	implementerID, _ := cmd.Flags().GetString("implementer-id")
	reviewerID, _ := cmd.Flags().GetString("reviewer-id")
	releaseAgentID, _ := cmd.Flags().GetString("release-agent-id")
	verifierID, _ := cmd.Flags().GetString("verifier-id")
	requiresOwner, _ := cmd.Flags().GetBool("requires-owner")
	requiresDeployment, _ := cmd.Flags().GetBool("requires-deployment")
	body := map[string]any{
		"squad_id": squadID, "implementer_id": implementerID, "reviewer_id": reviewerID,
		"release_agent_id": releaseAgentID, "verifier_id": verifierID,
		"requires_owner": requiresOwner, "requires_deployment": requiresDeployment,
	}
	var result map[string]any
	if err := client.PutJSON(ctx, "/api/issues/"+issueRef.ID+"/workflow-plan", body, &result); err != nil {
		return fmt.Errorf("save workflow plan: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runIssueWorkflowShow(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	issueRef, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}
	var result map[string]any
	if err := client.GetJSON(ctx, "/api/issues/"+issueRef.ID+"/workflow-plan", &result); err != nil {
		return fmt.Errorf("read workflow plan: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runIssueWorkflowEvent(cmd *cobra.Command, args []string) error {
	body, err := workflowEventBody(cmd, args[1])
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	issueRef, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/issues/"+issueRef.ID+"/workflow-events", body, &result); err != nil {
		return fmt.Errorf("record workflow event: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func workflowEventBody(cmd *cobra.Command, event string) (map[string]any, error) {
	reason, _ := cmd.Flags().GetString("reason")
	if reason == "" {
		return nil, fmt.Errorf("--reason is required")
	}
	body := map[string]any{"event": event, "reason": reason}
	candidate, _ := cmd.Flags().GetString("candidate")
	if candidate != "" {
		body["candidate"] = candidate
	}
	if path, _ := cmd.Flags().GetString("stage-evidence-file"); path != "" {
		evidence, err := readWorkflowArtifactEvidence(path)
		if err != nil {
			return nil, err
		}
		body["stage_evidence"] = evidence
	}
	if event == "review_passed" {
		path, _ := cmd.Flags().GetString("test-evidence-file")
		if path != "" {
			testEvidence, err := readWorkflowArtifactEvidence(path)
			if err != nil {
				return nil, err
			}
			body["test_evidence"] = testEvidence
		}
		blastRadiusPath, _ := cmd.Flags().GetString("blast-radius-file")
		if blastRadiusPath != "" {
			blastRadiusReview, err := readWorkflowArtifactEvidence(blastRadiusPath)
			if err != nil {
				return nil, err
			}
			body["blast_radius_review"] = blastRadiusReview
		}
	}
	if event == "implementation_ready" {
		path, _ := cmd.Flags().GetString("artifact-evidence-file")
		if path == "" {
			return nil, fmt.Errorf("implementation_ready requires --artifact-evidence-file")
		}
		artifactEvidence, err := readWorkflowArtifactEvidence(path)
		if err != nil {
			return nil, err
		}
		body["artifact_evidence"] = artifactEvidence
	}
	if event == "review_failed" {
		failedCriteria, _ := cmd.Flags().GetStringSlice("failed-criterion")
		evidence, _ := cmd.Flags().GetString("evidence")
		requiredCorrection, _ := cmd.Flags().GetString("required-correction")
		candidate, _ := cmd.Flags().GetString("candidate")
		if len(failedCriteria) == 0 || evidence == "" || requiredCorrection == "" {
			return nil, fmt.Errorf("review_failed requires --failed-criterion, --evidence, and --required-correction")
		}
		body["correction"] = map[string]any{
			"failed_criteria": failedCriteria, "evidence": evidence,
			"required_correction": requiredCorrection, "candidate": candidate,
		}
	}
	if event == "owner_decision_required" {
		question, _ := cmd.Flags().GetString("question")
		recommendation, _ := cmd.Flags().GetString("recommendation")
		consequence, _ := cmd.Flags().GetString("consequence")
		options, _ := cmd.Flags().GetStringSlice("option")
		if question == "" || recommendation == "" || consequence == "" {
			return nil, fmt.Errorf("owner_decision_required requires --question, --recommendation, and --consequence")
		}
		body["owner_decision"] = map[string]any{"question": question, "recommendation": recommendation, "consequence": consequence, "options": options}
	}
	return body, nil
}

const maxWorkflowArtifactEvidenceBytes = 256 << 10

func readWorkflowArtifactEvidence(path string) (json.RawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open artifact evidence: %w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxWorkflowArtifactEvidenceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read artifact evidence: %w", err)
	}
	if len(raw) > maxWorkflowArtifactEvidenceBytes {
		return nil, fmt.Errorf("artifact evidence exceeds %d bytes", maxWorkflowArtifactEvidenceBytes)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("artifact evidence is not valid JSON")
	}
	return json.RawMessage(raw), nil
}

func runIssueWorkflowDecide(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	issueRef, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}
	requestEventID, _ := cmd.Flags().GetString("request-event-id")
	answer, _ := cmd.Flags().GetString("answer")
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/issues/"+issueRef.ID+"/workflow-owner-decision", map[string]any{"request_event_id": requestEventID, "answer": answer}, &result); err != nil {
		return fmt.Errorf("record owner decision: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func init() {
	issueWorkflowPlanCmd.Flags().String("squad-id", "", "Owning squad UUID (required)")
	issueWorkflowPlanCmd.Flags().String("implementer-id", "", "Implementer agent UUID (required)")
	issueWorkflowPlanCmd.Flags().String("reviewer-id", "", "Independent reviewer agent UUID (required)")
	issueWorkflowPlanCmd.Flags().String("release-agent-id", "", "Release agent UUID")
	issueWorkflowPlanCmd.Flags().String("verifier-id", "", "Independent production verifier UUID")
	issueWorkflowPlanCmd.Flags().Bool("requires-owner", false, "Require owner acceptance after technical review")
	issueWorkflowPlanCmd.Flags().Bool("requires-deployment", false, "Continue through release and production verification")
	_ = issueWorkflowPlanCmd.MarkFlagRequired("squad-id")
	_ = issueWorkflowPlanCmd.MarkFlagRequired("implementer-id")
	_ = issueWorkflowPlanCmd.MarkFlagRequired("reviewer-id")

	issueWorkflowEventCmd.Flags().String("reason", "", "Short evidence-backed reason (required)")
	issueWorkflowEventCmd.Flags().StringSlice("failed-criterion", nil, "Failed acceptance criterion (repeatable; review_failed only)")
	issueWorkflowEventCmd.Flags().String("evidence", "", "Evidence for the failed criterion (review_failed only)")
	issueWorkflowEventCmd.Flags().String("required-correction", "", "Exact bounded correction required (review_failed only)")
	issueWorkflowEventCmd.Flags().String("candidate", "", "Candidate or revision being reviewed (optional)")
	issueWorkflowEventCmd.Flags().String("stage-evidence-file", "", "Structured review/release/verification evidence for an owner-declared ordinary workflow")
	issueWorkflowEventCmd.Flags().String("artifact-evidence-file", "", "Repository/worktree inventory evidence JSON (implementation_ready only)")
	issueWorkflowEventCmd.Flags().String("test-evidence-file", "", "Build/test command evidence JSON proving the reviewed candidate actually builds and passes (review_passed on an ordinary, non owner-declared workflow)")
	issueWorkflowEventCmd.Flags().String("blast-radius-file", "", "Design/blast-radius reasoning JSON: what related code the reviewer actually checked and why this change is not shipping something stupid (review_passed on an ordinary, non owner-declared workflow)")
	issueWorkflowEventCmd.Flags().String("question", "", "Exact question for the accountable owner")
	issueWorkflowEventCmd.Flags().String("recommendation", "", "Recommended choice with reason")
	issueWorkflowEventCmd.Flags().String("consequence", "", "What happens after the decision")
	issueWorkflowEventCmd.Flags().StringSlice("option", nil, "Concrete owner option (repeatable)")
	_ = issueWorkflowEventCmd.MarkFlagRequired("reason")
	issueWorkflowDecideCmd.Flags().String("request-event-id", "", "Owner decision request event UUID (required)")
	issueWorkflowDecideCmd.Flags().String("answer", "", "Owner answer (required)")
	_ = issueWorkflowDecideCmd.MarkFlagRequired("request-event-id")
	_ = issueWorkflowDecideCmd.MarkFlagRequired("answer")

	issueCmd.AddCommand(issueWorkflowPlanCmd, issueWorkflowShowCmd, issueWorkflowEventCmd, issueWorkflowDecideCmd)
}
