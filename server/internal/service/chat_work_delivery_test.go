package service

import "testing"

func TestReviewFromArtifactsBuildsVersionedArtifactContract(t *testing.T) {
	raw := []byte(`[
		{"id":"preview","label":"Clients.html","kind":"page","url":"/preview","original":true},
		{"id":"capture","label":"Clients-390.png","kind":"image","url":"/capture","original":true},
		{"id":"notes","label":"notes.txt","kind":"file","url":"/notes","original":true}
	]`)

	review := reviewFromArtifacts("task-1", raw)
	if review == nil {
		t.Fatal("expected a review payload")
	}
	if review.SchemaVersion != 1 || review.SourceTaskID != "task-1" {
		t.Fatalf("review identity = version %d task %q", review.SchemaVersion, review.SourceTaskID)
	}
	if len(review.Artifacts) != 3 {
		t.Fatalf("artifact count = %d, want 3", len(review.Artifacts))
	}
	if review.Artifacts[0].Kind != "page" || review.Artifacts[1].Kind != "image" || review.Artifacts[2].Kind != "file" {
		t.Fatalf("artifact kinds = %#v", review.Artifacts)
	}
	for _, artifact := range review.Artifacts {
		if !artifact.Original {
			t.Fatalf("artifact %q was not identified as original evidence", artifact.ID)
		}
	}
}

func TestReviewFromArtifactsIgnoresMissingOrMalformedData(t *testing.T) {
	if review := reviewFromArtifacts("task-1", nil); review != nil {
		t.Fatalf("unexpected review payload: %#v", review)
	}
	if review := reviewFromArtifacts("task-1", []byte(`{"not":"an array"}`)); review != nil {
		t.Fatalf("unexpected review payload: %#v", review)
	}
}
