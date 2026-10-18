package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	plan := flag.Bool("plan", false, "show the next topology-v8 observation without calling a provider")
	execute := flag.Bool("execute", false, "execute the exact next topology-v8 observation after reviewed activation")
	recover := flag.Bool("recover", false, "recover a stranded topology-v8 observation without retry")
	reconcilePairHalt := flag.Bool("reconcile-r04-pair-halt", false, "clear the proven run-bound R04 pair-integrity false halt without provider calls")
	observation := flag.String("observation", "", "exact next topology-v8 observation id")
	root := flag.String("artifact-root", "", "topology-v8 write-once evidence root")
	flag.Parse()
	if *root == "" {
		cwd, err := os.Getwd()
		fatal(err)
		*root = filepath.Join(cwd, "..", ".qualification-evidence", "topology-v8")
	}
	if *execute {
		if *observation == "" {
			fatal(fmt.Errorf("topology-v8 execution requires the exact next observation id"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		result, err := executeTopologyV8Observation(ctx, *root, *observation)
		fatal(err)
		write(result)
		return
	}
	if *recover {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := recoverTopologyV8Observation(ctx, *root)
		fatal(err)
		write(result)
		return
	}
	if *reconcilePairHalt {
		result, err := reconcileTopologyV8R04PairHalt(*root)
		fatal(err)
		write(result)
		return
	}
	if !*plan {
		fatal(fmt.Errorf("use --plan; provider execution is locked"))
	}
	result, err := topologyV8Plan(*root)
	fatal(err)
	result["schema_version"] = "silver-mediums.topology-v8-plan.v1"
	result["artifact_root"] = *root
	write(result)
}

func write(value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	fatal(err)
	fmt.Println(string(raw))
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
