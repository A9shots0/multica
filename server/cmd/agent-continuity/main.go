package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/multica-ai/multica/server/pkg/taskcontinuity"
)

func main() {
	provider := flag.String("provider", "", "render the adapter for codex, claude, or copilot")
	printHash := flag.Bool("hash", false, "print the canonical contract hash")
	install := flag.String("install", "", "upsert the adapter in this instruction file")
	check := flag.String("check", "", "verify the installed contract in this instruction file")
	flag.Parse()

	if *printHash {
		fmt.Println(taskcontinuity.ContractHash())
		return
	}
	if *check != "" {
		content, err := os.ReadFile(*check)
		if err != nil {
			fatal(err)
		}
		if !taskcontinuity.InstalledContractMatches(string(content)) {
			fatal(fmt.Errorf("%s does not contain contract %s", *check, taskcontinuity.ContractHash()))
		}
		fmt.Printf("%s %s\n", *check, taskcontinuity.ContractHash())
		return
	}
	adapter, err := taskcontinuity.ProviderAdapter(*provider)
	if err != nil {
		fatal(err)
	}
	if *install != "" {
		if err := installAdapter(*install, *provider); err != nil {
			fatal(err)
		}
		fmt.Printf("%s %s\n", *install, taskcontinuity.ContractHash())
		return
	}
	fmt.Println(adapter)
}

func installAdapter(path, provider string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	updated, err := taskcontinuity.UpsertProviderAdapter(string(existing), provider)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, []byte(updated), mode)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
