package main

import "github.com/multica-ai/multica/server/internal/agentqualification"

func topologyV3ProviderReady(definition agentqualification.TopologyExperimentDefinition, bundle agentqualification.TopologyFreezeBundle) bool {
	return agentqualification.TopologyExperimentV3ProviderReady(definition) && bundle.Status == "frozen_reviewed" && bundle.ProviderCallsAllowed
}
