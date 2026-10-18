package main

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type trialProtocol struct {
	contracts agentqualification.QualificationContracts
}

func serveTrialBrowser(ctx context.Context, root, observationID string) error {
	return (trialProtocol{}).serveTrialBrowser(ctx, root, observationID)
}

func runFakeTrial(ctx context.Context, root string) (trialLedger, error) {
	return (trialProtocol{}).runFakeTrial(ctx, root)
}

func requireTrialDefinition() agentqualification.PrimarySubscriptionRoleTrialV1 {
	return (trialProtocol{}).requireTrialDefinition()
}

func activeTrialDefinition() (agentqualification.PrimarySubscriptionRoleTrialV1, error) {
	return (trialProtocol{}).activeTrialDefinition()
}

func buildActiveTrialPrompt(invocation agentqualification.PrimarySubscriptionTrialInvocationV1, workspace agentqualification.RealisticCaseWorkspace, priorSpecialist json.RawMessage) (string, []byte, error) {
	return (trialProtocol{}).buildActiveTrialPrompt(invocation, workspace, priorSpecialist)
}

func activeTrialResponseSchema(invocation agentqualification.PrimarySubscriptionTrialInvocationV1) ([]byte, error) {
	return (trialProtocol{}).activeTrialResponseSchema(invocation)
}

func parseActiveTrialOutput(raw []byte, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, workspace agentqualification.RealisticCaseWorkspace) (agentqualification.PrimarySubscriptionTrialParticipantOutputV1, error) {
	return (trialProtocol{}).parseActiveTrialOutput(raw, invocation, workspace)
}

func baseSessionReceipt(invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) trialSessionReceipt {
	return (trialProtocol{}).baseSessionReceipt(invocation, prompt, schema)
}

func invokeClaudeTrial(ctx context.Context, sessionRoot string, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) (receipt trialSessionReceipt) {
	return (trialProtocol{}).invokeClaudeTrial(ctx, sessionRoot, invocation, prompt, schema)
}

func invokeCodexTrial(ctx context.Context, sessionRoot string, invocation agentqualification.PrimarySubscriptionTrialInvocationV1, prompt string, schema []byte) (receipt trialSessionReceipt) {
	return (trialProtocol{}).invokeCodexTrial(ctx, sessionRoot, invocation, prompt, schema)
}

func prepareTrial(root string) (trialLedger, error) { return (trialProtocol{}).prepareTrial(root) }

func executeNextObservation(ctx context.Context, root string, invoker trialInvoker) (trialLedger, error) {
	return (trialProtocol{}).executeNextObservation(ctx, root, invoker)
}

func applyCodexTransportCorrection(root string) (trialLedger, error) {
	return (trialProtocol{}).applyCodexTransportCorrection(root)
}

func finalizeTrialObservation(root, observationID string) (trialLedger, error) {
	return (trialProtocol{}).finalizeTrialObservation(root, observationID)
}

func cleanupAbandonedTrial(root string) (trialLedger, error) {
	return (trialProtocol{}).cleanupAbandonedTrial(root)
}

func validateTrialLedger(definition agentqualification.PrimarySubscriptionRoleTrialV1, ledger trialLedger, requireComplete bool) error {
	return (trialProtocol{}).validateTrialLedger(definition, ledger, requireComplete)
}

func verifyTrial(root string) (trialLedger, error) { return (trialProtocol{}).verifyTrial(root) }
