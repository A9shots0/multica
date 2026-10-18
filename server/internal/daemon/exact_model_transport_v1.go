package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const (
	guardedExactModelCredentialMaxBytesV1 = 16 * 1024
	guardedExactModelResponseMaxBytesV1   = 2 * 1024 * 1024
)

type guardedExactModelCredentialOpenerV1 func() (io.ReadCloser, error)

// guardedExactModelTransportV1 is deliberately single-use. It is a prepared
// alternative to Claude Code, not an active runtime route: callers must supply
// both a credential opener and an HTTP transport, so construction alone can
// neither inspect a credential nor contact a provider. The injected transport
// is a fake-provider seam only. Future real wiring must construct and bind a
// fresh known standard transport rather than accept an arbitrary RoundTripper.
type guardedExactModelTransportV1 struct {
	policy    agentqualification.GuardedExactModelTransportV1
	transport http.RoundTripper
	used      atomic.Bool
}

type guardedExactModelReceiptV1 struct {
	RequestedModel   string
	ObservedModel    string
	ProviderRequests int
	InputTokens      int64
	OutputTokens     int64
	ThinkingTokens   int64
	CostUSDMicros    int64
	ServiceTier      string
	InferenceGeo     string
	Output           string
	RequestSHA256    string
	ResponseSHA256   string
}

func newGuardedExactModelTransportV1(policy agentqualification.GuardedExactModelTransportV1, expectedModel, boundPrompt string, transport http.RoundTripper) (*guardedExactModelTransportV1, error) {
	if transport == nil {
		return nil, fmt.Errorf("exact-model HTTP transport is required")
	}
	if err := agentqualification.ValidateGuardedExactModelTransportV1(policy, expectedModel, boundPrompt); err != nil {
		return nil, err
	}
	return &guardedExactModelTransportV1{policy: policy, transport: transport}, nil
}

func (t *guardedExactModelTransportV1) execute(ctx context.Context, prompt string, openCredential guardedExactModelCredentialOpenerV1) (guardedExactModelReceiptV1, error) {
	if t == nil || !t.used.CompareAndSwap(false, true) {
		return guardedExactModelReceiptV1{}, fmt.Errorf("exact-model transport is already consumed")
	}
	if err := agentqualification.ValidateGuardedExactModelTransportV1(t.policy, t.policy.Model, prompt); err != nil {
		return guardedExactModelReceiptV1{}, err
	}
	if openCredential == nil {
		return guardedExactModelReceiptV1{}, fmt.Errorf("exact-model credential opener is required")
	}
	credential, err := readGuardedExactModelCredentialV1(openCredential)
	if err != nil {
		return guardedExactModelReceiptV1{}, err
	}
	defer func() {
		for i := range credential {
			credential[i] = 0
		}
	}()

	requestBody, err := json.Marshal(struct {
		Model        string `json:"model"`
		MaxTokens    int    `json:"max_tokens"`
		ServiceTier  string `json:"service_tier"`
		InferenceGeo string `json:"inference_geo"`
		Messages     []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}{
		Model: t.policy.Model, MaxTokens: t.policy.MaxOutputTokens,
		ServiceTier: t.policy.ServiceTier, InferenceGeo: t.policy.InferenceGeo,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return guardedExactModelReceiptV1{}, fmt.Errorf("encode exact-model request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.policy.Endpoint, nil)
	if err != nil {
		return guardedExactModelReceiptV1{}, fmt.Errorf("construct exact-model request: %w", err)
	}
	request.Body = io.NopCloser(bytes.NewReader(requestBody))
	request.ContentLength = int64(len(requestBody))
	request.GetBody = nil // A failed POST cannot be replayed by net/http.
	request.Header.Set("content-type", "application/json")
	request.Header.Set("accept", "application/json")
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("x-api-key", string(credential))

	gate := &guardedExactModelRoundTripperV1{policy: t.policy, base: t.transport}
	client := &http.Client{
		Transport: gate,
		Timeout:   time.Duration(t.policy.RequestTimeoutMS) * time.Millisecond,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	request.Header.Del("x-api-key")
	receipt := guardedExactModelReceiptV1{
		RequestedModel:   t.policy.Model,
		ProviderRequests: int(gate.requests.Load()),
		RequestSHA256:    sha256HexV1(requestBody),
	}
	if err != nil {
		return receipt, fmt.Errorf("exact-model provider request failed without retry: %w", err)
	}
	if response == nil || response.Body == nil {
		return receipt, fmt.Errorf("exact-model provider returned no response")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return receipt, fmt.Errorf("exact-model provider returned status %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("content-type"))
	if err != nil || mediaType != "application/json" {
		return receipt, fmt.Errorf("exact-model provider returned non-JSON content")
	}
	responseBody, err := readLimitedV1(response.Body, guardedExactModelResponseMaxBytesV1)
	if err != nil {
		return receipt, fmt.Errorf("read exact-model provider response: %w", err)
	}
	receipt.ResponseSHA256 = sha256HexV1(responseBody)

	var decoded struct {
		ID           string          `json:"id"`
		Type         string          `json:"type"`
		Role         string          `json:"role"`
		Model        string          `json:"model"`
		StopReason   string          `json:"stop_reason"`
		StopSequence *string         `json:"stop_sequence"`
		Container    json.RawMessage `json:"container"`
		StopDetails  json.RawMessage `json:"stop_details"`
		Content      []struct {
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			Citations json.RawMessage `json:"citations"`
		} `json:"content"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreation            *struct {
				Ephemeral1HInputTokens int64 `json:"ephemeral_1h_input_tokens"`
				Ephemeral5MInputTokens int64 `json:"ephemeral_5m_input_tokens"`
			} `json:"cache_creation"`
			OutputTokenDetails struct {
				ThinkingTokens int64 `json:"thinking_tokens"`
			} `json:"output_tokens_details"`
			ServerToolUse map[string]int64 `json:"server_tool_use"`
			ServiceTier   string           `json:"service_tier"`
			InferenceGeo  string           `json:"inference_geo"`
			Speed         string           `json:"speed"`
		} `json:"usage"`
	}
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return receipt, fmt.Errorf("decode exact-model provider response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return receipt, fmt.Errorf("decode exact-model provider response: trailing JSON")
	}
	receipt.ObservedModel = decoded.Model
	receipt.InputTokens = decoded.Usage.InputTokens
	receipt.OutputTokens = decoded.Usage.OutputTokens
	receipt.ThinkingTokens = decoded.Usage.OutputTokenDetails.ThinkingTokens
	receipt.ServiceTier = decoded.Usage.ServiceTier
	receipt.InferenceGeo = decoded.Usage.InferenceGeo
	receipt.CostUSDMicros = decoded.Usage.InputTokens*t.policy.InputPriceMicros + decoded.Usage.OutputTokens*t.policy.OutputPriceMicros
	if strings.TrimSpace(decoded.ID) == "" || decoded.Type != "message" || decoded.Role != "assistant" || decoded.StopReason != "end_turn" ||
		decoded.Model != t.policy.Model || decoded.Usage.InputTokens < 1 || decoded.Usage.OutputTokens < 1 ||
		decoded.Usage.InputTokens > int64(t.policy.MaxInputTokens) || decoded.Usage.OutputTokens > int64(t.policy.MaxOutputTokens) ||
		!guardedExactModelAbsentOrNullV1(decoded.Container) || !guardedExactModelAbsentOrNullV1(decoded.StopDetails) ||
		decoded.Usage.OutputTokenDetails.ThinkingTokens < 0 || decoded.Usage.OutputTokenDetails.ThinkingTokens > decoded.Usage.OutputTokens ||
		decoded.Usage.CacheCreationInputTokens != 0 || decoded.Usage.CacheReadInputTokens != 0 ||
		(decoded.Usage.CacheCreation != nil && (decoded.Usage.CacheCreation.Ephemeral1HInputTokens != 0 || decoded.Usage.CacheCreation.Ephemeral5MInputTokens != 0)) ||
		decoded.Usage.ServiceTier != "standard" || decoded.Usage.InferenceGeo != t.policy.InferenceGeo ||
		(decoded.Usage.Speed != "" && decoded.Usage.Speed != "standard") ||
		receipt.CostUSDMicros < 1 || receipt.CostUSDMicros > t.policy.MaxCostUSDMicros || len(decoded.Content) == 0 {
		return receipt, fmt.Errorf("exact-model provider response failed the bound receipt contract")
	}
	for name, count := range decoded.Usage.ServerToolUse {
		if strings.TrimSpace(name) == "" || count != 0 {
			return receipt, fmt.Errorf("exact-model provider reported server tool use")
		}
	}
	var output strings.Builder
	for _, block := range decoded.Content {
		if block.Type != "text" || block.Text == "" || !guardedExactModelAbsentNullOrEmptyArrayV1(block.Citations) {
			return receipt, fmt.Errorf("exact-model provider returned a non-text or empty content block")
		}
		output.WriteString(block.Text)
	}
	receipt.Output = output.String()
	if receipt.ProviderRequests != 1 {
		return receipt, fmt.Errorf("exact-model provider request count was %d", receipt.ProviderRequests)
	}
	return receipt, nil
}

type guardedExactModelRoundTripperV1 struct {
	policy   agentqualification.GuardedExactModelTransportV1
	base     http.RoundTripper
	requests atomic.Int32
}

func (g *guardedExactModelRoundTripperV1) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || !g.requests.CompareAndSwap(0, 1) {
		return nil, fmt.Errorf("exact-model transport denied a duplicate provider request")
	}
	if request.Method != http.MethodPost || request.URL.String() != g.policy.Endpoint ||
		request.GetBody != nil || request.Header.Get("content-type") != "application/json" ||
		request.Header.Get("accept") != "application/json" ||
		request.Header.Get("anthropic-version") != "2023-06-01" ||
		strings.TrimSpace(request.Header.Get("x-api-key")) == "" {
		return nil, fmt.Errorf("exact-model provider request envelope drifted")
	}
	raw, err := readLimitedV1(request.Body, guardedExactModelResponseMaxBytesV1)
	if err != nil {
		return nil, fmt.Errorf("read exact-model request: %w", err)
	}
	request.Body = io.NopCloser(bytes.NewReader(raw))
	request.ContentLength = int64(len(raw))
	request.GetBody = nil
	var decoded struct {
		Model        string `json:"model"`
		MaxTokens    int    `json:"max_tokens"`
		ServiceTier  string `json:"service_tier"`
		InferenceGeo string `json:"inference_geo"`
		Messages     []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		decoded.Model != g.policy.Model || decoded.MaxTokens != g.policy.MaxOutputTokens ||
		decoded.ServiceTier != g.policy.ServiceTier || decoded.InferenceGeo != g.policy.InferenceGeo ||
		len(decoded.Messages) != 1 || decoded.Messages[0].Role != "user" ||
		agentqualification.GuardedPromptSHA256V1(decoded.Messages[0].Content) != g.policy.PromptSHA256 {
		return nil, fmt.Errorf("exact-model provider request body drifted")
	}
	return g.base.RoundTrip(request)
}

// newGuardedExactModelDirectHTTPTransportV1 returns the only real-network
// transport eligible for this proof. It never consults ambient proxy settings,
// disables connection reuse and compression, and uses no retry wrapper. The
// guarded POST has no GetBody or idempotency key, so net/http cannot replay it.
func newGuardedExactModelDirectHTTPTransportV1() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: -1}
	return &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      false,
		DisableKeepAlives:      true,
		DisableCompression:     true,
		MaxConnsPerHost:        1,
		ResponseHeaderTimeout:  45 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		MaxResponseHeaderBytes: 64 * 1024,
		ExpectContinueTimeout:  0,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

func readGuardedExactModelCredentialV1(openCredential guardedExactModelCredentialOpenerV1) ([]byte, error) {
	source, err := openCredential()
	if err != nil {
		return nil, fmt.Errorf("open exact-model credential: %w", err)
	}
	if source == nil {
		return nil, fmt.Errorf("exact-model credential source is unavailable")
	}
	raw, readErr := readLimitedV1(source, guardedExactModelCredentialMaxBytesV1)
	defer func() {
		for index := range raw {
			raw[index] = 0
		}
	}()
	closeErr := source.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read exact-model credential: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close exact-model credential: %w", closeErr)
	}
	value := bytes.TrimSpace(raw)
	if len(value) == 0 || bytes.ContainsAny(value, "\x00\r\n") {
		return nil, fmt.Errorf("exact-model credential is malformed")
	}
	credential := append([]byte(nil), value...)
	return credential, nil
}

func guardedExactModelAbsentOrNullV1(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func guardedExactModelAbsentNullOrEmptyArrayV1(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]"))
}

func readLimitedV1(reader io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("payload exceeds %d-byte limit", limit)
	}
	return raw, nil
}

func sha256HexV1(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
