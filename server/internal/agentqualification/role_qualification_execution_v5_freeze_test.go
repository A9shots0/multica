package agentqualification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoleQualificationExecutionV5AtomicFreeze(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV5()
	if err != nil {
		t.Fatal(err)
	}
	if f.V4FreezeSHA256 != "2c747be170367ea9eafc36551ffe344e18d7d5ff0c97b60e098ac6226368b2b0" || len(f.Artifacts) != 10 {
		t.Fatalf("freeze coverage %+v", f)
	}
	want := []string{
		"server/internal/agentqualification/role_qualification_execution_v5.json",
		"server/internal/agentqualification/role_qualification_execution_v5.go",
		"server/internal/agentqualification/role_qualification_execution_v5_test.go",
		"server/internal/agentqualification/role_qualification_execution_v5_prevalidation.md",
		"server/internal/agentqualification/role_qualification_execution_v5_freeze.go",
		"server/internal/agentqualification/role_qualification_execution_v5_freeze_test.go",
		"server/cmd/role-qualification-run-v5/main.go",
		"server/cmd/role-qualification-run-v5/main_test.go",
		"server/cmd/role-qualification-run-v5/provider.go",
		"server/cmd/role-qualification-run-v5/provider_test.go",
	}
	for _, path := range want {
		if _, ok := f.Artifacts[path]; !ok {
			t.Fatalf("missing frozen artifact %s", path)
		}
	}
}

func TestRoleQualificationExecutionV5FreezeRejectsDrift(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV5()
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
