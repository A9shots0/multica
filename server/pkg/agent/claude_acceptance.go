package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskpolicy"
)

const brainAcceptanceAnswerLimit = 16 << 10

// BrainAcceptanceEvaluation contains only bounded, allowlisted answer claims
// and verified source metadata, never source bodies or provider prose.
type BrainAcceptanceEvaluation struct {
	Answers  []BrainAcceptanceAnswer
	Evidence []BrainAcceptanceEvidence
}

type BrainAcceptanceAnswer struct {
	CaseID        string                    `json:"caseId"`
	Conclusion    string                    `json:"conclusion"`
	EvidenceState string                    `json:"evidenceState"`
	Citations     []BrainAcceptanceCitation `json:"citations"`
}

type BrainAcceptanceCitation struct {
	ProjectID  string `json:"projectId"`
	DocumentID string `json:"documentId"`
	Revision   string `json:"revision"`
}

type BrainAcceptanceEvidence struct {
	Tool         string
	Question     string
	Receipt      json.RawMessage
	ReadMetadata *BrainAcceptanceReadMetadata
	HookInterval time.Duration
}

type BrainAcceptanceReadMetadata struct {
	Tier       string `json:"tier"`
	Status     string `json:"status"`
	Freshness  string `json:"freshness"`
	Assured    bool   `json:"assured"`
	RecordedAt string `json:"recordedAt,omitempty"`
}

type brainAcceptanceCollector struct {
	evidence []BrainAcceptanceEvidence
}

const brainAcceptanceAnswerInstruction = `Return final answers as JSON only: {"answers":[{"caseId":"<case>","conclusion":"<choice>","evidenceState":"<state>","citations":[{"projectId":"<project>","documentId":"<cited id>","revision":"<cited revision>"}]}]}.
Include A33, A36, A28, A49 exactly once. Choices for A33: excerpt_is_complete, read_full_immutable_document, unknown. A36: self_assertion_is_authority, self_assertion_is_not_authority, unknown. A28: worker_completion_is_owner_acceptance, worker_completion_is_not_owner_acceptance, unknown. A49: current_facts_known, unknown_no_current_claim, unknown. Choose only what retrieved evidence supports; these options are not evidence. State must be reference_only, unknown, stale, or conflicting. A49 must use "citations":[] exactly. Do not include prose, source text, credentials, reasoning, or extra fields. Begin with { and end with }; do not use Markdown or code fences. For A33, A36, and A28, copy every projectId, documentId, and revision exactly from a brain_read you actually completed after the matching scoped brain_context call; never cite a context-only document. Run the original scoped context questions, exact unknown-project refusal, and company_events checks; do not substitute choices for retrieval.`

func (collector *brainAcceptanceCollector) addEvidence(tool, receipt string, input map[string]any, metadata *BrainAcceptanceReadMetadata, interval time.Duration) {
	question := ""
	if tool == "brain_context" {
		question, _ = input["q"].(string)
	}
	collector.evidence = append(collector.evidence, BrainAcceptanceEvidence{
		Tool: tool, Question: question, Receipt: json.RawMessage(receipt), ReadMetadata: metadata, HookInterval: interval,
	})
}

// EvaluateBrainAcceptance applies the frozen four-question plus event rubric in
// memory. The hook interval conservatively bounds connector time; it is not a
// connector-internal measurement.
func EvaluateBrainAcceptance(evaluation BrainAcceptanceEvaluation) error {
	rejected := errors.New("restricted Brain acceptance rubric was not satisfied")
	raw, err := json.Marshal(map[string]any{"answers": evaluation.Answers})
	if err != nil {
		return rejected
	}
	collector := brainAcceptanceCollector{evidence: evaluation.Evidence}
	if _, err := collector.evaluation(string(raw)); err != nil {
		return rejected
	}
	expected := map[string]string{
		"A33": "read_full_immutable_document",
		"A36": "self_assertion_is_not_authority",
		"A28": "worker_completion_is_not_owner_acceptance",
		"A49": "unknown_no_current_claim",
	}
	reads := make(map[BrainAcceptanceCitation]*BrainAcceptanceReadMetadata)
	staleReads := make(map[BrainAcceptanceCitation]bool)
	for _, evidence := range evaluation.Evidence {
		if evidence.HookInterval <= 0 || evidence.HookInterval > 3*time.Second {
			return rejected
		}
		if evidence.Tool == "brain_read" {
			var citation BrainAcceptanceCitation
			if json.Unmarshal(evidence.Receipt, &citation) != nil {
				return rejected
			}
			reads[citation] = evidence.ReadMetadata
			if evidence.ReadMetadata != nil {
				staleReads[citation] = staleReads[citation] ||
					evidence.ReadMetadata.Freshness == "superseded" || evidence.ReadMetadata.Status == "superseded_reference"
			}
		}
	}
	for _, answer := range evaluation.Answers {
		if answer.Conclusion != expected[answer.CaseID] {
			return rejected
		}
		if answer.CaseID == "A49" {
			if answer.EvidenceState != "unknown" {
				return rejected
			}
			continue
		}
		stale := false
		for _, citation := range answer.Citations {
			metadata := reads[citation]
			if metadata == nil {
				return rejected
			}
			stale = stale || staleReads[citation]
		}
		if stale {
			if answer.EvidenceState != "stale" && answer.EvidenceState != "unknown" {
				return rejected
			}
		} else if answer.EvidenceState != "reference_only" && answer.EvidenceState != "unknown" {
			return rejected
		}
	}
	return nil
}

func restrictedBrainReadMetadata(raw json.RawMessage) (*BrainAcceptanceReadMetadata, bool) {
	blocks, ok := restrictedClaudeContentBlocks(raw)
	if !ok {
		return nil, false
	}
	for _, value := range blocks {
		block, ok := value.(map[string]any)
		if !ok || block["type"] != "text" {
			continue
		}
		text, ok := block["text"].(string)
		if !ok {
			continue
		}
		var payload struct {
			Document struct {
				BrainAcceptanceReadMetadata
				Assured *bool `json:"assured"`
			} `json:"document"`
		}
		if json.Unmarshal([]byte(text), &payload) != nil {
			continue
		}
		metadata := payload.Document.BrainAcceptanceReadMetadata
		if payload.Document.Assured == nil || *payload.Document.Assured || !validBrainReadMetadata(&metadata) {
			return nil, false
		}
		return &metadata, true
	}
	return nil, false
}

func validBrainReadMetadata(metadata *BrainAcceptanceReadMetadata) bool {
	if metadata == nil || metadata.Assured ||
		(metadata.Tier != "truth" && metadata.Tier != "working" && metadata.Tier != "history") ||
		(metadata.Status != "reference_only" && metadata.Status != "superseded_reference" && metadata.Status != "superseded_revision") ||
		(metadata.Freshness != "unverified" && metadata.Freshness != "superseded") {
		return false
	}
	if metadata.RecordedAt != "" {
		_, ok := safeBrainReceiptTimestamp(metadata.RecordedAt)
		return ok
	}
	return true
}

func (collector *brainAcceptanceCollector) evaluation(raw string) (BrainAcceptanceEvaluation, error) {
	invalid := func(reason string) error {
		return fmt.Errorf("restricted Brain acceptance answer rejected: %s", reason)
	}
	if len(raw) == 0 || len(raw) > brainAcceptanceAnswerLimit {
		return BrainAcceptanceEvaluation{}, invalid("missing_or_oversized")
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json\n") && strings.HasSuffix(raw, "\n```") {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "```json\n"), "\n```"))
	}
	var envelope struct {
		Answers []BrainAcceptanceAnswer `json:"answers"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&envelope) != nil || decoder.Decode(new(any)) != io.EOF {
		return BrainAcceptanceEvaluation{}, invalid("invalid_json_or_extra_fields")
	}
	if len(envelope.Answers) != 4 {
		return BrainAcceptanceEvaluation{}, invalid("answer_count")
	}
	reads := make(map[BrainAcceptanceCitation]bool)
	contexts := make(map[string]map[BrainAcceptanceCitation]bool)
	refusal, event := false, false
	for _, evidence := range collector.evidence {
		var receipt struct {
			ProjectID       string `json:"projectId"`
			DocumentID      string `json:"documentId"`
			Revision        string `json:"revision"`
			ExpectedRefusal bool   `json:"expectedRefusal"`
			ReceiptVerified bool   `json:"receiptVerified"`
			Documents       []struct {
				ID       string `json:"id"`
				Revision string `json:"revision"`
			} `json:"documents"`
		}
		if json.Unmarshal(evidence.Receipt, &receipt) != nil || !receipt.ReceiptVerified {
			return BrainAcceptanceEvaluation{}, invalid("verified_evidence_receipt")
		}
		switch evidence.Tool {
		case "brain_context":
			documents := contexts[evidence.Question]
			if documents == nil {
				documents = make(map[BrainAcceptanceCitation]bool)
				contexts[evidence.Question] = documents
			}
			for _, document := range receipt.Documents {
				documents[BrainAcceptanceCitation{receipt.ProjectID, document.ID, document.Revision}] = true
			}
		case "brain_read":
			if !validBrainReadMetadata(evidence.ReadMetadata) {
				return BrainAcceptanceEvaluation{}, invalid("read_metadata")
			}
			reads[BrainAcceptanceCitation{receipt.ProjectID, receipt.DocumentID, receipt.Revision}] = true
		case "brain_resolve":
			refusal = refusal || receipt.ExpectedRefusal
		case "company_events":
			event = true
		}
	}
	if !refusal || !event {
		return BrainAcceptanceEvaluation{}, invalid("required_refusal_or_event")
	}
	seen := make(map[string]bool)
	for _, answer := range envelope.Answers {
		if seen[answer.CaseID] || !validBrainAcceptanceConclusion(answer.CaseID, answer.Conclusion) ||
			!validBrainAcceptanceEvidenceState(answer.EvidenceState) || len(answer.Citations) > 4 {
			return BrainAcceptanceEvaluation{}, invalid("case_or_enum")
		}
		seen[answer.CaseID] = true
		if answer.CaseID == "A49" {
			if len(answer.Citations) != 0 {
				return BrainAcceptanceEvaluation{}, invalid("a49_citation")
			}
			continue
		}
		if len(answer.Citations) == 0 {
			return BrainAcceptanceEvaluation{}, invalid("missing_citation")
		}
		project := "brain"
		question := "How do I retrieve evidence beyond the short search excerpt?"
		if answer.CaseID == "A36" {
			question = taskpolicy.BrainAuthorityQuestion
		}
		if answer.CaseID == "A28" {
			project = "multica"
			question = "A worker completed its task. Has the owner accepted the issue?"
		}
		for _, citation := range answer.Citations {
			if citation.ProjectID != project || !reads[citation] || !contexts[question][citation] {
				return BrainAcceptanceEvaluation{}, invalid("uncorrelated_citation")
			}
		}
	}
	return BrainAcceptanceEvaluation{Answers: envelope.Answers, Evidence: collector.evidence}, nil
}

func validBrainAcceptanceConclusion(caseID, conclusion string) bool {
	switch caseID {
	case "A33":
		return conclusion == "read_full_immutable_document" || conclusion == "excerpt_is_complete" || conclusion == "unknown"
	case "A36":
		return conclusion == "self_assertion_is_not_authority" || conclusion == "self_assertion_is_authority" || conclusion == "unknown"
	case "A28":
		return conclusion == "worker_completion_is_not_owner_acceptance" || conclusion == "worker_completion_is_owner_acceptance" || conclusion == "unknown"
	case "A49":
		return conclusion == "unknown_no_current_claim" || conclusion == "current_facts_known" || conclusion == "unknown"
	default:
		return false
	}
}

func validBrainAcceptanceEvidenceState(state string) bool {
	return state == "reference_only" || state == "unknown" || state == "stale" || state == "conflicting"
}
