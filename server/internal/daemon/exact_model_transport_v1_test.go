package daemon

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const (
	guardedExactModelFakeModel  = "fake-exact-model-v1"
	guardedExactModelFakePrompt = "fixed provider-free exact-model prompt"
)

type guardedExactModelFakeProviderV1 struct {
	mu           sync.Mutex
	requests     int
	credentialOK bool
	requestBody  []byte
	response     string
	status       int
	location     string
	err          error
}

func (f *guardedExactModelFakeProviderV1) RoundTrip(request *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	f.credentialOK = request.Header.Get("x-api-key") == "fake-credential-only"
	f.requestBody, _ = io.ReadAll(request.Body)
	if f.err != nil {
		return nil, f.err
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	response := f.response
	if response == "" {
		response = guardedExactModelFakeResponseV1(guardedExactModelFakeModel, `{"ok":true}`)
	}
	header := http.Header{"Content-Type": []string{"application/json"}}
	if f.location != "" {
		header.Set("Location", f.location)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(response)),
		Request:    request,
	}, nil
}

func guardedExactModelFakeResponseV1(model, text string) string {
	encodedText, _ := json.Marshal(text)
	return `{"id":"msg_fake","type":"message","role":"assistant","model":"` + model + `","container":null,"content":[{"type":"text","text":` + string(encodedText) + `,"citations":[]}],"stop_reason":"end_turn","stop_sequence":null,"stop_details":null,"usage":{"input_tokens":7,"output_tokens":3,"cache_creation":{"ephemeral_1h_input_tokens":0,"ephemeral_5m_input_tokens":0},"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens_details":{"thinking_tokens":1},"server_tool_use":{"web_fetch_requests":0,"web_search_requests":0},"service_tier":"standard","inference_geo":"global"}}`
}

func guardedExactModelFakeCredentialV1(opens, closes *int) guardedExactModelCredentialOpenerV1 {
	return func() (io.ReadCloser, error) {
		*opens++
		return &guardedExactModelTrackingReadCloserV1{
			Reader: strings.NewReader("fake-credential-only\n"),
			closed: closes,
		}, nil
	}
}

type guardedExactModelTrackingReadCloserV1 struct {
	io.Reader
	closed *int
}

func (r *guardedExactModelTrackingReadCloserV1) Close() error {
	*r.closed++
	return nil
}

func newGuardedExactModelFakeExecutorV1(t *testing.T, provider http.RoundTripper) *guardedExactModelTransportV1 {
	t.Helper()
	policy, err := agentqualification.NewGuardedExactModelTransportV1(guardedExactModelFakeModel, guardedExactModelFakePrompt)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := newGuardedExactModelTransportV1(policy, guardedExactModelFakeModel, guardedExactModelFakePrompt, provider)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func TestGuardedExactModelTransportOneFakeProviderRequest(t *testing.T) {
	provider := &guardedExactModelFakeProviderV1{}
	executor := newGuardedExactModelFakeExecutorV1(t, provider)
	opens, closes := 0, 0
	receipt, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes))
	if err != nil {
		t.Fatal(err)
	}
	if opens != 1 || closes != 1 || provider.requests != 1 || !provider.credentialOK {
		t.Fatalf("custody/requests opens=%d closes=%d requests=%d credential=%t", opens, closes, provider.requests, provider.credentialOK)
	}
	if receipt.RequestedModel != guardedExactModelFakeModel || receipt.ObservedModel != guardedExactModelFakeModel ||
		receipt.ProviderRequests != 1 || receipt.InputTokens != 7 || receipt.OutputTokens != 3 || receipt.ThinkingTokens != 1 ||
		receipt.CostUSDMicros != 66 || receipt.ServiceTier != "standard" || receipt.InferenceGeo != "global" || receipt.Output != `{"ok":true}` ||
		len(receipt.RequestSHA256) != 64 || len(receipt.ResponseSHA256) != 64 {
		t.Fatalf("exact-model receipt = %+v", receipt)
	}
	if bytes.Contains(provider.requestBody, []byte("tools")) || !bytes.Contains(provider.requestBody, []byte(`"model":"`+guardedExactModelFakeModel+`"`)) ||
		!bytes.Contains(provider.requestBody, []byte(`"service_tier":"standard_only"`)) ||
		!bytes.Contains(provider.requestBody, []byte(`"inference_geo":"global"`)) {
		t.Fatalf("exact request body = %s", provider.requestBody)
	}
}

func TestReadGuardedExactModelCredentialWipesEveryLocalCopy(t *testing.T) {
	for name, source := range map[string]*guardedExactModelCredentialTestSourceV1{
		"success":     {raw: []byte("fake-secret")},
		"malformed":   {raw: []byte("fake\nsecret")},
		"read error":  {raw: []byte("fake-secret"), readErr: errors.New("synthetic read failure")},
		"close error": {raw: []byte("fake-secret"), closeErr: errors.New("synthetic close failure")},
	} {
		t.Run(name, func(t *testing.T) {
			originalLength := len(source.raw)
			credential, err := readGuardedExactModelCredentialV1(func() (io.ReadCloser, error) { return source, nil })
			if name == "success" {
				if err != nil || string(credential) != "fake-secret" {
					t.Fatalf("successful credential read = %q, error = %v", credential, err)
				}
				for index := range credential {
					credential[index] = 0
				}
			} else if err == nil {
				t.Fatal("credential error path unexpectedly succeeded")
			}
			if !source.closed {
				t.Fatal("credential source was not closed")
			}
			if len(source.raw) != originalLength {
				t.Fatal("credential test source length changed")
			}
			for index, value := range source.raw {
				if value != 0 {
					t.Fatalf("credential byte %d was not wiped", index)
				}
			}
		})
	}
}

type guardedExactModelCredentialTestSourceV1 struct {
	raw      []byte
	offset   int
	readErr  error
	closeErr error
	closed   bool
}

func (s *guardedExactModelCredentialTestSourceV1) Read(destination []byte) (int, error) {
	if s.offset < len(s.raw) {
		count := copy(destination, s.raw[s.offset:])
		s.offset += count
		return count, nil
	}
	if s.readErr != nil {
		return 0, s.readErr
	}
	return 0, io.EOF
}

func (s *guardedExactModelCredentialTestSourceV1) Close() error {
	s.closed = true
	for index := range s.raw {
		s.raw[index] = 0
	}
	return s.closeErr
}

func TestGuardedExactModelDirectHTTPTransportIsPinned(t *testing.T) {
	transport := newGuardedExactModelDirectHTTPTransportV1()
	defer transport.CloseIdleConnections()
	if transport.Proxy != nil || transport.DialContext == nil || transport.ForceAttemptHTTP2 ||
		!transport.DisableKeepAlives || !transport.DisableCompression || transport.MaxConnsPerHost != 1 ||
		transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("direct transport is not pinned to the no-proxy/single-connection policy: %+v", transport)
	}
}

func TestGuardedExactModelTransportRejectsAlteredPromptBeforeCredentialOrProvider(t *testing.T) {
	provider := &guardedExactModelFakeProviderV1{}
	executor := newGuardedExactModelFakeExecutorV1(t, provider)
	opens, closes := 0, 0
	if _, err := executor.execute(t.Context(), guardedExactModelFakePrompt+" altered", guardedExactModelFakeCredentialV1(&opens, &closes)); err == nil {
		t.Fatal("altered prompt was accepted")
	}
	if opens != 0 || closes != 0 || provider.requests != 0 {
		t.Fatalf("altered prompt reached custody/provider opens=%d closes=%d requests=%d", opens, closes, provider.requests)
	}
	if _, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes)); err == nil {
		t.Fatal("failed first attempt did not consume the executor")
	}
}

func TestGuardedExactModelTransportRejectsProviderModelDrift(t *testing.T) {
	provider := &guardedExactModelFakeProviderV1{response: guardedExactModelFakeResponseV1("fake-background-model", "wrong")}
	executor := newGuardedExactModelFakeExecutorV1(t, provider)
	opens, closes := 0, 0
	receipt, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes))
	if err == nil {
		t.Fatal("provider model drift was accepted")
	}
	if receipt.ObservedModel != "fake-background-model" || receipt.ProviderRequests != 1 || provider.requests != 1 || opens != 1 || closes != 1 {
		t.Fatalf("drift receipt/custody = %+v opens=%d closes=%d requests=%d", receipt, opens, closes, provider.requests)
	}
}

func TestGuardedExactModelTransportCannotDuplicateOrRetry(t *testing.T) {
	t.Run("duplicate executor call", func(t *testing.T) {
		provider := &guardedExactModelFakeProviderV1{}
		executor := newGuardedExactModelFakeExecutorV1(t, provider)
		opens, closes := 0, 0
		if _, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes)); err != nil {
			t.Fatal(err)
		}
		if _, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes)); err == nil {
			t.Fatal("duplicate exact-model execution was accepted")
		}
		if provider.requests != 1 || opens != 1 || closes != 1 {
			t.Fatalf("duplicate execution reached provider/custody requests=%d opens=%d closes=%d", provider.requests, opens, closes)
		}
	})
	t.Run("concurrent duplicate", func(t *testing.T) {
		provider := &guardedExactModelFakeProviderV1{}
		executor := newGuardedExactModelFakeExecutorV1(t, provider)
		opens, closes := 0, 0
		start := make(chan struct{})
		errors := make(chan error, 2)
		var callers sync.WaitGroup
		for range 2 {
			callers.Add(1)
			go func() {
				defer callers.Done()
				<-start
				_, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes))
				errors <- err
			}()
		}
		close(start)
		callers.Wait()
		close(errors)
		successes := 0
		for err := range errors {
			if err == nil {
				successes++
			}
		}
		if successes != 1 || provider.requests != 1 || opens != 1 || closes != 1 {
			t.Fatalf("concurrent execution successes=%d requests=%d opens=%d closes=%d", successes, provider.requests, opens, closes)
		}
	})
	t.Run("transport failure", func(t *testing.T) {
		provider := &guardedExactModelFakeProviderV1{err: errors.New("synthetic provider failure")}
		executor := newGuardedExactModelFakeExecutorV1(t, provider)
		opens, closes := 0, 0
		receipt, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes))
		if err == nil || receipt.ProviderRequests != 1 || provider.requests != 1 || opens != 1 || closes != 1 {
			t.Fatalf("transport failure retried or lost receipt: receipt=%+v err=%v requests=%d opens=%d closes=%d", receipt, err, provider.requests, opens, closes)
		}
	})
}

func TestGuardedExactModelTransportRejectsToolOrRedirectResponse(t *testing.T) {
	for name, provider := range map[string]*guardedExactModelFakeProviderV1{
		"tool block":              {response: `{"id":"msg_fake","type":"message","role":"assistant","model":"fake-exact-model-v1","content":[{"type":"tool_use","id":"tool-1","name":"Bash","input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":7,"output_tokens":3}}`},
		"server tool metadata":    {response: `{"id":"msg_fake","type":"message","role":"assistant","model":"fake-exact-model-v1","content":[{"type":"text","text":"unsafe"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":7,"output_tokens":3,"server_tool_use":{"web_search_requests":1},"service_tier":"standard","inference_geo":"global"}}`},
		"unknown response field":  {response: `{"id":"msg_fake","type":"message","role":"assistant","model":"fake-exact-model-v1","content":[{"type":"text","text":"unsafe"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":7,"output_tokens":3,"service_tier":"standard","inference_geo":"global"},"surprise":true}`},
		"container allocated":     {response: `{"id":"msg_fake","type":"message","role":"assistant","model":"fake-exact-model-v1","container":{"id":"unexpected"},"content":[{"type":"text","text":"unsafe"}],"stop_reason":"end_turn","stop_sequence":null,"stop_details":null,"usage":{"input_tokens":7,"output_tokens":3,"output_tokens_details":{"thinking_tokens":0},"service_tier":"standard","inference_geo":"global"}}`},
		"stop details present":    {response: `{"id":"msg_fake","type":"message","role":"assistant","model":"fake-exact-model-v1","container":null,"content":[{"type":"text","text":"unsafe"}],"stop_reason":"end_turn","stop_sequence":null,"stop_details":{"type":"refusal"},"usage":{"input_tokens":7,"output_tokens":3,"output_tokens_details":{"thinking_tokens":0},"service_tier":"standard","inference_geo":"global"}}`},
		"thinking exceeds output": {response: `{"id":"msg_fake","type":"message","role":"assistant","model":"fake-exact-model-v1","container":null,"content":[{"type":"text","text":"unsafe"}],"stop_reason":"end_turn","stop_sequence":null,"stop_details":null,"usage":{"input_tokens":7,"output_tokens":3,"output_tokens_details":{"thinking_tokens":4},"service_tier":"standard","inference_geo":"global"}}`},
		"redirect":                {status: http.StatusTemporaryRedirect, location: agentqualification.GuardedExactModelEndpointAnthropicMessagesV1},
	} {
		t.Run(name, func(t *testing.T) {
			executor := newGuardedExactModelFakeExecutorV1(t, provider)
			opens, closes := 0, 0
			_, err := executor.execute(t.Context(), guardedExactModelFakePrompt, guardedExactModelFakeCredentialV1(&opens, &closes))
			if err == nil {
				t.Fatal("unsafe provider response was accepted")
			}
			if name == "redirect" && !strings.Contains(err.Error(), "status 307") {
				t.Fatalf("redirect was not stopped before a second RoundTrip: %v", err)
			}
			if provider.requests != 1 || opens != 1 || closes != 1 {
				t.Fatalf("unsafe response custody/requests requests=%d opens=%d closes=%d", provider.requests, opens, closes)
			}
		})
	}
}
