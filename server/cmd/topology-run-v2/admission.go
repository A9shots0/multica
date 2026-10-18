package main

import "github.com/multica-ai/multica/server/internal/agentqualification"

func topologyV2ProviderReady(definition agentqualification.TopologyExperimentDefinition, bundle agentqualification.TopologyFreezeBundle) bool {
	return agentqualification.TopologyExperimentV2ProviderReady(definition) && bundle.Status == "frozen_reviewed" && bundle.ProviderCallsAllowed
}
