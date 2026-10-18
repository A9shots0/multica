package designgate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesignCheckpointSeparatesClientFeedbackFromFinalDelivery(t *testing.T) {
	binding := Binding{SchemaVersion: 1, ProductID: "phone-farm", PackSHA256: strings.Repeat("a", 64)}
	notReady := false
	valid := Checkpoint{
		SchemaVersion: 1, ProductID: binding.ProductID, PackSHA256: binding.PackSHA256,
		Stage: "wireframes", Status: "awaiting_client_feedback", ImplementationReady: &notReady,
		Summary:  "Two compositions for filling a multi-phone schedule.",
		Question: "Which composition should I develop?", Recommendation: "A keeps media recognizable while scanning.",
		Artifacts: []CheckpointArtifact{
			{Label: "A", Location: "https://preview.example/wireframe-a", SHA256: strings.Repeat("b", 64)},
			{Label: "B", Location: "https://preview.example/wireframe-b", SHA256: strings.Repeat("c", 64)},
		},
	}
	encode := func(value Checkpoint) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return CheckpointOpenTag + string(raw) + CheckpointCloseTag
	}
	for _, stage := range []string{"research", "wireframes", "visual_design", "prototype"} {
		candidate := valid
		candidate.Stage = stage
		if _, err := ValidatedCheckpoint(encode(candidate), binding); err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
	}
	if _, err := ValidatedCompletion(encode(valid), binding); err == nil {
		t.Fatal("checkpoint was accepted as final delivery")
	}
	for name, mutate := range map[string]func(*Checkpoint){
		"wrong product":     func(c *Checkpoint) { c.ProductID = "crm" },
		"wrong pack":        func(c *Checkpoint) { c.PackSHA256 = strings.Repeat("d", 64) },
		"final stage":       func(c *Checkpoint) { c.Stage = "implementation" },
		"claims acceptance": func(c *Checkpoint) { c.Status = "approved" },
		"missing readiness": func(c *Checkpoint) { c.ImplementationReady = nil },
		"claims readiness":  func(c *Checkpoint) { ready := true; c.ImplementationReady = &ready },
		"no question":       func(c *Checkpoint) { c.Question = " " },
		"no recommendation": func(c *Checkpoint) { c.Recommendation = "" },
		"one wireframe":     func(c *Checkpoint) { c.Artifacts = c.Artifacts[:1] },
		"same composition":  func(c *Checkpoint) { c.Artifacts[1] = c.Artifacts[0] },
		"invalid hash":      func(c *Checkpoint) { c.Artifacts[0].SHA256 = "unverified" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.Artifacts = append([]CheckpointArtifact(nil), valid.Artifacts...)
			mutate(&candidate)
			if _, err := ValidatedCheckpoint(encode(candidate), binding); err == nil {
				t.Fatal("invalid checkpoint accepted")
			}
		})
	}
	for _, output := range []string{encode(valid) + OpenTag + "{}" + CloseTag, encode(valid) + ReviewOpenTag + "{}" + ReviewCloseTag, encode(valid) + encode(valid)} {
		if _, err := ValidatedCheckpoint(output, binding); err == nil {
			t.Fatal("ambiguous/mixed completion accepted")
		}
	}
}
