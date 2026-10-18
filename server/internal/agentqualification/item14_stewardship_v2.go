package agentqualification

import (
	_ "embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const Item14StewardshipSchemaV2 = "silver-mediums.item14-stewardship.v2"

const item14StewardshipPredecessorManifestPathV2 = "server/internal/agentqualification/item14_stewardship_manifest_v1.json"

type Item14StewardshipManifestV2 struct {
	SchemaVersion                 string                      `json:"schema_version"`
	ManifestID                    string                      `json:"manifest_id"`
	Status                        string                      `json:"status"`
	PredecessorManifestSHA256     string                      `json:"predecessor_manifest_sha256"`
	ProviderCallsAllowed          bool                        `json:"provider_calls_allowed"`
	CredentialReadsAllowed        bool                        `json:"credential_reads_allowed"`
	ProductionActionsAllowed      bool                        `json:"production_actions_allowed"`
	ModelOrSeatSelectionAllowed   bool                        `json:"model_or_seat_selection_allowed"`
	AutomaticDispatchAllowed      bool                        `json:"automatic_dispatch_allowed"`
	PhoneFarmWindowsAccessAllowed bool                        `json:"phone_farm_windows_access_allowed"`
	Retries                       int                         `json:"retries"`
	Fallbacks                     bool                        `json:"fallbacks"`
	Sources                       []Item14StewardshipSourceV1 `json:"sources"`
}

type Item14StewardshipVerificationV2 struct {
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

//go:embed item14_stewardship_manifest_v2.json
var item14StewardshipManifestV2JSON []byte

func DecodeItem14StewardshipManifestV2(raw []byte) (Item14StewardshipManifestV2, error) {
	var manifest Item14StewardshipManifestV2
	if err := strictJSON(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("decode Item 14 stewardship v2 manifest: %w", err)
	}
	if err := ValidateItem14StewardshipManifestV2(manifest); err != nil {
		return Item14StewardshipManifestV2{}, err
	}
	return manifest, nil
}

func CurrentItem14StewardshipManifestV2() (Item14StewardshipManifestV2, error) {
	return DecodeItem14StewardshipManifestV2(item14StewardshipManifestV2JSON)
}

func Item14StewardshipManifestV2SHA256() string {
	return item14StewardshipSHA256V1(item14StewardshipManifestV2JSON)
}

func ValidateItem14StewardshipManifestV2(manifest Item14StewardshipManifestV2) error {
	if manifest.SchemaVersion != Item14StewardshipSchemaV2 ||
		manifest.ManifestID != "item-14-role-squad-successor-authority-20260914" ||
		manifest.Status != "accepted_provider_free_successor_chain" {
		return fmt.Errorf("Item 14 stewardship v2 identity drifted")
	}
	if manifest.PredecessorManifestSHA256 != Item14StewardshipManifestV1SHA256() {
		return fmt.Errorf("Item 14 stewardship v2 predecessor drifted")
	}
	if manifest.ProviderCallsAllowed || manifest.CredentialReadsAllowed || manifest.ProductionActionsAllowed ||
		manifest.ModelOrSeatSelectionAllowed || manifest.AutomaticDispatchAllowed ||
		manifest.PhoneFarmWindowsAccessAllowed || manifest.Retries != 0 || manifest.Fallbacks {
		return fmt.Errorf("Item 14 stewardship v2 expanded authority")
	}
	want := requiredItem14StewardshipSourcesV2()
	if len(manifest.Sources) != len(want) {
		return fmt.Errorf("Item 14 stewardship v2 source coverage is incomplete")
	}
	ids := make([]string, 0, len(manifest.Sources))
	seenPaths := make(map[string]struct{}, len(manifest.Sources))
	for _, source := range manifest.Sources {
		expected, ok := want[source.ID]
		if !ok || source.Path != expected.Path || source.EvidenceClass != expected.EvidenceClass ||
			source.Mutable != expected.Mutable || !validSHA256(source.SHA256) {
			return fmt.Errorf("Item 14 stewardship v2 source %s drifted", source.ID)
		}
		if path.Clean(source.Path) != source.Path || strings.HasPrefix(source.Path, "/") ||
			strings.Contains(source.Path, "\\") || strings.Contains(source.Path, "..") {
			return fmt.Errorf("Item 14 stewardship v2 source %s has unsafe path", source.ID)
		}
		if _, duplicate := seenPaths[source.Path]; duplicate {
			return fmt.Errorf("Item 14 stewardship v2 path %s is duplicated", source.Path)
		}
		seenPaths[source.Path] = struct{}{}
		ids = append(ids, source.ID)
	}
	if !slices.IsSorted(ids) {
		return fmt.Errorf("Item 14 stewardship v2 sources are not canonical")
	}
	return nil
}

func VerifyItem14StewardshipV2(sourceFS fs.FS) (Item14StewardshipVerificationV2, error) {
	manifest, err := CurrentItem14StewardshipManifestV2()
	if err != nil {
		return Item14StewardshipVerificationV2{}, err
	}
	return VerifyItem14StewardshipManifestV2(manifest, sourceFS)
}

func VerifyItem14StewardshipManifestV2(manifest Item14StewardshipManifestV2, sourceFS fs.FS) (Item14StewardshipVerificationV2, error) {
	if err := ValidateItem14StewardshipManifestV2(manifest); err != nil {
		return Item14StewardshipVerificationV2{}, err
	}
	predecessor, err := verifyItem14StewardshipPredecessorManifestV2(sourceFS, manifest.PredecessorManifestSHA256)
	if err != nil {
		return Item14StewardshipVerificationV2{}, err
	}
	result := Item14StewardshipVerificationV2{
		SchemaVersion:             Item14StewardshipSchemaV2,
		ManifestSHA256:            Item14StewardshipManifestV2SHA256(),
		PredecessorManifestSHA256: manifest.PredecessorManifestSHA256,
		ExtensionSourceCount:      len(manifest.Sources),
	}
	seenPaths := make(map[string]struct{}, len(predecessor.Sources)+len(manifest.Sources))
	for _, source := range predecessor.Sources {
		if source.Mutable {
			continue
		}
		if err := verifyItem14StewardshipSourceV2(source, sourceFS); err != nil {
			return Item14StewardshipVerificationV2{}, err
		}
		seenPaths[source.Path] = struct{}{}
		result.PredecessorImmutableSourceCount++
		countItem14StewardshipEvidenceClassV2(&result, source.EvidenceClass)
	}
	for _, source := range manifest.Sources {
		if _, duplicate := seenPaths[source.Path]; duplicate {
			return Item14StewardshipVerificationV2{}, fmt.Errorf("Item 14 stewardship v2 source path duplicates predecessor: %s", source.Path)
		}
		if err := verifyItem14StewardshipSourceV2(source, sourceFS); err != nil {
			return Item14StewardshipVerificationV2{}, err
		}
		seenPaths[source.Path] = struct{}{}
		countItem14StewardshipEvidenceClassV2(&result, source.EvidenceClass)
	}
	result.SourceCount = result.PredecessorImmutableSourceCount + result.ExtensionSourceCount
	if result.PredecessorImmutableSourceCount != 31 || result.ExtensionSourceCount != 16 || result.SourceCount != 47 ||
		result.FrozenAuthorityCount != 2 || result.MachineContractCount != 21 ||
		result.AcceptedEvidenceCount != 21 || result.CurrentStatusCount != 3 {
		return Item14StewardshipVerificationV2{}, fmt.Errorf("Item 14 stewardship v2 evidence-class coverage drifted")
	}
	result.Valid = true
	return result, nil
}

func verifyItem14StewardshipPredecessorManifestV2(sourceFS fs.FS, expectedSHA256 string) (Item14StewardshipManifestV1, error) {
	raw, err := fs.ReadFile(sourceFS, item14StewardshipPredecessorManifestPathV2)
	if err != nil {
		return Item14StewardshipManifestV1{}, fmt.Errorf("read Item 14 stewardship v1 predecessor manifest: %w", err)
	}
	if actual := item14StewardshipSHA256V1(raw); actual != expectedSHA256 {
		return Item14StewardshipManifestV1{}, fmt.Errorf("Item 14 stewardship v1 predecessor manifest hash mismatch: expected %s, actual %s", expectedSHA256, actual)
	}
	return DecodeItem14StewardshipManifestV1(raw)
}

func verifyItem14StewardshipSourceV2(source Item14StewardshipSourceV1, sourceFS fs.FS) error {
	raw, err := fs.ReadFile(sourceFS, source.Path)
	if err != nil {
		return fmt.Errorf("read Item 14 stewardship source %s: %w", source.ID, err)
	}
	actual := item14StewardshipSHA256V1(raw)
	if actual != source.SHA256 {
		return fmt.Errorf("Item 14 stewardship source %s hash mismatch: expected %s, actual %s", source.ID, source.SHA256, actual)
	}
	return nil
}

func countItem14StewardshipEvidenceClassV2(result *Item14StewardshipVerificationV2, evidenceClass string) {
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

func requiredItem14StewardshipSourcesV2() map[string]Item14StewardshipSourceV1 {
	return map[string]Item14StewardshipSourceV1{
		"current-continuation":                  {Path: "docs/silver-mediums/CURRENT-CONTINUATION.md", EvidenceClass: "current_status", Mutable: true},
		"documentation-stewardship-v2":          {Path: "docs/silver-mediums/ITEM-14-DOCUMENTATION-STEWARDSHIP-V2.md", EvidenceClass: "accepted_evidence"},
		"primary-reliability-authority-v2":      {Path: "docs/silver-mediums/ITEM-14-PRIMARY-SUBSCRIPTION-RELIABILITY-TRIAL-V2-AUTHORITY.md", EvidenceClass: "frozen_authority"},
		"primary-reliability-definition-v2":     {Path: "server/internal/agentqualification/primary_subscription_reliability_trial_v2.json", EvidenceClass: "machine_contract"},
		"primary-reliability-result-doc-v2":     {Path: "docs/silver-mediums/ITEM-14-PRIMARY-SUBSCRIPTION-RELIABILITY-TRIAL-V2-RESULT.md", EvidenceClass: "accepted_evidence"},
		"primary-reliability-result-machine-v2": {Path: "server/internal/agentqualification/primary_subscription_reliability_trial_result_v2.json", EvidenceClass: "machine_contract"},
		"product-ui-r02-contract-v3":            {Path: "server/internal/agentqualification/product_ui_r02_action_contract_v3.json", EvidenceClass: "machine_contract"},
		"product-ui-r02-correction-doc-v3":      {Path: "docs/silver-mediums/ITEM-14-PRODUCT-UI-R02-CORRECTION-CANDIDATE-V3.md", EvidenceClass: "accepted_evidence"},
		"product-ui-r02-executor-overlay-v3":    {Path: "server/internal/agentqualification/role_artifacts/v3/shared_windows_executor_r02.instructions.md", EvidenceClass: "machine_contract"},
		"product-ui-r02-skill-v3":               {Path: "server/internal/agentqualification/role_artifacts/v3/product_ui_r02.skill.md", EvidenceClass: "machine_contract"},
		"provisional-configuration-doc-v1":      {Path: "docs/silver-mediums/ITEM-14-PROVISIONAL-ROLE-CONFIGURATION-V1.md", EvidenceClass: "accepted_evidence"},
		"provisional-configuration-v1":          {Path: "server/internal/agentqualification/provisional_role_configuration_v1.json", EvidenceClass: "machine_contract"},
		"provisional-rehearsal-v1":              {Path: "server/internal/agentqualification/provisional_role_configuration_rehearsal_v1_result.json", EvidenceClass: "machine_contract"},
		"role-build-progress":                   {Path: "docs/silver-mediums/ITEM-14-ROLE-BUILD-PROGRESS.md", EvidenceClass: "current_status", Mutable: true},
		"role-squad-readiness-v2":               {Path: "server/internal/agentqualification/role_squad_readiness_matrix_v2.json", EvidenceClass: "machine_contract"},
		"run-journal":                           {Path: "docs/silver-mediums/RUN-JOURNAL.md", EvidenceClass: "current_status", Mutable: true},
	}
}
