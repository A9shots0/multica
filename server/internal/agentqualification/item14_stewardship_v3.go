package agentqualification

import (
	_ "embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const Item14StewardshipSchemaV3 = "silver-mediums.item14-stewardship.v3"

const item14StewardshipPredecessorManifestPathV3 = "server/internal/agentqualification/item14_stewardship_manifest_v2.json"

type Item14StewardshipManifestV3 struct {
	SchemaVersion                     string                      `json:"schema_version"`
	ManifestID                        string                      `json:"manifest_id"`
	Status                            string                      `json:"status"`
	PredecessorManifestSHA256         string                      `json:"predecessor_manifest_sha256"`
	ProviderCallsAllowed              bool                        `json:"provider_calls_allowed"`
	CredentialReadsAllowed            bool                        `json:"credential_reads_allowed"`
	ProductionActionsAllowed          bool                        `json:"production_actions_allowed"`
	ModelOrSeatSelectionAllowed       bool                        `json:"model_or_seat_selection_allowed"`
	AutomaticDispatchAllowed          bool                        `json:"automatic_dispatch_allowed"`
	PhoneFarmWindowsAccessAllowed     bool                        `json:"phone_farm_windows_access_allowed"`
	Retries                           int                         `json:"retries"`
	Fallbacks                         bool                        `json:"fallbacks"`
	ProvisionalLocalOperatingDefaults int                         `json:"provisional_local_operating_defaults"`
	PermanentConfigurations           int                         `json:"permanent_configurations"`
	Sources                           []Item14StewardshipSourceV1 `json:"sources"`
}

type Item14StewardshipVerificationV3 struct {
	SchemaVersion                   string `json:"schema_version"`
	ManifestSHA256                  string `json:"manifest_sha256"`
	PredecessorManifestSHA256       string `json:"predecessor_manifest_sha256"`
	PredecessorImmutableSourceCount int    `json:"predecessor_immutable_source_count"`
	ExtensionSourceCount            int    `json:"extension_source_count"`
	SourceCount                     int    `json:"source_count"`
	FrozenAuthorityCount            int    `json:"frozen_authority_count"`
	MachineContractCount            int    `json:"machine_contract_count"`
	AcceptedEvidenceCount           int    `json:"accepted_evidence_count"`
	CurrentStatusCount              int    `json:"current_status_count"`
	ProvisionalLocalDefaults        int    `json:"provisional_local_defaults"`
	PermanentConfigurations         int    `json:"permanent_configurations"`
	ProviderRequests                int    `json:"provider_requests"`
	CredentialReads                 int    `json:"credential_reads"`
	ProductionActions               int    `json:"production_actions"`
	ModelOrSeatSelections           int    `json:"model_or_seat_selections"`
	AutomaticDispatches             int    `json:"automatic_dispatches"`
	PhoneFarmWindowsAccesses        int    `json:"phone_farm_windows_accesses"`
	Retries                         int    `json:"retries"`
	Fallbacks                       int    `json:"fallbacks"`
	Valid                           bool   `json:"valid"`
}

//go:embed item14_stewardship_manifest_v3.json
var item14StewardshipManifestV3JSON []byte

func DecodeItem14StewardshipManifestV3(raw []byte) (Item14StewardshipManifestV3, error) {
	var manifest Item14StewardshipManifestV3
	if err := strictJSON(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("decode Item 14 stewardship v3 manifest: %w", err)
	}
	if err := ValidateItem14StewardshipManifestV3(manifest); err != nil {
		return Item14StewardshipManifestV3{}, err
	}
	return manifest, nil
}

func CurrentItem14StewardshipManifestV3() (Item14StewardshipManifestV3, error) {
	return DecodeItem14StewardshipManifestV3(item14StewardshipManifestV3JSON)
}

func Item14StewardshipManifestV3SHA256() string {
	return item14StewardshipSHA256V1(item14StewardshipManifestV3JSON)
}

func ValidateItem14StewardshipManifestV3(manifest Item14StewardshipManifestV3) error {
	if manifest.SchemaVersion != Item14StewardshipSchemaV3 || manifest.ManifestID != "item-14-reversible-local-default-successor-20260915" || manifest.Status != "accepted_reversible_local_default_successor_chain" {
		return fmt.Errorf("Item 14 stewardship v3 identity drifted")
	}
	if manifest.PredecessorManifestSHA256 != Item14StewardshipManifestV2SHA256() {
		return fmt.Errorf("Item 14 stewardship v3 predecessor drifted")
	}
	if manifest.ProviderCallsAllowed || manifest.CredentialReadsAllowed || manifest.ProductionActionsAllowed || manifest.ModelOrSeatSelectionAllowed || manifest.AutomaticDispatchAllowed || manifest.PhoneFarmWindowsAccessAllowed || manifest.Retries != 0 || manifest.Fallbacks || manifest.ProvisionalLocalOperatingDefaults != 1 || manifest.PermanentConfigurations != 0 {
		return fmt.Errorf("Item 14 stewardship v3 expanded authority")
	}
	want := requiredItem14StewardshipSourcesV3()
	if len(manifest.Sources) != len(want) {
		return fmt.Errorf("Item 14 stewardship v3 source coverage is incomplete")
	}
	ids := make([]string, 0, len(manifest.Sources))
	seenPaths := make(map[string]struct{}, len(manifest.Sources))
	for _, source := range manifest.Sources {
		expected, ok := want[source.ID]
		if !ok || source.Path != expected.Path || source.EvidenceClass != expected.EvidenceClass || source.Mutable != expected.Mutable || !validSHA256(source.SHA256) {
			return fmt.Errorf("Item 14 stewardship v3 source %s drifted", source.ID)
		}
		if path.Clean(source.Path) != source.Path || strings.HasPrefix(source.Path, "/") || strings.Contains(source.Path, "\\") || strings.Contains(source.Path, "..") {
			return fmt.Errorf("Item 14 stewardship v3 source %s has unsafe path", source.ID)
		}
		if _, duplicate := seenPaths[source.Path]; duplicate {
			return fmt.Errorf("Item 14 stewardship v3 path %s is duplicated", source.Path)
		}
		seenPaths[source.Path] = struct{}{}
		ids = append(ids, source.ID)
	}
	if !slices.IsSorted(ids) {
		return fmt.Errorf("Item 14 stewardship v3 sources are not canonical")
	}
	return nil
}

func VerifyItem14StewardshipV3(sourceFS fs.FS) (Item14StewardshipVerificationV3, error) {
	manifest, err := CurrentItem14StewardshipManifestV3()
	if err != nil {
		return Item14StewardshipVerificationV3{}, err
	}
	return VerifyItem14StewardshipManifestV3(manifest, sourceFS)
}

func VerifyItem14StewardshipManifestV3(manifest Item14StewardshipManifestV3, sourceFS fs.FS) (Item14StewardshipVerificationV3, error) {
	if err := ValidateItem14StewardshipManifestV3(manifest); err != nil {
		return Item14StewardshipVerificationV3{}, err
	}
	predecessorV2, err := verifyItem14StewardshipPredecessorManifestV3(sourceFS, manifest.PredecessorManifestSHA256)
	if err != nil {
		return Item14StewardshipVerificationV3{}, err
	}
	predecessorV1, err := verifyItem14StewardshipPredecessorManifestV2(sourceFS, predecessorV2.PredecessorManifestSHA256)
	if err != nil {
		return Item14StewardshipVerificationV3{}, err
	}
	result := Item14StewardshipVerificationV3{
		SchemaVersion: Item14StewardshipSchemaV3, ManifestSHA256: Item14StewardshipManifestV3SHA256(), PredecessorManifestSHA256: manifest.PredecessorManifestSHA256,
		ExtensionSourceCount: len(manifest.Sources), ProvisionalLocalDefaults: manifest.ProvisionalLocalOperatingDefaults, PermanentConfigurations: manifest.PermanentConfigurations,
	}
	seenPaths := make(map[string]struct{}, len(predecessorV1.Sources)+len(predecessorV2.Sources)+len(manifest.Sources))
	verifyPredecessor := func(sources []Item14StewardshipSourceV1) error {
		for _, source := range sources {
			if source.Mutable {
				continue
			}
			if _, duplicate := seenPaths[source.Path]; duplicate {
				return fmt.Errorf("Item 14 stewardship v3 predecessor path duplicated: %s", source.Path)
			}
			if err := verifyItem14StewardshipSourceV2(source, sourceFS); err != nil {
				return err
			}
			seenPaths[source.Path] = struct{}{}
			result.PredecessorImmutableSourceCount++
			countItem14StewardshipEvidenceClassV3(&result, source.EvidenceClass)
		}
		return nil
	}
	if err := verifyPredecessor(predecessorV1.Sources); err != nil {
		return Item14StewardshipVerificationV3{}, err
	}
	if err := verifyPredecessor(predecessorV2.Sources); err != nil {
		return Item14StewardshipVerificationV3{}, err
	}
	for _, source := range manifest.Sources {
		if _, duplicate := seenPaths[source.Path]; duplicate {
			return Item14StewardshipVerificationV3{}, fmt.Errorf("Item 14 stewardship v3 source path duplicates predecessor: %s", source.Path)
		}
		if err := verifyItem14StewardshipSourceV2(source, sourceFS); err != nil {
			return Item14StewardshipVerificationV3{}, err
		}
		seenPaths[source.Path] = struct{}{}
		countItem14StewardshipEvidenceClassV3(&result, source.EvidenceClass)
	}
	result.SourceCount = result.PredecessorImmutableSourceCount + result.ExtensionSourceCount
	if result.PredecessorImmutableSourceCount != 44 || result.ExtensionSourceCount != 16 || result.SourceCount != 60 || result.FrozenAuthorityCount != 2 || result.MachineContractCount != 28 || result.AcceptedEvidenceCount != 25 || result.CurrentStatusCount != 5 {
		return Item14StewardshipVerificationV3{}, fmt.Errorf("Item 14 stewardship v3 evidence-class coverage drifted")
	}
	result.Valid = true
	return result, nil
}

func verifyItem14StewardshipPredecessorManifestV3(sourceFS fs.FS, expectedSHA256 string) (Item14StewardshipManifestV2, error) {
	raw, err := fs.ReadFile(sourceFS, item14StewardshipPredecessorManifestPathV3)
	if err != nil {
		return Item14StewardshipManifestV2{}, fmt.Errorf("read Item 14 stewardship v2 predecessor manifest: %w", err)
	}
	if actual := item14StewardshipSHA256V1(raw); actual != expectedSHA256 {
		return Item14StewardshipManifestV2{}, fmt.Errorf("Item 14 stewardship v2 predecessor manifest hash mismatch: expected %s, actual %s", expectedSHA256, actual)
	}
	return DecodeItem14StewardshipManifestV2(raw)
}

func countItem14StewardshipEvidenceClassV3(result *Item14StewardshipVerificationV3, evidenceClass string) {
	switch evidenceClass {
	case "frozen_authority":
		result.FrozenAuthorityCount++
	case "machine_contract":
		result.MachineContractCount++
	case "accepted_evidence":
		result.AcceptedEvidenceCount++
	case "current_status":
		result.CurrentStatusCount++
	}
}

func requiredItem14StewardshipSourcesV3() map[string]Item14StewardshipSourceV1 {
	return map[string]Item14StewardshipSourceV1{
		"current-continuation":                   {Path: "docs/silver-mediums/CURRENT-CONTINUATION.md", EvidenceClass: "current_status", Mutable: true},
		"documentation-stewardship-v3":           {Path: "docs/silver-mediums/ITEM-14-DOCUMENTATION-STEWARDSHIP-V3.md", EvidenceClass: "accepted_evidence"},
		"final-configuration-decision-v2":        {Path: "docs/silver-mediums/ITEM-14-FINAL-CONFIGURATION-DECISION-V2.md", EvidenceClass: "current_status", Mutable: true},
		"integration-pilot-progress":             {Path: "docs/silver-mediums/ITEM-14-INTEGRATION-PILOT-PROGRESS.md", EvidenceClass: "current_status", Mutable: true},
		"integration-replay-evidence":            {Path: ".qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/EVIDENCE.md", EvidenceClass: "accepted_evidence"},
		"integration-replay-inventory":           {Path: ".qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/inventory.json", EvidenceClass: "machine_contract"},
		"integration-replay-launch-contract":     {Path: ".qualification-evidence/item14-integration-replay-20260915-v3/EXACT-LAUNCH-CONTRACT.json", EvidenceClass: "machine_contract"},
		"integration-replay-result-machine":      {Path: "server/internal/agentqualification/integration_replay_result_v1.json", EvidenceClass: "machine_contract"},
		"integration-replay-windows-report":      {Path: ".qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md", EvidenceClass: "accepted_evidence"},
		"provisional-configuration-doc-v2":       {Path: "docs/silver-mediums/ITEM-14-PROVISIONAL-ROLE-CONFIGURATION-V2.md", EvidenceClass: "accepted_evidence"},
		"provisional-configuration-lifecycle-v2": {Path: "server/internal/agentqualification/provisional_role_configuration_lifecycle_v2.json", EvidenceClass: "machine_contract"},
		"provisional-configuration-v2":           {Path: "server/internal/agentqualification/provisional_role_configuration_v2.json", EvidenceClass: "machine_contract"},
		"provisional-rehearsal-v2":               {Path: "server/internal/agentqualification/provisional_role_configuration_rehearsal_v2_result.json", EvidenceClass: "machine_contract"},
		"role-build-progress":                    {Path: "docs/silver-mediums/ITEM-14-ROLE-BUILD-PROGRESS.md", EvidenceClass: "current_status", Mutable: true},
		"role-squad-readiness-v3":                {Path: "server/internal/agentqualification/role_squad_readiness_matrix_v3.json", EvidenceClass: "machine_contract"},
		"run-journal":                            {Path: "docs/silver-mediums/RUN-JOURNAL.md", EvidenceClass: "current_status", Mutable: true},
	}
}
