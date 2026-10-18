package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskpolicy"
)

func syntheticBrainAcceptanceAnswers() string {
	answers := []BrainAcceptanceAnswer{}
	for _, item := range []struct{ id, conclusion, project string }{
		{"A33", "read_full_immutable_document", "brain"},
		{"A36", "self_assertion_is_not_authority", "brain"},
		{"A28", "worker_completion_is_not_owner_acceptance", "multica"},
		{"A49", "unknown_no_current_claim", ""},
	} {
		answer := BrainAcceptanceAnswer{CaseID: item.id, Conclusion: item.conclusion, EvidenceState: "reference_only"}
		if item.project != "" {
			answer.Citations = []BrainAcceptanceCitation{{item.project, item.project + "-doc", revisionHashFixture()}}
		} else {
			answer.EvidenceState = "unknown"
			answer.Citations = []BrainAcceptanceCitation{}
		}
		answers = append(answers, answer)
	}
	raw, _ := json.Marshal(map[string]any{"answers": answers})
	return string(raw)
}

func syntheticBrainAcceptanceCollector() *brainAcceptanceCollector {
	collector := &brainAcceptanceCollector{}
	for _, item := range []struct{ project, question string }{
		{"brain", "How do I retrieve evidence beyond the short search excerpt?"},
		{"brain", taskpolicy.BrainAuthorityQuestion},
		{"multica", "A worker completed its task. Has the owner accepted the issue?"},
	} {
		collector.addEvidence("brain_context", fmt.Sprintf(`{"receiptVerified":true,"projectId":%q,"documents":[{"id":%q,"revision":%q}]}`, item.project, item.project+"-doc", revisionHashFixture()), map[string]any{"q": item.question}, nil)
		collector.addEvidence("brain_read", fmt.Sprintf(`{"receiptVerified":true,"projectId":%q,"documentId":%q,"revision":%q}`, item.project, item.project+"-doc", revisionHashFixture()), nil,
			&BrainAcceptanceReadMetadata{Tier: "working", Status: "reference_only", Freshness: "unverified"})
	}
	collector.addEvidence("brain_resolve", `{"receiptVerified":true,"expectedRefusal":true}`, nil, nil)
	collector.addEvidence("company_events", `{"receiptVerified":true}`, nil, nil)
	return collector
}

func TestBrainAcceptanceEvaluationRejectsUncorrelatedOrPrivateAnswers(t *testing.T) {
	valid := syntheticBrainAcceptanceAnswers()
	if _, err := syntheticBrainAcceptanceCollector().evaluation(valid); err != nil {
		t.Fatal(err)
	}
	wrong := strings.Replace(valid, "read_full_immutable_document", "excerpt_is_complete", 1)
	if observed, err := syntheticBrainAcceptanceCollector().evaluation(wrong); err != nil ||
		observed.Answers[0].Conclusion != "excerpt_is_complete" {
		t.Fatal("a wrong semantic answer must reach the evaluator distinctly from verified retrieval")
	}
	for _, raw := range []string{
		strings.Replace(valid, `"answers":`, `"private":"SENSITIVE_TEXT","answers":`, 1),
		strings.Replace(valid, `"conclusion":"read_full_immutable_document"`, `"conclusion":"SENSITIVE_BODY"`, 1),
		strings.Replace(valid, `"documentId":"brain-doc"`, `"documentId":"uncited-doc"`, 1),
		strings.Replace(valid, `"projectId":"multica"`, `"projectId":"brain"`, 1),
		strings.Replace(valid, `"caseId":"A28"`, `"caseId":"A33"`, 1),
		valid + `{}`,
		strings.Repeat("x", brainAcceptanceAnswerLimit+1),
	} {
		if _, err := syntheticBrainAcceptanceCollector().evaluation(raw); err == nil {
			t.Fatal("accepted private, oversized, duplicate, or uncorrelated answer")
		}
	}
	collector := syntheticBrainAcceptanceCollector()
	collector.evidence = collector.evidence[:len(collector.evidence)-1]
	if _, err := collector.evaluation(valid); err == nil {
		t.Fatal("accepted answers without the exact event evidence")
	}
	collector = syntheticBrainAcceptanceCollector()
	collector.evidence[2].Question = "wrong question"
	if _, err := collector.evaluation(valid); err == nil {
		t.Fatal("accepted citations without the A36 scoped context")
	}
}

func TestBrainAcceptanceReadMetadataUsesOnlySourceVerifiedFields(t *testing.T) {
	document := `{"tier":"history","status":"superseded_revision","freshness":"unverified","assured":false,"recordedAt":"2000-01-01T00:00:00Z","body":"SENSITIVE_BODY","sourceMetadata":{"secret":"SENSITIVE_METADATA"}}`
	content := func(document string) json.RawMessage {
		payload := `{"document":` + document + `}`
		return json.RawMessage(fmt.Sprintf(`[{"type":"text","text":%q}]`, payload))
	}
	metadata, ok := restrictedBrainReadMetadata(content(document))
	if !ok || metadata.Status != "superseded_revision" || metadata.Freshness != "unverified" || metadata.Assured {
		t.Fatal("historical read changed source freshness or assurance")
	}
	safe, _ := json.Marshal(metadata)
	if bytes.Contains(safe, []byte("SENSITIVE")) {
		t.Fatal("read metadata included arbitrary source content")
	}
	for _, invalid := range []string{
		strings.Replace(document, `"assured":false`, `"assured":true`, 1),
		strings.Replace(document, `"assured":false`, `"assured":"false"`, 1),
		strings.Replace(document, `"assured":false,`, "", 1),
		strings.Replace(document, `"history"`, `"SENSITIVE_TIER"`, 1),
		strings.Replace(document, `"superseded_revision"`, `"verified"`, 1),
		strings.Replace(document, `"unverified"`, `"current"`, 1),
		strings.Replace(document, `"2000-01-01T00:00:00Z"`, `"bad-time"`, 1),
	} {
		if _, ok := restrictedBrainReadMetadata(content(invalid)); ok {
			t.Fatal("unsupported source metadata was accepted")
		}
	}
}

func TestClaudeRestrictedTransientEvaluation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake executable")
	}
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprintf("reject=%t", reject), func(t *testing.T) {
			fakePath := filepath.Join(t.TempDir(), "claude")
			writeTestExecutable(t, fakePath, []byte("#!/bin/sh\nexec "+shellQuote(os.Args[0])+" -test.run=^TestRestrictedClaudeFakeCLIHelper$ -- \"$@\"\n"))
			var logs bytes.Buffer
			backend, err := New("claude", Config{
				ExecutablePath: fakePath, CLIVersion: "2.1.300", BuiltinRuntime: true,
				Env: map[string]string{
					"IS_SANDBOX": "1", "MULTICA_RESTRICTED_CLAUDE_HELPER": "1",
					"MULTICA_RESTRICTED_CLAUDE_EVALUATION": "1",
				},
				Logger: slog.New(slog.NewTextHandler(&logs, nil)),
			})
			if err != nil {
				t.Fatal(err)
			}
			var observed BrainAcceptanceEvaluation
			session, err := backend.Execute(t.Context(), "synthetic acceptance only", ExecOptions{
				ExecutionPolicy: brainAcceptancePolicy(t),
				McpConfig:       json.RawMessage(`{"mcpServers":{"brain":{"type":"http","url":"https://brain.invalid/mcp"}}}`),
				Timeout:         5 * time.Second,
				EvaluateBrainAcceptance: func(evaluation BrainAcceptanceEvaluation) error {
					observed = evaluation
					want := map[string]string{
						"A33": "read_full_immutable_document",
						"A36": "self_assertion_is_not_authority",
						"A28": "worker_completion_is_not_owner_acceptance",
						"A49": "unknown_no_current_claim",
					}
					for _, answer := range evaluation.Answers {
						if answer.Conclusion != want[answer.CaseID] {
							return errors.New("incorrect synthetic answer")
						}
					}
					if reject {
						return errors.New("SENSITIVE_EVALUATOR_ERROR")
					}
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			var messages []Message
			for message := range session.Messages {
				messages = append(messages, message)
			}
			result := <-session.Result
			if (result.Status == "completed") == reject {
				t.Fatalf("evaluator rejection=%t status=%s error=%s", reject, result.Status, result.Error)
			}
			if len(observed.Answers) != 4 || len(observed.Evidence) != 8 {
				t.Fatalf("transient evaluator did not receive all four answers and eight correlated receipts")
			}
			transient, _ := json.Marshal(observed)
			if bytes.Contains(transient, []byte("SENSITIVE")) {
				t.Fatal("source bodies or provider prose reached the transient evaluator")
			}
			durable, _ := json.Marshal(struct {
				Result   Result
				Messages []Message
			}{result, messages})
			combined := string(durable) + logs.String()
			for _, private := range []string{"brain-doc", "multica-doc", revisionHashFixture(), "read_full_immutable_document", "SENSITIVE", "source_event:199901"} {
				if strings.Contains(combined, private) {
					t.Fatalf("transient evaluator data leaked into durable output or logs: %q", private)
				}
			}
		})
	}
}
