package main

import (
	"crypto/sha256"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed main.go
var shimSource []byte

//go:embed main_test.go
var shimTestSource []byte

func main() {
	var plan, execute bool
	var artifactRoot string
	flag.BoolVar(&plan, "plan", false, "print the provider-free v31 residual plan")
	flag.BoolVar(&execute, "execute", false, "execute only after the independently reviewed freeze unlocks")
	flag.StringVar(&artifactRoot, "artifact-root", "", "absolute write-once evidence root")
	flag.Parse()
	if plan == execute {
		fatal(fmt.Errorf("choose exactly one of -plan or -execute"))
	}
	if err := verifyShimSources(); err != nil {
		fatal(err)
	}
	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}
	mode := "plan"
	if execute {
		mode = "execute"
		if artifactRoot == "" || !filepath.IsAbs(artifactRoot) {
			fatal(fmt.Errorf("v31 residual execution requires an explicit absolute artifact root"))
		}
	}
	command := exec.Command("go", "run", "./cmd/screening-run")
	command.Dir = root
	command.Env = append(os.Environ(), "SM_REALISTIC_RESIDUAL_V2_MODE="+mode, "SM_REALISTIC_RESIDUAL_V2_ARTIFACT_ROOT="+artifactRoot)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		fatal(err)
	}
}

func verifyShimSources() error {
	required, err := agentqualification.RequiredRealisticResidualV2CommandSources()
	if err != nil {
		return err
	}
	actual := map[string][]byte{
		"realistic-residual-run-v2/main.go":      shimSource,
		"realistic-residual-run-v2/main_test.go": shimTestSource,
	}
	for name, raw := range actual {
		if required[name] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			return fmt.Errorf("v31 residual shim source drifted: %s", name)
		}
	}
	return nil
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir || strings.TrimSpace(parent) == "" {
			return "", fmt.Errorf("could not locate module root")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
