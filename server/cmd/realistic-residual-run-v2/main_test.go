package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShimSourcesAreBoundAndModuleRootResolves(t *testing.T) {
	if err := verifyShimSources(); err != nil {
		t.Fatal(err)
	}
	root, err := findModuleRoot()
	if err != nil || root == "" {
		t.Fatalf("module root did not resolve: %q err=%v", root, err)
	}
}

func TestRealCLIPlanIsProviderFreeAndReviewed(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	plan := exec.Command("go", "run", "./cmd/realistic-residual-run-v2", "-plan")
	plan.Dir = root
	output, err := plan.CombinedOutput()
	if err != nil {
		t.Fatalf("provider-free CLI plan failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `"provider_called": false`) || !strings.Contains(string(output), `"provider_ready": true`) {
		t.Fatalf("CLI plan did not prove reviewed provider-free readiness:\n%s", output)
	}
}
