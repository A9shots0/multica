package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	residualV2ModeEnv         = "SM_REALISTIC_RESIDUAL_V2_MODE"
	residualV2ArtifactRootEnv = "SM_REALISTIC_RESIDUAL_V2_ARTIFACT_ROOT"
)

// init supplies a separate v31 command path without modifying main.go, whose
// bytes are part of the immutable v30 freeze. The outer v31 shim sets the
// private mode variable and this entry exits before legacy flag dispatch.
func init() {
	mode := os.Getenv(residualV2ModeEnv)
	if mode == "" {
		return
	}
	if err := runRealisticResidualV2Entry(mode, os.Getenv(residualV2ArtifactRootEnv)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runRealisticResidualV2Entry(mode, artifactRoot string) error {
	switch mode {
	case "plan":
		plan, err := realisticResidualV2Plan()
		if err != nil {
			return err
		}
		writeJSON(plan)
		return nil
	case "execute":
		if artifactRoot == "" {
			return fmt.Errorf("v31 residual execution requires an explicit absolute artifact root")
		}
		absolute, err := filepath.Abs(artifactRoot)
		if err != nil {
			return err
		}
		if absolute != artifactRoot {
			return fmt.Errorf("v31 residual artifact root must already be absolute")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		result, err := executeRealisticResidualV2(ctx, absolute)
		if err != nil {
			return err
		}
		writeJSON(result)
		return nil
	default:
		return fmt.Errorf("unknown v31 residual command mode %q", mode)
	}
}
