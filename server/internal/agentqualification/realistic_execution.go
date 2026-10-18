package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const RealisticExecutionReceiptSchemaVersion = "silver-mediums.realistic-execution-receipt.v1"

type RealisticToolEvent struct {
	Sequence     int      `json:"sequence"`
	Tool         string   `json:"tool"`
	Arguments    []string `json:"arguments"`
	ExitCode     int      `json:"exit_code"`
	StartedAt    string   `json:"started_at"`
	CompletedAt  string   `json:"completed_at"`
	OutputSHA256 string   `json:"output_sha256"`
	TouchedPaths []string `json:"touched_paths,omitempty"`
}

type RealisticEvidenceReceipt struct {
	EvidenceID   string `json:"evidence_id"`
	Kind         string `json:"kind"`
	ArtifactPath string `json:"artifact_path"`
	SHA256       string `json:"sha256"`
}

type RealisticExecutionReceipt struct {
	SchemaVersion                 string                       `json:"schema_version"`
	GroupID                       string                       `json:"group_id"`
	CaseID                        string                       `json:"case_id"`
	CaseVersion                   string                       `json:"case_version"`
	ConfigurationID               string                       `json:"configuration_id"`
	RunFamilyID                   string                       `json:"run_family_id"`
	Attempt                       int                          `json:"attempt"`
	TerminalState                 string                       `json:"terminal_state"`
	StartingStateSHA256           string                       `json:"starting_state_sha256"`
	FinalStateSHA256              string                       `json:"final_state_sha256"`
	WorkspaceBoundary             string                       `json:"workspace_boundary"`
	NetworkBoundary               string                       `json:"network_boundary"`
	ToolEvents                    []RealisticToolEvent         `json:"tool_events"`
	BrowserProofs                 []RealisticBrowserProof      `json:"browser_proofs,omitempty"`
	AttemptHistory                []RealisticAttemptCheckpoint `json:"attempt_history,omitempty"`
	Evidence                      []RealisticEvidenceReceipt   `json:"evidence"`
	OwnerInterventions            int                          `json:"owner_interventions"`
	Retries                       int                          `json:"retries"`
	ExternalSideEffects           []string                     `json:"external_side_effects"`
	ProhibitedEvents              []string                     `json:"prohibited_events"`
	ProviderReceiptSHA256         string                       `json:"provider_receipt_sha256"`
	ProviderReceiptArtifactPath   string                       `json:"provider_receipt_artifact_path"`
	ProviderReceiptHashes         []string                     `json:"provider_receipt_hashes"`
	ProviderCalls                 int                          `json:"provider_calls"`
	InputTokens                   int64                        `json:"input_tokens"`
	OutputTokens                  int64                        `json:"output_tokens"`
	ReasoningTokens               int64                        `json:"reasoning_tokens"`
	CostMicros                    int64                        `json:"cost_micros"`
	ProviderAccountingStatus      string                       `json:"provider_accounting_status"`
	ProviderTokenAccountingStatus string                       `json:"provider_token_accounting_status"`
	ProviderCostAccountingStatus  string                       `json:"provider_cost_accounting_status"`
	ControllerResultSHA256        string                       `json:"controller_result_sha256"`
	ControllerResultArtifactPath  string                       `json:"controller_result_artifact_path"`
	FixtureTreeSHA256             string                       `json:"fixture_tree_sha256"`
	ManifestSHA256                string                       `json:"manifest_sha256"`
}

func ValidateRealisticExecutionReceipt(receipt RealisticExecutionReceipt, group RealisticWorkflowGroup) []string {
	reasons := []string{}
	add := func(reason string) { reasons = append(reasons, reason) }
	if receipt.SchemaVersion != RealisticExecutionReceiptSchemaVersion || receipt.GroupID != group.GroupID {
		add("receipt_identity_invalid")
	}
	caseFound := false
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID == receipt.CaseID && candidateCase.CaseVersion == receipt.CaseVersion {
			caseFound = true
			break
		}
	}
	if !caseFound {
		add("case_not_in_group")
	}
	for name, value := range map[string]string{
		"configuration_id":   receipt.ConfigurationID,
		"run_family_id":      receipt.RunFamilyID,
		"workspace_boundary": receipt.WorkspaceBoundary,
		"network_boundary":   receipt.NetworkBoundary,
	} {
		if strings.TrimSpace(value) == "" {
			add(name + "_missing")
		}
	}
	if receipt.Attempt < 1 || !contains([]string{"completed", "failed", "interrupted", "invalid"}, receipt.TerminalState) {
		add("attempt_or_terminal_state_invalid")
	}
	if !validSHA256(receipt.StartingStateSHA256) || !validSHA256(receipt.FinalStateSHA256) || !validSHA256(receipt.ProviderReceiptSHA256) || !safeFixtureRelativePath(receipt.ProviderReceiptArtifactPath) || !validSHA256(receipt.FixtureTreeSHA256) || !validSHA256(receipt.ManifestSHA256) {
		add("state_or_provider_hash_invalid")
	}
	if receipt.ProviderCalls < 1 || len(receipt.ProviderReceiptHashes) != receipt.ProviderCalls || receipt.InputTokens < 0 || receipt.OutputTokens < 0 || receipt.ReasoningTokens < 0 || receipt.CostMicros < 0 || receipt.ProviderAccountingStatus != "exact" || receipt.ProviderTokenAccountingStatus != "exact" || receipt.ProviderCostAccountingStatus != "exact" || !validSHA256(receipt.ControllerResultSHA256) || !safeFixtureRelativePath(receipt.ControllerResultArtifactPath) {
		add("provider_accounting_binding_invalid")
	}
	wantNetwork := "provider_api_only"
	if receipt.CaseID == "R02" {
		wantNetwork = "provider_api_and_case_localhost_only"
	}
	if receipt.WorkspaceBoundary != "fresh_disposable_case_workspace" || receipt.NetworkBoundary != wantNetwork {
		add("execution_boundary_invalid")
	}
	if receipt.OwnerInterventions != 0 || receipt.Retries != 0 {
		add("owner_intervention_or_retry_present")
	}
	if len(receipt.ExternalSideEffects) != 0 || len(receipt.ProhibitedEvents) != 0 {
		add("forbidden_side_effect_or_event")
	}
	seenSequences := map[int]struct{}{}
	allowedTools := stringSet("read", "search", "edit_within_workspace", "run_allowlisted_local_checks", "browser_on_local_preview_when_case_requires")
	for _, event := range receipt.ToolEvents {
		if event.Sequence < 1 {
			add("tool_sequence_invalid")
		}
		if _, duplicate := seenSequences[event.Sequence]; duplicate {
			add("tool_sequence_duplicate")
		}
		seenSequences[event.Sequence] = struct{}{}
		if _, ok := allowedTools[event.Tool]; !ok {
			add("tool_not_allowlisted:" + event.Tool)
		}
		if !validSHA256(event.OutputSHA256) || strings.TrimSpace(event.StartedAt) == "" || strings.TrimSpace(event.CompletedAt) == "" {
			add("tool_evidence_incomplete")
		}
		for _, path := range event.TouchedPaths {
			if strings.Contains(path, "..") || strings.Contains(path, ":") || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
				add("tool_path_outside_relative_workspace")
			}
		}
	}
	seenEvidence := map[string]struct{}{}
	for _, evidence := range receipt.Evidence {
		if strings.TrimSpace(evidence.EvidenceID) == "" || strings.TrimSpace(evidence.Kind) == "" || !safeFixtureRelativePath(evidence.ArtifactPath) || !validSHA256(evidence.SHA256) {
			add("evidence_invalid")
		}
		if _, duplicate := seenEvidence[evidence.EvidenceID]; duplicate {
			add("evidence_duplicate")
		}
		seenEvidence[evidence.EvidenceID] = struct{}{}
	}
	return unique(reasons)
}

// MissingRequiredRealisticEvidence reports semantic evidence omissions without
// treating the immutable execution receipt itself as corrupt. A candidate can
// complete honestly yet fail to produce every required proof; that is a blind
// grading failure, while malformed hashes, unsafe paths, and tampering remain
// receipt-integrity failures in ValidateRealisticExecutionReceipt.
func MissingRequiredRealisticEvidence(receipt RealisticExecutionReceipt, group RealisticWorkflowGroup) []string {
	seenEvidence := map[string]struct{}{}
	for _, evidence := range receipt.Evidence {
		seenEvidence[evidence.EvidenceID] = struct{}{}
	}
	reasons := []string{}
	for _, candidateCase := range group.Cases {
		if candidateCase.CaseID != receipt.CaseID || candidateCase.CaseVersion != receipt.CaseVersion {
			continue
		}
		for _, required := range candidateCase.RequiredEvidence {
			if _, ok := seenEvidence[required]; !ok {
				reasons = append(reasons, "required_evidence_missing:"+required)
			}
		}
	}
	return unique(reasons)
}

// VerifyRealisticExecutionReceipt recomputes every filesystem binding instead
// of trusting the candidate's receipt. evidenceRoot is harness-owned and is
// never an editable candidate path.
func VerifyRealisticExecutionReceipt(receipt RealisticExecutionReceipt, group RealisticWorkflowGroup, manifest RealisticFixtureManifest, initialRoot, finalRoot, evidenceRoot string) []string {
	reasons := ValidateRealisticExecutionReceipt(receipt, group)
	add := func(reason string) { reasons = append(reasons, reason) }
	if manifest.GroupID != group.GroupID || manifest.Status != group.Status {
		add("group_manifest_status_or_identity_mismatch")
	}
	manifestHash := sha256.Sum256(realisticFixtureManifestV1JSON)
	if receipt.ManifestSHA256 != hex.EncodeToString(manifestHash[:]) {
		add("manifest_hash_mismatch")
	}
	var fixture RealisticFixture
	for _, candidate := range manifest.Fixtures {
		if candidate.CaseID == receipt.CaseID {
			fixture = candidate
			break
		}
	}
	if fixture.CaseID == "" || receipt.FixtureTreeSHA256 != fixture.TreeSHA256 {
		add("fixture_or_starting_hash_mismatch")
	}
	initialHash, initialErr := RealisticFixtureTreeSHA256(os.DirFS(initialRoot), ".")
	finalHash, finalErr := RealisticFixtureTreeSHA256(os.DirFS(finalRoot), ".")
	if initialErr != nil || initialHash != receipt.StartingStateSHA256 {
		add("initial_tree_recompute_failed")
	}
	if finalErr != nil || finalHash != receipt.FinalStateSHA256 {
		add("final_tree_recompute_failed")
	}
	changed, err := realisticChangedPaths(initialRoot, finalRoot)
	if err != nil {
		add("changed_path_recompute_failed")
	} else {
		for _, path := range changed {
			if !contains(fixture.EditablePaths, path) && !realisticHarnessGeneratedPath(receipt.CaseID, receipt.ToolEvents, path) {
				add("changed_path_not_allowlisted:" + path)
			}
		}
	}
	for _, event := range receipt.ToolEvents {
		for _, path := range event.TouchedPaths {
			if !contains(fixture.EditablePaths, filepath.ToSlash(path)) && !realisticHarnessGeneratedPath(receipt.CaseID, receipt.ToolEvents, filepath.ToSlash(path)) {
				add("tool_touched_path_not_allowlisted:" + filepath.ToSlash(path))
			}
		}
		// Rejected check requests are immutable evidence of candidate behavior,
		// not evidence that an unapproved command executed. The runner records
		// them with a non-zero exit code. Only an executed check must match the
		// fixture allowlist; otherwise a safely rejected request invalidates the
		// entire observation instead of remaining available to the grader.
		if event.Tool == "run_allowlisted_local_checks" && event.ExitCode == 0 && !contains(fixture.AllowlistedChecks, strings.Join(event.Arguments, " ")) {
			add("tool_check_not_allowlisted")
		}
		if event.Tool == "browser_on_local_preview_when_case_requires" && !fixture.BrowserRequired {
			add("browser_tool_used_for_non_browser_case")
		}
	}
	for _, evidence := range receipt.Evidence {
		path := filepath.Join(evidenceRoot, filepath.FromSlash(evidence.ArtifactPath))
		rootAbs, _ := filepath.Abs(evidenceRoot)
		pathAbs, _ := filepath.Abs(path)
		if !strings.HasPrefix(strings.ToLower(pathAbs), strings.ToLower(rootAbs)+string(os.PathSeparator)) {
			add("evidence_path_escape")
			continue
		}
		raw, err := os.ReadFile(pathAbs)
		if err != nil {
			add("evidence_artifact_missing:" + evidence.EvidenceID)
			continue
		}
		hash := sha256.Sum256(raw)
		if evidence.SHA256 != hex.EncodeToString(hash[:]) {
			add("evidence_hash_mismatch:" + evidence.EvidenceID)
		}
		var sealed realisticEvidenceIndex
		if err := json.Unmarshal(raw, &sealed); err != nil {
			add("evidence_index_invalid:" + evidence.EvidenceID)
			continue
		}
		expected, supported, err := deriveRealisticEvidenceIndex(receipt.CaseID, evidence.EvidenceID, receipt.RunFamilyID, receipt.FinalStateSHA256, initialRoot, finalRoot, receipt.ToolEvents, receipt.BrowserProofs, receipt.AttemptHistory)
		if err != nil {
			add("evidence_runtime_verification_failed:" + evidence.EvidenceID)
			continue
		}
		if !supported {
			add("evidence_not_supported:" + evidence.EvidenceID)
			continue
		}
		if !reflect.DeepEqual(sealed, expected) {
			add("evidence_index_mismatch:" + evidence.EvidenceID)
		}
	}
	providerReceiptPath := filepath.Join(evidenceRoot, filepath.FromSlash(receipt.ProviderReceiptArtifactPath))
	providerRaw, providerErr := os.ReadFile(providerReceiptPath)
	if providerErr != nil {
		add("provider_receipt_artifact_missing")
	} else {
		providerHash := sha256.Sum256(providerRaw)
		if receipt.ProviderReceiptSHA256 != hex.EncodeToString(providerHash[:]) {
			add("provider_receipt_hash_mismatch")
		}
		var providerMessages []sealedRealisticProviderReceipt
		if json.Unmarshal(providerRaw, &providerMessages) != nil {
			add("provider_receipt_artifact_invalid")
		} else {
			rawReceipts := make([][]byte, 0, len(providerMessages))
			for _, message := range providerMessages {
				hash := sha256.Sum256(message.Raw)
				if message.SHA256 != hex.EncodeToString(hash[:]) {
					add("provider_receipt_entry_hash_mismatch")
				}
				rawReceipts = append(rawReceipts, append([]byte(nil), message.Raw...))
			}
			accounting, accountingErr := recomputeRealisticProviderAccounting(rawReceipts)
			if accountingErr != nil || !realisticReceiptAccountingMatches(receipt, accounting) {
				add("provider_accounting_recompute_mismatch")
			}
		}
	}
	controllerPath := filepath.Join(evidenceRoot, filepath.FromSlash(receipt.ControllerResultArtifactPath))
	controllerRaw, controllerErr := os.ReadFile(controllerPath)
	if controllerErr != nil {
		add("controller_result_artifact_missing")
	} else {
		controllerHash := sha256.Sum256(controllerRaw)
		if receipt.ControllerResultSHA256 != hex.EncodeToString(controllerHash[:]) {
			add("controller_result_hash_mismatch")
		}
		var controller RealisticControllerResult
		if json.Unmarshal(controllerRaw, &controller) != nil || controller.ProviderCalls != receipt.ProviderCalls || controller.InputTokens != receipt.InputTokens || controller.OutputTokens != receipt.OutputTokens || controller.ReasoningTokens != receipt.ReasoningTokens || controller.CostMicros != receipt.CostMicros || !equalStrings(controller.ProviderReceiptHashes, receipt.ProviderReceiptHashes) || controller.ProviderAccountingStatus != receipt.ProviderAccountingStatus || controller.ProviderTokenAccountingStatus != receipt.ProviderTokenAccountingStatus || controller.ProviderCostAccountingStatus != receipt.ProviderCostAccountingStatus {
			add("controller_result_accounting_mismatch")
		} else if controllerAccounting, err := recomputeRealisticProviderAccounting(controller.ProviderReceipts); err != nil || !realisticReceiptAccountingMatches(receipt, controllerAccounting) {
			add("controller_provider_receipts_mismatch")
		}
	}
	return unique(reasons)
}

func realisticHarnessGeneratedPath(caseID string, events []RealisticToolEvent, path string) bool {
	if caseID != "R04" || !contains([]string{"state/resume-action.json", "state/step_counts.json", "FINAL-OUTPUT.json"}, filepath.ToSlash(path)) {
		return false
	}
	for _, event := range events {
		if event.Tool == "run_allowlisted_local_checks" && event.ExitCode == 0 && strings.Join(event.Arguments, " ") == "python resume.py" {
			return true
		}
	}
	return false
}

func realisticChangedPaths(initialRoot, finalRoot string) ([]string, error) {
	initial, err := realisticFileHashes(initialRoot)
	if err != nil {
		return nil, err
	}
	final, err := realisticFileHashes(finalRoot)
	if err != nil {
		return nil, err
	}
	paths := map[string]struct{}{}
	for path := range initial {
		paths[path] = struct{}{}
	}
	for path := range final {
		paths[path] = struct{}{}
	}
	changed := []string{}
	for path := range paths {
		if initial[path] != final[path] {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

func realisticFileHashes(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return fs.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) == ".pyc" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		relative, _ := filepath.Rel(root, path)
		result[filepath.ToSlash(relative)] = hex.EncodeToString(hash[:])
		return nil
	})
	return result, err
}

func RequireRealisticWorkflowFrozen(group RealisticWorkflowGroup) error {
	if err := ValidateRealisticWorkflowGroup(group); err != nil {
		return err
	}
	return RequireCompleteRealisticFreeze()
}
