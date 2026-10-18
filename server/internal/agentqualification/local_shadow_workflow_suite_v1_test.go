package agentqualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/qualificationarchive"
	"testing"
)

func TestLocalShadowWorkflowSuiteV1FrozenCoverageAndFakeRun(t *testing.T) {
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Workflows) != 4 || len(suite.RequiredSurfaces) != 9 || len(suite.EvidenceContracts) != 12 {
		t.Fatalf("suite coverage drifted: workflows=%d surfaces=%d contracts=%d", len(suite.Workflows), len(suite.RequiredSurfaces), len(suite.EvidenceContracts))
	}
	run, err := historicalContracts(t).RunLocalShadowWorkflowSuiteFakeV1(suite)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Receipts) != 22 {
		t.Fatalf("fake receipts = %d, want 22", len(run.Receipts))
	}
	if run.ProviderRequestCount != 0 || run.CredentialReadCount != 0 || run.ToolCallCount != 0 || run.RepositoryAccessCount != 0 || run.ProductionAccessCount != 0 || run.OutboundActionCount != 0 || run.PhoneFarmAccessCount != 0 {
		t.Fatal("provider-free fake run crossed a forbidden boundary")
	}
	if err := historicalContracts(t).ValidateLocalShadowFakeRunV1(suite, run); err != nil {
		t.Fatal(err)
	}
}

func TestLocalShadowWorkflowSuiteV1BindsExactAcceptedSources(t *testing.T) {
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range suite.SourceArtifacts {
		raw, err := qualificationarchive.Read("suite-v1", artifact.Path, artifact.SHA256)
		if err != nil {
			t.Fatalf("read %s: %v", artifact.Path, err)
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != artifact.SHA256 {
			t.Errorf("source %s: got %s want %s", artifact.Path, got, artifact.SHA256)
		}
	}
}

func TestShadowRoleArtifactRefV1IsModelNeutralAndAllowlisted(t *testing.T) {
	for _, bindingID := range []string{
		"silver-mediums.role-artifact.v1.mika_owner_interface",
		"silver-mediums.role-artifact.v1.shared_windows_executor",
		"silver-mediums.role-artifact.v1.product_ui",
		"silver-mediums.role-artifact.v1.quality_checker",
		"silver-mediums.role-artifact.v2.architecture_assurance",
		"silver-mediums.role-artifact.v2.security_privacy_review",
		"silver-mediums.role-artifact.v2.data_sync_integrity",
		"silver-mediums.role-artifact.v2.release_runtime_assurance",
	} {
		artifact, err := ValidatedShadowRoleArtifactRefV1(bindingID)
		if err != nil {
			t.Fatalf("resolve %s: %v", bindingID, err)
		}
		if artifact.BindingID != bindingID || artifact.RoleID == "" || artifact.InstructionPath == "" || artifact.InstructionSHA256 == "" || artifact.SkillPath == "" || artifact.SkillSHA256 == "" || artifact.SourceManifestSHA256 == "" {
			t.Fatalf("incomplete shadow artifact for %s: %#v", bindingID, artifact)
		}
		raw, err := json.Marshal(artifact)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"model", "provider", "reasoning_effort", "runtime_id", "configuration_id", "credential", "allowed_tools"} {
			if _, present := fields[forbidden]; present {
				t.Fatalf("shadow artifact %s contains forbidden runtime field %q", bindingID, forbidden)
			}
		}
	}
	for _, bindingID := range []string{
		"silver-mediums.role-artifact.v1.crm_product_outcome",
		"silver-mediums.role-artifact.v2.missing",
		"silver-mediums.role-catalog.v1.crm_product_outcome",
	} {
		if _, err := ValidatedShadowRoleArtifactRefV1(bindingID); err == nil {
			t.Fatalf("unapproved shadow artifact %s was accepted", bindingID)
		}
	}
}

func TestLocalShadowWorkflowSuiteV1RejectsSelectionRuntimeAndAuthorityFields(t *testing.T) {
	for _, field := range []string{"model", "provider", "reasoning_effort", "runtime_id", "credential", "allowed_tools"} {
		injected := []byte(string(localShadowWorkflowSuiteV1JSON))
		needle := []byte(`"execution_mode": "deterministic_fake_provider_free",`)
		replacement := []byte(`"execution_mode": "deterministic_fake_provider_free", "` + field + `": "forbidden",`)
		injected = bytes.Replace(injected, needle, replacement, 1)
		var suite LocalShadowWorkflowSuiteV1
		if err := strictJSON(injected, &suite); err == nil {
			t.Errorf("unknown selection/runtime field %q was accepted", field)
		}
	}
}

func TestLocalShadowWorkflowSuiteV1FailsClosedOnManifestDrift(t *testing.T) {
	base := decodeLocalShadowSuiteForTest(t)
	mutations := map[string]func(*LocalShadowWorkflowSuiteV1){
		"status":          func(v *LocalShadowWorkflowSuiteV1) { v.Status = "executable" },
		"provider":        func(v *LocalShadowWorkflowSuiteV1) { v.ProviderCallsAllowed = true },
		"credential":      func(v *LocalShadowWorkflowSuiteV1) { v.CredentialsAllowed = true },
		"tools":           func(v *LocalShadowWorkflowSuiteV1) { v.ToolsAllowed = true },
		"repository":      func(v *LocalShadowWorkflowSuiteV1) { v.RepositoryAccessAllowed = true },
		"production":      func(v *LocalShadowWorkflowSuiteV1) { v.ProductionAccessAllowed = true },
		"outbound":        func(v *LocalShadowWorkflowSuiteV1) { v.OutboundActionsAllowed = true },
		"phone farm":      func(v *LocalShadowWorkflowSuiteV1) { v.PhoneFarmAccessAllowed = true },
		"selection":       func(v *LocalShadowWorkflowSuiteV1) { v.RoleModelSelection = "allowed" },
		"corrections":     func(v *LocalShadowWorkflowSuiteV1) { v.MaxAutomaticCorrections = 2 },
		"review attempts": func(v *LocalShadowWorkflowSuiteV1) { v.MaxReviewAttempts = 3 },
		"review count":    func(v *LocalShadowWorkflowSuiteV1) { v.IndependentReviews = 2 },
		"second review":   func(v *LocalShadowWorkflowSuiteV1) { v.SecondReview = true },
		"surface":         func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].Surfaces[0] = "other" },
		"harness":         func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].HarnessArtifactIDs[0] = "missing" },
		"source hash":     func(v *LocalShadowWorkflowSuiteV1) { v.SourceArtifacts[0].SHA256 = "00" },
		"contract source": func(v *LocalShadowWorkflowSuiteV1) { v.EvidenceContracts[0].SourceArtifactIDs[0] = "missing" },
		"squad":           func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].SquadID = "cross_boundary_change" },
		"controller":      func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].ControllerOwnsMoves = false },
		"concurrency":     func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].MaxConcurrentWorkers = 3 },
		"role":            func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].Stages[1].RoleID = "quality_checker" },
		"artifact":        func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].Stages[1].ArtifactBindingID = "latest" },
		"missing stage":   func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].Stages = v.Workflows[0].Stages[1:] },
		"release":         func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[3].RequiresRelease = false },
		"evidence":        func(v *LocalShadowWorkflowSuiteV1) { v.Workflows[0].Stages[0].EvidenceContractIDs = nil },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := cloneLocalShadowSuiteForTest(t, base)
			mutate(&candidate)
			if err := historicalContracts(t).ValidateLocalShadowWorkflowSuiteV1(candidate); err == nil {
				t.Fatal("manifest drift was accepted")
			}
		})
	}
}

func TestLocalShadowFakeRunV1FailsClosedOnAlterationDuplicateAndReplay(t *testing.T) {
	suite, err := historicalContracts(t).CurrentLocalShadowWorkflowSuiteV1()
	if err != nil {
		t.Fatal(err)
	}
	base, err := historicalContracts(t).RunLocalShadowWorkflowSuiteFakeV1(suite)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*LocalShadowFakeRunV1){
		"suite":       func(v *LocalShadowFakeRunV1) { v.SuiteSHA256 = "00" },
		"provider":    func(v *LocalShadowFakeRunV1) { v.ProviderRequestCount = 1 },
		"credential":  func(v *LocalShadowFakeRunV1) { v.CredentialReadCount = 1 },
		"tool":        func(v *LocalShadowFakeRunV1) { v.ToolCallCount = 1 },
		"repository":  func(v *LocalShadowFakeRunV1) { v.RepositoryAccessCount = 1 },
		"production":  func(v *LocalShadowFakeRunV1) { v.ProductionAccessCount = 1 },
		"outbound":    func(v *LocalShadowFakeRunV1) { v.OutboundActionCount = 1 },
		"phone farm":  func(v *LocalShadowFakeRunV1) { v.PhoneFarmAccessCount = 1 },
		"role":        func(v *LocalShadowFakeRunV1) { v.Receipts[1].RoleID = "other" },
		"harness":     func(v *LocalShadowFakeRunV1) { v.Receipts[1].HarnessArtifactIDs[0] = "other" },
		"side effect": func(v *LocalShadowFakeRunV1) { v.Receipts[1].SideEffects = true },
		"missing":     func(v *LocalShadowFakeRunV1) { v.Receipts = v.Receipts[:len(v.Receipts)-1] },
		"duplicate":   func(v *LocalShadowFakeRunV1) { v.Receipts[1].ReceiptID = v.Receipts[0].ReceiptID },
		"replay":      func(v *LocalShadowFakeRunV1) { v.Receipts[1].ReplayKey = v.Receipts[0].ReplayKey },
		"evidence":    func(v *LocalShadowFakeRunV1) { v.Receipts[0].Evidence[0].Fields["issue_id"] = "altered" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := cloneLocalShadowRunForTest(t, base)
			mutate(&candidate)
			if err := historicalContracts(t).ValidateLocalShadowFakeRunV1(suite, candidate); err == nil {
				t.Fatal("fake run alteration or replay was accepted")
			}
		})
	}
}

func decodeLocalShadowSuiteForTest(t *testing.T) LocalShadowWorkflowSuiteV1 {
	t.Helper()
	var suite LocalShadowWorkflowSuiteV1
	if err := strictJSON(historicalContracts(t).suiteJSON(), &suite); err != nil {
		t.Fatal(err)
	}
	return suite
}

func cloneLocalShadowSuiteForTest(t *testing.T, source LocalShadowWorkflowSuiteV1) LocalShadowWorkflowSuiteV1 {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone LocalShadowWorkflowSuiteV1
	if err := strictJSON(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func cloneLocalShadowRunForTest(t *testing.T, source LocalShadowFakeRunV1) LocalShadowFakeRunV1 {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone LocalShadowFakeRunV1
	if err := strictJSON(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
