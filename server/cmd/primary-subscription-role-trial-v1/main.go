package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	mode := flag.String("mode", "prepare", "prepare, fake, apply-codex-transport-correction, execute-next, serve, finalize, verify, or cleanup-abandoned")
	definition := flag.String("definition", "v1", "v1 or reliability-v2")
	root := flag.String("artifact-root", "", "absolute evidence root")
	observation := flag.String("observation", "", "pending R02 observation for serve/finalize")
	flag.Parse()
	if *root == "" {
		fatal(fmt.Errorf("artifact-root is required"))
	}
	if *definition != "v1" && *definition != "reliability-v2" {
		fatal(fmt.Errorf("unknown definition %s", *definition))
	}
	trialDefinitionVariant = *definition
	if *mode == "apply-codex-transport-correction" && trialDefinitionVariant != "v1" {
		fatal(fmt.Errorf("the v1 correction cannot apply to %s", trialDefinitionVariant))
	}
	absolute, err := filepath.Abs(*root)
	fatal(err)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	var value any
	switch *mode {
	case "prepare":
		value, err = prepareTrial(absolute)
	case "fake":
		value, err = runFakeTrial(ctx, absolute)
	case "apply-codex-transport-correction":
		value, err = applyCodexTransportCorrection(absolute)
	case "execute-next":
		value, err = executeNextObservation(ctx, absolute, realTrialInvoker{})
	case "serve":
		err = serveTrialBrowser(ctx, absolute, *observation)
		return
	case "finalize":
		value, err = finalizeTrialObservation(absolute, *observation)
	case "verify":
		value, err = verifyTrial(absolute)
	case "cleanup-abandoned":
		value, err = cleanupAbandonedTrial(absolute)
	default:
		err = fmt.Errorf("unknown mode %s", *mode)
	}
	fatal(err)
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

func (protocol trialProtocol) requireTrialDefinition() agentqualification.PrimarySubscriptionRoleTrialV1 {
	definition, err := protocol.activeTrialDefinition()
	fatal(err)
	return definition
}

var trialDefinitionVariant = "v1"

func (protocol trialProtocol) activeTrialDefinition() (agentqualification.PrimarySubscriptionRoleTrialV1, error) {
	if trialDefinitionVariant == "reliability-v2" {
		return agentqualification.CurrentPrimarySubscriptionReliabilityTrialV2()
	}
	return protocol.contracts.CurrentPrimarySubscriptionRoleTrialV1()
}

func activeTrialDefinitionSHA256() string {
	if trialDefinitionVariant == "reliability-v2" {
		return agentqualification.PrimarySubscriptionReliabilityTrialV2SHA256()
	}
	return agentqualification.PrimarySubscriptionRoleTrialV1SHA256()
}

func activeTrialLedgerSchema() string {
	if trialDefinitionVariant == "reliability-v2" {
		return reliabilityTrialLedgerSchema
	}
	return trialLedgerSchema
}

func (protocol trialProtocol) buildActiveTrialPrompt(invocation agentqualification.PrimarySubscriptionTrialInvocationV1, workspace agentqualification.RealisticCaseWorkspace, priorSpecialist json.RawMessage) (string, []byte, error) {
	if trialDefinitionVariant == "reliability-v2" {
		if len(priorSpecialist) != 0 {
			return "", nil, fmt.Errorf("reliability v2 forbids specialist evidence")
		}
		return agentqualification.BuildPrimarySubscriptionReliabilityTrialPromptV2(invocation, workspace)
	}
	return protocol.contracts.BuildPrimarySubscriptionTrialPromptV1(invocation, workspace, priorSpecialist)
}

func (protocol trialProtocol) activeTrialResponseSchema(invocation agentqualification.PrimarySubscriptionTrialInvocationV1) ([]byte, error) {
	if trialDefinitionVariant == "reliability-v2" {
		return agentqualification.PrimarySubscriptionReliabilityTrialResponseSchemaV2(invocation)
	}
	return protocol.contracts.PrimarySubscriptionTrialResponseSchemaV1(invocation)
}

func (protocol trialProtocol) parseActiveTrialOutput(raw []byte, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, workspace agentqualification.RealisticCaseWorkspace) (agentqualification.PrimarySubscriptionTrialParticipantOutputV1, error) {
	if trialDefinitionVariant == "reliability-v2" {
		return agentqualification.ParsePrimarySubscriptionReliabilityTrialParticipantOutputV2(raw, invocation, workspace)
	}
	return protocol.contracts.ParsePrimarySubscriptionTrialParticipantOutputV1(raw, invocation, workspace)
}
