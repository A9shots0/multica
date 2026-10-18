package main

import (
	"context"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var issueWorkflowCompleteCmd = &cobra.Command{
	Use:   "workflow-complete <issue-id>",
	Short: "Accept a published handoff after independent review (requesting member only)",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("evidence-file")
		body, err := readWorkflowArtifactEvidence(path)
		if err != nil {
			return err
		}
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := cli.APIContext(context.Background())
		defer cancel()
		issue, err := resolveIssueRef(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("resolve issue: %w", err)
		}
		var result map[string]any
		if err := client.PostJSON(ctx, "/api/issues/"+issue.ID+"/operator-completion", body, &result); err != nil {
			return fmt.Errorf("accept operator-assisted completion: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

func init() {
	issueWorkflowCompleteCmd.Flags().String("evidence-file", "", "Completion JSON with exact revisions, review task, publication comment, candidate SHA256 and reason")
	_ = issueWorkflowCompleteCmd.MarkFlagRequired("evidence-file")
	issueCmd.AddCommand(issueWorkflowCompleteCmd)
}
