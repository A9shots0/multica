package agentqualification

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestR04EffortV2DefinitionIsExactAndDirectOnly(t *testing.T) {
	d, err := CurrentR04EffortExperimentV2()
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "frozen_reviewed" || d.Executor.ConfigurationID != "realistic.gpt54.high.executor.r04-effort-v2" || d.Executor.Model != "openrouter/openai/gpt-5.4" || d.Executor.Effort != "high" || d.Executor.ServingProvider != "openai" {
		t.Fatalf("unexpected executor: %+v", d.Executor)
	}
	if len(d.Arms) != 1 || d.Arms[0].Reviewed || d.Arms[0].MaxProviderTurns != 15 || d.Arms[0].TotalTokenReservation != 75000 || d.Arms[0].GeneratedTokenCeiling != 30000 || d.Arms[0].CostCeilingMicros != 350000 {
		t.Fatalf("unexpected arm: %+v", d.Arms)
	}
	if len(d.ExecutionOrder) != 2 || d.ExecutionOrder[0].ObservationID != "r04-gpt54-high-repeat-1" || d.ExecutionOrder[1].ObservationID != "r04-gpt54-high-repeat-2" || d.ExecutionOrder[0].Repeat != 1 || d.ExecutionOrder[1].Repeat != 2 {
		t.Fatalf("unexpected order: %+v", d.ExecutionOrder)
	}
	if d.MaxTotalTokens != 150000 || d.MaxIncrementalSpendMicros != 700000 || d.MaxCorrections != 0 {
		t.Fatalf("unexpected campaign caps")
	}
}

type r04EffortV2CostProvider struct{ calls int }

func (p *r04EffortV2CostProvider) Next(context.Context, RealisticProviderRequest) (RealisticProviderResponse, error) {
	p.calls++
	return RealisticProviderResponse{CostMicros: 350000}, nil
}

func TestR04EffortV2CostEqualityStopsBeforeSecondCall(t *testing.T) {
	p := &r04EffortV2CostProvider{}
	stage := &r04EffortV2StageProvider{inner: p, task: "sealed"}
	if _, err := stage.Next(context.Background(), RealisticProviderRequest{}); err != nil {
		t.Fatal(err)
	}
	_, err := stage.Next(context.Background(), RealisticProviderRequest{})
	if !errors.Is(err, ErrTopologyStageBudgetExceeded) {
		t.Fatalf("expected local stage exhaustion, got %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("expected one paid call, got %d", p.calls)
	}
}

func TestR04EffortV2PlanIsProviderFreeAndFrozen(t *testing.T) {
	d, err := CurrentR04EffortExperimentV2()
	if err != nil {
		t.Fatal(err)
	}
	p, err := PrevalidateR04EffortV2Routes(d)
	if err != nil {
		t.Fatal(err)
	}
	if p.ProviderCalls != 0 {
		t.Fatalf("unexpected prevalidation: %+v", p)
	}
	if err := RequireCompleteR04EffortFreezeV2(); err != nil {
		t.Fatalf("reviewed freeze must admit execution: %v", err)
	}
}

func TestR04EffortV2AllowsConfiguredGlobalHaltReason(t *testing.T) {
	d, err := CurrentR04EffortExperimentV2()
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	result := TopologyExperimentResult{
		Outcome: "global_halt", FailureClass: "credential_failure",
		InitialTreeSHA256: hash, TaskSHA256: hash, FixtureSHA256: hash, FinalTreeSHA256: hash, HiddenGradeSHA256: hash,
		StageReceipts: []TopologyStageReceipt{{RunID: "test:execute", Stage: "execute"}},
	}
	if err := validateR04EffortV2Result(d, result); err != nil {
		t.Fatalf("configured global halt was rejected: %v", err)
	}
	result.FailureClass = "candidate_failure"
	if err := validateR04EffortV2Result(d, result); err == nil {
		t.Fatal("unconfigured global halt was accepted")
	}
}
