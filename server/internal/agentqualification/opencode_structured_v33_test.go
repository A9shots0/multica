package agentqualification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func v33StructuredFor(t *testing.T, c OpenCodeStructuredV33Capsule) string {
	t.Helper()
	var r openCodeStructuredV33Request
	if err := json.Unmarshal(c.RequestBody, &r); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(r.Format.Schema, []byte(`"entry_step"`)) {
		return `{"entry_step":"s1","steps":[{"id":"s1","action":{"kind":"finish","summary":"done"},"on_success":"done","on_failure":"stop"}]}`
	}
	return `{"action":{"kind":"finish","summary":"done"}}`
}
func v33Response(t *testing.T, c OpenCodeStructuredV33Capsule, structured string) []byte {
	t.Helper()
	return []byte(fmt.Sprintf(`{"info":{"id":"msg_assistant","sessionID":%q,"role":"assistant","time":{"created":1,"completed":2},"parentID":%q,"modelID":%q,"providerID":"openrouter","mode":"primary","agent":"qualification","path":{"cwd":"C:/sealed/r05","root":"C:/sealed/r05"},"cost":0.01,"tokens":{"total":17,"input":10,"output":2,"reasoning":3,"cache":{"read":1,"write":1}},"structured":%s,"variant":%q,"finish":"stop"},"parts":[{"id":"prt_start","sessionID":%q,"messageID":"msg_assistant","type":"step-start"},{"id":"prt_finish","sessionID":%q,"messageID":"msg_assistant","type":"step-finish","reason":"stop","cost":0.01,"tokens":{"total":17,"input":10,"output":2,"reasoning":3,"cache":{"read":1,"write":1}}}]}`, c.SessionID, c.RequestMessageID, strings.TrimPrefix(c.BindingID[strings.Index(c.BindingID, ".")+1:], ""), structured, func() string {
		var r openCodeStructuredV33Request
		_ = json.Unmarshal(c.RequestBody, &r)
		return r.Variant
	}(), c.SessionID, c.SessionID))
}

func validV33Response(t *testing.T, c OpenCodeStructuredV33Capsule) []byte {
	var r openCodeStructuredV33Request
	_ = json.Unmarshal(c.RequestBody, &r)
	raw := v33Response(t, c, v33StructuredFor(t, c))
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	env["info"].(map[string]any)["modelID"] = r.Model.ModelID
	out, _ := json.Marshal(env)
	return out
}

func TestOpenCodeStructuredV33CapsulesBindExactSchemaRouteAndNoCLIFormat(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	for _, b := range ProtocolAblationV32Bindings(d) {
		c, err := BuildOpenCodeStructuredV33Capsule(d, b)
		if err != nil {
			t.Fatal(err)
		}
		var r openCodeStructuredV33Request
		_ = json.Unmarshal(c.RequestBody, &r)
		p, _ := BuildProtocolAblationV32OpenCodePlan(d, b)
		if !bytes.Equal(r.Format.Schema, p.Schema) || r.Format.RetryCount != 0 || r.Parts[0].Text != p.Prompt || r.MessageID != c.RequestMessageID {
			t.Fatal("exact request drift")
		}
		if bytes.Count(c.RequestBody, []byte(`"schema":`)) != 1 {
			t.Fatal("schema was not sent exactly once")
		}
		if c.RoutePolicy != openCodeV33RoutePolicy || c.ResolvedConfigSHA256 == "" || contains(c.ServerArgs, "--format") || contains(c.ServerArgs, "run") || c.CLIFormatJSONEnforcement {
			t.Fatal("route/CLI boundary drift")
		}
	}
}

func TestOpenCodeStructuredV33ValidatesSchemaBeforeReceipt(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	for _, b := range ProtocolAblationV32Bindings(d) {
		c, _ := BuildOpenCodeStructuredV33Capsule(d, b)
		raw := validV33Response(t, c)
		r, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json; charset=utf-8", raw)
		if err != nil {
			t.Fatalf("%s: %v", b.BindingID, err)
		}
		if err := ValidateOpenCodeStructuredV33Receipt(d, b, c, r); err != nil {
			t.Fatal(err)
		}
		bad := bytes.Replace(raw, []byte(`"finish"`), []byte(`"bogus"`), 1)
		if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", bad); err == nil {
			t.Fatal("schema-invalid output accepted")
		}
	}
}

func TestOpenCodeStructuredV33RejectsRetryToolsExtraStepsAndAccountingDrift(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	c, _ := BuildOpenCodeStructuredV33Capsule(d, b)
	raw := validV33Response(t, c)
	for _, kind := range []string{"retry", "tool", "subtask", "patch"} {
		bad := bytes.Replace(raw, []byte(`"step-start"`), []byte(`"`+kind+`"`), 1)
		if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", bad); err == nil {
			t.Fatalf("%s accepted", kind)
		}
	}
	extra := bytes.Replace(raw, []byte(`"parts":[`), []byte(`"parts":[{"id":"extra","sessionID":"`+c.SessionID+`","messageID":"msg_assistant","type":"step-start"},`), 1)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", extra); err == nil {
		t.Fatal("multiple steps accepted")
	}
	var driftEnvelope map[string]any
	_ = json.Unmarshal(raw, &driftEnvelope)
	driftEnvelope["info"].(map[string]any)["cost"] = 0.02
	drift, _ := json.Marshal(driftEnvelope)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", drift); err == nil {
		t.Fatal("assistant/step accounting mismatch accepted")
	}
}

func TestOpenCodeStructuredV33RequiresReconciledDocumentedTokenTotals(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	c, _ := BuildOpenCodeStructuredV33Capsule(d, b)
	raw := validV33Response(t, c)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", raw); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	_ = json.Unmarshal(raw, &envelope)
	infoTokens := envelope["info"].(map[string]any)["tokens"].(map[string]any)
	infoTokens["total"] = float64(18)
	bad, _ := json.Marshal(envelope)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", bad); err == nil {
		t.Fatal("assistant token total mismatch accepted")
	}
	_ = json.Unmarshal(raw, &envelope)
	parts := envelope["parts"].([]any)
	parts[1].(map[string]any)["tokens"].(map[string]any)["total"] = float64(16)
	bad, _ = json.Marshal(envelope)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", bad); err == nil {
		t.Fatal("step token total mismatch accepted")
	}
	_ = json.Unmarshal(raw, &envelope)
	delete(envelope["info"].(map[string]any)["tokens"].(map[string]any), "total")
	bad, _ = json.Marshal(envelope)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", bad); err == nil {
		t.Fatal("missing documented token total accepted")
	}
}

func TestOpenCodeStructuredV33ReceiptReparseDetectsEveryDerivedMutation(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	c, _ := BuildOpenCodeStructuredV33Capsule(d, b)
	r, _ := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", validV33Response(t, c))
	r.PromptSHA256 = "tampered"
	if err := ValidateOpenCodeStructuredV33Receipt(d, b, c, r); err == nil {
		t.Fatal("derived mutation accepted")
	}
}

func TestOpenCodeStructuredV33HTTPClientMakesExactlyOneExactCall(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	c, _ := BuildOpenCodeStructuredV33Capsule(d, b)
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || !bytes.Equal(body, c.RequestBody) || r.Header.Get("Content-Type") != "application/json" {
			t.Error("request drift")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(validV33Response(t, c))
	}))
	defer s.Close()
	client := OpenCodeStructuredV33HTTPClient{Client: s.Client(), BaseURL: s.URL}
	if _, err := client.PromptOnce(context.Background(), d, b, c); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	client.BaseURL = "https://example.com"
	if _, err := client.PromptOnce(context.Background(), d, b, c); err == nil {
		t.Fatal("non-loopback accepted")
	}
}

type v33RoundTripFunc func(*http.Request) (*http.Response, error)

func (f v33RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenCodeStructuredV33TimeoutIsHashBoundAndNeverRetried(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	c, _ := BuildOpenCodeStructuredV33Capsule(d, b)
	if c.HTTPTimeoutSeconds != 120 {
		t.Fatalf("timeout=%d", c.HTTPTimeoutSeconds)
	}
	tampered := c
	tampered.HTTPTimeoutSeconds = 15
	if err := ValidateOpenCodeStructuredV33Capsule(d, b, tampered); err == nil || hashOpenCodeV33Capsule(tampered) == hashOpenCodeV33Capsule(c) {
		t.Fatal("timeout drift was not rejected and hash-bound")
	}
	calls := 0
	h := OpenCodeStructuredV33HTTPClient{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Transport: v33RoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, context.DeadlineExceeded
	})}}
	if _, err := h.PromptOnce(context.Background(), d, b, c); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout not preserved: %v", err)
	}
	if calls != 1 {
		t.Fatalf("HTTP/model call retried: %d", calls)
	}
}

func TestOpenCodeStructuredV33FailureClassification(t *testing.T) {
	for _, class := range []string{"competence_failure", "response_or_tool_protocol_failure", "candidate_resource_exhaustion"} {
		if got := ClassifyOpenCodeStructuredV33Failure(class); got != "candidate_local" {
			t.Fatalf("%s: %s", class, got)
		}
	}
	for _, class := range []string{"credential_failure", "runtime_failure", "config_failure", "credits_failure", "transport_failure", "evidence_integrity_failure", "unsafe_execution", "campaign_budget_breach"} {
		if got := ClassifyOpenCodeStructuredV33Failure(class); got != "global_halt" {
			t.Fatalf("%s: %s", class, got)
		}
	}
	if got := ClassifyOpenCodeStructuredV33Failure("mystery"); got != "global_halt_unclassified" {
		t.Fatal(got)
	}
	if e, err := SealOpenCodeStructuredV33GlobalHalt("transport_failure", "abc"); err != nil || e.Kind != "campaign_global_halt_sealed" {
		t.Fatalf("%+v %v", e, err)
	}
	if _, err := SealOpenCodeStructuredV33GlobalHalt("competence_failure", "abc"); err == nil {
		t.Fatal("local failure sealed as campaign halt")
	}
}

func TestOpenCodeStructuredV33HTTPAndEnvelopeBoundaries(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	c, _ := BuildOpenCodeStructuredV33Capsule(d, ProtocolAblationV32Bindings(d)[0])
	raw := validV33Response(t, c)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 500, "application/json", raw); err == nil {
		t.Fatal("status accepted")
	}
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "text/plain", raw); err == nil {
		t.Fatal("content type accepted")
	}
	bad := bytes.Replace(raw, []byte(c.RequestMessageID), []byte("msg_wrong"), 1)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", bad); err == nil {
		t.Fatal("wrong parent accepted")
	}
}

func TestOpenCodeStructuredV33StructuredErrorIsDistinctAndZeroRetry(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	c, _ := BuildOpenCodeStructuredV33Capsule(d, ProtocolAblationV32Bindings(d)[0])
	raw := validV33Response(t, c)
	var e map[string]any
	_ = json.Unmarshal(raw, &e)
	info := e["info"].(map[string]any)
	delete(info, "structured")
	info["error"] = map[string]any{"name": "StructuredOutputError", "data": map[string]any{"message": "bad", "retries": 0}}
	raw, _ = json.Marshal(e)
	r, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", raw)
	if err != nil || r.Outcome != "structured_output_error" {
		t.Fatalf("%+v %v", r, err)
	}
	raw = bytes.Replace(raw, []byte(`"retries":0`), []byte(`"retries":1`), 1)
	if _, err := ParseOpenCodeStructuredV33HTTPResponse(c, 200, "application/json", raw); err == nil {
		t.Fatal("retry accepted")
	}
}

func TestOpenCodeStructuredV33AggregateAdmissionOverflowAndBoundaries(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	reserve, okPrice := OpenCodeStructuredV33CostReservation(d, b.Candidate)
	if !okPrice {
		t.Fatal("valid fixed price overflowed")
	}
	ok := OpenCodeStructuredV33Aggregate{Tokens: d.MaxCombinedTokens - d.PerObservationTotalTokenReservation, CostMicros: d.MaxCombinedCostMicros - reserve, ProviderTurns: d.MaxCombinedProviderTurns - b.Protocol.MaxProviderTurns}
	if err := AdmitOpenCodeStructuredV33Observation(d, b, ok); err != nil {
		t.Fatal(err)
	}
	for _, a := range []OpenCodeStructuredV33Aggregate{{Tokens: math.MaxInt64}, {CostMicros: math.MaxInt64}, {ProviderTurns: math.MaxInt}} {
		if err := AdmitOpenCodeStructuredV33Observation(d, b, a); err == nil {
			t.Fatal("overflow accepted")
		}
	}
	toxic := b.Candidate
	toxic.InputMicrosPerMillion = math.MaxInt64
	if _, ok := OpenCodeStructuredV33CostReservation(d, toxic); ok {
		t.Fatal("price multiplication overflow accepted")
	}
}

func TestOpenCodeStructuredV33StepAdmissionCannotOvershootObservationCaps(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	c, _ := BuildOpenCodeStructuredV33Capsule(d, ProtocolAblationV32Bindings(d)[0])
	used := ProtocolAblationV32Usage{InputTokens: 40_000, OutputTokens: 8_000, ReasoningTokens: 2_000}
	next, err := PrepareOpenCodeStructuredV33Step(d, c, used, 15_000)
	if err != nil {
		t.Fatal(err)
	}
	if next.GeneratedTokenCeiling != 10_000 {
		t.Fatalf("ceiling=%d", next.GeneratedTokenCeiling)
	}
	combined, ok := safeAddV33(used.InputTokens, used.OutputTokens, used.ReasoningTokens, 15_000, next.GeneratedTokenCeiling)
	if !ok || combined > d.PerObservationTotalTokenReservation {
		t.Fatal("next call can exceed combined cap")
	}
	if used.OutputTokens+used.ReasoningTokens+next.GeneratedTokenCeiling > d.PerObservationGeneratedTokenCeiling {
		t.Fatal("next call can exceed generated cap")
	}
	if _, err := PrepareOpenCodeStructuredV33Step(d, c, used, 25_001); !errors.Is(err, ErrOpenCodeStructuredV33CandidateResourceExhaustion) {
		t.Fatal("unadmittable input accepted")
	}
}

func TestOpenCodeStructuredV33ResourceExhaustionCompletesOnlyCandidate(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	ledger := ProtocolAblationV32Ledger{}
	decision, _, err := BeginProtocolAblationV32Observation(d, &ledger, b)
	if err != nil || decision != "launch" {
		t.Fatalf("claim: %s %v", decision, err)
	}
	observation := sealProtocolAblationV32Observation(b, ProtocolAblationV32Observation{
		BindingID:    b.BindingID,
		Outcome:      "failed",
		FailureClass: "candidate_resource_exhaustion",
		FailureScope: "observation",
		Usage:        ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: 55_000, OutputTokens: 19_000, ReasoningTokens: 1_000, CostMicros: 1},
	})
	if err := CompleteOpenCodeStructuredV33ResourceExhaustion(d, &ledger, b, observation); err != nil {
		t.Fatal(err)
	}
	completed := ledger.Claims[b.BindingID].Completed
	if ledger.GlobalHalt || completed == nil || completed.FailureClass != "candidate_resource_exhaustion" || completed.FailureScope != "observation" {
		t.Fatalf("resource exhaustion was not candidate-local: %+v", ledger)
	}
}

type fakeV33Lifecycle struct {
	events    *[]string
	failReady bool
}

func (f fakeV33Lifecycle) Start(context.Context, []string, []string) error {
	*f.events = append(*f.events, "start")
	return nil
}
func (f fakeV33Lifecycle) Ready(context.Context) (string, string, string, string, string, error) {
	*f.events = append(*f.events, "ready")
	if f.failReady {
		return "", "", "", "", "", fmt.Errorf("not ready")
	}
	return OpenCodeStructuredV33Version, OpenCodeStructuredV33ExecutableSHA256, OpenCodeStructuredV33OpenAPISHA256, "resolved", "http://127.0.0.1:1234", nil
}
func (f fakeV33Lifecycle) Shutdown(context.Context) error {
	*f.events = append(*f.events, "shutdown")
	return nil
}
func TestOpenCodeStructuredV33LifecycleAndRecovery(t *testing.T) {
	d, _ := CurrentProtocolAblationV32()
	b := ProtocolAblationV32Bindings(d)[0]
	c, _ := BuildOpenCodeStructuredV33CapsuleForSession(d, b, "ses_x", "msg_x", "C:/sealed/r05", "resolved")
	events := []string{}
	p, err := RunOpenCodeStructuredV33ProviderFreePreflight(context.Background(), c, fakeV33Lifecycle{events: &events})
	if err != nil || !p.ShutdownObserved || strings.Join(events, ",") != "start,ready,shutdown" {
		t.Fatalf("%+v %v %v", p, err, events)
	}
	if err := ValidateOpenCodeStructuredV33Preflight(p); err != nil {
		t.Fatal(err)
	}
	events = nil
	_, _ = RunOpenCodeStructuredV33ProviderFreePreflight(context.Background(), c, fakeV33Lifecycle{events: &events, failReady: true})
	if strings.Join(events, ",") != "start,ready,shutdown" {
		t.Fatal("shutdown not guaranteed")
	}
	claim := []OpenCodeStructuredV33DurableEvent{{Kind: "preflight_sealed", EvidenceSHA256: "a"}, {Kind: "claim_sealed", BindingID: b.BindingID, EvidenceSHA256: "b"}}
	if got := RecoverOpenCodeStructuredV33Claim(claim); got != "global_halt_ambiguous_transport_no_retry" {
		t.Fatal(got)
	}
	full := append(claim, OpenCodeStructuredV33DurableEvent{Kind: "raw_response_sealed", BindingID: b.BindingID, EvidenceSHA256: "c"}, OpenCodeStructuredV33DurableEvent{Kind: "observation_sealed", BindingID: b.BindingID, EvidenceSHA256: "d"}, OpenCodeStructuredV33DurableEvent{Kind: "ledger_sealed", BindingID: b.BindingID, EvidenceSHA256: "e"})
	if err := ValidateOpenCodeStructuredV33DurableOrder(full); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCodeStructuredV33CreatedSessionIsFreshAndBound(t *testing.T) {
	raw := []byte(`{"id":"ses_x","slug":"x","projectID":"p","directory":"C:/sealed/r05","title":"v33","version":"1.18.18","time":{"created":1,"updated":2}}`)
	r, err := ParseOpenCodeStructuredV33CreatedSession(200, "application/json", raw)
	if err != nil || r.ID != "ses_x" || r.RawSHA256 != hashRealisticNextBytes(raw) {
		t.Fatalf("%+v %v", r, err)
	}
	bad := bytes.Replace(raw, []byte(`"title"`), []byte(`"parentID":"ses_parent","title"`), 1)
	if _, err := ParseOpenCodeStructuredV33CreatedSession(200, "application/json", bad); err == nil {
		t.Fatal("parented session accepted")
	}
}
