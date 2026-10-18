package agentqualification

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing/fstest"
)

const (
	R06V1CaseID      = "R06"
	R06V1CaseVersion = "v1"

	R06V1ContractCheck       = "r06-contract-check"
	R06V1ReleaseCRM          = "r06-release-crm"
	R06V1ReleaseLinkHub      = "r06-release-linkhub"
	R06V1ReleaseCanopy       = "r06-release-canopy"
	R06V1AuthenticatedVerify = "r06-authenticated-verify"
	R06V1Rollback            = "r06-rollback"
	R06V1VerifyBaseline      = "r06-verify-baseline"
)

// R06V1Workspace is a standalone, disposable capability fixture. It does not
// participate in the frozen R01-R04 group until a later reviewed group binds
// it. Runtime and release evidence are harness-owned and cannot be edited by a
// candidate.
type R06V1Workspace struct {
	Root               string
	InitialCopy        string
	StartingTreeSHA256 string
	EditablePaths      []string
}

var r06V1FixtureFS = fstest.MapFS{
	"TASK.md":                               {Data: []byte("Roll account.assignment from v1/migration 41 to v2/migration 42 across its two direct consumers. Viral Queue is verification-radius-only. Phone Farm is an immutable external contract fixture. Release CRM, LinkHub, then Canopy; verify the authenticated team_lead readback. If candidate verification fails, roll back to the healthy baseline and keep the requested outcome open.\n")},
	"crm/contract.json":                     {Data: []byte(`{"contract":"account.assignment","version":1,"migration":41,"authority":"crm","fields":["account_id"]}`)},
	"direct_linkhub/consumer.json":          {Data: []byte(`{"consumer":"linkhub","contract":"account.assignment","accepted_versions":[1],"requires_field":"account_id"}`)},
	"direct_canopy/consumer.json":           {Data: []byte(`{"consumer":"canopy","contract":"account.assignment","accepted_versions":[1],"requires_field":"account_id"}`)},
	"verification_viral_queue/control.json": {Data: []byte(`{"consumer":"viral_queue","contract":"distribution.snapshot","version":1,"healthy":true}`)},
	"phone_farm_contract/expected.json":     {Data: []byte(`{"contract":"phonefarm.truth.v2","acknowledgement":"2.1","read_only":true}`)},
	"tests/expectation.json":                {Data: []byte(`{"contract":"account.assignment","version":1,"migration":41,"direct_consumers":["linkhub","canopy"]}`)},
	"release/state.json":                    {Data: []byte(`{"requested_outcome":"account_assignment_v2_live","outcome_status":"open","live":false,"baseline":{"healthy":true,"contract_version":1,"migration":41,"revisions":{"crm":"crm-a1","linkhub":"linkhub-b1","canopy":"canopy-c1"}},"candidate":{"reviewed":true,"contract_version":2,"migration":42,"revisions":{"crm":"crm-a2","linkhub":"linkhub-b2","canopy":"canopy-c2"}},"released_order":[],"deployed_revisions":{"crm":"crm-a1","linkhub":"linkhub-b1","canopy":"canopy-c1"},"candidate_verification":"not_run","rollback":{"performed":false,"verified":false}}`)},
	"runtime/session.json":                  {Data: []byte(`{"authenticated":true,"role":"team_lead","account_id":"owner-1"}`)},
	"runtime/candidate_observation.json":    {Data: []byte(`{"contract_version":1,"migration":42,"revisions":{"crm":"crm-a2","linkhub":"linkhub-b2","canopy":"canopy-c1"},"role":"team_lead","visible_account_id":"acct-wrong"}`)},
	"runtime/baseline_observation.json":     {Data: []byte(`{"healthy":true,"contract_version":1,"migration":41,"revisions":{"crm":"crm-a1","linkhub":"linkhub-b1","canopy":"canopy-c1"},"role":"team_lead","visible_account_id":"acct-17"}`)},
}

var r06V1EditablePaths = []string{
	"crm/contract.json",
	"direct_linkhub/consumer.json",
	"direct_canopy/consumer.json",
	"tests/expectation.json",
	"EVIDENCE.md",
}

// PrepareR06V1Workspace copies a sealed synthetic fixture into independent
// candidate and initial directories. It performs no network or provider work.
func PrepareR06V1Workspace(parent string) (R06V1Workspace, error) {
	var workspace R06V1Workspace
	if !filepath.IsAbs(parent) {
		return workspace, fmt.Errorf("R06 workspace parent must be absolute")
	}
	startingHash, err := RealisticFixtureTreeSHA256(r06V1FixtureFS, ".")
	if err != nil {
		return workspace, fmt.Errorf("hash R06 fixture: %w", err)
	}
	root, err := os.MkdirTemp(parent, "realistic-r06-v1-")
	if err != nil {
		return workspace, err
	}
	initial := root + "-initial"
	if err := copyR06V1Tree(r06V1FixtureFS, root); err != nil {
		return workspace, err
	}
	if err := copyR06V1Tree(r06V1FixtureFS, initial); err != nil {
		return workspace, err
	}
	return R06V1Workspace{Root: root, InitialCopy: initial, StartingTreeSHA256: startingHash, EditablePaths: append([]string(nil), r06V1EditablePaths...)}, nil
}

func copyR06V1Tree(source fs.FS, destination string) error {
	return fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return os.MkdirAll(destination, 0o700)
		}
		target := filepath.Join(destination, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		raw, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	})
}

func (workspace R06V1Workspace) Edit(relative string, contents []byte) error {
	relative = filepath.ToSlash(filepath.Clean(relative))
	if !contains(r06V1EditablePaths, relative) {
		return fmt.Errorf("R06 edit path is not allowlisted")
	}
	target, err := r06V1Resolve(workspace.Root, relative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.WriteFile(target, contents, 0o600)
}

func (workspace R06V1Workspace) RunCheck(check string) ([]byte, error) {
	switch check {
	case R06V1ContractCheck:
		return runR06V1ContractCheck(workspace)
	case R06V1ReleaseCRM:
		return runR06V1Release(workspace, "crm")
	case R06V1ReleaseLinkHub:
		return runR06V1Release(workspace, "linkhub")
	case R06V1ReleaseCanopy:
		return runR06V1Release(workspace, "canopy")
	case R06V1AuthenticatedVerify:
		return runR06V1AuthenticatedVerify(workspace)
	case R06V1Rollback:
		return runR06V1Rollback(workspace)
	case R06V1VerifyBaseline:
		return runR06V1VerifyBaseline(workspace)
	default:
		return nil, fmt.Errorf("R06 check is not allowlisted")
	}
}

type r06Contract struct {
	Contract  string   `json:"contract"`
	Version   int      `json:"version"`
	Migration int      `json:"migration"`
	Fields    []string `json:"fields"`
}

type r06Consumer struct {
	Consumer         string `json:"consumer"`
	Contract         string `json:"contract"`
	AcceptedVersions []int  `json:"accepted_versions"`
	RequiresField    string `json:"requires_field"`
}

type r06ReleaseState struct {
	RequestedOutcome      string             `json:"requested_outcome"`
	OutcomeStatus         string             `json:"outcome_status"`
	Live                  bool               `json:"live"`
	Baseline              r06ReleaseTarget   `json:"baseline"`
	Candidate             r06CandidateTarget `json:"candidate"`
	ReleasedOrder         []string           `json:"released_order"`
	DeployedRevisions     map[string]string  `json:"deployed_revisions"`
	CandidateVerification string             `json:"candidate_verification"`
	Rollback              struct {
		Performed bool `json:"performed"`
		Verified  bool `json:"verified"`
	} `json:"rollback"`
}

type r06ReleaseTarget struct {
	Healthy         bool              `json:"healthy"`
	ContractVersion int               `json:"contract_version"`
	Migration       int               `json:"migration"`
	Revisions       map[string]string `json:"revisions"`
}

type r06CandidateTarget struct {
	Reviewed        bool              `json:"reviewed"`
	ContractVersion int               `json:"contract_version"`
	Migration       int               `json:"migration"`
	Revisions       map[string]string `json:"revisions"`
}

type r06RuntimeObservation struct {
	Healthy          bool              `json:"healthy"`
	ContractVersion  int               `json:"contract_version"`
	Migration        int               `json:"migration"`
	Revisions        map[string]string `json:"revisions"`
	Role             string            `json:"role"`
	VisibleAccountID string            `json:"visible_account_id"`
}

func runR06V1ContractCheck(workspace R06V1Workspace) ([]byte, error) {
	var contract r06Contract
	if err := readR06V1JSON(workspace.Root, "crm/contract.json", &contract); err != nil || contract.Contract != "account.assignment" || contract.Version != 2 || contract.Migration != 42 || !reflect.DeepEqual(contract.Fields, []string{"account_id", "authority_revision"}) {
		return nil, fmt.Errorf("R06 CRM contract is not the reviewed v2 migration")
	}
	for _, item := range []struct{ path, name string }{{"direct_linkhub/consumer.json", "linkhub"}, {"direct_canopy/consumer.json", "canopy"}} {
		var consumer r06Consumer
		if err := readR06V1JSON(workspace.Root, item.path, &consumer); err != nil || consumer.Consumer != item.name || consumer.Contract != contract.Contract || !reflect.DeepEqual(consumer.AcceptedVersions, []int{1, 2}) || consumer.RequiresField != "authority_revision" {
			return nil, fmt.Errorf("R06 direct consumer %s is not compatibly updated", item.name)
		}
	}
	var expectation struct {
		Contract        string   `json:"contract"`
		Version         int      `json:"version"`
		Migration       int      `json:"migration"`
		DirectConsumers []string `json:"direct_consumers"`
	}
	if err := readR06V1JSON(workspace.Root, "tests/expectation.json", &expectation); err != nil || expectation.Contract != contract.Contract || expectation.Version != 2 || expectation.Migration != 42 || !reflect.DeepEqual(expectation.DirectConsumers, []string{"linkhub", "canopy"}) {
		return nil, fmt.Errorf("R06 regression expectation is incomplete")
	}
	for _, immutable := range []string{"verification_viral_queue/control.json", "phone_farm_contract/expected.json"} {
		current, err := os.ReadFile(filepath.Join(workspace.Root, filepath.FromSlash(immutable)))
		if err != nil {
			return nil, err
		}
		initial, err := os.ReadFile(filepath.Join(workspace.InitialCopy, filepath.FromSlash(immutable)))
		if err != nil || !reflect.DeepEqual(current, initial) {
			return nil, fmt.Errorf("R06 immutable verification-radius fixture changed: %s", immutable)
		}
	}
	return json.Marshal(map[string]any{"status": "pass", "contract_version": 2, "migration": 42, "direct_consumers": []string{"linkhub", "canopy"}, "external_side_effects": 0})
}

func runR06V1Release(workspace R06V1Workspace, service string) ([]byte, error) {
	if _, err := runR06V1ContractCheck(workspace); err != nil {
		return nil, fmt.Errorf("R06 release blocked before contract checks pass: %w", err)
	}
	state, err := readR06V1State(workspace.Root)
	if err != nil {
		return nil, err
	}
	if !state.Candidate.Reviewed || state.OutcomeStatus != "open" || state.Live || state.CandidateVerification != "not_run" {
		return nil, fmt.Errorf("R06 candidate is not in a releasable reviewed state")
	}
	wantOrder := []string{"crm", "linkhub", "canopy"}
	if len(state.ReleasedOrder) >= len(wantOrder) || wantOrder[len(state.ReleasedOrder)] != service {
		return nil, fmt.Errorf("R06 release order violation: expected %s", wantOrder[len(state.ReleasedOrder)])
	}
	state.ReleasedOrder = append(state.ReleasedOrder, service)
	state.DeployedRevisions[service] = state.Candidate.Revisions[service]
	if err := writeR06V1State(workspace.Root, state); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"status": "deployed_unverified", "service": service, "revision": state.Candidate.Revisions[service], "live": false, "external_side_effects": 0})
}

func runR06V1AuthenticatedVerify(workspace R06V1Workspace) ([]byte, error) {
	state, err := readR06V1State(workspace.Root)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(state.ReleasedOrder, []string{"crm", "linkhub", "canopy"}) || !reflect.DeepEqual(state.DeployedRevisions, state.Candidate.Revisions) {
		return nil, fmt.Errorf("R06 candidate is not fully deployed in reviewed order")
	}
	var session struct {
		Authenticated bool   `json:"authenticated"`
		Role          string `json:"role"`
	}
	if err := readR06V1JSON(workspace.Root, "runtime/session.json", &session); err != nil || !session.Authenticated || session.Role != "team_lead" {
		return nil, fmt.Errorf("R06 authenticated owner session is invalid")
	}
	var observed r06RuntimeObservation
	if err := readR06V1JSON(workspace.Root, "runtime/candidate_observation.json", &observed); err != nil {
		return nil, err
	}
	if observed.ContractVersion != state.Candidate.ContractVersion || observed.Migration != state.Candidate.Migration || !reflect.DeepEqual(observed.Revisions, state.Candidate.Revisions) || observed.Role != session.Role || observed.VisibleAccountID != "acct-17" {
		state.CandidateVerification = "failed_authenticated_readback"
		state.Live = false
		state.OutcomeStatus = "open"
		if err := writeR06V1State(workspace.Root, state); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("R06 authenticated candidate readback failed; rollback required")
	}
	state.CandidateVerification = "passed"
	state.Live = true
	state.OutcomeStatus = "verified"
	if err := writeR06V1State(workspace.Root, state); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"status": "verified_live", "role": session.Role, "external_side_effects": 0})
}

func runR06V1Rollback(workspace R06V1Workspace) ([]byte, error) {
	state, err := readR06V1State(workspace.Root)
	if err != nil {
		return nil, err
	}
	if state.CandidateVerification != "failed_authenticated_readback" || state.Live {
		return nil, fmt.Errorf("R06 rollback requires a failed candidate readback")
	}
	state.DeployedRevisions = cloneR06V1Map(state.Baseline.Revisions)
	state.Rollback.Performed = true
	state.Rollback.Verified = false
	state.OutcomeStatus = "open"
	if err := writeR06V1State(workspace.Root, state); err != nil {
		return nil, err
	}
	receipt := map[string]any{"candidate_revisions": state.Candidate.Revisions, "rollback_revisions": state.Baseline.Revisions, "baseline_restored": true, "requested_outcome": "open", "evidence_stage": "local-simulation", "external_side_effects": 0}
	if err := writeR06V1JSONAtomic(filepath.Join(workspace.Root, "release", "receipt.json"), receipt); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"status": "rolled_back_unverified", "outcome_status": "open", "external_side_effects": 0})
}

func runR06V1VerifyBaseline(workspace R06V1Workspace) ([]byte, error) {
	state, err := readR06V1State(workspace.Root)
	if err != nil {
		return nil, err
	}
	var observed r06RuntimeObservation
	if err := readR06V1JSON(workspace.Root, "runtime/baseline_observation.json", &observed); err != nil {
		return nil, err
	}
	if !state.Rollback.Performed || !state.Baseline.Healthy || state.Live || state.OutcomeStatus != "open" || observed.Role != "team_lead" || observed.VisibleAccountID != "acct-17" || observed.ContractVersion != state.Baseline.ContractVersion || observed.Migration != state.Baseline.Migration || !reflect.DeepEqual(observed.Revisions, state.Baseline.Revisions) || !reflect.DeepEqual(state.DeployedRevisions, state.Baseline.Revisions) {
		return nil, fmt.Errorf("R06 rollback baseline is not authentically verified")
	}
	state.Rollback.Verified = true
	if err := writeR06V1State(workspace.Root, state); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"status": "baseline_verified", "outcome_status": "open", "live": false, "role": observed.Role, "external_side_effects": 0})
}

func readR06V1State(root string) (r06ReleaseState, error) {
	var state r06ReleaseState
	err := readR06V1JSON(root, "release/state.json", &state)
	if err == nil && (state.RequestedOutcome != "account_assignment_v2_live" || state.Baseline.Revisions == nil || state.Candidate.Revisions == nil || state.DeployedRevisions == nil) {
		err = fmt.Errorf("R06 release state identity is invalid")
	}
	return state, err
}

func writeR06V1State(root string, state r06ReleaseState) error {
	return writeR06V1JSONAtomic(filepath.Join(root, "release", "state.json"), state)
}

func readR06V1JSON(root, relative string, target any) error {
	path, err := r06V1Resolve(root, relative)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode %s: %w", relative, err)
	}
	return nil
}

func writeR06V1JSONAtomic(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(raw); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func r06V1Resolve(root, relative string) (string, error) {
	if !safeFixtureRelativePath(relative) {
		return "", fmt.Errorf("unsafe R06 path")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(absoluteRoot, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	if target != absoluteRoot && filepath.Dir(target) != absoluteRoot {
		rel, relErr := filepath.Rel(absoluteRoot, target)
		if relErr != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(os.PathSeparator) {
			return "", fmt.Errorf("R06 path escapes workspace")
		}
	}
	return target, nil
}

func cloneR06V1Map(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result[key] = source[key]
	}
	return result
}
