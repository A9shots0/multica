package main

import "github.com/multica-ai/multica/server/internal/agentqualification"

func r04EffortV1ProviderReady(definition agentqualification.TopologyExperimentDefinition, bundle agentqualification.TopologyFreezeBundle) bool {
	return agentqualification.R04EffortExperimentV1ProviderReady(definition) && bundle.Status == "frozen_reviewed" && bundle.ProviderCallsAllowed
}
