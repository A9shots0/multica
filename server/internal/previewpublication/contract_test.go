package previewpublication

import (
	"errors"
	"testing"
	"time"
)

func validFixture(t *testing.T) (Binding, ProducerClaim, time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 20, 20, 0, 0, 0, time.UTC)
	binding := Binding{
		WorkspaceID:       "11111111-1111-4111-8111-111111111111",
		IssueID:           "22222222-2222-4222-8222-222222222222",
		AgentID:           "33333333-3333-4333-8333-333333333333",
		AgentRunID:        "44444444-4444-4444-8444-444444444444",
		CandidateID:       "55555555-5555-4555-8555-555555555555",
		CandidateRevision: 1,
		SourceRevision:    "git:0123456789abcdef",
		Worktree: WorktreeProvenance{
			DaemonID:  "bluefin-host",
			RuntimeID: "66666666-6666-4666-8666-666666666666",
			Path:      "/workspace/issue-42",
			Branch:    "agent/issue-42",
		},
	}
	claim := ProducerClaim{
		PreviewURL:  "http://127.0.0.1:4173/",
		IdentityURL: "http://127.0.0.1:4173" + IdentityPath,
		DataPolicy:  ReadOnlyDataPolicy(),
		ExpiresAt:   now.Add(MaxLifetime),
	}
	return binding, claim, now
}

func TestSealAndIdentityRequireExactBinding(t *testing.T) {
	binding, claim, now := validFixture(t)
	publication, err := Seal(binding, claim, now)
	if err != nil {
		t.Fatal(err)
	}
	identity := Identity{
		SchemaVersion: IdentitySchemaVersion,
		Binding:       publication.Binding,
		DataPolicy:    publication.DataPolicy,
		ExpiresAt:     publication.ExpiresAt,
	}
	if err := VerifyIdentity(publication, identity, now); err != nil {
		t.Fatalf("exact identity rejected: %v", err)
	}
	identity.Binding.IssueID = "77777777-7777-4777-8777-777777777777"
	if err := VerifyIdentity(publication, identity, now); !errors.Is(err, ErrInvalidPublication) {
		t.Fatalf("cross-issue identity accepted: %v", err)
	}
}

func TestSealRejectsUnsafeOrUnboundPublication(t *testing.T) {
	binding, claim, now := validFixture(t)
	cases := []struct {
		name string
		edit func(*Binding, *ProducerClaim)
	}{
		{"different identity origin", func(_ *Binding, c *ProducerClaim) { c.IdentityURL = "https://example.invalid" + IdentityPath }},
		{"wrong identity path", func(_ *Binding, c *ProducerClaim) { c.IdentityURL = "http://127.0.0.1:4173/identity" }},
		{"remote https", func(_ *Binding, c *ProducerClaim) {
			c.PreviewURL, c.IdentityURL = "https://example.invalid/", "https://example.invalid"+IdentityPath
		}},
		{"remote plaintext", func(_ *Binding, c *ProducerClaim) {
			c.PreviewURL, c.IdentityURL = "http://example.invalid/", "http://example.invalid"+IdentityPath
		}},
		{"credentials in URL", func(_ *Binding, c *ProducerClaim) { c.PreviewURL = "https://secret@example.invalid/" }},
		{"policy drift", func(_ *Binding, c *ProducerClaim) { c.DataPolicy.Mutations = "allowed" }},
		{"expired", func(_ *Binding, c *ProducerClaim) { c.ExpiresAt = now }},
		{"excess lifetime", func(_ *Binding, c *ProducerClaim) { c.ExpiresAt = now.Add(MaxLifetime + time.Second) }},
		{"relative worktree", func(b *Binding, _ *ProducerClaim) { b.Worktree.Path = "workspace" }},
		{"missing run", func(b *Binding, _ *ProducerClaim) { b.AgentRunID = "" }},
		{"non-canonical UUID", func(b *Binding, _ *ProducerClaim) { b.IssueID = "22222222222242228222222222222222" }},
		{"mutable candidate revision", func(b *Binding, _ *ProducerClaim) { b.CandidateRevision = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, c := binding, claim
			tc.edit(&b, &c)
			if _, err := Seal(b, c, now); !errors.Is(err, ErrInvalidPublication) {
				t.Fatalf("invalid publication accepted: %v", err)
			}
		})
	}
}

func TestSealAcceptsWindowsWorktreeProvenance(t *testing.T) {
	binding, claim, now := validFixture(t)
	binding.Worktree.Path = `C:\workspace\issue-42`
	if _, err := Seal(binding, claim, now); err != nil {
		t.Fatalf("Windows worktree rejected: %v", err)
	}
}

func TestIdentityExpiryIsEnforced(t *testing.T) {
	binding, claim, now := validFixture(t)
	publication, err := Seal(binding, claim, now)
	if err != nil {
		t.Fatal(err)
	}
	identity := Identity{SchemaVersion: IdentitySchemaVersion, Binding: binding, DataPolicy: claim.DataPolicy, ExpiresAt: claim.ExpiresAt}
	if err := VerifyIdentity(publication, identity, claim.ExpiresAt); !errors.Is(err, ErrInvalidPublication) {
		t.Fatalf("expired identity accepted: %v", err)
	}
}
