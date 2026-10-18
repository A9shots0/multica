package designgate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const CheckpointOpenTag = "<design-checkpoint>"
const CheckpointCloseTag = "</design-checkpoint>"

// Checkpoint is an intermediate result awaiting human feedback, never a ship
// verdict or evidence that a previous phase was accepted. It uses the existing
// task result and issue continuation path, not a parallel workflow engine.
type Checkpoint struct {
	SchemaVersion       int                  `json:"schema_version"`
	ProductID           string               `json:"product_id"`
	PackSHA256          string               `json:"pack_sha256"`
	Stage               string               `json:"stage"`
	Status              string               `json:"status"`
	ImplementationReady *bool                `json:"implementation_ready"`
	Summary             string               `json:"summary"`
	Question            string               `json:"question"`
	Recommendation      string               `json:"recommendation"`
	Artifacts           []CheckpointArtifact `json:"artifacts"`
}

type CheckpointArtifact struct {
	Label    string `json:"label"`
	Location string `json:"location"`
	SHA256   string `json:"sha256"`
}

// ValidatedCheckpoint validates a bounded handoff contract. Artifact hashes
// are references, not independent proof the artifacts render or are useful.
func ValidatedCheckpoint(output string, binding Binding) (*Checkpoint, error) {
	if strings.Contains(output, OpenTag) || strings.Contains(output, ReviewOpenTag) {
		return nil, errors.New("a design checkpoint cannot also claim final delivery or a ship review")
	}
	raw, err := extractSingleTaggedBlock(output, CheckpointOpenTag, CheckpointCloseTag)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var checkpoint Checkpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return nil, fmt.Errorf("design checkpoint JSON does not match the required contract: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, err
	}
	if checkpoint.SchemaVersion != 1 || binding.SchemaVersion != 1 ||
		checkpoint.ProductID != binding.ProductID || checkpoint.PackSHA256 != binding.PackSHA256 {
		return nil, errors.New("design checkpoint does not match the bound product pack")
	}
	minimumArtifacts := 1
	switch checkpoint.Stage {
	case "research", "visual_design", "prototype":
	case "wireframes":
		minimumArtifacts = 2
	default:
		return nil, errors.New("design checkpoint stage must be research, wireframes, visual_design or prototype")
	}
	if checkpoint.Status != "awaiting_client_feedback" || checkpoint.ImplementationReady == nil || *checkpoint.ImplementationReady {
		return nil, errors.New("design checkpoint must explicitly await client feedback and not claim implementation readiness")
	}
	if !allNonEmpty([]string{checkpoint.Summary, checkpoint.Question, checkpoint.Recommendation}) {
		return nil, errors.New("design checkpoint needs a summary, focused question and recommendation")
	}
	if len(checkpoint.Artifacts) < minimumArtifacts || len(checkpoint.Artifacts) > 12 {
		return nil, errors.New("design checkpoint needs bounded artifacts; wireframes require at least two distinct compositions")
	}
	seenLocations, seenHashes := map[string]bool{}, map[string]bool{}
	for _, artifact := range checkpoint.Artifacts {
		location := strings.TrimSpace(artifact.Location)
		if strings.TrimSpace(artifact.Label) == "" || location == "" || !sha256Pattern.MatchString(artifact.SHA256) ||
			seenLocations[location] || seenHashes[artifact.SHA256] {
			return nil, errors.New("design checkpoint artifacts need labels, distinct locations and distinct SHA-256 references")
		}
		seenLocations[location], seenHashes[artifact.SHA256] = true, true
	}
	return &checkpoint, nil
}
