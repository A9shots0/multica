//go:build item14_openrouter_shadow

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
	var mode string
	flag.StringVar(&mode, "mode", "plan", "one of plan, fake, prepare, execute, reconcile")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var value any
	var err error
	switch mode {
	case "plan":
		value, err = item14OpenRouterShadowPlan()
	case "fake":
		value, err = item14OpenRouterShadowFake()
	case "prepare":
		value, err = item14OpenRouterShadowPrepare(ctx)
	case "execute":
		var artifactRoot string
		artifactRoot, err = fixedItem14OpenRouterShadowRoot()
		if err == nil {
			value, err = executeItem14OpenRouterShadow(ctx, artifactRoot)
		}
	case "reconcile":
		var artifactRoot string
		artifactRoot, err = fixedItem14OpenRouterShadowRoot()
		if err == nil {
			value, err = reconcileItem14OpenRouterShadowFailure(artifactRoot)
		}
	default:
		err = fmt.Errorf("unknown Item 14 OpenRouter shadow mode %q", mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(raw))
}

func fixedItem14OpenRouterShadowRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(cwd) != "server" {
		return "", fmt.Errorf("Item 14 OpenRouter shadow must run from the repository server directory")
	}
	root, err := filepath.Abs(filepath.Join(cwd, "..", ".qualification-evidence", "item14-openrouter-local-shadow-v1"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return "", err
	}
	return root, nil
}
