// Package usagesample defines the durable, provider-neutral evidence captured
// while one provider execution is still running.
package usagesample

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strings"
	"unicode"

	"github.com/multica-ai/multica/server/pkg/agent"
)

const SchemaVersion = 1

var attemptIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+\-]{0,255}$`)

// Sample is append-only evidence. The daemon creates it from agent telemetry;
// the server recomputes its digest and derives any shadow policy decision.
type Sample struct {
	SchemaVersion                int                          `json:"schema_version"`
	TaskAttemptID                string                       `json:"task_attempt"`
	ProviderAttempt              int32                        `json:"provider_attempt"`
	BackendAttempt               int32                        `json:"backend_attempt"`
	Sequence                     int64                        `json:"sequence"`
	Provider                     string                       `json:"provider"`
	Model                        string                       `json:"model"`
	InputTokens                  int64                        `json:"input_tokens"`
	OutputTokens                 int64                        `json:"output_tokens"`
	CacheReadTokens              int64                        `json:"cache_read_tokens"`
	CacheWriteTokens             int64                        `json:"cache_write_tokens"`
	ProviderCalls                int64                        `json:"provider_calls"`
	SourceEvent                  string                       `json:"source_event"`
	ModelContextWindow           int64                        `json:"model_context_window"`
	ModelContextWindowKind       agent.ContextMeasurementKind `json:"model_context_window_kind"`
	ContextTokens                int64                        `json:"context_tokens"`
	ContextTokensMeasurementKind agent.ContextMeasurementKind `json:"context_tokens_measurement_kind"`
}

func New(taskAttemptID string, providerAttempt int, live agent.UsageSample) (Sample, error) {
	sample := Sample{
		SchemaVersion: SchemaVersion, TaskAttemptID: taskAttemptID, ProviderAttempt: int32(providerAttempt),
		Sequence: live.Sequence, BackendAttempt: live.BackendAttempt, Provider: live.Provider, Model: live.Model,
		InputTokens: live.Usage.InputTokens, OutputTokens: live.Usage.OutputTokens,
		CacheReadTokens: live.Usage.CacheReadTokens, CacheWriteTokens: live.Usage.CacheWriteTokens,
		ProviderCalls: live.ProviderCalls, SourceEvent: live.SourceEvent,
		ModelContextWindow: live.ModelContextWindow, ModelContextWindowKind: live.ModelContextWindowKind,
		ContextTokens: live.ContextTokens, ContextTokensMeasurementKind: live.ContextTokensMeasurementKind,
	}
	return sample, sample.Validate()
}

func (s Sample) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("usage sample: unsupported schema version %d", s.SchemaVersion)
	}
	if !attemptIdentityPattern.MatchString(s.TaskAttemptID) {
		return errors.New("usage sample: malformed task attempt")
	}
	if s.ProviderAttempt < 1 || s.ProviderAttempt > 16 {
		return errors.New("usage sample: provider attempt must be between 1 and 16")
	}
	if s.BackendAttempt < 1 || s.BackendAttempt > 16 {
		return errors.New("usage sample: backend attempt must be between 1 and 16")
	}
	if s.Sequence < 1 || s.ProviderCalls < 1 {
		return errors.New("usage sample: sequence and provider_calls must be positive")
	}
	if err := validateText("provider", s.Provider, 32); err != nil {
		return err
	}
	if err := validateText("model", s.Model, 256); err != nil {
		return err
	}
	if err := validateText("source_event", s.SourceEvent, 128); err != nil {
		return err
	}
	for name, value := range map[string]int64{
		"input_tokens": s.InputTokens, "output_tokens": s.OutputTokens,
		"cache_read_tokens": s.CacheReadTokens, "cache_write_tokens": s.CacheWriteTokens,
		"model_context_window": s.ModelContextWindow, "context_tokens": s.ContextTokens,
	} {
		if value < 0 {
			return fmt.Errorf("usage sample: %s must be non-negative", name)
		}
	}
	if err := validateMeasurement("model_context_window", s.ModelContextWindow, s.ModelContextWindowKind, false); err != nil {
		return err
	}
	return validateMeasurement("context_tokens", s.ContextTokens, s.ContextTokensMeasurementKind, true)
}

func (s Sample) CanonicalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(s); err != nil {
		return nil, fmt.Errorf("usage sample: encode: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func (s Sample) Digest() (string, error) {
	raw, err := s.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func Parse(raw []byte) (Sample, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var sample Sample
	if err := decoder.Decode(&sample); err != nil {
		return Sample{}, fmt.Errorf("usage sample: decode: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Sample{}, errors.New("usage sample: trailing JSON")
		}
		return Sample{}, fmt.Errorf("usage sample: trailing JSON: %w", err)
	}
	if err := sample.Validate(); err != nil {
		return Sample{}, err
	}
	return sample, nil
}

func validateMeasurement(name string, value int64, kind agent.ContextMeasurementKind, proxyAllowed bool) error {
	switch kind {
	case agent.ContextMeasurementUnknown:
		if value != 0 {
			return fmt.Errorf("usage sample: unknown %s must be zero", name)
		}
	case agent.ContextMeasurementExact:
		if value <= 0 {
			return fmt.Errorf("usage sample: exact %s must be positive", name)
		}
	case agent.ContextMeasurementProxy:
		if !proxyAllowed || value <= 0 {
			return fmt.Errorf("usage sample: invalid proxy %s", name)
		}
	default:
		return fmt.Errorf("usage sample: unknown %s measurement kind %q", name, kind)
	}
	return nil
}

func validateText(name, value string, limit int) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > limit {
		return fmt.Errorf("usage sample: malformed %s", name)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("usage sample: malformed %s", name)
		}
	}
	return nil
}

// ShadowDecision deliberately requires exact numerator and denominator. A
// provider proxy is useful evidence, but not permission to interrupt work.
func ShadowDecision(s Sample) (decision string, ratioPPM *int64) {
	if s.ContextTokensMeasurementKind != agent.ContextMeasurementExact ||
		s.ModelContextWindowKind != agent.ContextMeasurementExact || s.ModelContextWindow <= 0 {
		return "unknown", nil
	}
	var numerator big.Int
	numerator.Mul(big.NewInt(s.ContextTokens), big.NewInt(1_000_000))
	numerator.Quo(&numerator, big.NewInt(s.ModelContextWindow))
	ratio := int64(math.MaxInt64)
	if numerator.IsInt64() {
		ratio = numerator.Int64()
	}
	switch {
	case ratio >= 800_000:
		decision = "would_stop"
	case ratio >= 700_000:
		decision = "would_request_checkpoint"
	case ratio >= 550_000:
		decision = "would_prepare"
	default:
		decision = "continue"
	}
	return decision, &ratio
}
