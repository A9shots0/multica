package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestProtocolAblationV33AtomicFreeze(t *testing.T) {
	f, err := CurrentProtocolAblationV33Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.ExecutionOrder) != 4 || f.MaxCombinedCostMicros != 1_805_000 || len(ProtocolAblationV33FreezeSHA256()) != 64 {
		t.Fatalf("bad v33 freeze: %+v", f)
	}
	doc, err := os.ReadFile("../../../docs/silver-mediums/ITEM-14-OPENCODE-STRUCTURED-OUTPUT-V33-PREVALIDATION.md")
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(doc)
	if f.Documents["docs/silver-mediums/ITEM-14-OPENCODE-STRUCTURED-OUTPUT-V33-PREVALIDATION.md"] != hex.EncodeToString(h[:]) {
		t.Fatal("v33 doc drift")
	}
}
func TestProtocolAblationV33RejectsHashDrift(t *testing.T) {
	f, err := CurrentProtocolAblationV33Freeze()
	if err != nil {
		t.Fatal(err)
	}
	f.LocalComponents = cloneStringMap(f.LocalComponents)
	f.LocalComponents["opencode_structured_v33.go"] = "drift"
	if ValidateProtocolAblationV33Freeze(f) == nil {
		t.Fatal("v33 source drift accepted")
	}
}
