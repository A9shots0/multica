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
	plan := flag.Bool("plan", false, "show the next topology-v9 observation without calling a provider")
	execute := flag.Bool("execute", false, "execute the exact next topology-v9 observation after reviewed activation")
	recover := flag.Bool("recover", false, "recover a stranded topology-v9 observation without retry")
	observation := flag.String("observation", "", "exact next topology-v9 observation id")
	root := flag.String("artifact-root", "", "topology-v9 write-once evidence root")
	flag.Parse()
	if *root == "" {
		cwd, err := os.Getwd()
		fatal(err)
		*root = filepath.Join(cwd, "..", ".qualification-evidence", "topology-v9")
	}
	if *execute {
		if *observation == "" {
			fatal(fmt.Errorf("topology-v9 execution requires the exact next observation id"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		result, err := executeTopologyV9Observation(ctx, *root, *observation)
		fatal(err)
		write(result)
		return
	}
	if *recover {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := recoverTopologyV9Observation(ctx, *root)
		fatal(err)
		write(result)
		return
	}
	if !*plan {
		fatal(fmt.Errorf("use --plan; provider execution is locked"))
	}
	result, err := topologyV9Plan(*root)
	fatal(err)
	result["schema_version"] = "silver-mediums.topology-v9-plan.v1"
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
