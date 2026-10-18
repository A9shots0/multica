package agentqualification

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestItem14StewardshipV1ManifestDefinitionRemainsValid(t *testing.T) {
	manifest, err := CurrentItem14StewardshipManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Sources) != 34 || Item14StewardshipManifestV1SHA256() == "" {
		t.Fatalf("unexpected historical Item 14 stewardship manifest: %+v", manifest)
	}
}

func TestItem14StewardshipV1RejectsDefinitionMutations(t *testing.T) {
	base, err := CurrentItem14StewardshipManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Item14StewardshipManifestV1){
		"provider":  func(v *Item14StewardshipManifestV1) { v.ProviderCallsAllowed = true },
		"selection": func(v *Item14StewardshipManifestV1) { v.ModelOrSeatSelectionAllowed = true },
		"missing":   func(v *Item14StewardshipManifestV1) { v.Sources = v.Sources[1:] },
		"path":      func(v *Item14StewardshipManifestV1) { v.Sources[0].Path = "../outside" },
		"class":     func(v *Item14StewardshipManifestV1) { v.Sources[0].EvidenceClass = "current_status" },
		"mutable":   func(v *Item14StewardshipManifestV1) { v.Sources[0].Mutable = true },
		"duplicate": func(v *Item14StewardshipManifestV1) { v.Sources[1] = v.Sources[0] },
		"order": func(v *Item14StewardshipManifestV1) {
			v.Sources[0], v.Sources[1] = v.Sources[1], v.Sources[0]
		},
	} {
		t.Run(name, func(t *testing.T) {
			clone := cloneItem14StewardshipManifestV1(t, base)
			mutate(&clone)
			if err := ValidateItem14StewardshipManifestV1(clone); err == nil {
				t.Fatal("mutated Item 14 stewardship manifest was accepted")
			}
		})
	}
}

func TestItem14StewardshipV1RejectsBoundSourceHashDrift(t *testing.T) {
	manifest, err := CurrentItem14StewardshipManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Sources[0].SHA256 = strings.Repeat("0", 64)
	if _, err := VerifyItem14StewardshipManifestV1(manifest, os.DirFS("../../..")); err == nil {
		t.Fatal("changed source hash was accepted")
	}
}

func TestItem14StewardshipV1RejectsUnknownFields(t *testing.T) {
	injected := strings.Replace(string(item14StewardshipManifestV1JSON), `"status":`, `"unexpected":true,"status":`, 1)
	if _, err := DecodeItem14StewardshipManifestV1([]byte(injected)); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

func cloneItem14StewardshipManifestV1(t *testing.T, value Item14StewardshipManifestV1) Item14StewardshipManifestV1 {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone Item14StewardshipManifestV1
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
