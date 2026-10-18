package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	command := "plan"
	if len(args) > 0 {
		command = args[0]
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	switch command {
	case "plan":
		p, err := agentqualification.BuildRoleQualificationExecutionPlanV3()
		if err != nil {
			return err
		}
		return enc.Encode(p)
	case "validate":
		if len(args) != 1 {
			return fmt.Errorf("usage: role-qualification-run-v3 validate")
		}
		p, err := agentqualification.BuildRoleQualificationExecutionPlanV3()
		if err != nil {
			return err
		}
		return enc.Encode(map[string]any{"valid": true, "provider_calls_allowed": p.ProviderCallsAllowed, "runtime_wiring": p.RuntimeWiring, "cells": len(p.Cells), "freeze_sha256": p.ExecutionFreezeSHA256})
	case "status", "replay":
		if len(args) != 2 {
			return fmt.Errorf("usage: role-qualification-run-v3 %s EVIDENCE_ROOT", command)
		}
		s, err := agentqualification.RoleQualificationExecutionReplayV3(args[1])
		if err != nil {
			return err
		}
		return enc.Encode(s)
	default:
		return fmt.Errorf("unknown command %q; use plan, validate, status, or replay", command)
	}
}
