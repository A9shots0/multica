// Package continuitycheckpoint defines the bounded, provider-neutral wire
// contract used when one work package yields to a fresh successor.
package continuitycheckpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	googleuuid "github.com/google/uuid"
)

const (
	SchemaVersion      = 1
	MaxCanonicalBytes  = 2048
	MaxPackages        = 3
	MaxListEntries     = 8
	MaxSummaryRunes    = 480
	MaxItemRunes       = 240
	MaxNextActionRunes = 480
)

type Plan struct {
	SchemaVersion      int    `json:"schema_version"`
	RootTaskID         string `json:"root_task_id"`
	PackageStep        int32  `json:"package_step"`
	MaxPackages        int32  `json:"max_packages"`
	MaxCheckpointBytes int    `json:"max_checkpoint_bytes"`
}

type Receipt struct {
	SchemaVersion int      `json:"schema_version"`
	PackageStep   int32    `json:"package_step"`
	Summary       string   `json:"summary"`
	Completed     []string `json:"completed"`
	NextAction    string   `json:"next_action"`
	Verification  []string `json:"verification"`
	OpenRisks     []string `json:"open_risks"`
}

func (receipt *Receipt) UnmarshalJSON(raw []byte) error {
	type wire Receipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var decoded wire
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := expectEOF(decoder); err != nil {
		return err
	}
	*receipt = Receipt(decoded)
	return nil
}

type Validated struct {
	Receipt   Receipt
	Canonical []byte
	SHA256    string
}

func ValidatePlan(plan Plan) error {
	if plan.SchemaVersion != SchemaVersion {
		return fmt.Errorf("continuity checkpoint plan schema_version must be %d", SchemaVersion)
	}
	if _, err := googleuuid.Parse(plan.RootTaskID); err != nil {
		return errors.New("continuity checkpoint plan root_task_id is invalid")
	}
	if plan.MaxPackages < 2 || plan.MaxPackages > MaxPackages || plan.PackageStep < 1 || plan.PackageStep >= plan.MaxPackages {
		return errors.New("continuity checkpoint plan package bounds are invalid")
	}
	if plan.MaxCheckpointBytes != MaxCanonicalBytes {
		return errors.New("continuity checkpoint plan byte limit is invalid")
	}
	return nil
}

func DecodeAndValidate(raw json.RawMessage, expectedStep int32) (Validated, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Validated{}, errors.New("continuity checkpoint is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var receipt Receipt
	if err := decoder.Decode(&receipt); err != nil {
		return Validated{}, fmt.Errorf("decode continuity checkpoint: %w", err)
	}
	if err := expectEOF(decoder); err != nil {
		return Validated{}, fmt.Errorf("decode continuity checkpoint: %w", err)
	}
	if receipt.SchemaVersion != SchemaVersion {
		return Validated{}, fmt.Errorf("continuity checkpoint schema_version must be %d", SchemaVersion)
	}
	if expectedStep < 1 || expectedStep >= MaxPackages || receipt.PackageStep != expectedStep {
		return Validated{}, errors.New("continuity checkpoint package_step does not match the current package")
	}
	receipt.Summary = strings.TrimSpace(receipt.Summary)
	receipt.NextAction = strings.TrimSpace(receipt.NextAction)
	if receipt.Summary == "" || receipt.NextAction == "" {
		return Validated{}, errors.New("continuity checkpoint summary and next_action are required")
	}
	if hasControlRune(receipt.Summary) || hasControlRune(receipt.NextAction) {
		return Validated{}, errors.New("continuity checkpoint contains an unsupported control character")
	}
	if utf8.RuneCountInString(receipt.Summary) > MaxSummaryRunes || utf8.RuneCountInString(receipt.NextAction) > MaxNextActionRunes {
		return Validated{}, errors.New("continuity checkpoint summary or next_action is too long")
	}
	var err error
	if receipt.Completed, err = normalizeList("completed", receipt.Completed); err != nil {
		return Validated{}, err
	}
	if len(receipt.Completed) == 0 {
		return Validated{}, errors.New("continuity checkpoint completed must contain at least one item")
	}
	if receipt.Verification, err = normalizeList("verification", receipt.Verification); err != nil {
		return Validated{}, err
	}
	if receipt.OpenRisks, err = normalizeList("open_risks", receipt.OpenRisks); err != nil {
		return Validated{}, err
	}
	canonical, err := json.Marshal(receipt)
	if err != nil {
		return Validated{}, fmt.Errorf("encode continuity checkpoint: %w", err)
	}
	if len(canonical) > MaxCanonicalBytes {
		return Validated{}, fmt.Errorf("continuity checkpoint is %d bytes; maximum is %d", len(canonical), MaxCanonicalBytes)
	}
	digest := sha256.Sum256(canonical)
	return Validated{Receipt: receipt, Canonical: canonical, SHA256: hex.EncodeToString(digest[:])}, nil
}

func normalizeList(name string, values []string) ([]string, error) {
	if len(values) > MaxListEntries {
		return nil, fmt.Errorf("continuity checkpoint %s has too many entries", name)
	}
	if values == nil {
		return []string{}, nil
	}
	cleaned := make([]string, len(values))
	for i, value := range values {
		cleaned[i] = strings.TrimSpace(value)
		if cleaned[i] == "" || utf8.RuneCountInString(cleaned[i]) > MaxItemRunes {
			return nil, fmt.Errorf("continuity checkpoint %s entry %d is empty or too long", name, i+1)
		}
		if hasControlRune(cleaned[i]) {
			return nil, fmt.Errorf("continuity checkpoint %s entry %d contains an unsupported control character", name, i+1)
		}
	}
	return cleaned, nil
}

func hasControlRune(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func expectEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
