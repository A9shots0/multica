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
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	switch command {
	case "plan":
		plan, err := agentqualification.BuildRoleQualificationExecutionPlanV2()
		if err != nil {
			return err
		}
		return encoder.Encode(plan)
	case "validate":
		if len(args) != 1 {
			return fmt.Errorf("usage: role-qualification-run validate")
		}
		plan, err := agentqualification.BuildRoleQualificationExecutionPlanV2()
		if err != nil {
			return err
		}
		return encoder.Encode(map[string]any{"valid": true, "provider_calls_allowed": plan.ProviderCallsAllowed, "runtime_wiring": plan.RuntimeWiring, "cells": len(plan.Cells), "freeze_sha256": plan.ExecutionFreezeSHA256})
	case "status", "replay":
		if len(args) != 2 {
			return fmt.Errorf("usage: role-qualification-run %s EVIDENCE_ROOT", command)
		}
		status, err := agentqualification.RoleQualificationExecutionReplayV2(args[1])
		if err != nil {
			return err
		}
		return encoder.Encode(status)
	default:
		return fmt.Errorf("unknown command %q; use plan, validate, status, or replay", command)
	}
}
