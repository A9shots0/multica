package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

const (
	receiptStreamStateVersion = 1
	maxReceiptStreamLineBytes = maxOutcomeReceiptFileBytes
)

type receiptStreamState struct {
	Version      int    `json:"version"`
	Server       string `json:"server"`
	IssueRef     string `json:"issue_ref"`
	StreamPath   string `json:"stream_path"`
	Provider     string `json:"provider"`
	WorkID       string `json:"work_id"`
	Offset       int64  `json:"offset"`
	LastRevision int64  `json:"last_revision"`
}

type receiptStreamItem struct {
	Receipt publicapiv1.OutcomeReceiptV02
	Digest  string
	Offset  int64
}

type receiptStreamOutput struct {
	Status    string                         `json:"status"`
	IssueRef  string                         `json:"issue_ref"`
	Provider  string                         `json:"provider"`
	WorkID    string                         `json:"work_id"`
	Processed int                            `json:"processed"`
	Receipt   *publicapiv1.OutcomeReceiptV02 `json:"receipt,omitempty"`
	Import    *receiptImportOutput           `json:"import,omitempty"`
}

func newIssueReceiptObserveStreamCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observe-stream <issue-ref>",
		Short: "Observe one explicitly linked structured receipt stream",
		Long: "Read only one named newline-delimited Outcome Receipt v0.2 stream for one exact provider and work ID. " +
			"This provider-neutral bridge never discovers conversations, reads chat prose, or controls the outside worker.",
		Args: exactArgs(1),
		RunE: runIssueReceiptObserveStream,
	}
	cmd.Flags().String("stream-file", "", "Exact newline-delimited Outcome Receipt v0.2 file to observe")
	cmd.Flags().String("provider", "", "Exact linked provider: codex, claude, chatgpt, copilot, evo, or other")
	cmd.Flags().String("work-id", "", "Exact stable outside work ID")
	cmd.Flags().Bool("follow", false, "Keep watching the same file for complete new receipt lines")
	cmd.Flags().Duration("poll-interval", 5*time.Second, "How often --follow checks the linked file")
	cmd.Flags().Bool("dry-run", false, "Validate and preview without credentials, network access, or saved state")
	cmd.Flags().String("output", "text", "Output format: text or json")
	cmd.Flags().Bool("allow-external-file", false, "Allow --stream-file to read outside the current working directory")
	return cmd
}

func runIssueReceiptObserveStream(cmd *cobra.Command, args []string) error {
	streamPath, _ := cmd.Flags().GetString("stream-file")
	provider, _ := cmd.Flags().GetString("provider")
	workID, _ := cmd.Flags().GetString("work-id")
	outputFormat, _ := cmd.Flags().GetString("output")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	follow, _ := cmd.Flags().GetBool("follow")
	pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
	issueRef := strings.TrimSpace(args[0])
	streamPath, provider, workID = strings.TrimSpace(streamPath), strings.TrimSpace(provider), strings.TrimSpace(workID)

	if streamPath == "" || provider == "" || workID == "" {
		return receiptValidationError("Choose one exact stream, provider, and work ID.", nil)
	}
	if outputFormat != "text" && outputFormat != "json" {
		return receiptValidationError("--output must be text or json.", nil)
	}
	if follow && dryRun {
		return receiptValidationError("--dry-run is one-shot; remove --follow.", nil)
	}
	if pollInterval < time.Second {
		return receiptValidationError("--poll-interval must be at least 1s.", nil)
	}
	if err := ensureFileFlagWithinWorkdir(cmd, "stream-file", "receipt-stream", streamPath); err != nil {
		return receiptValidationError("The receipt stream is outside this work directory.", err)
	}
	canonicalPath, err := filepath.EvalSymlinks(streamPath)
	if err != nil {
		return receiptValidationError("The receipt stream could not be resolved.", err)
	}
	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return receiptValidationError("The receipt stream could not be resolved.", err)
	}
	if err := validateReceiptStreamFile(canonicalPath); err != nil {
		return err
	}

	serverIdentity := "offline"
	if !dryRun {
		serverURL := tryResolveServerURL(cmd)
		if serverURL == "" {
			return receiptValidationError("No Multica server is configured. Set --server-url, MULTICA_SERVER_URL, or the profile server_url.", nil)
		}
		serverIdentity, err = safeReceiptServerIdentity(serverURL)
		if err != nil {
			return receiptValidationError("The configured Multica server URL is invalid.", err)
		}
	}

	statePath, err := receiptStreamStatePath(resolveProfile(cmd), serverIdentity, issueRef, canonicalPath, provider, workID)
	if err != nil {
		return fmt.Errorf("prepare receipt stream state: %w", err)
	}
	var state *receiptStreamState
	if !dryRun {
		state, err = loadReceiptStreamState(statePath)
		if err != nil {
			return receiptValidationError("The saved receipt stream state is invalid; nothing was sent.", err)
		}
		if state != nil {
			if err := state.matches(serverIdentity, issueRef, canonicalPath, provider, workID); err != nil {
				return receiptValidationError("The saved receipt stream state does not match this link; nothing was sent.", err)
			}
		}
	}

	for {
		processed, output, err := observeReceiptStreamOnce(cmd, statePath, issueRef, canonicalPath, provider, workID, serverIdentity, dryRun, &state)
		if err != nil {
			return err
		}
		if !follow || dryRun {
			return printReceiptStreamOutput(cmd, outputFormat, output)
		}
		if processed > 0 {
			if err := printReceiptStreamOutput(cmd, outputFormat, output); err != nil {
				return err
			}
		}
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-time.After(pollInterval):
		}
	}
}

func observeReceiptStreamOnce(cmd *cobra.Command, statePath, issueRef, streamPath, provider, workID, serverIdentity string, dryRun bool, state **receiptStreamState) (int, receiptStreamOutput, error) {
	offset, lastRevision := int64(0), int64(0)
	if *state != nil {
		offset, lastRevision = (*state).Offset, (*state).LastRevision
	}
	items, err := scanReceiptStream(streamPath, offset, provider, workID, lastRevision)
	if err != nil {
		return 0, receiptStreamOutput{}, err
	}
	base := receiptStreamOutput{Status: "unchanged", IssueRef: issueRef, Provider: provider, WorkID: workID}
	if len(items) == 0 {
		return 0, base, nil
	}
	if dryRun {
		latest := items[len(items)-1].Receipt
		base.Status, base.Processed, base.Receipt = "preview", len(items), &latest
		return len(items), base, nil
	}
	if *state == nil {
		*state = &receiptStreamState{
			Version: receiptStreamStateVersion, Server: serverIdentity, IssueRef: issueRef,
			StreamPath: streamPath, Provider: provider, WorkID: workID,
		}
	}
	var latestReceipt publicapiv1.OutcomeReceiptV02
	var latestImport receiptImportOutput
	for _, item := range items {
		result, err := submitOutcomeReceipt(cmd, issueRef, item.Receipt, item.Digest)
		if err != nil {
			return base.Processed, base, err
		}
		(*state).Offset, (*state).LastRevision = item.Offset, item.Receipt.ReceiptRevision
		if err := saveReceiptStreamState(statePath, *state); err != nil {
			return base.Processed, base, fmt.Errorf("receipt was accepted, but the stream checkpoint could not be saved; rerun safely: %w", err)
		}
		latestReceipt, latestImport = item.Receipt, result
		base.Processed++
	}
	base.Status, base.Receipt, base.Import = "recorded", &latestReceipt, &latestImport
	return base.Processed, base, nil
}

func validateReceiptStreamFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return receiptValidationError("The receipt stream could not be opened.", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return receiptValidationError("The receipt stream must be a regular file.", nil)
	}
	return nil
}

func scanReceiptStream(path string, offset int64, provider, workID string, lastRevision int64) ([]receiptStreamItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, receiptValidationError("The receipt stream could not be opened.", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if offset < 0 || offset > info.Size() {
		return nil, receiptValidationError("The linked receipt stream was truncated or replaced; relink it explicitly.", nil)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	reader := bufio.NewReaderSize(f, 64*1024)
	items := make([]receiptStreamItem, 0)
	nextRevision := lastRevision + 1
	currentOffset := offset
	for {
		line, readErr := reader.ReadBytes('\n')
		if errors.Is(readErr, io.EOF) {
			break // Ignore a trailing partial line until its producer commits '\n'.
		}
		if readErr != nil {
			return nil, readErr
		}
		currentOffset += int64(len(line))
		if len(line) > maxReceiptStreamLineBytes {
			return nil, receiptValidationError("A receipt stream line exceeds 1 MiB.", nil)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			return nil, receiptValidationError("The receipt stream contains a blank committed line.", nil)
		}
		var receipt publicapiv1.OutcomeReceiptV02
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&receipt); err != nil {
			return nil, receiptValidationError("A committed stream line is not valid Outcome Receipt v0.2 JSON.", err)
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return nil, receiptValidationError("A committed stream line must contain exactly one JSON object.", err)
		}
		if err := receipt.Validate(); err != nil {
			return nil, receiptValidationError("A committed stream receipt does not satisfy Outcome Receipt v0.2.", err)
		}
		if receipt.Provider != provider || receipt.WorkID != workID {
			return nil, receiptValidationError("A receipt crossed the exact provider or work-ID link.", nil)
		}
		if receipt.ReceiptRevision != nextRevision {
			return nil, receiptValidationError(fmt.Sprintf("Receipt revision %d was found; expected %d.", receipt.ReceiptRevision, nextRevision), nil)
		}
		digest, err := outcomeReceiptDigest(receipt)
		if err != nil {
			return nil, err
		}
		items = append(items, receiptStreamItem{Receipt: receipt, Digest: digest, Offset: currentOffset})
		nextRevision++
	}
	return items, nil
}

func receiptStreamStatePath(profile, server, issueRef, streamPath, provider, workID string) (string, error) {
	profileDir, err := cli.ProfileDir(profile)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{server, issueRef, streamPath, provider, workID}, "\n")))
	return filepath.Join(profileDir, "receipt-streams", hex.EncodeToString(sum[:])+".json"), nil
}

func loadReceiptStreamState(path string) (*receiptStreamState, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
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
		return nil, fmt.Errorf("stream checkpoint is not a small regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state receiptStreamState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("stream checkpoint must contain exactly one JSON object")
	}
	if state.Version != receiptStreamStateVersion || state.Server == "" || state.IssueRef == "" || state.StreamPath == "" || state.Provider == "" || state.WorkID == "" || state.Offset < 0 || state.LastRevision < 0 {
		return nil, fmt.Errorf("stream checkpoint is incomplete or unsupported")
	}
	return &state, nil
}

func (s *receiptStreamState) matches(server, issueRef, streamPath, provider, workID string) error {
	if s.Server != server || s.IssueRef != issueRef || s.StreamPath != streamPath || s.Provider != provider || s.WorkID != workID {
		return fmt.Errorf("server, issue, stream, provider, or work ID differs")
	}
	return nil
}

func saveReceiptStreamState(path string, state *receiptStreamState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".receipt-stream-*.tmp")
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

func printReceiptStreamOutput(cmd *cobra.Command, format string, output receiptStreamOutput) error {
	if format == "json" {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}
	switch output.Status {
	case "preview":
		fmt.Fprintf(cmd.OutOrStdout(), "Validated %d receipt(s) for %s. No data was sent.\nLatest milestone: %s\n", output.Processed, output.IssueRef, output.Receipt.CurrentMilestone)
	case "unchanged":
		fmt.Fprintf(cmd.OutOrStdout(), "No complete new receipt for %s. Nothing was sent.\n", output.IssueRef)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Recorded %d outside-work update(s) for %s.\nLatest milestone: %s\n", output.Processed, output.IssueRef, output.Receipt.CurrentMilestone)
	}
	return nil
}
