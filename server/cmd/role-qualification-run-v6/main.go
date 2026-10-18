package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"os"
	"time"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("use plan, validate, status, replay, preflight, or execute-batch")
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	switch args[0] {
	case "plan":
		p, err := agentqualification.BuildRoleQualificationExecutionPlanV3()
		if err != nil {
			return err
		}
		return enc.Encode(p)
	case "validate":
		if len(args) != 1 {
			return errors.New("usage: role-qualification-run-v6 validate")
		}
		f, err := agentqualification.CurrentRoleQualificationExecutionFreezeV6()
		if err != nil {
			return err
		}
		return enc.Encode(map[string]any{"valid": true, "freeze_sha256": agentqualification.RoleQualificationExecutionFreezeV6SHA256(), "v5_freeze_sha256": f.V5FreezeSHA256, "provider_calls_require_fresh_preflight": true, "max_cells_per_batch": 12, "claude_subscription_status": "held_incomplete_route_evidence"})
	case "status", "replay":
		if len(args) != 2 {
			return fmt.Errorf("usage: role-qualification-run-v6 %s EVIDENCE_ROOT", args[0])
		}
		r, err := agentqualification.RoleQualificationExecutionReplayV6(args[1])
		if err != nil {
			return err
		}
		return enc.Encode(r)
	case "reconcile":
		if len(args) != 2 {
			return errors.New("usage: role-qualification-run-v6 reconcile EVIDENCE_ROOT")
		}
		if err := agentqualification.ReconcileRoleQualificationOpenAttemptsV6(args[1], time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		r, err := agentqualification.RoleQualificationExecutionReplayV6(args[1])
		if err != nil {
			return err
		}
		return enc.Encode(r)
	case "recover-v4-stranded":
		if len(args) != 3 {
			return errors.New("usage: role-qualification-run-v6 recover-v4-stranded LEGACY_V4_ROOT V6_ROOT")
		}
		if err := agentqualification.RecoverLegacyRoleQualificationV4StrandedV6(args[1], args[2], time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		return enc.Encode(map[string]any{"recovered": true, "cell_id": agentqualification.LegacyV4StrandedCellIDV6, "provider_calls": 0})
	case "preflight":
		if len(args) != 3 {
			return errors.New("usage: role-qualification-run-v6 preflight CLASS_ID EVIDENCE_ROOT")
		}
		p, err := runLivePreflightV6(ctx, args[1])
		if err != nil {
			return err
		}
		if err := agentqualification.WriteRoleQualificationPreflightV6(args[2], p); err != nil {
			return err
		}
		return enc.Encode(p)
	case "execute-batch":
		if len(args) != 3 {
			return errors.New("usage: role-qualification-run-v6 execute-batch CLASS_ID EVIDENCE_ROOT")
		}
		provider, err := newLiveProviderV6(ctx, args[1])
		if err != nil {
			return err
		}
		defer provider.Close()
		if err := provider.validateAgainstSealedPreflight(args[2]); err != nil {
			return err
		}
		r, err := agentqualification.ExecuteRoleQualificationBatchV6(ctx, args[2], args[1], time.Now().UTC(), provider)
		if err != nil {
			return err
		}
		return enc.Encode(r)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
