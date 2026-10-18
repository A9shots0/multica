package previewpublication

import "testing"

func TestReplayMatchesIgnoresControllerTimestampOnly(t *testing.T) {
	binding, claim, now := validFixture(t)
	stored, err := Seal(binding, claim, now)
	if err != nil {
		t.Fatal(err)
	}
	replay := stored
	replay.PublishedAt = replay.PublishedAt.Add(1)
	if !replayMatches(stored, replay) {
		t.Fatal("identical producer claim was not idempotent")
	}
	for _, mutate := range []func(*Publication){
		func(p *Publication) { p.Binding.Worktree.Branch = "other" },
		func(p *Publication) { p.PreviewURL = "http://127.0.0.1:9999/" },
		func(p *Publication) { p.DataPolicy.Credentials = "allowed" },
		func(p *Publication) { p.ExpiresAt = p.ExpiresAt.Add(-1) },
	} {
		changed := stored
		mutate(&changed)
		if replayMatches(stored, changed) {
			t.Fatal("non-identical publication replay matched")
		}
	}
}
