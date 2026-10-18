package agentqualification

import "testing"

func TestRealisticProviderExecutionUnlockedAfterCompleteFreeze(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireRealisticWorkflowFrozen(group); err != nil {
		t.Fatalf("provider execution remained locked after independent review: %v", err)
	}
}

func TestRealisticReceiptRejectsForbiddenToolAndSideEffect(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	groupCase := group.Cases[0]
	evidence := make([]RealisticEvidenceReceipt, 0, len(groupCase.RequiredEvidence))
	for _, evidenceID := range groupCase.RequiredEvidence {
		evidence = append(evidence, RealisticEvidenceReceipt{EvidenceID: evidenceID, Kind: "fixture", ArtifactPath: "evidence.json", SHA256: hash})
	}
	receipt := RealisticExecutionReceipt{
		SchemaVersion: RealisticExecutionReceiptSchemaVersion, GroupID: group.GroupID,
		CaseID: "R01", CaseVersion: "v2", ConfigurationID: "fixture", RunFamilyID: "run-fixture", Attempt: 1,
		TerminalState: "completed", StartingStateSHA256: hash, FinalStateSHA256: hash,
		WorkspaceBoundary: "fresh_disposable_case_workspace", NetworkBoundary: "provider_api_only",
		ProviderReceiptSHA256: hash, ProviderReceiptArtifactPath: "provider.json", FixtureTreeSHA256: hash, ManifestSHA256: hash,
		ProviderReceiptHashes: []string{hash}, ProviderCalls: 1, InputTokens: 1, OutputTokens: 1, CostMicros: 1,
		ProviderAccountingStatus: "exact", ProviderTokenAccountingStatus: "exact", ProviderCostAccountingStatus: "exact", ControllerResultSHA256: hash, ControllerResultArtifactPath: "controller.json",
		ToolEvents: []RealisticToolEvent{{Sequence: 1, Tool: "read", StartedAt: "x", CompletedAt: "y", OutputSHA256: hash}}, Evidence: evidence,
	}
	if reasons := ValidateRealisticExecutionReceipt(receipt, group); len(reasons) != 0 {
		t.Fatalf("valid fixture receipt rejected: %v", reasons)
	}
	receipt.ToolEvents = []RealisticToolEvent{{Sequence: 1, Tool: "git_push", StartedAt: "x", CompletedAt: "y", OutputSHA256: hash}}
	receipt.ExternalSideEffects = []string{"deployment"}
	if reasons := ValidateRealisticExecutionReceipt(receipt, group); !contains(reasons, "tool_not_allowlisted:git_push") || !contains(reasons, "forbidden_side_effect_or_event") {
		t.Fatalf("unsafe receipt was not rejected: %v", reasons)
	}
}
