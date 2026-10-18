//go:build agentintegration

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

const (
	guardedExactModelRealRunEnv        = "MULTICA_RUN_REAL_EXACT_MODEL_SHADOW"
	guardedExactModelRealCredentialEnv = "ANTHROPIC_API_KEY"
	guardedExactModelRealModel         = "claude-sonnet-4-6"
	guardedExactModelRealPrompt        = "This is an execution-boundary probe. No tools, repository, production system, or outbound action is available or authorized. Return exactly ITEM14_EXACT_MODEL_BOUNDARY_OK and nothing else."
	guardedExactModelRealOutput        = "ITEM14_EXACT_MODEL_BOUNDARY_OK"
)

// TestGuardedExactModelDirectMessagesRealOnce is the sole real call site for
// the 2026-09-14 owner grant. Do not add subtests, retries, fallback transports,
// alternate credential sources, or a second execute call.
func TestGuardedExactModelDirectMessagesRealOnce(t *testing.T) {
	policy, err := agentqualification.NewGuardedExactModelTransportV1(guardedExactModelRealModel, guardedExactModelRealPrompt)
	if err != nil {
		t.Fatal(err)
	}
	reservedCostMicros := int64(policy.MaxInputTokens)*policy.InputPriceMicros + int64(policy.MaxOutputTokens)*policy.OutputPriceMicros
	if reservedCostMicros < 1 || reservedCostMicros > policy.MaxCostUSDMicros {
		t.Fatalf("real exact-model cost reservation %d exceeds authority", reservedCostMicros)
	}
	transport := newGuardedExactModelDirectHTTPTransportV1()
	defer transport.CloseIdleConnections()
	executor, err := newGuardedExactModelTransportV1(policy, guardedExactModelRealModel, guardedExactModelRealPrompt, transport)
	if err != nil {
		t.Fatal(err)
	}

	if os.Getenv(guardedExactModelRealRunEnv) != "1" {
		t.Logf("ITEM14_DIRECT_PREPARE_ONLY model=%s requests=0 retries=0 redirects=0 proxies=0 tools=0 max_cost_usd_micros=%d credential_opened=false provider_started=false", policy.Model, policy.MaxCostUSDMicros)
		return
	}

	var opens atomic.Int32
	var sourceClosed atomic.Bool
	openCredential := func() (io.ReadCloser, error) {
		if !opens.CompareAndSwap(0, 1) {
			return nil, fmt.Errorf("approved credential mechanism is already consumed")
		}
		value, ok := os.LookupEnv(guardedExactModelRealCredentialEnv)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("approved credential mechanism is unavailable")
		}
		if err := os.Unsetenv(guardedExactModelRealCredentialEnv); err != nil {
			return nil, fmt.Errorf("clear approved credential mechanism: %w", err)
		}
		raw := []byte(value)
		value = ""
		return &guardedExactModelZeroingReadCloser{
			Reader: bytes.NewReader(raw),
			raw:    raw,
			closed: &sourceClosed,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(policy.RequestTimeoutMS)*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	receipt, executeErr := executor.execute(ctx, guardedExactModelRealPrompt, openCredential)
	duration := time.Since(startedAt)
	_, credentialStillPresent := os.LookupEnv(guardedExactModelRealCredentialEnv)
	cleanupVerified := opens.Load() == 1 && sourceClosed.Load() && !credentialStillPresent
	if executeErr != nil {
		t.Fatalf("ITEM14_DIRECT_FAIL model=%s requests=%d retries=0 input_tokens=%d output_tokens=%d cost_usd_micros=%d duration_ms=%d cleanup_verified=%t error=%v",
			guardedExactModelRealModel, receipt.ProviderRequests, receipt.InputTokens, receipt.OutputTokens,
			receipt.CostUSDMicros, duration.Milliseconds(), cleanupVerified, executeErr)
	}
	if !cleanupVerified {
		t.Fatal("exact-model credential custody cleanup was not verified")
	}
	if strings.TrimSpace(receipt.Output) != guardedExactModelRealOutput {
		t.Fatalf("exact-model harmless result violated the frozen output contract: %q", receipt.Output)
	}
	if receipt.ProviderRequests != 1 || receipt.ObservedModel != guardedExactModelRealModel ||
		receipt.InputTokens < 1 || receipt.OutputTokens < 1 || receipt.CostUSDMicros < 1 ||
		receipt.CostUSDMicros > policy.MaxCostUSDMicros {
		t.Fatalf("exact-model real receipt failed its authority contract: %+v", receipt)
	}
	t.Logf("ITEM14_DIRECT_PASS model=%s requests=%d retries=0 input_tokens=%d output_tokens=%d thinking_tokens=%d cost_usd_micros=%d duration_ms=%d service_tier=%s inference_geo=%s result=%q cleanup_verified=true credential_persisted=false request_sha256=%s response_sha256=%s",
		receipt.ObservedModel, receipt.ProviderRequests, receipt.InputTokens, receipt.OutputTokens,
		receipt.ThinkingTokens, receipt.CostUSDMicros, duration.Milliseconds(), receipt.ServiceTier,
		receipt.InferenceGeo, receipt.Output, receipt.RequestSHA256, receipt.ResponseSHA256)
}

type guardedExactModelZeroingReadCloser struct {
	*bytes.Reader
	raw    []byte
	closed *atomic.Bool
}

func (r *guardedExactModelZeroingReadCloser) Close() error {
	for index := range r.raw {
		r.raw[index] = 0
	}
	r.raw = nil
	r.closed.Store(true)
	return nil
}
