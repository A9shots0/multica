package workflowresume

import (
	"encoding/json"
	"strings"
	"testing"
)

var (
	testTarget = Target{
		TaskID: "11111111-1111-4111-8111-111111111111", TaskAttempt: 2,
		WorkspaceID: "22222222-2222-4222-8222-222222222222",
		IssueID:     "33333333-3333-4333-8333-333333333333",
	}
	testSource = Source{
		TaskID:            "44444444-4444-4444-8444-444444444444",
		ExecutionID:       "55555555-5555-4555-8555-555555555555",
		ExecutionRevision: 7, SnapshotSHA256: strings.Repeat("a", 64),
	}
)

func TestBuildParseCanonicalAndBound(t *testing.T) {
	first, err := Build(testTarget, testSource, 9, json.RawMessage(`{"z":2,"a":{"done":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(testTarget, testSource, 9, json.RawMessage(`{ "a": { "done": true }, "z": 2 }`))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("equivalent checkpoints produced different capsules:\n%s\n%s", first, second)
	}
	parsed, err := Parse(first, testTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(parsed.Checkpoint) != `{"a":{"done":true},"z":2}` || parsed.Source.ExecutionRevision != 7 {
		t.Fatalf("parsed capsule = %#v checkpoint=%s", parsed, parsed.Checkpoint)
	}

	wrong := testTarget
	wrong.TaskAttempt++
	if _, err := Parse(first, wrong); err == nil {
		t.Fatal("target-swapped capsule accepted")
	}
}

func TestParseRejectsTamperingUnknownFieldsAndOversize(t *testing.T) {
	raw, err := Build(testTarget, testSource, 9, json.RawMessage(`{"next_action":"run tests"}`))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["current_picture_revision"] = float64(10)
	tampered, _ := json.Marshal(document)
	if _, err := Parse(tampered, testTarget); err == nil {
		t.Fatal("digest-tampered capsule accepted")
	}

	unknown := append(raw[:len(raw)-1], []byte(`,"authority":"forged"}`)...)
	if _, err := Parse(unknown, testTarget); err == nil {
		t.Fatal("unknown field accepted")
	}

	large := json.RawMessage(`{"value":"` + strings.Repeat("x", MaxCheckpointBytes) + `"}`)
	if _, err := Build(testTarget, testSource, 9, large); err == nil {
		t.Fatal("oversized checkpoint accepted")
	}
}

func TestBuildRejectsNonObjectAndInvalidBindings(t *testing.T) {
	if _, err := Build(testTarget, testSource, 9, json.RawMessage(`[]`)); err == nil {
		t.Fatal("array checkpoint accepted")
	}
	invalid := testTarget
	invalid.TaskID = "not-a-uuid"
	if _, err := Build(invalid, testSource, 9, json.RawMessage(`{}`)); err == nil {
		t.Fatal("invalid target accepted")
	}
}
