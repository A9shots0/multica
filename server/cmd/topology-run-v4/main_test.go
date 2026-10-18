package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestTopologyV4PlanIsProviderFreeAndUsesFreshRoot(t *testing.T) {
	result, err := topologyV4Plan(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	next, ok := result["next_observation"].(*agentqualification.TopologyExperimentObservation)
	if !ok || next == nil || next.ObservationID != "v4-r03-p1-direct" || result["provider_called"] != false || result["provider_ready"] != true {
		t.Fatalf("unexpected reviewed plan: %+v", result)
	}
}

func TestTopologyV4WrongObservationFailsBeforeClaimOrProvider(t *testing.T) {
	root := t.TempDir()
	_, err := executeTopologyV4Observation(context.Background(), root, "not-the-next-observation")
	if err == nil {
		t.Fatal("wrong topology v4 observation was admitted")
	}
	if _, statErr := os.Stat(filepath.Join(root, "campaign.json")); !os.IsNotExist(statErr) {
		t.Fatal("invalid topology v4 execution wrote a claim")
	}
}

func TestTopologyV4CurrentCommandDriftCannotReuseHistoricalAdmission(t *testing.T) {
	if err := verifyPreservedTopologyCommandChain(); err == nil || !strings.Contains(err.Error(), "screening-run") {
		t.Fatalf("altered screening entrypoint must require fresh qualification: %v", err)
	}
}

func TestTopologyV4RejectsNestedTopologyV2CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "topology-run-v2", "topology_v2_cli.go")
	if err := os.WriteFile(path, []byte("nested v2 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v2") {
		t.Fatalf("nested topology-v2 command drift was accepted: %v", err)
	}
}

func TestTopologyV4RejectsNestedTopologyV1CommandDrift(t *testing.T) {
	root := copyTopologyCommandChain(t)
	path := filepath.Join(root, "screening-run", "main.go")
	if err := os.WriteFile(path, []byte("nested v1 drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreservedTopologyCommandChainAt(root); err == nil || !strings.Contains(err.Error(), "topology-v1") {
		t.Fatalf("nested topology-v1 command drift was accepted: %v", err)
	}
}

func copyTopologyCommandChain(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate topology-v4 test source")
	}
	sourceRoot := filepath.Join(filepath.Dir(current), "..")
	targetRoot := t.TempDir()
	files := []struct{ directory, name string }{
		{"topology-run-v3", "topology_v3_cli.go"},
		{"topology-run-v3", "main.go"},
		{"topology-run-v2", "topology_v2_cli.go"},
		{"topology-run-v2", "main.go"},
		{"screening-run", "topology_cli.go"},
		{"screening-run", "main.go"},
	}
	for _, file := range files {
		source := filepath.Join(sourceRoot, file.directory, file.name)
		targetDir := filepath.Join(targetRoot, file.directory)
		if err := os.MkdirAll(targetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, file.name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return targetRoot
}

// Positive verification uses explicit synthetic byte custody. Unrecovered old
// entrypoint bytes cannot be asserted equivalent to the evolving current main.
func TestExactNestedSourceVerifierAcceptsOnlyCompleteUnchangedBytes(t *testing.T) {
	root := t.TempDir()
	executor := []byte("synthetic frozen executor\n")
	main := []byte("synthetic frozen main\n")
	for _, name := range []string{"screening-run", "topology-run-v2", "topology-run-v3", "topology-run-v4", "topology-run-v5", "topology-run-v6", "topology-run-v7"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for file, raw := range map[string][]byte{"executor.go": executor, "main.go": main} {
			if err := os.WriteFile(filepath.Join(dir, file), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		verify := func() error {
			return verifyTopologyCommandSourcesAt(dir, "executor.go", sha256Hex(executor), sha256Hex(main))
		}
		if err := verify(); err != nil {
			t.Fatal(err)
		}
		for file, raw := range map[string][]byte{"executor.go": executor, "main.go": main} {
			path := filepath.Join(dir, file)
			if err := os.WriteFile(path, []byte("drift"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err == nil {
				t.Fatalf("%s/%s mutation accepted", name, file)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err == nil {
				t.Fatalf("missing %s/%s accepted", name, file)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := verify(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
