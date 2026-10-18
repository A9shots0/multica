package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	result, _, err := agentqualification.RunRoleStageEvidenceRehearsalV1()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(raw))
}
