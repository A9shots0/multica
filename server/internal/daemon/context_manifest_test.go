package daemon

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/contextmanifest"
	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestBuildTaskContextManifestAccountsKnownAndUnknownInputs(t *testing.T) {
	task := Task{ID: "11111111-1111-4111-8111-111111111111", Attempt: 3}
	manifest, err := buildTaskContextManifest(task, "owner prompt", agent.ExecOptions{
		SystemPrompt: "system brief", AuthorityPrompt: "owner authority",
		McpConfig: []byte(`{"mcpServers":{}}`), ResumeSessionID: "private-session-id",
	}, "unused file brief", 2)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.TaskAttemptID != task.ID+":3" || manifest.ProviderAttemptID != "2" {
		t.Fatalf("attempt identity = %#v", manifest)
	}
	var exact, unknown int
	for _, block := range manifest.Blocks {
		switch block.MeasurementState {
		case contextmanifest.MeasurementExact:
			exact++
		case contextmanifest.MeasurementUnknown:
			unknown++
			if block.ContentBytes != 0 {
				t.Fatalf("unknown block fabricated a byte count: %#v", block)
			}
		}
	}
	if exact != 4 || unknown != 4 {
		t.Fatalf("exact=%d unknown=%d blocks=%#v", exact, unknown, manifest.Blocks)
	}
	wantBytes := int64(len("owner prompt") + len("system brief") + len("owner authority") + len(`{"mcpServers":{}}`))
	if manifest.TotalBytes != wantBytes {
		t.Fatalf("total bytes = %d, want %d", manifest.TotalBytes, wantBytes)
	}
}

func TestBuildTaskContextManifestDistinguishesProviderAttempts(t *testing.T) {
	task := Task{ID: "11111111-1111-4111-8111-111111111111", Attempt: 1}
	first, err := buildTaskContextManifest(task, "prompt", agent.ExecOptions{}, "brief", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildTaskContextManifest(task, "prompt", agent.ExecOptions{}, "brief", 2)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, _ := first.Digest()
	secondDigest, _ := second.Digest()
	if firstDigest == secondDigest {
		t.Fatal("provider attempt was not bound into the manifest")
	}
}
