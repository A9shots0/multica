package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	var execute, replay bool
	var root string
	flag.BoolVar(&execute, "execute", false, "execute the frozen batch only after an independent review unlocks it")
	flag.BoolVar(&replay, "replay", false, "validate existing evidence without a provider call")
	flag.StringVar(&root, "artifact-root", "", "absolute durable evidence root")
	flag.Parse()
	if execute && replay {
		fatal(fmt.Errorf("choose execute or replay"))
	}
	if !execute && !replay {
		p, err := planV34()
		fatal(err)
		printJSON(p)
		return
	}
	if root == "" {
		fatal(fmt.Errorf("artifact-root is required"))
	}
	abs, err := filepath.Abs(root)
	fatal(err)
	if replay {
		r, err := replayV34(abs)
		fatal(err)
		printJSON(r)
		return
	}
	d, err := agentqualification.ClaudeSubscriptionStaticV34()
	fatal(err)
	freeze, err := agentqualification.CurrentClaudeSubscriptionStaticV34Freeze()
	fatal(err)
	if d.Status != "frozen_reviewed" || !d.ProviderCallsAllowed || freeze.Status != "frozen_reviewed" || !freeze.ProviderCallsAllowed {
		fatal(fmt.Errorf("v34 provider calls remain locked pending independent review"))
	}
	fatal(verifyClaudeV34RuntimeIdentity(d))
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(d.Caps.MaxWallSeconds)*time.Second)
	defer cancel()
	r, err := executeV34(ctx, abs, realClaudeV34Call)
	fatal(err)
	printJSON(r)
}

func planV34() (map[string]any, error) {
	return planV34WithContracts(agentqualification.QualificationContracts{})
}
func planV34WithContracts(q agentqualification.QualificationContracts) (map[string]any, error) {
	d, err := agentqualification.ClaudeSubscriptionStaticV34()
	if err != nil {
		return nil, err
	}
	f, err := q.CurrentClaudeSubscriptionStaticV34Freeze()
	if err != nil {
		return nil, err
	}
	cases := make([]string, 0, len(d.Cases))
	for _, c := range d.Cases {
		cases = append(cases, c.CaseID+":"+c.CaseVersion)
	}
	return map[string]any{"status": d.Status, "provider_called": false, "provider_calls_allowed": d.ProviderCallsAllowed, "batch_id": d.BatchID, "candidate_id": d.Configuration.ID, "model": d.Configuration.Model, "effort": d.Configuration.ReasoningEffort, "runtime": d.RuntimeIdentity.VersionOutput, "runtime_identity": d.RuntimeIdentity, "transport": d.Transport, "execution_order": cases, "caps": d.Caps, "freeze_sha256": agentqualification.ClaudeSubscriptionStaticV34FreezeSHA256(), "freeze_status": f.Status, "retries": 0, "role_selection": false}, nil
}

func verifyClaudeV34RuntimeIdentity(d agentqualification.ClaudeSubscriptionStaticV34Definition) error {
	return verifyClaudeV34RuntimeIdentityWithDependencies(d, exec.LookPath, fileHash, func(ctx context.Context, path string) ([]byte, error) {
		return exec.CommandContext(ctx, path, "--version").CombinedOutput()
	})
}
func verifyClaudeV34RuntimeIdentityWithDependencies(d agentqualification.ClaudeSubscriptionStaticV34Definition, lookPath func(string) (string, error), hash func(string) (string, error), version func(context.Context, string) ([]byte, error)) error {
	executable := "claude"
	if runtime.GOOS == "windows" {
		executable = "claude.cmd"
	}
	path, err := lookPath(executable)
	if err != nil {
		return failV34(v34ObservationTransport, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil || !strings.EqualFold(filepath.Clean(abs), filepath.Clean(d.RuntimeIdentity.ExecutablePath)) {
		return failV34(v34EvidenceCorruption, fmt.Errorf("Claude executable path drift"))
	}
	h, err := hash(abs)
	if err != nil || h != d.RuntimeIdentity.ExecutableSHA256 {
		return failV34(v34EvidenceCorruption, fmt.Errorf("Claude executable hash drift"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := version(ctx, abs)
	if err != nil || strings.TrimSpace(string(out)) != d.RuntimeIdentity.VersionOutput {
		return failV34(v34EvidenceCorruption, fmt.Errorf("Claude version drift"))
	}
	return nil
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func printJSON(v any) {
	raw, err := json.MarshalIndent(v, "", "  ")
	fatal(err)
	fmt.Println(string(raw))
}
