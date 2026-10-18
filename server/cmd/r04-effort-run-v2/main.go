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
	plan := flag.Bool("plan", false, "show the next r04-effort-v2 observation without calling a provider")
	execute := flag.Bool("execute", false, "execute the exact next r04-effort-v2 observation after reviewed activation")
	recover := flag.Bool("recover", false, "recover a stranded r04-effort-v2 observation without retry")
	observation := flag.String("observation", "", "exact next r04-effort-v2 observation id")
	root := flag.String("artifact-root", "", "r04-effort-v2 write-once evidence root")
	flag.Parse()
	if *root == "" {
		cwd, err := os.Getwd()
		fatal(err)
		*root = filepath.Join(cwd, "..", ".qualification-evidence", "r04-effort-v2")
	}
	if *execute {
		if *observation == "" {
			fatal(fmt.Errorf("r04-effort-v2 execution requires the exact next observation id"))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cancel()
		result, err := executeR04EffortV2Observation(ctx, *root, *observation)
		fatal(err)
		write(result)
		return
	}
	if *recover {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := recoverR04EffortV2Observation(ctx, *root)
		fatal(err)
		write(result)
		return
	}
	if !*plan {
		fatal(fmt.Errorf("use --plan; provider execution is locked"))
	}
	result, err := r04EffortV2Plan(*root)
	fatal(err)
	result["schema_version"] = "silver-mediums.r04-effort-v2-plan.v1"
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
