package agentqualification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoleQualificationExecutionV6AtomicFreeze(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV6()
	if err != nil {
		t.Fatal(err)
	}
	if f.V5FreezeSHA256 != "cbe3d5125cb28b716c0db5423b223db1b192aaea21ec73a7925f9c46e4c90c8b" || len(f.Artifacts) != 12 {
		t.Fatalf("freeze coverage %+v", f)
	}
	want := []string{
		"server/internal/agentqualification/role_qualification_execution_v6.json",
		"server/internal/agentqualification/role_qualification_execution_v6.go",
		"server/internal/agentqualification/role_qualification_execution_v6_test.go",
		"server/internal/agentqualification/role_qualification_execution_v6_prevalidation.md",
		"server/internal/agentqualification/role_qualification_execution_v6_freeze.go",
		"server/internal/agentqualification/role_qualification_execution_v6_freeze_test.go",
		"server/cmd/role-qualification-run-v6/main.go",
		"server/cmd/role-qualification-run-v6/main_test.go",
		"server/cmd/role-qualification-run-v6/provider.go",
		"server/cmd/role-qualification-run-v6/provider_test.go",
		"server/internal/agentqualification/opencode_structured_v33.go",
		"server/internal/agentqualification/opencode_structured_v33_test.go",
	}
	for _, path := range want {
		if _, ok := f.Artifacts[path]; !ok {
			t.Fatalf("missing frozen artifact %s", path)
		}
	}
}

func TestRoleQualificationExecutionV6FreezeRejectsDrift(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV6()
	if err != nil {
		t.Fatal(err)
	}
	root, err := roleQualificationRepoRootV3()
	if err != nil {
		t.Fatal(err)
	}
	for path, hash := range f.Artifacts {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if sha256Hex(raw) != hash || !strings.HasPrefix(path, "server/") {
			t.Fatalf("artifact drift %s", path)
		}
	}
}
