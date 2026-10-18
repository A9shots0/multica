// Package previewpublication defines and stores the controller-owned contract
// for a short-lived native-shell preview. The preview producer may be on a
// daemon host that the controller cannot reach, so the native shell performs
// the reachability and identity check.
package previewpublication

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SchemaVersion         = "multica.preview-publication.v1"
	IdentitySchemaVersion = "multica.preview-identity.v1"
	IdentityPath          = "/.well-known/multica-preview-identity"
	MaxLifetime           = 5 * time.Minute
)

var ErrInvalidPublication = errors.New("invalid preview publication")

type Binding struct {
	WorkspaceID       string             `json:"workspace_id"`
	IssueID           string             `json:"issue_id"`
	AgentID           string             `json:"agent_id"`
	AgentRunID        string             `json:"agent_run_id"`
	CandidateID       string             `json:"candidate_id"`
	CandidateRevision int64              `json:"candidate_revision"`
	SourceRevision    string             `json:"source_revision"`
	Worktree          WorktreeProvenance `json:"worktree"`
}

type WorktreeProvenance struct {
	DaemonID  string `json:"daemon_id"`
	RuntimeID string `json:"runtime_id"`
	Path      string `json:"path"`
	Branch    string `json:"branch"`
}

type DataPolicy struct {
	ID              string `json:"id"`
	DataClass       string `json:"data_class"`
	Mutations       string `json:"mutations"`
	ExternalNetwork string `json:"external_network"`
	Credentials     string `json:"credentials"`
}

func ReadOnlyDataPolicy() DataPolicy {
	return DataPolicy{
		ID:              "synthetic-or-redacted-read-only-v1",
		DataClass:       "synthetic_or_redacted",
		Mutations:       "denied",
		ExternalNetwork: "denied",
		Credentials:     "denied",
	}
}

// ProducerClaim is the only producer-selected part of a publication. The
// controller supplies every identity and provenance field to Seal.
type ProducerClaim struct {
	PreviewURL  string     `json:"preview_url"`
	IdentityURL string     `json:"identity_url"`
	DataPolicy  DataPolicy `json:"data_policy"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

type Publication struct {
	SchemaVersion string     `json:"schema_version"`
	Binding       Binding    `json:"binding"`
	PreviewURL    string     `json:"preview_url"`
	IdentityURL   string     `json:"identity_url"`
	DataPolicy    DataPolicy `json:"data_policy"`
	PublishedAt   time.Time  `json:"published_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
}

// Identity is returned by the preview producer. Repeating the complete
// controller binding prevents a reachable but stale or cross-issue preview
// from being rendered by the native shell.
type Identity struct {
	SchemaVersion string     `json:"schema_version"`
	Binding       Binding    `json:"binding"`
	DataPolicy    DataPolicy `json:"data_policy"`
	ExpiresAt     time.Time  `json:"expires_at"`
}

func Seal(binding Binding, claim ProducerClaim, now time.Time) (Publication, error) {
	now = now.UTC()
	publication := Publication{
		SchemaVersion: SchemaVersion,
		Binding:       binding,
		PreviewURL:    claim.PreviewURL,
		IdentityURL:   claim.IdentityURL,
		DataPolicy:    claim.DataPolicy,
		PublishedAt:   now,
		ExpiresAt:     claim.ExpiresAt.UTC(),
	}
	if err := Validate(publication, now); err != nil {
		return Publication{}, err
	}
	return publication, nil
}

func Validate(publication Publication, now time.Time) error {
	if publication.SchemaVersion != SchemaVersion {
		return invalid("schema version")
	}
	if err := validateBinding(publication.Binding); err != nil {
		return err
	}
	if publication.DataPolicy != ReadOnlyDataPolicy() {
		return invalid("data policy")
	}
	if publication.PublishedAt.IsZero() || publication.ExpiresAt.IsZero() ||
		publication.PublishedAt.After(now.UTC()) ||
		!publication.ExpiresAt.After(now.UTC()) ||
		publication.ExpiresAt.Sub(publication.PublishedAt) > MaxLifetime {
		return invalid("expiry")
	}
	preview, err := validateURL(publication.PreviewURL)
	if err != nil {
		return err
	}
	identity, err := validateURL(publication.IdentityURL)
	if err != nil {
		return err
	}
	if identity.Path != IdentityPath || identity.RawQuery != "" ||
		preview.Scheme != identity.Scheme || preview.Host != identity.Host {
		return invalid("identity endpoint")
	}
	return nil
}

// VerifyIdentity is called only after the native shell has fetched IdentityURL
// with GET and received 200 application/json. It does not accept a partial
// repetition of the binding.
func VerifyIdentity(publication Publication, identity Identity, now time.Time) error {
	if err := Validate(publication, now); err != nil {
		return err
	}
	if identity.SchemaVersion != IdentitySchemaVersion ||
		!reflect.DeepEqual(identity.Binding, publication.Binding) ||
		identity.DataPolicy != publication.DataPolicy ||
		!identity.ExpiresAt.Equal(publication.ExpiresAt) {
		return invalid("identity mismatch")
	}
	return nil
}

func validateBinding(binding Binding) error {
	for name, value := range map[string]string{
		"workspace_id": binding.WorkspaceID,
		"issue_id":     binding.IssueID,
		"agent_id":     binding.AgentID,
		"agent_run_id": binding.AgentRunID,
		"candidate_id": binding.CandidateID,
		"runtime_id":   binding.Worktree.RuntimeID,
	} {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed.String() != value {
			return invalid(name)
		}
	}
	if binding.CandidateRevision != 1 || strings.TrimSpace(binding.SourceRevision) == "" ||
		len(binding.SourceRevision) > 256 || strings.ContainsAny(binding.SourceRevision, "\r\n\x00") {
		return invalid("candidate revision")
	}
	if strings.TrimSpace(binding.Worktree.DaemonID) == "" ||
		strings.TrimSpace(binding.Worktree.Branch) == "" ||
		!absoluteWorktreePath(binding.Worktree.Path) {
		return invalid("worktree provenance")
	}
	return nil
}

func absoluteWorktreePath(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\\`) {
		return true
	}
	return len(value) >= 3 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

func validateURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.Fragment != "" {
		return nil, invalid("url")
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if parsed.Scheme != "http" || !loopback {
		return nil, invalid("url transport")
	}
	return parsed, nil
}

func invalid(field string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPublication, field)
}
