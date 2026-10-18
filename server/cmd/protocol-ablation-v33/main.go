package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

//go:embed main.go
var v33MainSource []byte

//go:embed execute.go
var v33ExecuteSource []byte

//go:embed main_test.go
var v33MainTestSource []byte

//go:embed recovery.go
var v33RecoverySource []byte

//go:embed recovery_test.go
var v33RecoveryTestSource []byte

func main() {
	if len(os.Args) > 1 && os.Args[1] == "recover-readiness" {
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: protocol-ablation-v33 recover-readiness SOURCE_ROOT CONTINUATION_ROOT")
			os.Exit(2)
		}
		result, err := recoverV33ReadinessFailure(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "recover-timeout" {
		if len(os.Args) != 5 {
			fmt.Fprintln(os.Stderr, "usage: protocol-ablation-v33 recover-timeout SESSION SOURCE_ROOT CONTINUATION_ROOT")
			os.Exit(2)
		}
		result, err := recoverV33TimeoutFromSession(context.Background(), os.Getenv("OPENCODE_V33_EXECUTABLE"), os.Args[2], os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "recover" {
		if len(os.Args) != 5 {
			fmt.Fprintln(os.Stderr, "usage: protocol-ablation-v33 recover SESSION CURRENT_ROOT CONTINUATION_ROOT")
			os.Exit(2)
		}
		result, err := recoverV33FromSession(context.Background(), os.Getenv("OPENCODE_V33_EXECUTABLE"), os.Args[2], os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "execute" {
		root := "artifacts/protocol-ablation-v33"
		if len(os.Args) > 2 {
			root = os.Args[2]
		}
		ledger, err := executeV33(context.Background(), root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(ledger)
		return
	}
	plan, err := v33Plan()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func v33Plan() (map[string]any, error) {
	d, err := agentqualification.CurrentProtocolAblationV32()
	if err != nil {
		return nil, err
	}
	if err := verifyV33CommandFreeze(); err != nil {
		return nil, err
	}
	order, err := agentqualification.ProtocolAblationV32ExecutionOrder(d)
	if err != nil {
		return nil, err
	}
	caps := make([]agentqualification.OpenCodeStructuredV33Capsule, 0, 4)
	var maxCost int64
	for _, b := range order {
		c, err := agentqualification.BuildOpenCodeStructuredV33Capsule(d, b)
		if err != nil {
			return nil, err
		}
		caps = append(caps, c)
		cost, ok := agentqualification.OpenCodeStructuredV33CostReservation(d, b.Candidate)
		if !ok {
			return nil, fmt.Errorf("v33 price overflow")
		}
		maxCost += cost
	}
	if len(caps) != 4 || maxCost != d.MaxCombinedCostMicros {
		return nil, fmt.Errorf("v33 plan is not exact four-cell budget")
	}
	return map[string]any{"status": "frozen_ready_not_executed", "provider_called": false, "cells": caps, "execution_order": order, "max_combined_tokens": d.MaxCombinedTokens, "max_combined_cost_micros": d.MaxCombinedCostMicros, "max_combined_provider_turns": d.MaxCombinedProviderTurns, "retries": 0, "fallbacks": false, "freeze_sha256": agentqualification.ProtocolAblationV33FreezeSHA256(), "opencode_version": agentqualification.OpenCodeStructuredV33Version, "opencode_executable_sha256": agentqualification.OpenCodeStructuredV33ExecutableSHA256, "opencode_openapi_sha256": agentqualification.OpenCodeStructuredV33OpenAPISHA256}, nil
}
func v33CommandHashes() map[string]string {
	out := map[string]string{}
	for name, raw := range map[string][]byte{"protocol-ablation-v33/main.go": v33MainSource, "protocol-ablation-v33/execute.go": v33ExecuteSource, "protocol-ablation-v33/main_test.go": v33MainTestSource, "protocol-ablation-v33/recovery.go": v33RecoverySource, "protocol-ablation-v33/recovery_test.go": v33RecoveryTestSource} {
		d := sha256.Sum256(raw)
		out[name] = hex.EncodeToString(d[:])
	}
	return out
}
func verifyV33CommandFreeze() error {
	return agentqualification.RequireProtocolAblationV33Freeze(v33CommandHashes())
}
