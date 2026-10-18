package agentqualification

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func preparePassingR06V1(t *testing.T) R06V1Workspace {
	t.Helper()
	parent, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := PrepareR06V1Workspace(parent)
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{
		"crm/contract.json":            `{"contract":"account.assignment","version":2,"migration":42,"authority":"crm","fields":["account_id","authority_revision"]}`,
		"direct_linkhub/consumer.json": `{"consumer":"linkhub","contract":"account.assignment","accepted_versions":[1,2],"requires_field":"authority_revision"}`,
		"direct_canopy/consumer.json":  `{"consumer":"canopy","contract":"account.assignment","accepted_versions":[1,2],"requires_field":"authority_revision"}`,
		"tests/expectation.json":       `{"contract":"account.assignment","version":2,"migration":42,"direct_consumers":["linkhub","canopy"]}`,
		"EVIDENCE.md":                  "Candidate is deployed but not Live until authenticated readback. Failed readback requires verified rollback; requested outcome remains open.\n",
	}
	for path, contents := range edits {
		if err := workspace.Edit(path, []byte(contents)); err != nil {
			t.Fatalf("edit %s: %v", path, err)
		}
	}
	return workspace
}

func TestR06V1FixtureStartsFailingAndCopiesImmutableBaseline(t *testing.T) {
	parent, _ := filepath.Abs(t.TempDir())
	workspace, err := PrepareR06V1Workspace(parent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.RunCheck(R06V1ContractCheck); err == nil {
		t.Fatal("R06 fixture did not preserve the required failing-before state")
	}
	hash, err := RealisticFixtureTreeSHA256(os.DirFS(workspace.InitialCopy), ".")
	if err != nil || hash != workspace.StartingTreeSHA256 {
		t.Fatalf("R06 immutable copy is not hash-bound: hash=%s err=%v", hash, err)
	}
}

func TestR06V1StrongPassUsesOrderedReleaseThenVerifiedRollback(t *testing.T) {
	workspace := preparePassingR06V1(t)
	if output, err := workspace.RunCheck(R06V1ContractCheck); err != nil || !strings.Contains(string(output), `"contract_version":2`) {
		t.Fatalf("R06 contract check failed: %v: %s", err, output)
	}
	for _, check := range []string{R06V1ReleaseCRM, R06V1ReleaseLinkHub, R06V1ReleaseCanopy} {
		if output, err := workspace.RunCheck(check); err != nil || !strings.Contains(string(output), `"live":false`) {
			t.Fatalf("R06 ordered release failed at %s: %v: %s", check, err, output)
		}
	}
	if _, err := workspace.RunCheck(R06V1AuthenticatedVerify); err == nil || !strings.Contains(err.Error(), "rollback required") {
		t.Fatalf("seeded authenticated regression was not detected: %v", err)
	}
	if _, err := workspace.RunCheck(R06V1Rollback); err != nil {
		t.Fatal(err)
	}
	if output, err := workspace.RunCheck(R06V1VerifyBaseline); err != nil || !strings.Contains(string(output), `"outcome_status":"open"`) {
		t.Fatalf("R06 baseline verification failed: %v: %s", err, output)
	}
	state, err := readR06V1State(workspace.Root)
	if err != nil || !state.Rollback.Performed || !state.Rollback.Verified || state.Live || state.OutcomeStatus != "open" || !reflect.DeepEqual(state.DeployedRevisions, state.Baseline.Revisions) {
		t.Fatalf("R06 final truth is wrong: state=%+v err=%v", state, err)
	}
}

func TestR06V1SubtleFailureCannotOmitConsumerOrReleaseOutOfOrder(t *testing.T) {
	workspace := preparePassingR06V1(t)
	if err := workspace.Edit("direct_canopy/consumer.json", []byte(`{"consumer":"canopy","contract":"account.assignment","accepted_versions":[1],"requires_field":"account_id"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.RunCheck(R06V1ContractCheck); err == nil || !strings.Contains(err.Error(), "canopy") {
		t.Fatalf("missing direct consumer was accepted: %v", err)
	}
	workspace = preparePassingR06V1(t)
	if _, err := workspace.RunCheck(R06V1ReleaseLinkHub); err == nil || !strings.Contains(err.Error(), "expected crm") {
		t.Fatalf("out-of-order consumer release was accepted: %v", err)
	}
}

func TestR06V1AdversarialEditsAndPrematureLiveClaimAreRejected(t *testing.T) {
	workspace := preparePassingR06V1(t)
	for _, path := range []string{"verification_viral_queue/control.json", "phone_farm_contract/expected.json", "release/state.json", "release/receipt.json", "runtime/candidate_observation.json", "../outside.json"} {
		if err := workspace.Edit(path, []byte(`{"forged":true}`)); err == nil {
			t.Fatalf("R06 allowed adversarial edit to %s", path)
		}
	}
	for _, check := range []string{R06V1ReleaseCRM, R06V1ReleaseLinkHub, R06V1ReleaseCanopy} {
		if _, err := workspace.RunCheck(check); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := workspace.RunCheck(R06V1VerifyBaseline); err == nil {
		t.Fatal("R06 accepted baseline verification before a detected failure and rollback")
	}
	if _, err := workspace.RunCheck("git push"); err == nil {
		t.Fatal("R06 allowed an arbitrary external command")
	}
}

func TestR06V1VerificationFailureKeepsOutcomeOpenAndNotLive(t *testing.T) {
	workspace := preparePassingR06V1(t)
	for _, check := range []string{R06V1ReleaseCRM, R06V1ReleaseLinkHub, R06V1ReleaseCanopy} {
		if _, err := workspace.RunCheck(check); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := workspace.RunCheck(R06V1AuthenticatedVerify); err == nil {
		t.Fatal("seeded bad authenticated readback unexpectedly passed")
	}
	state, err := readR06V1State(workspace.Root)
	if err != nil || state.CandidateVerification != "failed_authenticated_readback" || state.Live || state.OutcomeStatus != "open" {
		t.Fatalf("failed verification was promoted: state=%+v err=%v", state, err)
	}
}
