package agentqualification

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestItem14StewardshipV3VerifiesReversibleLocalDefaultChain(t *testing.T) {
	result, err := VerifyItem14StewardshipV3(stewardshipArchiveFixture(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.PredecessorImmutableSourceCount != 44 || result.ExtensionSourceCount != 16 || result.SourceCount != 60 || result.FrozenAuthorityCount != 2 || result.MachineContractCount != 28 || result.AcceptedEvidenceCount != 25 || result.CurrentStatusCount != 5 || result.ProvisionalLocalDefaults != 1 || result.PermanentConfigurations != 0 {
		t.Fatalf("unexpected Item 14 stewardship v3 result: %+v", result)
	}
	if result.ProviderRequests != 0 || result.CredentialReads != 0 || result.ProductionActions != 0 || result.ModelOrSeatSelections != 0 || result.AutomaticDispatches != 0 || result.PhoneFarmWindowsAccesses != 0 || result.Retries != 0 || result.Fallbacks != 0 {
		t.Fatalf("stewardship v3 verification expanded authority: %+v", result)
	}
}

func TestItem14StewardshipV3RejectsDefinitionMutations(t *testing.T) {
	base, err := CurrentItem14StewardshipManifestV3()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Item14StewardshipManifestV3){
		"predecessor": func(v *Item14StewardshipManifestV3) { v.PredecessorManifestSHA256 = strings.Repeat("0", 64) },
		"provider":    func(v *Item14StewardshipManifestV3) { v.ProviderCallsAllowed = true },
		"credential":  func(v *Item14StewardshipManifestV3) { v.CredentialReadsAllowed = true },
		"selection":   func(v *Item14StewardshipManifestV3) { v.ModelOrSeatSelectionAllowed = true },
		"dispatch":    func(v *Item14StewardshipManifestV3) { v.AutomaticDispatchAllowed = true },
		"phone-farm":  func(v *Item14StewardshipManifestV3) { v.PhoneFarmWindowsAccessAllowed = true },
		"retry":       func(v *Item14StewardshipManifestV3) { v.Retries = 1 },
		"fallback":    func(v *Item14StewardshipManifestV3) { v.Fallbacks = true },
		"defaults":    func(v *Item14StewardshipManifestV3) { v.ProvisionalLocalOperatingDefaults = 2 },
		"permanent":   func(v *Item14StewardshipManifestV3) { v.PermanentConfigurations = 1 },
		"missing":     func(v *Item14StewardshipManifestV3) { v.Sources = v.Sources[1:] },
		"path":        func(v *Item14StewardshipManifestV3) { v.Sources[0].Path = "../outside" },
		"class":       func(v *Item14StewardshipManifestV3) { v.Sources[0].EvidenceClass = "machine_contract" },
		"mutable":     func(v *Item14StewardshipManifestV3) { v.Sources[0].Mutable = false },
		"duplicate":   func(v *Item14StewardshipManifestV3) { v.Sources[1] = v.Sources[0] },
		"order": func(v *Item14StewardshipManifestV3) {
			v.Sources[0], v.Sources[1] = v.Sources[1], v.Sources[0]
		},
	} {
		t.Run(name, func(t *testing.T) {
			clone := cloneItem14StewardshipManifestV3(t, base)
			mutate(&clone)
			if err := ValidateItem14StewardshipManifestV3(clone); err == nil {
				t.Fatal("mutated Item 14 stewardship v3 manifest was accepted")
			}
		})
	}
}

func TestItem14StewardshipV3RejectsBoundSourceHashDrift(t *testing.T) {
	manifest, err := CurrentItem14StewardshipManifestV3()
	if err != nil {
		t.Fatal(err)
	}
	files := stewardshipArchiveFixture(t, 3)
	if _, err := VerifyItem14StewardshipManifestV3(manifest, files); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	sourceID := manifest.Sources[0].ID
	manifest.Sources[0].SHA256 = strings.Repeat("0", 64)
	if _, err := VerifyItem14StewardshipManifestV3(manifest, files); err == nil || !strings.Contains(err.Error(), sourceID+" hash mismatch") {
		t.Fatalf("targeted source drift rejection: %v", err)
	}
}

func TestItem14StewardshipV3RejectsOnDiskPredecessorManifestDrift(t *testing.T) {
	files := fstest.MapFS{
		item14StewardshipPredecessorManifestPathV3: &fstest.MapFile{Data: append([]byte(nil), item14StewardshipManifestV2JSON...)},
	}
	if _, err := verifyItem14StewardshipPredecessorManifestV3(files, Item14StewardshipManifestV2SHA256()); err != nil {
		t.Fatal(err)
	}
	files[item14StewardshipPredecessorManifestPathV3].Data[0] = 'X'
	if _, err := verifyItem14StewardshipPredecessorManifestV3(files, Item14StewardshipManifestV2SHA256()); err == nil {
		t.Fatal("changed on-disk v2 predecessor manifest was accepted")
	}
}

func TestItem14StewardshipV3RejectsUnknownFields(t *testing.T) {
	injected := strings.Replace(string(item14StewardshipManifestV3JSON), `"status":`, `"unexpected":true,"status":`, 1)
	if _, err := DecodeItem14StewardshipManifestV3([]byte(injected)); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

func cloneItem14StewardshipManifestV3(t *testing.T, value Item14StewardshipManifestV3) Item14StewardshipManifestV3 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone Item14StewardshipManifestV3
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
