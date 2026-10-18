package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	var execute, replay bool
	var root string
	flag.BoolVar(&execute, "execute", false, "execute only after the reviewed v36 freeze unlocks provider calls")
	flag.BoolVar(&replay, "replay", false, "replay sealed evidence without provider calls")
	flag.StringVar(&root, "artifact-root", "", "absolute durable evidence root")
	flag.Parse()
	if execute && replay {
		fatalV36(fmt.Errorf("choose execute or replay"))
	}
	if !execute && !replay {
		plan, err := planV36()
		fatalV36(err)
		printV36(plan)
		return
	}
	if root == "" {
		fatalV36(fmt.Errorf("artifact-root is required"))
	}
	abs, err := filepath.Abs(root)
	fatalV36(err)
	if replay {
		result, err := replayV36(abs)
		fatalV36(err)
		printV36(result)
		return
	}
	fatalV36(agentqualification.RequireClaudeSubscriptionRealisticV36Freeze())
	fatalV36(verifyClaudeV36RuntimeIdentity())
	ledger, err := executeV36(context.Background(), abs, newRealClaudeV36Provider(), func() error { return nil })
	fatalV36(err)
	printV36(ledger)
}

func planV36() (map[string]any, error) {
	d, err := agentqualification.ClaudeSubscriptionRealisticV36()
	if err != nil {
		return nil, err
	}
	freeze, err := agentqualification.CurrentClaudeSubscriptionRealisticV36Freeze()
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": freeze.Status, "provider_called": false, "provider_calls_allowed": freeze.ProviderCallsAllowed, "batch_id": d.BatchID, "model": d.Configuration.Model, "effort": d.Configuration.ReasoningEffort, "topology": d.Topology, "execution_order": d.ExecutionOrder, "case_caps": d.CaseCaps, "campaign_caps": d.CampaignCaps, "definition_sha256": agentqualification.ClaudeSubscriptionRealisticV36SHA256(), "freeze_sha256": agentqualification.ClaudeSubscriptionRealisticV36FreezeSHA256(), "retries": 0, "role_selection": false}, nil
}

func fatalV36(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func printV36(value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	fatalV36(err)
	fmt.Println(string(raw))
}
