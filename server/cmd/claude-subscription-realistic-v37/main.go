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
	execute := flag.Bool("execute", false, "execute only after an independently reviewed immutable v37 freeze unlocks provider calls")
	root := flag.String("artifact-root", "", "absolute durable v37 evidence root")
	flag.Parse()
	if *execute {
		fatalV37(agentqualification.RequireClaudeSubscriptionRealisticV37Freeze())
		if *root == "" {
			fatalV37(fmt.Errorf("artifact-root is required"))
		}
		absolute, err := filepath.Abs(*root)
		fatalV37(err)
		fatalV37(verifyClaudeV37RuntimeIdentity())
		ledger, err := executeV37(context.Background(), absolute, realClaudeV37Provider{}, func() error { return nil })
		fatalV37(err)
		printV37(ledger)
		return
	}
	d, err := agentqualification.ClaudeSubscriptionRealisticV37()
	fatalV37(err)
	printV37(map[string]any{
		"status":                 d.Status,
		"provider_called":        false,
		"provider_calls_allowed": d.ProviderCallsAllowed,
		"batch_id":               d.BatchID,
		"model":                  d.Configuration.Model,
		"effort":                 d.Configuration.ReasoningEffort,
		"topology":               d.Topology,
		"execution_order":        d.ExecutionOrder,
		"caps":                   d.Caps,
		"definition_sha256":      agentqualification.ClaudeSubscriptionRealisticV37SHA256(),
		"retries":                0,
		"role_selection":         false,
	})
}

func fatalV37(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printV37(value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	fatalV37(err)
	fmt.Println(string(raw))
}
