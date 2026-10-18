// Package loopguard detects repeated, completed tool cycles without retaining
// tool payloads. The first rollout is shadow-only and never interrupts work.
package loopguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/multica-ai/multica/server/pkg/redact"
)

const (
	SchemaVersion         = 1
	RepeatedCycleLimit    = 3
	ShadowWouldCheckpoint = "would_checkpoint"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Observation is safe to persist: inputs and outputs are represented only by
// hashes. FingerprintSHA256 excludes task, call, sequence, and time identities.
type Observation struct {
	SchemaVersion     int    `json:"schema_version"`
	Provider          string `json:"provider"`
	Tool              string `json:"tool"`
	InputSHA256       string `json:"input_sha256"`
	OutputSHA256      string `json:"output_sha256"`
	ResultStatus      string `json:"result_status"`
	RepeatedCycles    int    `json:"repeated_cycles"`
	ShadowDecision    string `json:"shadow_decision"`
	FingerprintSHA256 string `json:"fingerprint_sha256"`
}

type cycleStart struct {
	tool        string
	inputSHA256 string
}

// Detector matches tool use/result events by transient call ID. Call IDs are
// never copied into observations or fingerprints.
type Detector struct {
	provider        string
	pending         map[string]cycleStart
	lastFingerprint string
	repeated        int
	reported        bool
}

func NewDetector(provider string) *Detector {
	return &Detector{provider: normalizeToken(provider), pending: make(map[string]cycleStart)}
}

// ObserveToolUse retains only the normalized tool and a canonical redacted
// input hash.
func (d *Detector) ObserveToolUse(callID, tool string, input map[string]any) {
	if d == nil || strings.TrimSpace(callID) == "" {
		return
	}
	inputSHA256, err := canonicalInputSHA256(input)
	if err != nil {
		return
	}
	d.pending[callID] = cycleStart{tool: normalizeToken(tool), inputSHA256: inputSHA256}
}

// ObserveToolResult completes a matched cycle. Text, thinking, token updates,
// and unmatched results never reach this method and cannot count as progress.
func (d *Detector) ObserveToolResult(callID, fallbackTool, output, status string) *Observation {
	if d == nil || strings.TrimSpace(callID) == "" {
		return nil
	}
	start, ok := d.pending[callID]
	if !ok {
		return nil
	}
	delete(d.pending, callID)
	if start.tool == "unknown" {
		start.tool = normalizeToken(fallbackTool)
	}

	observation := Observation{
		SchemaVersion:  SchemaVersion,
		Provider:       d.provider,
		Tool:           start.tool,
		InputSHA256:    start.inputSHA256,
		OutputSHA256:   textSHA256(redact.Text(output)),
		ResultStatus:   normalizeStatus(status),
		RepeatedCycles: 1,
		ShadowDecision: ShadowWouldCheckpoint,
	}
	fingerprint, err := observation.fingerprint()
	if err != nil {
		return nil
	}
	if fingerprint == d.lastFingerprint {
		d.repeated++
	} else {
		d.lastFingerprint = fingerprint
		d.repeated = 1
		d.reported = false
	}
	if d.repeated != RepeatedCycleLimit || d.reported {
		return nil
	}
	d.reported = true
	observation.RepeatedCycles = d.repeated
	observation.FingerprintSHA256 = fingerprint
	return &observation
}

func (o Observation) Validate() error {
	if o.SchemaVersion != SchemaVersion {
		return fmt.Errorf("loop observation: unsupported schema version %d", o.SchemaVersion)
	}
	if o.Provider != normalizeToken(o.Provider) || o.Tool != normalizeToken(o.Tool) {
		return fmt.Errorf("loop observation: provider and tool must be normalized")
	}
	if len(o.Provider) > 64 || len(o.Tool) > 256 || len(o.ResultStatus) > 64 {
		return fmt.Errorf("loop observation: normalized identity is too long")
	}
	if !digestPattern.MatchString(o.InputSHA256) || !digestPattern.MatchString(o.OutputSHA256) || !digestPattern.MatchString(o.FingerprintSHA256) {
		return fmt.Errorf("loop observation: malformed digest")
	}
	if o.ResultStatus != normalizeStatus(o.ResultStatus) {
		return fmt.Errorf("loop observation: result status must be normalized")
	}
	if o.RepeatedCycles != RepeatedCycleLimit || o.ShadowDecision != ShadowWouldCheckpoint {
		return fmt.Errorf("loop observation: invalid shadow threshold or decision")
	}
	want, err := o.fingerprint()
	if err != nil {
		return err
	}
	if o.FingerprintSHA256 != want {
		return fmt.Errorf("loop observation: fingerprint mismatch")
	}
	return nil
}

func (o Observation) CanonicalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(o); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func (o Observation) Digest() (string, error) {
	raw, err := o.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return textSHA256(string(raw)), nil
}

func Parse(raw []byte) (Observation, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var observation Observation
	if err := decoder.Decode(&observation); err != nil {
		return Observation{}, fmt.Errorf("loop observation: decode: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Observation{}, fmt.Errorf("loop observation: trailing JSON")
	}
	if err := observation.Validate(); err != nil {
		return Observation{}, err
	}
	return observation, nil
}

func (o Observation) fingerprint() (string, error) {
	identity := struct {
		Provider     string `json:"provider"`
		Tool         string `json:"tool"`
		InputSHA256  string `json:"input_sha256"`
		OutputSHA256 string `json:"output_sha256"`
		ResultStatus string `json:"result_status"`
	}{o.Provider, o.Tool, o.InputSHA256, o.OutputSHA256, o.ResultStatus}
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	return textSHA256(string(raw)), nil
}

func canonicalInputSHA256(input map[string]any) (string, error) {
	raw, err := json.Marshal(redact.InputMap(input))
	if err != nil {
		return "", err
	}
	return textSHA256(string(raw)), nil
}

func textSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizeStatus(value string) string {
	value = normalizeToken(value)
	if value == "unknown" {
		return "completed"
	}
	return value
}
