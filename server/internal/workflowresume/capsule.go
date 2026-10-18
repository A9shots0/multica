// Package workflowresume defines the bounded, server-authored capsule used to
// start a fresh provider session from a verified workflow checkpoint.
//
// A capsule is continuity evidence only. It carries no authority and cannot
// close the workflow goal. The target binding prevents a capsule copied from
// another claim from being accepted by the daemon.
package workflowresume

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	ContractVersion    = 1
	MaxCheckpointBytes = 2 << 10
	MaxCapsuleBytes    = 8 << 10
)

var ErrInvalidCapsule = errors.New("invalid workflow resume capsule")

type Target struct {
	TaskID      string `json:"task_id"`
	TaskAttempt int32  `json:"task_attempt"`
	WorkspaceID string `json:"workspace_id"`
	IssueID     string `json:"issue_id"`
}

type Source struct {
	TaskID            string `json:"task_id"`
	ExecutionID       string `json:"execution_id"`
	ExecutionRevision int64  `json:"execution_revision"`
	SnapshotSHA256    string `json:"snapshot_sha256"`
}

type Capsule struct {
	ContractVersion        int             `json:"contract_version"`
	Target                 Target          `json:"target"`
	Source                 Source          `json:"source"`
	CurrentPictureRevision int64           `json:"current_picture_revision"`
	Checkpoint             json.RawMessage `json:"checkpoint"`
	CapsuleSHA256          string          `json:"capsule_sha256"`
}

// Build canonicalizes the untrusted checkpoint object and binds it to one
// exact claimed task. snapshotSHA256 is the digest of the append-only source
// snapshot verified by the server before this function is called.
func Build(target Target, source Source, currentPictureRevision int64, checkpoint json.RawMessage) (json.RawMessage, error) {
	canonicalCheckpoint, err := canonicalCheckpoint(checkpoint)
	if err != nil {
		return nil, err
	}
	capsule := Capsule{
		ContractVersion:        ContractVersion,
		Target:                 target,
		Source:                 source,
		CurrentPictureRevision: currentPictureRevision,
		Checkpoint:             canonicalCheckpoint,
	}
	if err := validate(capsule, false); err != nil {
		return nil, err
	}
	digest, err := digest(capsule)
	if err != nil {
		return nil, err
	}
	capsule.CapsuleSHA256 = digest
	raw, err := json.Marshal(capsule)
	if err != nil || len(raw) > MaxCapsuleBytes {
		return nil, ErrInvalidCapsule
	}
	return raw, nil
}

// Parse strictly validates wire data, its digest, and its exact claim binding.
func Parse(raw []byte, expected Target) (Capsule, error) {
	if len(raw) == 0 || len(raw) > MaxCapsuleBytes || !json.Valid(raw) {
		return Capsule{}, ErrInvalidCapsule
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var capsule Capsule
	if err := decoder.Decode(&capsule); err != nil {
		return Capsule{}, fmt.Errorf("%w: decode: %v", ErrInvalidCapsule, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Capsule{}, ErrInvalidCapsule
	}
	canonical, err := canonicalCheckpoint(capsule.Checkpoint)
	if err != nil {
		return Capsule{}, err
	}
	capsule.Checkpoint = canonical
	if err := validate(capsule, true); err != nil {
		return Capsule{}, err
	}
	if capsule.Target != expected {
		return Capsule{}, fmt.Errorf("%w: target binding mismatch", ErrInvalidCapsule)
	}
	want, err := digest(capsule)
	if err != nil || !strings.EqualFold(want, capsule.CapsuleSHA256) {
		return Capsule{}, fmt.Errorf("%w: digest mismatch", ErrInvalidCapsule)
	}
	capsule.CapsuleSHA256 = strings.ToLower(capsule.CapsuleSHA256)
	return capsule, nil
}

func digest(capsule Capsule) (string, error) {
	capsule.CapsuleSHA256 = ""
	raw, err := json.Marshal(capsule)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalCheckpoint(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > MaxCheckpointBytes || !json.Valid(raw) {
		return nil, fmt.Errorf("%w: invalid checkpoint", ErrInvalidCapsule)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("%w: checkpoint must be an object", ErrInvalidCapsule)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, ErrInvalidCapsule
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical) > MaxCheckpointBytes {
		return nil, ErrInvalidCapsule
	}
	return canonical, nil
}

func validate(capsule Capsule, requireDigest bool) error {
	if capsule.ContractVersion != ContractVersion || capsule.Target.TaskAttempt < 1 ||
		capsule.Source.ExecutionRevision < 1 || capsule.CurrentPictureRevision < 1 {
		return ErrInvalidCapsule
	}
	for _, value := range []string{
		capsule.Target.TaskID, capsule.Target.WorkspaceID, capsule.Target.IssueID,
		capsule.Source.TaskID, capsule.Source.ExecutionID,
	} {
		if !isUUID(value) {
			return ErrInvalidCapsule
		}
	}
	if !isSHA256(capsule.Source.SnapshotSHA256) || (requireDigest && !isSHA256(capsule.CapsuleSHA256)) {
		return ErrInvalidCapsule
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		switch index {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
				return false
			}
		}
	}
	return true
}
