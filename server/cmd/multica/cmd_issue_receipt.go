package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

const (
	maxOutcomeReceiptFileBytes = 1 << 20
	receiptSubmissionStateV1   = 1
	receiptTokenEnv            = "MULTICA_RECEIPT_TOKEN"
)

var outcomeReceiptNow = time.Now

type receiptSubmissionRecord struct {
	Version        int                                           `json:"version"`
	Server         string                                        `json:"server"`
	IssueRef       string                                        `json:"issue_ref"`
	IssueID        string                                        `json:"issue_id"`
	ReceiptSHA256  string                                        `json:"receipt_sha256"`
	IdempotencyKey string                                        `json:"idempotency_key"`
	IfMatch        string                                        `json:"if_match"`
	Status         string                                        `json:"status"`
	PreparedAt     string                                        `json:"prepared_at"`
	AcceptedAt     *string                                       `json:"accepted_at,omitempty"`
	Result         *publicapiv1.ExternalReceiptAdmissionResponse `json:"result,omitempty"`
}

type receiptImportOutput struct {
	ContractVersion string                                        `json:"contract_version"`
	Status          string                                        `json:"status"`
	IssueRef        string                                        `json:"issue_ref,omitempty"`
	ReceiptSHA256   string                                        `json:"receipt_sha256"`
	Sent            bool                                          `json:"sent"`
	Result          *publicapiv1.ExternalReceiptAdmissionResponse `json:"result,omitempty"`
}

func newIssueReceiptCommand() *cobra.Command {
	receiptCmd := &cobra.Command{
		Use:   "receipt",
		Short: "Validate and import external Outcome Receipts",
	}
	importCmd := &cobra.Command{
		Use:   "import <issue-ref>",
		Short: "Safely import one explicit Outcome Receipt v0.2 JSON file",
		Long: "Validate one explicitly named Outcome Receipt v0.2 JSON file and attach it to an existing issue. " +
			"Use --dry-run for offline validation. A real import reads the plugin credential only from " + receiptTokenEnv +
			" and stores a credential-free retry record before sending anything.",
		Args: exactArgs(1),
		RunE: runIssueReceiptImport,
	}
	importCmd.Flags().String("file", "", "Outcome Receipt v0.2 JSON file to import")
	importCmd.Flags().Bool("dry-run", false, "Validate locally without reading credentials, contacting Multica, or writing retry state")
	importCmd.Flags().String("output", "text", "Output format: text or json")
	importCmd.Flags().Bool("allow-external-file", false, "Allow --file to read outside the current working directory")
	receiptCmd.AddCommand(newIssueReceiptObserveCodexCommand())
	receiptCmd.AddCommand(newIssueReceiptObserveStreamCommand())
	receiptCmd.AddCommand(importCmd)
	return receiptCmd
}

func runIssueReceiptImport(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	if strings.TrimSpace(filePath) == "" {
		return receiptValidationError("Choose one receipt file with --file.", nil)
	}
	if err := ensureFileFlagWithinWorkdir(cmd, "file", "receipt", filePath); err != nil {
		return receiptValidationError("The receipt file is outside this work directory.", err)
	}
	receipt, digest, err := readOutcomeReceiptFile(filePath)
	if err != nil {
		return err
	}
	outputFormat, _ := cmd.Flags().GetString("output")
	if outputFormat != "text" && outputFormat != "json" {
		return receiptValidationError("--output must be text or json.", nil)
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	issueRef := strings.TrimSpace(args[0])
	if dryRun {
		return printReceiptImportOutput(cmd, outputFormat, receiptImportOutput{
			ContractVersion: publicapiv1.OutcomeReceiptContractVersion,
			Status:          "valid",
			IssueRef:        issueRef,
			ReceiptSHA256:   digest,
			Sent:            false,
		})
	}
	result, err := submitOutcomeReceipt(cmd, issueRef, receipt, digest)
	if err != nil {
		return err
	}
	return printReceiptImportOutput(cmd, outputFormat, result)
}

func submitOutcomeReceipt(cmd *cobra.Command, issueRef string, receipt publicapiv1.OutcomeReceiptV02, digest string) (receiptImportOutput, error) {
	token := strings.TrimSpace(os.Getenv(receiptTokenEnv))
	if !strings.HasPrefix(token, "mpi_") && !strings.HasPrefix(token, "mpc_") {
		return receiptImportOutput{}, cli.NewClassifiedError(cli.KindAuthRequired,
			"Set "+receiptTokenEnv+" to a Multica plugin installation or callback token; the receipt token is never accepted as a command-line argument.", nil)
	}
	serverURL := tryResolveServerURL(cmd)
	if serverURL == "" {
		return receiptImportOutput{}, receiptValidationError("No Multica server is configured. Set --server-url, MULTICA_SERVER_URL, or the profile server_url.", nil)
	}
	serverIdentity, err := safeReceiptServerIdentity(serverURL)
	if err != nil {
		return receiptImportOutput{}, receiptValidationError("The configured Multica server URL is invalid.", err)
	}

	recordPath, err := receiptSubmissionRecordPath(resolveProfile(cmd), serverIdentity, issueRef, digest)
	if err != nil {
		return receiptImportOutput{}, fmt.Errorf("prepare receipt retry state: %w", err)
	}
	record, err := loadReceiptSubmissionRecord(recordPath)
	if err != nil {
		return receiptImportOutput{}, receiptValidationError("The saved receipt retry record is invalid; no data was sent.", err)
	}
	if record != nil {
		if err := record.matches(serverIdentity, issueRef, digest); err != nil {
			return receiptImportOutput{}, receiptValidationError("The saved receipt retry record does not match this import; no data was sent.", err)
		}
		if record.Status == "accepted" && record.Result != nil {
			return receiptImportOutput{
				ContractVersion: publicapiv1.OutcomeReceiptContractVersion,
				Status:          "already_accepted",
				IssueRef:        issueRef,
				ReceiptSHA256:   digest,
				Sent:            false,
				Result:          record.Result,
			}, nil
		}
	}

	client := cli.NewAPIClient(serverURL, "", token)
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	if record == nil {
		issuePath := publicapiv1.BasePath + "/issues/" + url.PathEscape(issueRef)
		var issue publicapiv1.Issue
		headers, getErr := client.GetJSONWithHeaders(ctx, issuePath, &issue)
		if getErr != nil {
			return receiptImportOutput{}, fmt.Errorf("resolve receipt issue: %w", getErr)
		}
		etag := strings.TrimSpace(headers.Get("ETag"))
		if issue.ID == "" || issue.Revision < 1 || etag == "" {
			return receiptImportOutput{}, fmt.Errorf("resolve receipt issue: Multica returned no stable issue ID, revision, or ETag")
		}
		record = &receiptSubmissionRecord{
			Version:        receiptSubmissionStateV1,
			Server:         serverIdentity,
			IssueRef:       issueRef,
			IssueID:        issue.ID,
			ReceiptSHA256:  digest,
			IdempotencyKey: receiptIdempotencyKey(issue.ID, digest),
			IfMatch:        etag,
			Status:         "prepared",
			PreparedAt:     outcomeReceiptNow().UTC().Format(time.RFC3339Nano),
		}
		if err := saveReceiptSubmissionRecord(recordPath, record); err != nil {
			return receiptImportOutput{}, fmt.Errorf("save receipt retry state before submission: %w", err)
		}
	}

	requestHeaders := make(http.Header)
	requestHeaders.Set("If-Match", record.IfMatch)
	requestHeaders.Set(publicapiv1.HeaderIdempotencyKey, record.IdempotencyKey)
	postPath := publicapiv1.BasePath + "/issues/" + url.PathEscape(record.IssueID) + "/external-receipts"
	var result publicapiv1.ExternalReceiptAdmissionResponse
	if err := client.PostJSONWithHeaders(ctx, postPath, receipt, requestHeaders, &result); err != nil {
		return receiptImportOutput{}, fmt.Errorf("import outcome receipt: %w", err)
	}
	acceptedAt := outcomeReceiptNow().UTC().Format(time.RFC3339Nano)
	record.Status = "accepted"
	record.AcceptedAt = &acceptedAt
	record.Result = &result
	if err := saveReceiptSubmissionRecord(recordPath, record); err != nil {
		return receiptImportOutput{}, fmt.Errorf("receipt was accepted, but the local retry record could not be updated; rerun this exact command safely: %w", err)
	}
	return receiptImportOutput{
		ContractVersion: publicapiv1.OutcomeReceiptContractVersion,
		Status:          "accepted",
		IssueRef:        issueRef,
		ReceiptSHA256:   digest,
		Sent:            true,
		Result:          &result,
	}, nil
}

func outcomeReceiptDigest(receipt publicapiv1.OutcomeReceiptV02) (string, error) {
	canonical, err := json.Marshal(receipt)
	if err != nil {
		return "", fmt.Errorf("canonicalize outcome receipt: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func readOutcomeReceiptFile(path string) (publicapiv1.OutcomeReceiptV02, string, error) {
	var receipt publicapiv1.OutcomeReceiptV02
	f, err := os.Open(path)
	if err != nil {
		return receipt, "", receiptValidationError("The receipt file could not be opened.", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return receipt, "", receiptValidationError("The receipt file could not be inspected.", err)
	}
	if !info.Mode().IsRegular() {
		return receipt, "", receiptValidationError("The receipt path must name one regular file.", nil)
	}
	if info.Size() > maxOutcomeReceiptFileBytes {
		return receipt, "", receiptValidationError("The receipt file is larger than the 1 MiB safety limit.", nil)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxOutcomeReceiptFileBytes+1))
	if err != nil {
		return receipt, "", receiptValidationError("The receipt file could not be read.", err)
	}
	if len(data) > maxOutcomeReceiptFileBytes {
		return receipt, "", receiptValidationError("The receipt file is larger than the 1 MiB safety limit.", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return receipt, "", receiptValidationError("The receipt is not valid Outcome Receipt v0.2 JSON.", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return receipt, "", receiptValidationError("The receipt file must contain exactly one JSON object.", err)
	}
	if err := receipt.Validate(); err != nil {
		return receipt, "", receiptValidationError("The receipt does not satisfy the Outcome Receipt v0.2 contract.", err)
	}
	digest, err := outcomeReceiptDigest(receipt)
	return receipt, digest, err
}

func receiptValidationError(message string, err error) error {
	return cli.NewClassifiedError(cli.KindValidation, message, err)
}

func safeReceiptServerIdentity(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("server URL must use http or https and include a host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("server URL must not contain credentials")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func receiptSubmissionRecordPath(profile, server, issueRef, receiptDigest string) (string, error) {
	profileDir, err := cli.ProfileDir(profile)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(server + "\n" + issueRef + "\n" + receiptDigest))
	return filepath.Join(profileDir, "receipt-submissions", hex.EncodeToString(sum[:])+".json"), nil
}

func receiptIdempotencyKey(issueID, receiptDigest string) string {
	sum := sha256.Sum256([]byte(issueID + "\n" + receiptDigest))
	return "outcome-receipt-v02-" + hex.EncodeToString(sum[:])
}

func loadReceiptSubmissionRecord(path string) (*receiptSubmissionRecord, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		// A cross-platform atomic replacement first moves the prior record out
		// of the way. If the process stopped in that tiny window, the previous
		// prepared record is still the safe retry source.
		path += ".previous"
		info, err = os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 64*1024 {
		return nil, fmt.Errorf("retry record is not a small regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record receiptSubmissionRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("retry record must contain exactly one JSON object")
	}
	if record.Version != receiptSubmissionStateV1 || (record.Status != "prepared" && record.Status != "accepted") ||
		record.Server == "" || record.IssueRef == "" || record.IssueID == "" || record.ReceiptSHA256 == "" ||
		record.IdempotencyKey == "" || record.IfMatch == "" || record.PreparedAt == "" {
		return nil, fmt.Errorf("retry record is incomplete or unsupported")
	}
	if record.Status == "accepted" && record.Result == nil {
		return nil, fmt.Errorf("accepted retry record has no result")
	}
	return &record, nil
}

func (r *receiptSubmissionRecord) matches(server, issueRef, receiptDigest string) error {
	if r.Server != server || r.IssueRef != issueRef || r.ReceiptSHA256 != receiptDigest {
		return fmt.Errorf("server, issue, or receipt hash differs")
	}
	if r.IdempotencyKey != receiptIdempotencyKey(r.IssueID, receiptDigest) {
		return fmt.Errorf("idempotency key does not match the saved issue and receipt")
	}
	return nil
}

func saveReceiptSubmissionRecord(path string, record *receiptSubmissionRecord) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".receipt-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	previousPath := path + ".previous"
	if _, err := os.Lstat(path); err == nil {
		_ = os.Remove(previousPath)
		if err := os.Rename(path, previousPath); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, path); err != nil {
			_ = os.Rename(previousPath, path)
			return err
		}
		_ = os.Remove(previousPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	_ = os.Remove(previousPath)
	return nil
}

func printReceiptImportOutput(cmd *cobra.Command, format string, result receiptImportOutput) error {
	if format == "json" {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	switch result.Status {
	case "valid":
		fmt.Fprintf(cmd.OutOrStdout(), "Receipt is valid Outcome Receipt v%s. No data was sent.\nSHA-256: %s\n", result.ContractVersion, result.ReceiptSHA256)
	case "already_accepted":
		fmt.Fprintf(cmd.OutOrStdout(), "This receipt was already accepted. Nothing was sent again.\nReceipt ID: %s\n", result.Result.ReceiptID)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Receipt accepted for %s.\nReceipt ID: %s\n", result.IssueRef, result.Result.ReceiptID)
	}
	return nil
}
