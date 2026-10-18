package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"os"
	"path/filepath"
)

func main() {
	execute := flag.Bool("execute", false, "execute only after reviewed v38 freeze")
	root := flag.String("artifact-root", "", "absolute durable v38 evidence root")
	flag.Parse()
	if *execute {
		fatal(agentqualification.RequireClaudeSubscriptionRealisticV38Freeze())
		if *root == "" {
			fatal(fmt.Errorf("artifact-root is required"))
		}
		absolute, err := filepath.Abs(*root)
		fatal(err)
		fatal(verifyClaudeV38RuntimeIdentity())
		ledger, err := executeV38(context.Background(), absolute, realClaudeV38Provider{}, func() error { return nil })
		fatal(err)
		printV38(ledger)
		return
	}
	d, err := agentqualification.ClaudeSubscriptionRealisticV38()
	fatal(err)
	raw, err := json.MarshalIndent(map[string]any{"status": d.Status, "provider_called": false, "provider_calls_allowed": d.ProviderCallsAllowed, "batch_id": d.BatchID, "model": d.Configuration.Model, "effort": d.Configuration.ReasoningEffort, "execution_order": d.ExecutionOrder, "caps": d.Caps, "definition_sha256": agentqualification.ClaudeSubscriptionRealisticV38SHA256(), "attempt_1": d.Attempt1, "attempt_2": d.Attempt2, "retries": 0}, "", "  ")
	fatal(err)
	fmt.Println(string(raw))
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printV38(value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	fatal(err)
	fmt.Println(string(raw))
}
