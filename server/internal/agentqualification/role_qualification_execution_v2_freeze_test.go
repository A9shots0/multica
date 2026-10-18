package agentqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRoleQualificationExecutionFreezeV2BindsAllSourcesCommandsAndTransportAntecedents(t *testing.T) {
	f, err := CurrentRoleQualificationExecutionFreezeV2()
	if err != nil {
		t.Fatal(err)
	}
	root := roleQualificationRepoRoot(t)
	for path, want := range f.Artifacts {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != want {
			t.Errorf("artifact drift %s", path)
		}
	}
	for path, want := range f.TransportAntecedents {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != want {
			t.Errorf("transport drift %s", path)
		}
	}
}
func TestRoleQualificationExecutionFreezeV2FailsClosed(t *testing.T) {
	base, err := CurrentRoleQualificationExecutionFreezeV2()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RoleQualificationExecutionFreezeV2){"status": func(f *RoleQualificationExecutionFreezeV2) { f.Status = "draft" }, "v1": func(f *RoleQualificationExecutionFreezeV2) { f.V1FreezeSHA256 = "00" }, "calls": func(f *RoleQualificationExecutionFreezeV2) { f.ProviderCallsAllowed = true }, "runtime": func(f *RoleQualificationExecutionFreezeV2) { f.RuntimeWiring = "live" }, "cells": func(f *RoleQualificationExecutionFreezeV2) { f.Cells = 35 }, "retry": func(f *RoleQualificationExecutionFreezeV2) { f.Retries = 1 }, "artifact": func(f *RoleQualificationExecutionFreezeV2) { f.Artifacts = map[string]string{} }} {
		copy := base
		mutate(&copy)
		if err := ValidateRoleQualificationExecutionFreezeV2(copy); err == nil {
			t.Errorf("%s drift accepted", name)
		}
	}
}
