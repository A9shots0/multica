package agentqualification

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const Item14StewardshipSchemaV1 = "silver-mediums.item14-stewardship.v1"

type Item14StewardshipSourceV1 struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	EvidenceClass string `json:"evidence_class"`
	Mutable       bool   `json:"mutable"`
}

type Item14StewardshipManifestV1 struct {
	SchemaVersion               string                      `json:"schema_version"`
	ManifestID                  string                      `json:"manifest_id"`
	Status                      string                      `json:"status"`
	ProviderCallsAllowed        bool                        `json:"provider_calls_allowed"`
	CredentialReadsAllowed      bool                        `json:"credential_reads_allowed"`
	ProductionActionsAllowed    bool                        `json:"production_actions_allowed"`
	ModelOrSeatSelectionAllowed bool                        `json:"model_or_seat_selection_allowed"`
	Sources                     []Item14StewardshipSourceV1 `json:"sources"`
}

type Item14StewardshipVerificationV1 struct {
	SchemaVersion         string `json:"schema_version"`
	ManifestSHA256        string `json:"manifest_sha256"`
	SourceCount           int    `json:"source_count"`
	FrozenAuthorityCount  int    `json:"frozen_authority_count"`
	MachineContractCount  int    `json:"machine_contract_count"`
	AcceptedEvidenceCount int    `json:"accepted_evidence_count"`
	CurrentStatusCount    int    `json:"current_status_count"`
	ProviderRequests      int    `json:"provider_requests"`
	CredentialReads       int    `json:"credential_reads"`
	ProductionActions     int    `json:"production_actions"`
	ModelOrSeatSelections int    `json:"model_or_seat_selections"`
	Valid                 bool   `json:"valid"`
}

//go:embed item14_stewardship_manifest_v1.json
var item14StewardshipManifestV1JSON []byte

func DecodeItem14StewardshipManifestV1(raw []byte) (Item14StewardshipManifestV1, error) {
	var manifest Item14StewardshipManifestV1
	if err := strictJSON(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("decode Item 14 stewardship manifest: %w", err)
	}
	if err := ValidateItem14StewardshipManifestV1(manifest); err != nil {
		return Item14StewardshipManifestV1{}, err
	}
	return manifest, nil
}

func CurrentItem14StewardshipManifestV1() (Item14StewardshipManifestV1, error) {
	return DecodeItem14StewardshipManifestV1(item14StewardshipManifestV1JSON)
}

func Item14StewardshipManifestV1SHA256() string {
	return item14StewardshipSHA256V1(item14StewardshipManifestV1JSON)
}

func ValidateItem14StewardshipManifestV1(manifest Item14StewardshipManifestV1) error {
	if manifest.SchemaVersion != Item14StewardshipSchemaV1 ||
		manifest.ManifestID != "item-14-role-squad-current-authority-20260914" ||
		manifest.Status != "accepted_provider_free_candidate" {
		return fmt.Errorf("Item 14 stewardship identity drifted")
	}
	if manifest.ProviderCallsAllowed || manifest.CredentialReadsAllowed || manifest.ProductionActionsAllowed || manifest.ModelOrSeatSelectionAllowed {
		return fmt.Errorf("Item 14 stewardship expanded authority")
	}
	want := requiredItem14StewardshipSourcesV1()
	if len(manifest.Sources) != len(want) {
		return fmt.Errorf("Item 14 stewardship source coverage is incomplete")
	}
	ids := make([]string, 0, len(manifest.Sources))
	seenPaths := make(map[string]struct{}, len(manifest.Sources))
	for _, source := range manifest.Sources {
		expected, ok := want[source.ID]
		if !ok || source.Path != expected.Path || source.EvidenceClass != expected.EvidenceClass || source.Mutable != expected.Mutable || !validSHA256(source.SHA256) {
			return fmt.Errorf("Item 14 stewardship source %s drifted", source.ID)
		}
		if path.Clean(source.Path) != source.Path || strings.HasPrefix(source.Path, "/") || strings.Contains(source.Path, "\\") || strings.Contains(source.Path, "..") {
			return fmt.Errorf("Item 14 stewardship source %s has unsafe path", source.ID)
		}
		if _, duplicate := seenPaths[source.Path]; duplicate {
			return fmt.Errorf("Item 14 stewardship path %s is duplicated", source.Path)
		}
		seenPaths[source.Path] = struct{}{}
		ids = append(ids, source.ID)
	}
	if !slices.IsSorted(ids) {
		return fmt.Errorf("Item 14 stewardship sources are not canonical")
	}
	return nil
}

func VerifyItem14StewardshipV1(sourceFS fs.FS) (Item14StewardshipVerificationV1, error) {
	manifest, err := CurrentItem14StewardshipManifestV1()
	if err != nil {
		return Item14StewardshipVerificationV1{}, err
	}
	return VerifyItem14StewardshipManifestV1(manifest, sourceFS)
}

func VerifyItem14StewardshipManifestV1(manifest Item14StewardshipManifestV1, sourceFS fs.FS) (Item14StewardshipVerificationV1, error) {
	if err := ValidateItem14StewardshipManifestV1(manifest); err != nil {
		return Item14StewardshipVerificationV1{}, err
	}
	result := Item14StewardshipVerificationV1{
		SchemaVersion:  Item14StewardshipSchemaV1,
		ManifestSHA256: Item14StewardshipManifestV1SHA256(),
		SourceCount:    len(manifest.Sources),
	}
	for _, source := range manifest.Sources {
		raw, err := fs.ReadFile(sourceFS, source.Path)
		if err != nil {
			return Item14StewardshipVerificationV1{}, fmt.Errorf("read Item 14 stewardship source %s: %w", source.ID, err)
		}
		actual := item14StewardshipSHA256V1(raw)
		if actual != source.SHA256 {
			return Item14StewardshipVerificationV1{}, fmt.Errorf("Item 14 stewardship source %s hash mismatch: expected %s, actual %s", source.ID, source.SHA256, actual)
		}
		switch source.EvidenceClass {
		case "frozen_authority":
			result.FrozenAuthorityCount++
		case "machine_contract":
			result.MachineContractCount++
		case "accepted_evidence":
			result.AcceptedEvidenceCount++
		case "current_status":
			result.CurrentStatusCount++
		default:
			return Item14StewardshipVerificationV1{}, fmt.Errorf("Item 14 stewardship source %s has unknown evidence class", source.ID)
		}
	}
	if result.FrozenAuthorityCount != 1 || result.MachineContractCount != 13 || result.AcceptedEvidenceCount != 17 || result.CurrentStatusCount != 3 {
		return Item14StewardshipVerificationV1{}, fmt.Errorf("Item 14 stewardship evidence-class coverage drifted")
	}
	result.Valid = true
	return result, nil
}

func requiredItem14StewardshipSourcesV1() map[string]Item14StewardshipSourceV1 {
	return map[string]Item14StewardshipSourceV1{
		"benchmark-closure":                         {Path: "docs/silver-mediums/ITEM-14-BENCHMARK-CLOSURE.md", EvidenceClass: "accepted_evidence"},
		"codex-subscription-boundary":               {Path: "docs/silver-mediums/ITEM-14-CODEX-SUBSCRIPTION-BOUNDARY.md", EvidenceClass: "accepted_evidence"},
		"current-continuation":                      {Path: "docs/silver-mediums/CURRENT-CONTINUATION.md", EvidenceClass: "current_status", Mutable: true},
		"documentation-stewardship":                 {Path: "docs/silver-mediums/ITEM-14-DOCUMENTATION-STEWARDSHIP-V1.md", EvidenceClass: "accepted_evidence"},
		"exact-model-transport":                     {Path: "docs/silver-mediums/ITEM-14-EXACT-MODEL-TRANSPORT-V1.md", EvidenceClass: "accepted_evidence"},
		"final-configuration-decision":              {Path: "docs/silver-mediums/ITEM-14-FINAL-CONFIGURATION-DECISION.md", EvidenceClass: "accepted_evidence"},
		"local-shadow-workflow-suite":               {Path: "docs/silver-mediums/ITEM-14-LOCAL-SHADOW-WORKFLOW-SUITE-V1.md", EvidenceClass: "accepted_evidence"},
		"openrouter-direct-shadow-v4":               {Path: "docs/silver-mediums/ITEM-14-OPENROUTER-DIRECT-SHADOW-V4.md", EvidenceClass: "accepted_evidence"},
		"provider-boundary-v2":                      {Path: "docs/silver-mediums/ITEM-14-PROVIDER-BOUNDARY-V2.md", EvidenceClass: "accepted_evidence"},
		"primary-subscription-trial-decision":       {Path: "docs/silver-mediums/ITEM-14-PRIMARY-SUBSCRIPTION-ROLE-TRIAL-DECISION.md", EvidenceClass: "accepted_evidence"},
		"primary-subscription-trial-definition":     {Path: "server/internal/agentqualification/primary_subscription_role_trial_v1.json", EvidenceClass: "machine_contract"},
		"primary-subscription-trial-gate":           {Path: "server/internal/agentqualification/primary_subscription_role_trial_gate_v1.json", EvidenceClass: "machine_contract"},
		"primary-subscription-trial-result-doc":     {Path: "docs/silver-mediums/ITEM-14-PRIMARY-SUBSCRIPTION-ROLE-TRIAL-RESULT.md", EvidenceClass: "accepted_evidence"},
		"primary-subscription-trial-result-machine": {Path: "server/internal/agentqualification/primary_subscription_role_trial_result_v1.json", EvidenceClass: "machine_contract"},
		"provisional-topology":                      {Path: "docs/silver-mediums/ITEM-14-PROVISIONAL-TOPOLOGY.md", EvidenceClass: "accepted_evidence"},
		"qualification-cases":                       {Path: "server/internal/agentqualification/cases.json", EvidenceClass: "machine_contract"},
		"responsibility-audit":                      {Path: "docs/silver-mediums/ITEM-14-RESPONSIBILITY-AUDIT.md", EvidenceClass: "accepted_evidence"},
		"role-architecture-evaluation":              {Path: "docs/silver-mediums/ITEM-14-ROLE-ARCHITECTURE-EVALUATION.md", EvidenceClass: "accepted_evidence"},
		"role-artifact-bindings-v2":                 {Path: "server/internal/agentqualification/role_artifact_bindings_v2.json", EvidenceClass: "machine_contract"},
		"role-artifact-manifest-v1":                 {Path: "server/internal/agentqualification/role_qualification_artifact_manifest_v1.json", EvidenceClass: "machine_contract"},
		"role-build-progress":                       {Path: "docs/silver-mediums/ITEM-14-ROLE-BUILD-PROGRESS.md", EvidenceClass: "current_status", Mutable: true},
		"role-catalog":                              {Path: "server/internal/systemgraph/roles.json", EvidenceClass: "machine_contract"},
		"role-design-decision-register":             {Path: "docs/silver-mediums/ITEM-14-ROLE-DESIGN-DECISION-REGISTER.md", EvidenceClass: "frozen_authority"},
		"role-instruction-contract":                 {Path: "docs/silver-mediums/ITEM-14-ROLE-INSTRUCTION-CONTRACT.md", EvidenceClass: "accepted_evidence"},
		"role-squad-contract":                       {Path: "server/internal/agentqualification/role_squad_operating_contract_v1.json", EvidenceClass: "machine_contract"},
		"role-squad-doc":                            {Path: "docs/silver-mediums/ITEM-14-ROLE-SQUAD-OPERATING-CONTRACT-V1.md", EvidenceClass: "accepted_evidence"},
		"role-squad-rehearsal-doc":                  {Path: "docs/silver-mediums/ITEM-14-ROLE-SQUAD-REHEARSAL-V1.md", EvidenceClass: "accepted_evidence"},
		"role-squad-rehearsal-result":               {Path: "server/internal/agentqualification/role_squad_rehearsal_v1_result.json", EvidenceClass: "machine_contract"},
		"role-squad-result":                         {Path: "server/internal/agentqualification/role_squad_operating_contract_v1_result.json", EvidenceClass: "machine_contract"},
		"role-stage-contract":                       {Path: "server/internal/agentqualification/role_stage_evidence_contract_v1.json", EvidenceClass: "machine_contract"},
		"role-stage-contract-doc":                   {Path: "docs/silver-mediums/ITEM-14-ROLE-STAGE-EVIDENCE-CONTRACT-V1.md", EvidenceClass: "accepted_evidence"},
		"role-stage-rehearsal-result":               {Path: "server/internal/agentqualification/role_stage_evidence_rehearsal_v1_result.json", EvidenceClass: "machine_contract"},
		"run-journal":                               {Path: "docs/silver-mediums/RUN-JOURNAL.md", EvidenceClass: "current_status", Mutable: true},
		"system-graph":                              {Path: "server/internal/systemgraph/default_graph.json", EvidenceClass: "machine_contract"},
	}
}

func item14StewardshipSHA256V1(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
