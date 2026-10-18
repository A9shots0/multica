package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type realisticArtifactBinding struct {
	Root   string `json:"root"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// deriveRealisticEvidenceIndex creates an evidence receipt only when a
// case-specific runtime artifact proves the named claim. Evidence IDs are
// never accepted merely because the workflow group declares them.
func deriveRealisticEvidenceIndex(caseID, evidenceID, runFamilyID, finalStateSHA256, initialRoot, finalRoot string, toolEvents []RealisticToolEvent, browserProofs []RealisticBrowserProof, attempts []RealisticAttemptCheckpoint) (realisticEvidenceIndex, bool, error) {
	index := realisticEvidenceIndex{EvidenceID: evidenceID, CaseID: caseID, RunFamilyID: runFamilyID, FinalStateSHA256: finalStateSHA256}
	bind := func(rootName, root, path string) (bool, error) {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		hash := sha256.Sum256(raw)
		index.ArtifactBindings = append(index.ArtifactBindings, realisticArtifactBinding{Root: rootName, Path: path, SHA256: hex.EncodeToString(hash[:])})
		return true, nil
	}
	bindFinal := func(path string) (bool, error) { return bind("final", finalRoot, path) }
	unchanged := func(path string) (bool, error) {
		initial, err := os.ReadFile(filepath.Join(initialRoot, filepath.FromSlash(path)))
		if err != nil {
			return false, err
		}
		final, err := os.ReadFile(filepath.Join(finalRoot, filepath.FromSlash(path)))
		if err != nil {
			return false, err
		}
		if string(initial) != string(final) {
			return false, nil
		}
		_, _ = bind("initial", initialRoot, path)
		_, _ = bind("final", finalRoot, path)
		return true, nil
	}
	checkEvents := func(exitCode int, command string) []int {
		sequences := []int{}
		for _, event := range toolEvents {
			if event.Tool == "run_allowlisted_local_checks" && event.ExitCode == exitCode && strings.Join(event.Arguments, " ") == command {
				sequences = append(sequences, event.Sequence)
			}
		}
		return sequences
	}
	bindCheck := func(exitCode int, command string) bool {
		sequences := checkEvents(exitCode, command)
		if len(sequences) == 0 {
			return false
		}
		index.ToolEventSequences = append(index.ToolEventSequences, sequences...)
		return true
	}
	bindJSON := func(path string, valid func(map[string]any) bool) (bool, error) {
		raw, err := os.ReadFile(filepath.Join(finalRoot, filepath.FromSlash(path)))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var decoded map[string]any
		if json.Unmarshal(raw, &decoded) != nil || !valid(decoded) {
			return false, nil
		}
		return bindFinal(path)
	}

	const pyCheck = "python -m unittest discover -s tests -v"
	supported := false
	switch caseID + ":" + evidenceID {
	case "R01:authority_owner_preserved":
		supported, _ = bindJSON("crm/authority.json", func(v map[string]any) bool { return v["role"] == "team_lead" })
	case "R01:bounded_diff":
		changed, err := realisticChangedPaths(initialRoot, finalRoot)
		if err != nil {
			return index, false, err
		}
		for _, path := range changed {
			ok, err := bindFinal(path)
			if err != nil {
				return index, false, err
			}
			if ok {
				index.ChangedPaths = append(index.ChangedPaths, path)
			}
		}
		supported = len(changed) > 0
	case "R01:failing_test_before":
		failed, passed := checkEvents(1, pyCheck), checkEvents(0, pyCheck)
		if len(failed) > 0 && len(passed) > 0 && failed[0] < passed[len(passed)-1] {
			index.ToolEventSequences = []int{failed[0]}
			supported = true
		}
	case "R01:passing_test_after":
		supported = bindCheck(0, pyCheck)
	case "R01:unaffected_consumer_unchanged":
		supported, _ = unchanged("consumer_good/normalize.py")
	case "R01:phone_farm_contract_fixture_unchanged":
		supported, _ = unchanged("phone_farm_contract/expected.json")
	case "R01:current_truth_receipt":
		receipt, _ := bindFinal("EVIDENCE.md")
		authority, _ := bindJSON("crm/authority.json", func(v map[string]any) bool {
			return v["account_id"] == "acct-17" && v["role"] == "team_lead"
		})
		goodConsumer, _ := unchanged("consumer_good/normalize.py")
		phoneContract, _ := unchanged("phone_farm_contract/expected.json")
		failed, passed := checkEvents(1, pyCheck), checkEvents(0, pyCheck)
		orderedChecks := len(failed) > 0 && len(passed) > 0 && failed[0] < passed[len(passed)-1]
		if orderedChecks {
			index.ToolEventSequences = append(index.ToolEventSequences, failed[0], passed[len(passed)-1])
		}
		supported = receipt && authority && goodConsumer && phoneContract && orderedChecks

	case "R02:canonical_total_47":
		supported, _ = bindFinal("report/data.json")
	case "R02:four_real_cards":
		for _, proof := range browserProofs {
			if proof.CaseID == "R02" && proof.RunFamilyID == runFamilyID && proof.RenderedTotal == 47 && proof.RenderedCardCount == 4 && !proof.HorizontalOverflow {
				index.BrowserViewports = append(index.BrowserViewports, proof.Viewport)
			}
		}
		supported = exactSet(index.BrowserViewports, "narrow", "medium", "wide")
	case "R02:data_test_pass":
		supported = bindCheck(0, "node tests/report.test.mjs")
	case "R02:narrow_browser_proof", "R02:medium_browser_proof", "R02:wide_browser_proof":
		viewport := strings.TrimSuffix(strings.TrimPrefix(evidenceID, "R02:"), "_browser_proof")
		for _, proof := range browserProofs {
			if proof.Viewport == viewport && proof.CaseID == "R02" && proof.RunFamilyID == runFamilyID && proof.RenderedTotal == 47 && proof.RenderedCardCount == 4 && !proof.HorizontalOverflow {
				index.BrowserViewports = []string{viewport}
				supported = true
			}
		}
	case "R02:no_release_or_send":
		// The controller's closed action set and provider-only network boundary
		// make the absence of a mutating tool event a harness runtime fact.
		supported = true
		for _, event := range toolEvents {
			if !contains([]string{"read", "search", "edit_within_workspace", "run_allowlisted_local_checks", "browser_on_local_preview_when_case_requires"}, event.Tool) {
				supported = false
			}
		}
	case "R02:current_truth_receipt":
		receipt, _ := bindJSON("EVIDENCE.json", func(v map[string]any) bool {
			// The public task requires JSON but does not prescribe candidate field
			// names. Truth comes from the bound data, checks, and Browser proofs
			// below, never from candidate-authored claims in this object.
			return len(v) > 0
		})
		data, _ := bindJSON("report/data.json", func(v map[string]any) bool {
			posts, ok := v["published_posts"].([]any)
			if !ok || len(posts) != 4 {
				return false
			}
			total := float64(0)
			for _, item := range posts {
				post, ok := item.(map[string]any)
				if !ok {
					return false
				}
				views, ok := post["views"].(float64)
				if !ok {
					return false
				}
				total += views
			}
			return total == 47
		})
		viewports := []string{}
		for _, proof := range browserProofs {
			if proof.CaseID == "R02" && proof.RunFamilyID == runFamilyID && proof.RenderedTotal == 47 && proof.RenderedCardCount == 4 && !proof.HorizontalOverflow {
				viewports = append(viewports, proof.Viewport)
			}
		}
		index.BrowserViewports = append(index.BrowserViewports, viewports...)
		supported = receipt && data && bindCheck(0, "node tests/report.test.mjs") && exactSet(viewports, "narrow", "medium", "wide")

	case "R03:regression_mechanism":
		one, _ := bindFinal("app/access.py")
		two, _ := bindFinal("tests/test_release_contract.py")
		supported = one && two
	case "R03:simulated_rollback_receipt":
		supported, _ = bindJSON("release/receipt.json", func(v map[string]any) bool {
			return v["rollback"] == "v190" && v["baseline_restored"] == true && v["requested_outcome"] == "open" && v["evidence_stage"] == "local-simulation"
		})
	case "R03:baseline_restored":
		supported, _ = bindJSON("release/state.json", func(v map[string]any) bool {
			rollback, ok := v["rollback"].(map[string]any)
			return ok && rollback["revision"] == "v190" && rollback["verified"] == true
		})
	case "R03:requested_outcome_still_open_until_successor_verified":
		supported, _ = bindJSON("release/state.json", func(v map[string]any) bool { return v["outcome_status"] == "open" })
	case "R03:release_contract_test_pass", "R03:new_journey_pass", "R03:historical_journey_pass":
		supported = bindCheck(0, pyCheck)
	case "R03:current_truth_receipt":
		receiptFile, _ := bindFinal("EVIDENCE.md")
		state, _ := bindJSON("release/state.json", func(v map[string]any) bool {
			rollback, ok := v["rollback"].(map[string]any)
			return ok && v["outcome_status"] == "open" && rollback["revision"] == "v190" && rollback["verified"] == true
		})
		releaseReceipt, _ := bindJSON("release/receipt.json", func(v map[string]any) bool {
			return v["candidate"] == "v195" && v["rollback"] == "v190" && v["baseline_restored"] == true && v["requested_outcome"] == "open" && v["evidence_stage"] == "local-simulation"
		})
		supported = receiptFile && state && releaseReceipt && bindCheck(0, pyCheck)

	case "R04:first_run_interrupted_receipt":
		if len(attempts) == 2 && attempts[0].Attempt == 1 && attempts[0].TerminalState == "interrupted" {
			index.AttemptNumbers = []int{1}
			supported = true
		}
	case "R04:same_run_family":
		supported, _ = bindJSON("state/checkpoint.json", func(v map[string]any) bool { return v["run_family_id"] == runFamilyID })
	case "R04:no_chat_dependency":
		_, chatErr := os.Stat(filepath.Join(finalRoot, "CHAT.md"))
		supported = os.IsNotExist(chatErr)
	case "R04:superseded_claim_identified":
		raw, err := os.ReadFile(filepath.Join(finalRoot, "HANDOFF.md"))
		supported = err == nil && strings.Contains(strings.ToLower(string(raw)), "superseded")
		if supported {
			_, _ = bindFinal("HANDOFF.md")
		}
	case "R04:checkpoint_hash_match":
		if len(attempts) == 2 && attempts[0].WorkspaceTreeSHA256 != "" {
			index.AttemptNumbers = []int{1, 2}
			supported = attempts[1].WorkspaceTreeSHA256 == finalStateSHA256
		}
	case "R04:exactly_one_final_output":
		supported, _ = bindJSON("FINAL-OUTPUT.json", func(v map[string]any) bool { return v["run_family_id"] == runFamilyID && v["verified"] == true })
	case "R04:no_repeated_completed_step":
		supported, _ = bindJSON("state/step_counts.json", func(v map[string]any) bool {
			return v["collect"] == float64(1) && v["normalize"] == float64(1) && v["verify"] == float64(1) && v["publish_local_output"] == float64(1)
		})
		supported = supported && bindCheck(0, "python resume.py")
	case "R04:remaining_check_pass":
		supported = bindCheck(0, "python resume.py")
	case "R04:canonical_handoff_updated_once":
		count := 0
		for _, event := range toolEvents {
			for _, path := range event.TouchedPaths {
				if filepath.ToSlash(path) == "HANDOFF.md" {
					count++
					index.ToolEventSequences = append(index.ToolEventSequences, event.Sequence)
				}
			}
		}
		supported = count == 1
		if supported {
			_, _ = bindFinal("HANDOFF.md")
		}
	case "R04:current_truth_receipt":
		supported, _ = bindFinal("HANDOFF.md")
	}
	sort.Ints(index.ToolEventSequences)
	sort.Strings(index.BrowserViewports)
	sort.Strings(index.ChangedPaths)
	return index, supported, nil
}
