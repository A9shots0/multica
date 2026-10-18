package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	root := flag.String("artifact-root", "", "sealed primary-subscription trial evidence root")
	output := flag.String("output", "", "write-once review result path")
	definition := flag.String("definition", "v1", "v1 or reliability-v2")
	flag.Parse()
	if *root == "" || *output == "" {
		fatal(fmt.Errorf("artifact-root and output are required"))
	}
	absoluteRoot, err := filepath.Abs(*root)
	fatal(err)
	absoluteOutput, err := filepath.Abs(*output)
	fatal(err)
	var report agentqualification.PrimarySubscriptionTrialReviewV1
	if *definition == "v1" {
		report, err = agentqualification.ReviewPrimarySubscriptionRoleTrialEvidenceV1(absoluteRoot)
	} else if *definition == "reliability-v2" {
		report, err = agentqualification.ReviewPrimarySubscriptionReliabilityTrialEvidenceV2(absoluteRoot)
	} else {
		fatal(fmt.Errorf("unknown definition %s", *definition))
	}
	fatal(err)
	fatal(agentqualification.WritePrimarySubscriptionRoleTrialReviewV1(absoluteOutput, report))
	raw, err := json.MarshalIndent(report, "", "  ")
	fatal(err)
	fmt.Println(string(raw))
	if report.Verdict != "PASS" {
		os.Exit(2)
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
