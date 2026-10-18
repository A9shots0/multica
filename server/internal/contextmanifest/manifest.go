// Package contextmanifest defines the provider-neutral record of the context
// supplied to one provider attempt. Manifests are deliberately map-free so
// their JSON representation is stable enough to hash and persist.
package contextmanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	SchemaVersion = 1
	maxBlocks     = 1024
	maxReasonLen  = 1024
)

// MeasurementState says how ContentBytes was obtained. Unknown is allowed only
// with a zero count so shadow mode can expose a provider-owned context class
// without fabricating its size.
type MeasurementState string

const (
	MeasurementExact     MeasurementState = "exact"
	MeasurementEstimated MeasurementState = "estimated"
	MeasurementUnknown   MeasurementState = "unknown"
)

var identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+\-]{0,255}$`)

// Block describes one ordered unit of context supplied to the provider.
// SourceIdentity is unique inside a manifest, even when two candidate blocks
// claim different revisions of that source.
type Block struct {
	Kind             string           `json:"kind"`
	SourceIdentity   string           `json:"source_identity"`
	SourceRevision   string           `json:"source_revision"`
	ContentBytes     int64            `json:"content_bytes"`
	Reason           string           `json:"reason"`
	Cacheable        bool             `json:"cacheable"`
	MeasurementState MeasurementState `json:"measurement_state"`
}

// Manifest binds an ordered context to both Multica's task attempt and the
// provider's attempt. LimitBytes is part of the evidence so a persisted record
// can be validated without recovering policy from another table.
type Manifest struct {
	SchemaVersion     int     `json:"schema_version"`
	TaskAttemptID     string  `json:"task_attempt"`
	ProviderAttemptID string  `json:"provider_attempt"`
	LimitBytes        int64   `json:"limit_bytes"`
	TotalBytes        int64   `json:"total_bytes"`
	Blocks            []Block `json:"blocks"`
}

// New validates and copies blocks so later changes to the caller's slice do
// not change the returned manifest.
func New(taskAttemptID, providerAttemptID string, limitBytes int64, blocks []Block) (Manifest, error) {
	manifest := Manifest{
		SchemaVersion:     SchemaVersion,
		TaskAttemptID:     taskAttemptID,
		ProviderAttemptID: providerAttemptID,
		LimitBytes:        limitBytes,
		Blocks:            append([]Block(nil), blocks...),
	}

	total, err := validateBlocks(manifest.Blocks, limitBytes)
	if err != nil {
		return Manifest{}, err
	}
	manifest.TotalBytes = total
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate checks a manifest loaded from a persistence boundary and rejects
// both malformed fields and internally inconsistent totals.
func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("context manifest: unsupported schema version %d", m.SchemaVersion)
	}
	if err := validateIdentity("task attempt", m.TaskAttemptID); err != nil {
		return err
	}
	if err := validateIdentity("provider attempt", m.ProviderAttemptID); err != nil {
		return err
	}

	total, err := validateBlocks(m.Blocks, m.LimitBytes)
	if err != nil {
		return err
	}
	if m.TotalBytes != total {
		return fmt.Errorf("context manifest: total_bytes is %d, want %d", m.TotalBytes, total)
	}
	return nil
}

// CanonicalJSON returns deterministic JSON. It uses a fixed struct field order,
// preserves block order, contains no maps, and disables HTML escaping so the
// byte representation does not depend on transport-oriented escaping.
func (m Manifest) CanonicalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(m); err != nil {
		return nil, fmt.Errorf("context manifest: encode canonical JSON: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// Digest returns the SHA-256 digest of CanonicalJSON in the repository's
// conventional sha256:<lowercase hex> form.
func (m Manifest) Digest() (string, error) {
	canonical, err := m.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

// Parse strictly decodes a persisted manifest. Unknown fields and trailing
// JSON are rejected so a producer cannot accidentally hash information that
// this version of the consumer ignores.
func Parse(data []byte) (Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("context manifest: decode: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateBlocks(blocks []Block, limitBytes int64) (int64, error) {
	if limitBytes <= 0 {
		return 0, errors.New("context manifest: limit_bytes must be positive")
	}
	if len(blocks) > maxBlocks {
		return 0, fmt.Errorf("context manifest: blocks exceed maximum of %d", maxBlocks)
	}

	seen := make(map[string]struct{}, len(blocks))
	var total int64
	for i, block := range blocks {
		if !validToken(block.Kind) {
			return 0, fmt.Errorf("context manifest: block %d has malformed kind", i)
		}
		if err := validateIdentity("source identity", block.SourceIdentity); err != nil {
			return 0, fmt.Errorf("context manifest: block %d: %w", i, err)
		}
		if _, exists := seen[block.SourceIdentity]; exists {
			return 0, fmt.Errorf("context manifest: block %d duplicates live source identity %q", i, block.SourceIdentity)
		}
		seen[block.SourceIdentity] = struct{}{}

		if err := validateIdentity("source revision", block.SourceRevision); err != nil {
			return 0, fmt.Errorf("context manifest: block %d: %w", i, err)
		}
		if block.ContentBytes < 0 {
			return 0, fmt.Errorf("context manifest: block %d has negative content_bytes", i)
		}
		if block.ContentBytes > math.MaxInt64-total {
			return 0, fmt.Errorf("context manifest: block %d makes total_bytes overflow", i)
		}
		if err := validateReason(block.Reason); err != nil {
			return 0, fmt.Errorf("context manifest: block %d: %w", i, err)
		}
		switch block.MeasurementState {
		case MeasurementExact, MeasurementEstimated:
		case MeasurementUnknown:
			if block.ContentBytes != 0 {
				return 0, fmt.Errorf("context manifest: block %d has unknown measurement with non-zero content_bytes", i)
			}
		default:
			return 0, fmt.Errorf("context manifest: block %d has unknown measurement_state %q", i, block.MeasurementState)
		}

		total += block.ContentBytes
		if total > limitBytes {
			return 0, fmt.Errorf("context manifest: total_bytes %d exceed limit_bytes %d", total, limitBytes)
		}
	}
	return total, nil
}

func validateIdentity(name, value string) error {
	if !identityPattern.MatchString(value) {
		return fmt.Errorf("context manifest: %s is malformed", name)
	}
	return nil
}

func validToken(value string) bool {
	if value == "" || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validateReason(reason string) error {
	if !utf8.ValidString(reason) || strings.TrimSpace(reason) == "" {
		return errors.New("reason is empty or invalid UTF-8")
	}
	if utf8.RuneCountInString(reason) > maxReasonLen {
		return fmt.Errorf("reason exceeds %d characters", maxReasonLen)
	}
	for _, r := range reason {
		if unicode.IsControl(r) {
			return errors.New("reason contains control characters")
		}
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("context manifest: trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("context manifest: decode trailing data: %w", err)
	}
	return nil
}
