package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/workhistory"
)

var historyCmd = newHistoryCommand()

// The Brain accepts at most 512 KiB of canonical fragment JSON. Keep the
// Multica request below that boundary too so mapping project_id server-side
// cannot turn a valid local scan into one oversized upstream write.
const maxHistoryImportRequestBytes = 480 * 1024

type historyImportOutput struct {
	Provider       string          `json:"provider"`
	ProjectID      string          `json:"project_id"`
	Source         string          `json:"source"`
	Fragments      int             `json:"fragments"`
	IgnoredRecords int             `json:"ignored_records"`
	Redactions     int             `json:"redactions"`
	Complete       bool            `json:"complete"`
	Checkpoint     int64           `json:"checkpoint_offset"`
	BrainReceipt   json.RawMessage `json:"brain_receipt,omitempty"`
}

type historyResolveOutput struct {
	Provider      string `json:"provider"`
	SessionID     string `json:"session_id"`
	Path          string `json:"path"`
	SourceOffset  int64  `json:"source_offset"`
	SourceOrdinal int64  `json:"source_ordinal"`
	Part          int    `json:"part"`
}

type historyBrainReceipt struct {
	SchemaVersion string `json:"schemaVersion"`
	ServedAt      string `json:"servedAt"`
	Recorded      struct {
		Submitted   int    `json:"submitted"`
		Inserted    int    `json:"inserted"`
		Unchanged   int    `json:"unchanged"`
		RecordClass string `json:"recordClass"`
	} `json:"recorded"`
}

func newHistoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Import and search private cross-agent work history",
		Long: "Import bounded, redacted conversation recollection from one exact local Codex or Claude session. " +
			"History remains separate from receipts, evidence, and accepted outcomes.",
	}
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import the next bounded slice of one exact session",
		Args:  cobra.NoArgs,
		RunE:  runHistoryImport,
	}
	importCmd.Flags().String("provider", "", "Session provider: codex or claude")
	importCmd.Flags().String("root", "", "Explicit allowed session root")
	importCmd.Flags().String("file", "", "Exact JSONL session file inside --root")
	importCmd.Flags().String("project-id", "", "Exact Multica project UUID")
	importCmd.Flags().Bool("dry-run", false, "Parse and redact without sending data or saving a checkpoint")
	importCmd.Flags().String("output", "text", "Output format: text or json")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search this workspace's conversation recollection",
		Args:  exactArgs(1),
		RunE:  runHistorySearch,
	}
	searchCmd.Flags().Int("limit", 20, "Maximum matching fragments (1 to 50)")
	searchCmd.Flags().String("output", "json", "Output format: text or json")

	resolveCmd := &cobra.Command{
		Use:   "resolve <source-reference>",
		Short: "Resolve a search result back to its private local session file",
		Args:  exactArgs(1),
		RunE:  runHistoryResolve,
	}
	resolveCmd.Flags().String("output", "text", "Output format: text or json")
	cmd.AddCommand(importCmd, searchCmd, resolveCmd)
	return cmd
}

func runHistoryImport(cmd *cobra.Command, _ []string) error {
	provider, _ := cmd.Flags().GetString("provider")
	root, _ := cmd.Flags().GetString("root")
	path, _ := cmd.Flags().GetString("file")
	projectID, _ := cmd.Flags().GetString("project-id")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	output, _ := cmd.Flags().GetString("output")
	provider = strings.TrimSpace(provider)
	root = strings.TrimSpace(root)
	path = strings.TrimSpace(path)
	projectID = strings.TrimSpace(projectID)
	if provider != workhistory.ProviderCodex && provider != workhistory.ProviderClaude {
		return fmt.Errorf("--provider must be codex or claude")
	}
	if root == "" || path == "" {
		return fmt.Errorf("--root and --file are required")
	}
	if _, err := uuid.Parse(projectID); err != nil {
		return fmt.Errorf("--project-id must be an exact Multica project UUID")
	}
	if output != "text" && output != "json" {
		return fmt.Errorf("--output must be text or json")
	}

	checkpointPath, err := historyCheckpointPath(resolveProfile(cmd), provider, projectID, root, path)
	if err != nil {
		return err
	}
	var checkpoint *workhistory.Checkpoint
	if !dryRun {
		checkpoint, err = loadHistoryCheckpoint(checkpointPath)
		if err != nil {
			return fmt.Errorf("load work history checkpoint: %w", err)
		}
	}
	result, err := workhistory.ScanFile(workhistory.ScanOptions{
		Provider: provider, Project: projectID, AllowedRoot: root, Path: path,
		Checkpoint: checkpoint, MaxFragments: 100,
	})
	if err != nil {
		return err
	}
	report := historyImportOutput{
		Provider: provider, ProjectID: projectID, Source: result.Checkpoint.SessionID,
		Fragments: len(result.Fragments), IgnoredRecords: result.IgnoredRecords,
		Redactions: result.Redactions, Complete: result.Complete, Checkpoint: result.Checkpoint.Offset,
	}
	if !dryRun && len(result.Fragments) != 0 {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := cli.APIContext(cmd.Context())
		defer cancel()
		batches, err := historyImportBatches(projectID, result.Fragments)
		if err != nil {
			return err
		}
		aggregate := historyBrainReceipt{}
		for _, batch := range batches {
			var receipt historyBrainReceipt
			if err := client.PostJSON(ctx, "/api/work-history/fragments", map[string]any{
				"project_id": projectID, "fragments": batch,
			}, &receipt); err != nil {
				return fmt.Errorf("import work history: %w", err)
			}
			if receipt.SchemaVersion != "silvermediums.brain.v2" || receipt.ServedAt == "" ||
				receipt.Recorded.RecordClass != "conversation_recollection" ||
				receipt.Recorded.Submitted != len(batch) || receipt.Recorded.Inserted < 0 || receipt.Recorded.Unchanged < 0 ||
				receipt.Recorded.Inserted+receipt.Recorded.Unchanged != receipt.Recorded.Submitted {
				return errors.New("import work history: invalid Brain receipt")
			}
			aggregate.SchemaVersion = receipt.SchemaVersion
			aggregate.ServedAt = receipt.ServedAt
			aggregate.Recorded.RecordClass = receipt.Recorded.RecordClass
			aggregate.Recorded.Submitted += receipt.Recorded.Submitted
			aggregate.Recorded.Inserted += receipt.Recorded.Inserted
			aggregate.Recorded.Unchanged += receipt.Recorded.Unchanged
		}
		report.BrainReceipt, err = json.Marshal(aggregate)
		if err != nil {
			return fmt.Errorf("prepare work history receipt: %w", err)
		}
	}
	if !dryRun {
		if err := saveHistoryCheckpoint(checkpointPath, result.Checkpoint); err != nil {
			return fmt.Errorf("save work history checkpoint: %w", err)
		}
	}
	return printHistoryImport(cmd, output, report, dryRun)
}

func historyImportBatches(projectID string, fragments []workhistory.Fragment) ([][]workhistory.Fragment, error) {
	var batches [][]workhistory.Fragment
	current := make([]workhistory.Fragment, 0, len(fragments))
	for _, fragment := range fragments {
		candidate := append(current, fragment)
		raw, err := json.Marshal(map[string]any{"project_id": projectID, "fragments": candidate})
		if err != nil {
			return nil, fmt.Errorf("prepare work history import: %w", err)
		}
		if len(raw) <= maxHistoryImportRequestBytes {
			current = candidate
			continue
		}
		if len(current) == 0 {
			return nil, errors.New("one work history fragment exceeds the import request limit")
		}
		batches = append(batches, current)
		current = []workhistory.Fragment{fragment}
		raw, err = json.Marshal(map[string]any{"project_id": projectID, "fragments": current})
		if err != nil {
			return nil, fmt.Errorf("prepare work history import: %w", err)
		}
		if len(raw) > maxHistoryImportRequestBytes {
			return nil, errors.New("one work history fragment exceeds the import request limit")
		}
	}
	if len(current) != 0 {
		batches = append(batches, current)
	}
	return batches, nil
}

func runHistorySearch(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	output, _ := cmd.Flags().GetString("output")
	query := strings.TrimSpace(args[0])
	if query == "" || len(query) > 2000 {
		return fmt.Errorf("query must be 1 to 2000 characters")
	}
	if limit < 1 || limit > 50 {
		return fmt.Errorf("--limit must be 1 to 50")
	}
	if output != "text" && output != "json" {
		return fmt.Errorf("--output must be text or json")
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	var response map[string]any
	if err := client.PostJSON(ctx, "/api/work-history/search", map[string]any{"query": query, "limit": limit}, &response); err != nil {
		return fmt.Errorf("search work history: %w", err)
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), response)
	}
	history, _ := response["history"].(map[string]any)
	groups, _ := history["groups"].([]any)
	if len(groups) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No conversation recollection matched.")
		return nil
	}
	for _, rawGroup := range groups {
		group, _ := rawGroup.(map[string]any)
		fmt.Fprintf(cmd.OutOrStdout(), "%v · %v\n", group["day"], group["project"])
		matches, _ := group["matches"].([]any)
		for _, rawMatch := range matches {
			match, _ := rawMatch.(map[string]any)
			fmt.Fprintf(cmd.OutOrStdout(), "  %v · %v · %v\n", match["provider"], match["role"], match["excerpt"])
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "These are unreviewed conversation recollections, not accepted outcomes.")
	return nil
}

func runHistoryResolve(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	if output != "text" && output != "json" {
		return fmt.Errorf("--output must be text or json")
	}
	location, err := workhistory.ParseSourceReference(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	resolved, err := resolveHistorySource(resolveProfile(cmd), location)
	if err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), resolved)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\nrecord %d at byte %d (part %d)\n", resolved.Path, resolved.SourceOrdinal, resolved.SourceOffset, resolved.Part)
	return nil
}

func resolveHistorySource(profile string, location workhistory.SourceLocation) (historyResolveOutput, error) {
	var output historyResolveOutput
	dir, err := cli.ProfileDir(profile)
	if err != nil {
		return output, err
	}
	entries, err := os.ReadDir(filepath.Join(dir, "work-history"))
	if errors.Is(err, os.ErrNotExist) {
		return output, errors.New("no local work history checkpoint matches this source")
	}
	if err != nil {
		return output, fmt.Errorf("read work history checkpoints: %w", err)
	}
	if len(entries) > 10000 {
		return output, errors.New("too many local work history checkpoints to resolve safely")
	}
	matchedPath := ""
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		checkpoint, checkpointErr := loadHistoryCheckpoint(filepath.Join(dir, "work-history", entry.Name()))
		if checkpointErr != nil || checkpoint == nil || checkpoint.Provider != location.Provider || checkpoint.SessionID != location.SessionID || location.Offset >= checkpoint.Offset {
			continue
		}
		canonicalPath, pathErr := filepath.EvalSymlinks(checkpoint.CanonicalPath)
		if pathErr != nil || !filepath.IsAbs(canonicalPath) {
			continue
		}
		info, pathErr := os.Stat(canonicalPath)
		if pathErr != nil || !info.Mode().IsRegular() || location.Offset >= info.Size() {
			continue
		}
		if matchedPath != "" && matchedPath != canonicalPath {
			return output, errors.New("source reference matches multiple local session files")
		}
		matchedPath = canonicalPath
	}
	if matchedPath == "" {
		return output, errors.New("no local work history checkpoint matches this source")
	}
	return historyResolveOutput{
		Provider: location.Provider, SessionID: location.SessionID, Path: matchedPath,
		SourceOffset: location.Offset, SourceOrdinal: location.Ordinal, Part: location.Part,
	}, nil
}

func historyCheckpointPath(profile, provider, projectID, root, path string) (string, error) {
	dir, err := cli.ProfileDir(profile)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(provider + "\n" + projectID + "\n" + root + "\n" + path))
	return filepath.Join(dir, "work-history", hex.EncodeToString(sum[:])+".json"), nil
}

func loadHistoryCheckpoint(path string) (*workhistory.Checkpoint, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || len(raw) > 64*1024 {
		return nil, errors.New("checkpoint unavailable or oversized")
	}
	var checkpoint workhistory.Checkpoint
	if json.Unmarshal(raw, &checkpoint) != nil {
		return nil, errors.New("checkpoint is invalid")
	}
	return &checkpoint, nil
}

func saveHistoryCheckpoint(path string, checkpoint workhistory.Checkpoint) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".history-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceHistoryCheckpoint(temporaryPath, path)
}

func printHistoryImport(cmd *cobra.Command, output string, report historyImportOutput, dryRun bool) error {
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), report)
	}
	mode := "Imported"
	if dryRun {
		mode = "Dry run found"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d redacted conversation fragments from %s.\n", mode, report.Fragments, report.Provider)
	fmt.Fprintf(cmd.OutOrStdout(), "Ignored %d non-visible records; applied %d redactions.\n", report.IgnoredRecords, report.Redactions)
	if report.Complete {
		fmt.Fprintln(cmd.OutOrStdout(), "This session is caught up.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "More complete records remain for the next bounded import.")
	}
	return nil
}
