package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	caseID := flag.String("case", "T11", "screening case id")
	caseVersion := flag.String("case-version", "", "screening case version; empty uses the legacy grader")
	flag.Parse()
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	var grade agentqualification.BlindGrade
	if *caseVersion == "" {
		grade, err = agentqualification.GradeBlind(*caseID, raw)
	} else {
		grade, err = agentqualification.GradeBlindVersion(*caseID, *caseVersion, raw)
	}
	if err != nil {
		fatal(err)
	}
	encoded, err := json.MarshalIndent(grade, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
