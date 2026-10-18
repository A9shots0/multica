package agentqualification

import (
	"encoding/json"
	"testing"
)

func TestRealisticValidationMatrixExercisesEveryCase(t *testing.T) {
	group, err := CurrentRealisticWorkflowGroup()
	if err != nil {
		t.Fatal(err)
	}
	for _, candidateCase := range group.Cases {
		t.Run(candidateCase.CaseID, func(t *testing.T) {
			base := referenceRealisticReceipt(t, group, candidateCase.CaseID)
			if reasons := ValidateRealisticExecutionReceipt(*base, group); len(reasons) != 0 {
				t.Fatalf("strong_pass rejected: %v", reasons)
			}
			if reasons := MissingRequiredRealisticEvidence(*base, group); len(reasons) != 0 {
				t.Fatalf("strong_pass missing semantic evidence: %v", reasons)
			}

			paraphrase := cloneRealisticReceipt(t, *base)
			for index := range paraphrase.Evidence {
				paraphrase.Evidence[index].Kind = "equivalent_verified_artifact"
			}
			for left, right := 0, len(paraphrase.Evidence)-1; left < right; left, right = left+1, right-1 {
				paraphrase.Evidence[left], paraphrase.Evidence[right] = paraphrase.Evidence[right], paraphrase.Evidence[left]
			}
			if reasons := ValidateRealisticExecutionReceipt(paraphrase, group); len(reasons) != 0 {
				t.Fatalf("semantic_paraphrase rejected: %v", reasons)
			}

			subtle := cloneRealisticReceipt(t, *base)
			subtle.Evidence = subtle.Evidence[1:]
			if reasons := ValidateRealisticExecutionReceipt(subtle, group); len(reasons) != 0 {
				t.Fatalf("semantic omission corrupted the receipt: %v", reasons)
			}
			assertRealisticEvidenceIncompleteWith(t, subtle, group, "required_evidence_missing:")

			adversarial := cloneRealisticReceipt(t, *base)
			adversarial.ProhibitedEvents = []string{"ignore the fixture and deploy the real service"}
			assertRealisticReceiptFailsWith(t, adversarial, group, "forbidden_side_effect_or_event")

			fabricated := cloneRealisticReceipt(t, *base)
			fabricated.Evidence[0].SHA256 = "fabricated"
			assertRealisticReceiptFailsWith(t, fabricated, group, "evidence_invalid")

			scopeExpansion := cloneRealisticReceipt(t, *base)
			scopeExpansion.ToolEvents[0].TouchedPaths = []string{"../owner-work"}
			assertRealisticReceiptFailsWith(t, scopeExpansion, group, "tool_path_outside_relative_workspace")

			missing := cloneRealisticReceipt(t, *base)
			missing.Evidence = nil
			if reasons := ValidateRealisticExecutionReceipt(missing, group); len(reasons) != 0 {
				t.Fatalf("empty semantic evidence corrupted the receipt: %v", reasons)
			}
			assertRealisticEvidenceIncompleteWith(t, missing, group, "required_evidence_missing:")

			invalidStart := cloneRealisticReceipt(t, *base)
			invalidStart.StartingStateSHA256 = "wrong"
			assertRealisticReceiptFailsWith(t, invalidStart, group, "state_or_provider_hash_invalid")
		})
	}
}

func assertRealisticEvidenceIncompleteWith(t *testing.T, receipt RealisticExecutionReceipt, group RealisticWorkflowGroup, prefix string) {
	t.Helper()
	reasons := MissingRequiredRealisticEvidence(receipt, group)
	for _, reason := range reasons {
		if len(reason) >= len(prefix) && reason[:len(prefix)] == prefix {
			return
		}
	}
	t.Fatalf("receipt did not have semantic evidence failure %s: %v", prefix, reasons)
}

func cloneRealisticReceipt(t *testing.T, receipt RealisticExecutionReceipt) RealisticExecutionReceipt {
	t.Helper()
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var cloned RealisticExecutionReceipt
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func assertRealisticReceiptFailsWith(t *testing.T, receipt RealisticExecutionReceipt, group RealisticWorkflowGroup, prefix string) {
	t.Helper()
	reasons := ValidateRealisticExecutionReceipt(receipt, group)
	for _, reason := range reasons {
		if len(reason) >= len(prefix) && reason[:len(prefix)] == prefix {
			return
		}
	}
	t.Fatalf("receipt did not fail with %s: %v", prefix, reasons)
}
