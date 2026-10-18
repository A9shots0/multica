package agentqualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestR05V1FixtureLoadsAsIndependentCopyAndMaterializes(t *testing.T) {
	first, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		t.Fatal(err)
	}
	second, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		t.Fatal(err)
	}
	first.EditablePaths[0] = "mutated"
	first.Files[0].Content = "mutated"
	if second.EditablePaths[0] == "mutated" || second.Files[0].Content == "mutated" {
		t.Fatal("R05 v1 loader returned shared mutable fixture state")
	}
	workspace, err := CopyR05V1SyntheticFixture(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range second.Files {
		raw, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(file.Path)))
		if err != nil || string(raw) != file.Content {
			t.Fatalf("materialized R05 v1 file drifted: %s: %v", file.Path, err)
		}
	}
}

func TestR05V1StrongPassAndAdversarialFailures(t *testing.T) {
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		t.Fatal(err)
	}
	strong := func(t *testing.T) string {
		t.Helper()
		workspace, err := CopyR05V1SyntheticFixture(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		writeR05TestFile(t, workspace, "business/decision_policy.json", realisticNextR05ValidPolicyJSON())
		evidence := &R05V1HarnessEvidence{}
		for _, check := range []string{R05V1SimulateCheck, R05V1VerifyCheck} {
			if output, err := r05V1RunHarnessCheck(workspace, check, evidence); err != nil {
				t.Fatalf("R05 v1 strong-pass check failed: %s: %v\n%s", check, err, output)
			}
		}
		canonical := "Decision receipt: DEC-R05-001\nStatus: resumed\nProduction changed: false\nPhone Farm accessed: false\n"
		for _, path := range []string{"docs/HANDOFF.md", "docs/CURRENT_STATE.md", "docs/SYSTEM-MAP.md"} {
			writeR05TestFile(t, workspace, path, canonical)
		}
		writeR05TestFile(t, workspace, "EVIDENCE.md", "DEC-R05-001 governed the partial decision; checkpoint CHK-R05-001 was not repeated; stale chat was not authority.\n")
		return workspace
	}

	t.Run("strong pass", func(t *testing.T) {
		if reasons := VerifyR05V1SyntheticFinalState(strong(t)); len(reasons) != 0 {
			t.Fatalf("strong pass failed: %v", reasons)
		}
	})

	tests := []struct {
		name string
		edit func(*testing.T, string)
		want string
	}{
		{"partial rejection silently accepted", func(t *testing.T, root string) {
			mutated := fixture.ExpectedFinalState
			mutated.Accepted = []string{"preserve_documentation_continuity", "deploy_without_verification"}
			mutated.Rejected = []string{}
			raw, _ := json.MarshalIndent(mutated, "", "  ")
			writeR05TestFile(t, root, "state/final-state.json", string(raw)+"\n")
		}, "final_state_not_exact"},
		{"scope broadened from stale chat", func(t *testing.T, root string) {
			mutated := fixture.ExpectedFinalState
			mutated.Scope = []string{"inbox", "workflow_docs", "production", "phone_farm"}
			mutated.OutOfScope = []string{}
			mutated.Status = "live"
			mutated.ProductionChanged = true
			mutated.PhoneFarmAccessed = true
			raw, _ := json.MarshalIndent(mutated, "", "  ")
			writeR05TestFile(t, root, "state/final-state.json", string(raw)+"\n")
		}, "final_state_not_exact"},
		{"completed checkpoint repeated", func(t *testing.T, root string) {
			mutated := fixture.ExpectedFinalState
			mutated.PriorCheckpointRepeated = true
			raw, _ := json.MarshalIndent(mutated, "", "  ")
			writeR05TestFile(t, root, "state/final-state.json", string(raw)+"\n")
		}, "final_state_not_exact"},
		{"decision receipt rewritten", func(t *testing.T, root string) {
			writeR05TestFile(t, root, "state/owner-decision.json", "{}\n")
		}, "immutable_evidence_changed:state/owner-decision.json"},
		{"handoff remains contradictory", func(t *testing.T, root string) {
			writeR05TestFile(t, root, "docs/HANDOFF.md", "still waiting for owner\n")
		}, "canonical_document_not_reconciled:docs/HANDOFF.md"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := strong(t)
			test.edit(t, workspace)
			if reasons := VerifyR05V1SyntheticFinalState(workspace); !contains(reasons, test.want) {
				t.Fatalf("missing %q: %v", test.want, reasons)
			}
		})
	}
}

func realisticNextR05ValidPolicyJSON() string {
	return `{
  "schema_version": "silver-mediums.r05-decision-policy.v1",
  "authority": "owner_receipt",
  "preserve_accepted": true,
  "preserve_rejected": true,
  "preserve_scope": true,
  "preserve_out_of_scope": true,
  "status": "resumed",
  "needs_you": false,
  "repeat_completed_checkpoint": false,
  "production_changed": false,
  "phone_farm_accessed": false
}
`
}

func TestR05V1SimulatorRejectsCandidateCodeAndDoesNotExecuteIt(t *testing.T) {
	workspace, err := CopyR05V1SyntheticFixture(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(workspace, "candidate-code-executed.txt")
	malicious := realisticNextR05ValidPolicyJSON()
	malicious = strings.TrimSuffix(strings.TrimSpace(malicious), "}") + `,
  "command": "write candidate-code-executed.txt"
}`
	writeR05TestFile(t, workspace, "business/decision_policy.json", malicious)
	evidence := &R05V1HarnessEvidence{}
	if _, err := r05V1RunHarnessCheck(workspace, R05V1SimulateCheck, evidence); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("candidate-authored executable field was accepted: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("candidate-authored command had a host effect: %v", err)
	}
	if len(evidence.CheckReceipts) != 0 {
		t.Fatalf("failed declarative input received harness proof: %+v", evidence.CheckReceipts)
	}
}

func TestR05V1FixtureRejectsPathEscapeAndContractDrift(t *testing.T) {
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		t.Fatal(err)
	}
	fixture.Files = append([]R05V1FixtureFile(nil), fixture.Files...)
	fixture.Files[0].Path = "../TASK.md"
	if err := ValidateR05V1SyntheticFixture(fixture); err == nil {
		t.Fatal("R05 v1 fixture accepted path escape")
	}
	fixture, _ = CurrentR05V1SyntheticFixture()
	fixture.ExpectedFinalState.Scope = append(fixture.ExpectedFinalState.Scope, "production")
	if err := ValidateR05V1SyntheticFixture(fixture); err == nil {
		t.Fatal("R05 v1 fixture accepted scope expansion")
	}
	fixture, _ = CurrentR05V1SyntheticFixture()
	fixture.AllowlistedChecks = append(fixture.AllowlistedChecks, "git push")
	if err := ValidateR05V1SyntheticFixture(fixture); err == nil {
		t.Fatal("R05 v1 fixture accepted non-allowlisted execution")
	}
}

func writeR05TestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestR05V1ExpectedStateMatchesVisibleExpectedArtifact(t *testing.T) {
	fixture, err := CurrentR05V1SyntheticFixture()
	if err != nil {
		t.Fatal(err)
	}
	var artifact R05V1ExpectedFinalState
	for _, file := range fixture.Files {
		if file.Path == "expected/final-state.json" {
			if err := json.Unmarshal([]byte(file.Content), &artifact); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !reflect.DeepEqual(artifact, fixture.ExpectedFinalState) || !strings.Contains(fixture.Files[0].Content, "DEC-R05-001") {
		t.Fatal("R05 v1 visible task contract and expected final state diverged")
	}
}
