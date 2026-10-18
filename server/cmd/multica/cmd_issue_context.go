package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

const (
	contextTokenEnv            = "MULTICA_CONTEXT_TOKEN"
	maxContextRequestFileBytes = 64 * 1024
)

func newIssueContextCommand() *cobra.Command {
	contextCmd := &cobra.Command{
		Use:   "context",
		Short: "Request bounded context for linked outside work",
	}
	requestCmd := &cobra.Command{
		Use:   "request <issue-ref>",
		Short: "Resolve one Context Request v0.2 JSON file",
		Long: "Validate one Context Request v0.2 file and ask Multica's bounded read-only broker. " +
			"The work id must already be linked to the issue by an admitted outcome receipt. Credentials are read only from " + contextTokenEnv + ".",
		Args: exactArgs(1),
		RunE: runIssueContextRequest,
	}
	requestCmd.Flags().String("file", "", "Context Request v0.2 JSON file")
	requestCmd.Flags().Bool("dry-run", false, "Validate locally without reading credentials or contacting Multica")
	requestCmd.Flags().String("output", "json", "Output format: text or json")
	requestCmd.Flags().Bool("allow-external-file", false, "Allow --file to read outside the current working directory")
	contextCmd.AddCommand(requestCmd)
	return contextCmd
}

func runIssueContextRequest(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("file")
	if strings.TrimSpace(path) == "" {
		return receiptValidationError("Choose one context request file with --file.", nil)
	}
	if err := ensureFileFlagWithinWorkdir(cmd, "file", "context-request", path); err != nil {
		return receiptValidationError("The context request file is outside this work directory.", err)
	}
	request, err := readContextRequestFile(path)
	if err != nil {
		return err
	}
	format, _ := cmd.Flags().GetString("output")
	if format != "text" && format != "json" {
		return receiptValidationError("--output must be text or json.", nil)
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		if format == "json" {
			return cli.PrintJSON(cmd.OutOrStdout(), map[string]any{"status": "valid", "sent": false, "request": request})
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Context request is valid. Nothing was sent.")
		return err
	}

	token := strings.TrimSpace(os.Getenv(contextTokenEnv))
	if token == "" {
		// Receipt and context access normally share one narrowly scoped plugin
		// installation. Keep the prior deployment usable while installations
		// migrate to the clearer context-specific variable.
		token = strings.TrimSpace(os.Getenv(receiptTokenEnv))
	}
	if !strings.HasPrefix(token, "mpi_") && !strings.HasPrefix(token, "mpc_") {
		return cli.NewClassifiedError(cli.KindAuthRequired,
			"Set "+contextTokenEnv+" to a Multica plugin installation or callback token; the token is never accepted as a command-line argument.", nil)
	}
	serverURL := tryResolveServerURL(cmd)
	if serverURL == "" {
		return receiptValidationError("No Multica server is configured. Set --server-url, MULTICA_SERVER_URL, or the profile server_url.", nil)
	}
	client := cli.NewAPIClient(serverURL, "", token)
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	var response publicapiv1.ContextResponseV02
	postPath := publicapiv1.BasePath + "/issues/" + url.PathEscape(strings.TrimSpace(args[0])) + "/context-requests"
	if err := client.PostJSON(ctx, postPath, request, &response); err != nil {
		return fmt.Errorf("resolve bounded context: %w", err)
	}
	if err := response.Validate(); err != nil {
		return fmt.Errorf("Multica returned an invalid context response: %w", err)
	}
	if format == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), response)
	}
	payload, _ := json.MarshalIndent(response.Payload, "", "  ")
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\nSource: %s\nPayload:\n%s\n", response.Status, response.Source.System, payload)
	return err
}

func readContextRequestFile(path string) (publicapiv1.ContextRequestV02, error) {
	var request publicapiv1.ContextRequestV02
	f, err := os.Open(path)
	if err != nil {
		return request, receiptValidationError("The context request file could not be opened.", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return request, receiptValidationError("The context request path must name one regular file.", err)
	}
	if info.Size() > maxContextRequestFileBytes {
		return request, receiptValidationError("The context request file is larger than the 64 KiB safety limit.", nil)
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxContextRequestFileBytes+1))
	if err != nil || len(raw) > maxContextRequestFileBytes {
		return request, receiptValidationError("The context request file could not be read within the 64 KiB safety limit.", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, receiptValidationError("The file is not valid Context Request v0.2 JSON.", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return request, receiptValidationError("The context request file must contain exactly one JSON object.", err)
	}
	if err := request.Validate(); err != nil {
		return request, receiptValidationError("The file does not satisfy the Context Request v0.2 contract.", err)
	}
	return request, nil
}
