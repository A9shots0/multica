package contextmanifest

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestDeterministicDigestAndRoundTrip(t *testing.T) {
	blocks := []Block{
		{
			Kind: "system", SourceIdentity: "prompt:system", SourceRevision: "git:abc123",
			ContentBytes: 120, Reason: "Base operating contract", Cacheable: true, MeasurementState: MeasurementExact,
		},
		{
			Kind: "brain", SourceIdentity: "brain:claim/42", SourceRevision: "revision:7",
			ContentBytes: 80, Reason: "Reviewed company memory", Cacheable: true, MeasurementState: MeasurementEstimated,
		},
	}

	first, err := New("task:attempt/1", "provider:attempt/1", 500, blocks)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("task:attempt/1", "provider:attempt/1", 500, append([]Block(nil), blocks...))
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := first.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := second.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("canonical JSON differs:\n%s\n%s", firstJSON, secondJSON)
	}
	firstDigest, err := first.Digest()
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := second.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || !strings.HasPrefix(firstDigest, "sha256:") || len(firstDigest) != len("sha256:")+64 {
		t.Fatalf("unexpected deterministic digests %q and %q", firstDigest, secondDigest)
	}

	parsed, err := Parse(firstJSON)
	if err != nil {
		t.Fatal(err)
	}
	parsedDigest, err := parsed.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if parsedDigest != firstDigest {
		t.Fatalf("round-trip digest = %s, want %s", parsedDigest, firstDigest)
	}
	if parsed.TotalBytes != 200 {
		t.Fatalf("total bytes = %d, want 200", parsed.TotalBytes)
	}
}

func TestRejectsDuplicateLiveSourceIdentity(t *testing.T) {
	blocks := validBlocks()
	blocks = append(blocks, Block{
		Kind: "skill", SourceIdentity: blocks[0].SourceIdentity, SourceRevision: "revision:2",
		ContentBytes: 1, Reason: "Different revision cannot coexist", MeasurementState: MeasurementExact,
	})
	if _, err := New("task:1", "provider:1", 100, blocks); err == nil || !strings.Contains(err.Error(), "duplicates live source identity") {
		t.Fatalf("expected duplicate-source error, got %v", err)
	}
}

func TestRejectsAggregateOverflow(t *testing.T) {
	t.Run("policy limit", func(t *testing.T) {
		blocks := validBlocks()
		blocks[0].ContentBytes = 11
		if _, err := New("task:1", "provider:1", 10, blocks); err == nil || !strings.Contains(err.Error(), "exceed limit_bytes") {
			t.Fatalf("expected aggregate-limit error, got %v", err)
		}
	})

	t.Run("integer overflow", func(t *testing.T) {
		blocks := []Block{
			{Kind: "system", SourceIdentity: "source:1", SourceRevision: "revision:1", ContentBytes: math.MaxInt64, Reason: "First", MeasurementState: MeasurementExact},
			{Kind: "skill", SourceIdentity: "source:2", SourceRevision: "revision:1", ContentBytes: 1, Reason: "Second", MeasurementState: MeasurementExact},
		}
		if _, err := New("task:1", "provider:1", math.MaxInt64, blocks); err == nil || !strings.Contains(err.Error(), "overflow") {
			t.Fatalf("expected integer-overflow error, got %v", err)
		}
	})

	t.Run("negative block", func(t *testing.T) {
		blocks := validBlocks()
		blocks[0].ContentBytes = -1
		if _, err := New("task:1", "provider:1", 10, blocks); err == nil || !strings.Contains(err.Error(), "negative") {
			t.Fatalf("expected negative-count error, got %v", err)
		}
	})
}

func TestRejectsMalformedIdentityAndUnknownState(t *testing.T) {
	tests := []struct {
		name     string
		taskID   string
		provider string
		edit     func([]Block)
		want     string
	}{
		{name: "task identity", taskID: "task id with spaces", provider: "provider:1", want: "task attempt"},
		{name: "provider identity", taskID: "task:1", provider: "provider#1", want: "provider attempt"},
		{name: "source identity", taskID: "task:1", provider: "provider:1", edit: func(blocks []Block) { blocks[0].SourceIdentity = "source id" }, want: "source identity"},
		{name: "source revision", taskID: "task:1", provider: "provider:1", edit: func(blocks []Block) { blocks[0].SourceRevision = "" }, want: "source revision"},
		{name: "measurement state", taskID: "task:1", provider: "provider:1", edit: func(blocks []Block) { blocks[0].MeasurementState = "guessed" }, want: "unknown measurement_state"},
		{name: "unknown with bytes", taskID: "task:1", provider: "provider:1", edit: func(blocks []Block) { blocks[0].MeasurementState = MeasurementUnknown }, want: "unknown measurement with non-zero"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks := validBlocks()
			if test.edit != nil {
				test.edit(blocks)
			}
			_, err := New(test.taskID, test.provider, 100, blocks)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q error, got %v", test.want, err)
			}
		})
	}
}

func TestUnknownMeasurementRecordsClassWithoutFabricatingSize(t *testing.T) {
	blocks := validBlocks()
	blocks = append(blocks, Block{
		Kind: "provider_history", SourceIdentity: "provider:history", SourceRevision: "session:opaque",
		ContentBytes: 0, Reason: "Provider-owned resumed history is not observable", MeasurementState: MeasurementUnknown,
	})
	manifest, err := New("task:1", "provider:1", 100, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.TotalBytes != 10 || manifest.Blocks[1].MeasurementState != MeasurementUnknown {
		t.Fatalf("unknown class changed measured subtotal: %#v", manifest)
	}
}

func TestProviderAttemptChangesDigestWithoutChangingTaskAttempt(t *testing.T) {
	first, err := New("task:stable", "provider:attempt/1", 100, validBlocks())
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("task:stable", "provider:attempt/2", 100, validBlocks())
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, err := first.Digest()
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := second.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest {
		t.Fatal("provider attempt identity was not bound into digest")
	}
	if first.TaskAttemptID != second.TaskAttemptID {
		t.Fatal("test setup changed task attempt identity")
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	manifest, err := New("task:1", "provider:1", 100, validBlocks())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := manifest.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded["ignored_future_field"] = true
	withUnknown, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(withUnknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func validBlocks() []Block {
	return []Block{{
		Kind: "system", SourceIdentity: "source:1", SourceRevision: "revision:1",
		ContentBytes: 10, Reason: "Required system instructions", Cacheable: true, MeasurementState: MeasurementExact,
	}}
}
