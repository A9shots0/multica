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
			return errors.New("usage: role-qualification-run-v4 validate")
		}
		f, err := agentqualification.CurrentRoleQualificationExecutionFreezeV4()
		if err != nil {
			return err
		}
		return enc.Encode(map[string]any{"valid": true, "freeze_sha256": agentqualification.RoleQualificationExecutionFreezeV4SHA256(), "v3_freeze_sha256": f.V3FreezeSHA256, "provider_calls_require_fresh_preflight": true, "max_cells_per_batch": 12})
	case "status", "replay":
		if len(args) != 2 {
			return fmt.Errorf("usage: role-qualification-run-v4 %s EVIDENCE_ROOT", args[0])
		}
		r, err := agentqualification.RoleQualificationExecutionReplayV3(args[1])
		if err != nil {
			return err
		}
		return enc.Encode(r)
	case "preflight":
		if len(args) != 3 {
			return errors.New("usage: role-qualification-run-v4 preflight CLASS_ID EVIDENCE_ROOT")
		}
		p, err := runLivePreflightV4(ctx, args[1])
		if err != nil {
			return err
		}
		if err := agentqualification.WriteRoleQualificationPreflightV4(args[2], p); err != nil {
			return err
		}
		return enc.Encode(p)
	case "execute-batch":
		if len(args) != 3 {
			return errors.New("usage: role-qualification-run-v4 execute-batch CLASS_ID EVIDENCE_ROOT")
		}
		provider, err := newLiveProviderV4(ctx, args[1])
		if err != nil {
			return err
		}
		defer provider.Close()
		if err := provider.validateAgainstSealedPreflight(args[2]); err != nil {
			return err
		}
		r, err := agentqualification.ExecuteRoleQualificationBatchV4(ctx, args[2], args[1], time.Now().UTC(), provider)
		if err != nil {
			return err
		}
		return enc.Encode(r)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
