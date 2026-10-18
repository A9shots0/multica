package agentqualification

import (
	"encoding/json"
	"fmt"
	"github.com/multica-ai/multica/server/internal/testutil"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestItem14StewardshipV2VerifiesImmutableChainWithExplicitSyntheticMutableStatus(t *testing.T) {
	// The three original mutable v2 status snapshots are unavailable. This
	// test covers the pure verifier with exact31+13 immutable originals and
	// explicitly synthetic current-status inputs; it does not certify old v2 custody.
	manifest, files := stewardshipV2ProtocolFixture(t)
	if _, err := VerifyItem14StewardshipV2(files); err == nil {
		t.Fatal("original historical wrapper admitted synthetic status snapshots")
	}
	result, err := VerifyItem14StewardshipManifestV2(manifest, files)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.PredecessorImmutableSourceCount != 31 || result.ExtensionSourceCount != 16 ||
		result.SourceCount != 47 || result.FrozenAuthorityCount != 2 || result.MachineContractCount != 21 ||
		result.AcceptedEvidenceCount != 21 || result.CurrentStatusCount != 3 {
		t.Fatalf("unexpected Item 14 stewardship v2 result: %+v", result)
	}
	if result.ProviderRequests != 0 || result.CredentialReads != 0 || result.ProductionActions != 0 ||
		result.ModelOrSeatSelections != 0 || result.AutomaticDispatches != 0 ||
		result.PhoneFarmWindowsAccesses != 0 || result.Retries != 0 || result.Fallbacks != 0 {
		t.Fatalf("stewardship v2 verification expanded authority: %+v", result)
	}
}

func TestItem14StewardshipV2RejectsDefinitionMutations(t *testing.T) {
	base, err := CurrentItem14StewardshipManifestV2()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Item14StewardshipManifestV2){
		"predecessor": func(v *Item14StewardshipManifestV2) { v.PredecessorManifestSHA256 = strings.Repeat("0", 64) },
		"provider":    func(v *Item14StewardshipManifestV2) { v.ProviderCallsAllowed = true },
		"credential":  func(v *Item14StewardshipManifestV2) { v.CredentialReadsAllowed = true },
		"selection":   func(v *Item14StewardshipManifestV2) { v.ModelOrSeatSelectionAllowed = true },
		"dispatch":    func(v *Item14StewardshipManifestV2) { v.AutomaticDispatchAllowed = true },
		"phone-farm":  func(v *Item14StewardshipManifestV2) { v.PhoneFarmWindowsAccessAllowed = true },
		"retry":       func(v *Item14StewardshipManifestV2) { v.Retries = 1 },
		"fallback":    func(v *Item14StewardshipManifestV2) { v.Fallbacks = true },
		"missing":     func(v *Item14StewardshipManifestV2) { v.Sources = v.Sources[1:] },
		"path":        func(v *Item14StewardshipManifestV2) { v.Sources[0].Path = "../outside" },
		"class":       func(v *Item14StewardshipManifestV2) { v.Sources[0].EvidenceClass = "machine_contract" },
		"mutable":     func(v *Item14StewardshipManifestV2) { v.Sources[0].Mutable = false },
		"duplicate":   func(v *Item14StewardshipManifestV2) { v.Sources[1] = v.Sources[0] },
		"order": func(v *Item14StewardshipManifestV2) {
			v.Sources[0], v.Sources[1] = v.Sources[1], v.Sources[0]
		},
	} {
		t.Run(name, func(t *testing.T) {
			clone := cloneItem14StewardshipManifestV2(t, base)
			mutate(&clone)
			if err := ValidateItem14StewardshipManifestV2(clone); err == nil {
				t.Fatal("mutated Item 14 stewardship v2 manifest was accepted")
			}
		})
	}
}

func TestItem14StewardshipV2RejectsBoundSourceHashDrift(t *testing.T) {
	manifest, files := stewardshipV2ProtocolFixture(t)
	if _, err := VerifyItem14StewardshipManifestV2(manifest, files); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	sourceID := manifest.Sources[0].ID
	manifest.Sources[0].SHA256 = strings.Repeat("0", 64)
	if _, err := VerifyItem14StewardshipManifestV2(manifest, files); err == nil || !strings.Contains(err.Error(), sourceID+" hash mismatch") {
		t.Fatalf("targeted source drift rejection: %v", err)
	}
}

func TestItem14StewardshipV2RejectsOnDiskPredecessorManifestDrift(t *testing.T) {
	files := fstest.MapFS{
		item14StewardshipPredecessorManifestPathV2: &fstest.MapFile{Data: append([]byte(nil), item14StewardshipManifestV1JSON...)},
	}
	if _, err := verifyItem14StewardshipPredecessorManifestV2(files, Item14StewardshipManifestV1SHA256()); err != nil {
		t.Fatal(err)
	}
	files[item14StewardshipPredecessorManifestPathV2].Data[0] = 'X'
	if _, err := verifyItem14StewardshipPredecessorManifestV2(files, Item14StewardshipManifestV1SHA256()); err == nil {
		t.Fatal("changed on-disk predecessor manifest was accepted")
	}
}

func TestItem14StewardshipV2RejectsUnknownFields(t *testing.T) {
	injected := strings.Replace(string(item14StewardshipManifestV2JSON), `"status":`, `"unexpected":true,"status":`, 1)
	if _, err := DecodeItem14StewardshipManifestV2([]byte(injected)); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

func cloneItem14StewardshipManifestV2(t *testing.T, value Item14StewardshipManifestV2) Item14StewardshipManifestV2 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone Item14StewardshipManifestV2
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func stewardshipArchiveFixture(t *testing.T, version int) fstest.MapFS {
	t.Helper()
	result := fstest.MapFS{}
	for i := 1; i <= version; i++ {
		source := testutil.QualificationArchiveFS(t, fmt.Sprintf("stewardship-v%d", i))
		if err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			raw, err := fs.ReadFile(source, name)
			if err != nil {
				return err
			}
			result[name] = &fstest.MapFile{Data: raw, Mode: 0600}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func stewardshipV2ProtocolFixture(t *testing.T) (Item14StewardshipManifestV2, fstest.MapFS) {
	t.Helper()
	files := stewardshipArchiveFixture(t, 2)
	manifest, err := CurrentItem14StewardshipManifestV2()
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range manifest.Sources {
		if !source.Mutable {
			continue
		}
		raw := []byte("Synthetic mutable status fixture for pure protocol testing: " + source.ID + "\n")
		files[source.Path] = &fstest.MapFile{Data: raw}
		manifest.Sources[i].SHA256 = item14StewardshipSHA256V1(raw)
	}

	return manifest, files
}
