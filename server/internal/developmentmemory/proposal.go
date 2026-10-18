package developmentmemory

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/multica-ai/multica/server/pkg/redact"
)

const (
	proposalOpenTag  = "<development-memory>"
	proposalCloseTag = "</development-memory>"
)

var memoryKinds = map[string]struct{}{
	"acceptance": {}, "blocker": {}, "constraint": {}, "decision": {},
	"design": {}, "fact": {}, "finding": {}, "next_action": {},
	"requirement": {}, "user_preference": {},
}

var memoryScopes = map[string]struct{}{
	"project": {}, "workstream": {},
}

// Durable memory is deliberately narrower than task reporting. Results,
// blockers, next steps and acceptance evidence still belong in the task and
// issue history; they are not rules or facts every future agent should carry.
var durableMemoryKinds = map[string]struct{}{
	"constraint": {}, "decision": {}, "design": {}, "fact": {},
	"requirement": {}, "user_preference": {},
}

var revisionLikeText = regexp.MustCompile(`(?i)\b[0-9a-f]{7,40}\b`)

var transientMemoryMarkers = []string{
	"acceptance test", "canary", "cached source ref", "clean at head",
	"commit check", "daemon upgrade", "exit code", "git head",
	"lint passed", "local head", "repository clean", "run completed",
	"test fixture", "test-only", "tests passed", "typecheck passed",
	"worktree clean", "write-back schema",
}

type BoundRepository struct {
	CanonicalIdentity string  `json:"canonical_identity"`
	BaseRevision      *string `json:"base_revision"`
	Branch            *string `json:"branch"`
}

type Binding struct {
	SchemaVersion int               `json:"schema_version"`
	WorkstreamID  string            `json:"workstream_id"`
	AgentName     string            `json:"agent_name"`
	Repositories  []BoundRepository `json:"repositories"`
}

type RepositoryEvidence struct {
	CanonicalIdentity string   `json:"canonical_identity"`
	Revision          *string  `json:"revision"`
	Branch            *string  `json:"branch"`
	Dirty             *bool    `json:"dirty"`
	EvidenceRefs      []string `json:"evidence_refs"`
}

type MemoryItemDraft struct {
	RepositoryIdentity *string  `json:"repository_identity"`
	Scope              string   `json:"scope"`
	Kind               string   `json:"kind"`
	Statement          string   `json:"statement"`
	EvidenceRefs       []string `json:"evidence_refs"`
}

type ProposalDraft struct {
	Summary              string               `json:"summary"`
	NextAction           string               `json:"next_action"`
	RemainingUncertainty []string             `json:"remaining_uncertainty"`
	MemoryItems          []MemoryItemDraft    `json:"memory_items"`
	RepositoryEvidence   []RepositoryEvidence `json:"repository_evidence"`
}

func BindingForTask(task Task, agentName string) (Binding, error) {
	repositories, err := normalizeRepositories(task.Repositories)
	if err != nil {
		return Binding{}, err
	}
	if len(repositories) == 0 {
		return Binding{}, errors.New("development memory task has no repositories")
	}
	bound := make([]BoundRepository, 0, len(repositories))
	for _, repo := range repositories {
		bound = append(bound, BoundRepository{
			CanonicalIdentity: repo.CanonicalIdentity,
			Branch:            repo.DefaultBranch,
		})
	}
	return Binding{
		SchemaVersion: 1,
		WorkstreamID:  "multica-issue-" + strings.TrimSpace(task.IssueID),
		AgentName:     boundedText(firstNonEmpty(agentName, "multica-agent"), 80),
		Repositories:  bound,
	}, nil
}

// ParseProposal treats the agent block as a draft only. Invalid or absent
// structured output remains useful as task history, but its zero-item fallback
// is later excluded from owner memory review.
func ParseProposal(output, finalBranch string, binding Binding) ProposalDraft {
	return parseProposal(output, finalBranch, "complete", binding)
}

func parseProposal(output, finalBranch, taskStatus string, binding Binding) ProposalDraft {
	if raw, ok := proposalBlock(output); ok {
		var draft ProposalDraft
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&draft) == nil && errors.Is(decoder.Decode(new(any)), io.EOF) {
			if normalized, err := ValidateProposal(draft, binding); err == nil {
				return normalized
			}
		}
	}
	summary := strings.TrimSpace(output)
	if index := strings.LastIndex(summary, proposalOpenTag); index >= 0 {
		summary = strings.TrimSpace(summary[:index])
	}
	if summary == "" {
		if normalizeTaskStatus(taskStatus) == "failed" {
			summary = "Task failed without a written error."
		} else {
			summary = "Task completed without a written summary."
		}
	}
	nextAction := "Review the completed work and record the next verified action."
	if normalizeTaskStatus(taskStatus) == "failed" {
		nextAction = "Review the failure, preserve the verified blocker, and record the next recovery action."
	}
	draft := ProposalDraft{
		Summary:              boundedText(summary, 4000),
		NextAction:           nextAction,
		RemainingUncertainty: []string{},
		MemoryItems:          []MemoryItemDraft{},
		RepositoryEvidence:   defaultRepositoryEvidence(binding, finalBranch),
	}
	return draft
}

func proposalBlock(output string) ([]byte, bool) {
	start := strings.LastIndex(output, proposalOpenTag)
	if start < 0 {
		return nil, false
	}
	start += len(proposalOpenTag)
	relativeEnd := strings.Index(output[start:], proposalCloseTag)
	if relativeEnd < 0 {
		return nil, false
	}
	raw := strings.TrimSpace(output[start : start+relativeEnd])
	if raw == "" || len(raw) > 64<<10 {
		return nil, false
	}
	return []byte(raw), true
}

func ValidateProposal(draft ProposalDraft, binding Binding) (ProposalDraft, error) {
	draft.Summary = redact.Text(strings.TrimSpace(draft.Summary))
	draft.NextAction = redact.Text(strings.TrimSpace(draft.NextAction))
	if draft.Summary == "" || len([]rune(draft.Summary)) > 4000 {
		return ProposalDraft{}, errors.New("summary must be 1-4000 characters")
	}
	if draft.NextAction == "" || len([]rune(draft.NextAction)) > 2000 {
		return ProposalDraft{}, errors.New("next_action must be 1-2000 characters")
	}
	if len(draft.RemainingUncertainty) > 50 || len(draft.MemoryItems) > 50 || len(draft.RepositoryEvidence) > 20 {
		return ProposalDraft{}, errors.New("development memory proposal exceeds its item limits")
	}
	bound := make(map[string]struct{}, len(binding.Repositories))
	for _, repo := range binding.Repositories {
		bound[repo.CanonicalIdentity] = struct{}{}
	}
	for i, value := range draft.RemainingUncertainty {
		value = redact.Text(strings.TrimSpace(value))
		if value == "" || len([]rune(value)) > 2000 {
			return ProposalDraft{}, errors.New("remaining uncertainty must be 1-2000 characters")
		}
		draft.RemainingUncertainty[i] = value
	}
	for i := range draft.MemoryItems {
		item := &draft.MemoryItems[i]
		item.Scope = strings.ToLower(strings.TrimSpace(item.Scope))
		if item.Scope == "" {
			item.Scope = "workstream"
		}
		if _, ok := memoryScopes[item.Scope]; !ok {
			return ProposalDraft{}, errors.New("memory item has an unsupported scope")
		}
		item.Kind = strings.TrimSpace(item.Kind)
		item.Statement = redact.Text(strings.TrimSpace(item.Statement))
		if _, ok := memoryKinds[item.Kind]; !ok {
			return ProposalDraft{}, errors.New("memory item has an unsupported kind")
		}
		if item.Statement == "" || len([]rune(item.Statement)) > 4000 {
			return ProposalDraft{}, errors.New("memory item statement must be 1-4000 characters")
		}
		if item.RepositoryIdentity != nil {
			identity := strings.ToLower(strings.TrimSpace(*item.RepositoryIdentity))
			if _, ok := bound[identity]; !ok {
				return ProposalDraft{}, errors.New("memory item references an unbound repository")
			}
			item.RepositoryIdentity = &identity
		}
		refs, err := normalizeEvidenceRefs(item.EvidenceRefs)
		if err != nil {
			return ProposalDraft{}, err
		}
		item.EvidenceRefs = refs
	}
	seenEvidence := map[string]struct{}{}
	for i := range draft.RepositoryEvidence {
		evidence := &draft.RepositoryEvidence[i]
		evidence.CanonicalIdentity = strings.ToLower(strings.TrimSpace(evidence.CanonicalIdentity))
		if _, ok := bound[evidence.CanonicalIdentity]; !ok {
			return ProposalDraft{}, errors.New("repository evidence references an unbound repository")
		}
		if _, exists := seenEvidence[evidence.CanonicalIdentity]; exists {
			return ProposalDraft{}, errors.New("repository evidence identities must be unique")
		}
		seenEvidence[evidence.CanonicalIdentity] = struct{}{}
		evidence.Revision = normalizeOptional(evidence.Revision, 160)
		evidence.Branch = normalizeOptional(evidence.Branch, 240)
		refs, err := normalizeEvidenceRefs(evidence.EvidenceRefs)
		if err != nil {
			return ProposalDraft{}, err
		}
		evidence.EvidenceRefs = refs
	}
	if draft.RemainingUncertainty == nil {
		draft.RemainingUncertainty = []string{}
	}
	if draft.MemoryItems == nil {
		draft.MemoryItems = []MemoryItemDraft{}
	}
	if draft.RepositoryEvidence == nil {
		draft.RepositoryEvidence = []RepositoryEvidence{}
	}
	return draft, nil
}

// ReviewableProposal reduces a valid agent draft to the durable statements an
// owner could reasonably want future work to remember. It is intentionally
// deterministic: model prose never decides whether its own run metadata is
// promoted into company memory.
func ReviewableProposal(draft ProposalDraft) (ProposalDraft, bool) {
	items := make([]MemoryItemDraft, 0, len(draft.MemoryItems))
	seen := make(map[string]struct{}, len(draft.MemoryItems))
	for _, item := range draft.MemoryItems {
		if _, ok := durableMemoryKinds[item.Kind]; !ok || looksTransientMemory(item.Statement) {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(item.Statement), " "))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, item)
		if len(items) == 10 {
			break
		}
	}
	if len(items) == 0 {
		return ProposalDraft{}, false
	}
	draft.MemoryItems = items
	statements := make([]string, 0, len(items))
	for _, item := range items {
		statements = append(statements, item.Statement)
	}
	draft.Summary = boundedText(strings.Join(statements, " "), 4000)
	draft.NextAction = "Use the owner-approved context in future work."
	draft.RemainingUncertainty = []string{}
	return draft, true
}

func looksTransientMemory(statement string) bool {
	value := strings.ToLower(strings.Join(strings.Fields(statement), " "))
	if value == "" || revisionLikeText.MatchString(value) {
		return true
	}
	for _, marker := range transientMemoryMarkers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func defaultRepositoryEvidence(binding Binding, finalBranch string) []RepositoryEvidence {
	result := make([]RepositoryEvidence, 0, len(binding.Repositories))
	for _, repo := range binding.Repositories {
		branch := repo.Branch
		if len(binding.Repositories) == 1 && strings.TrimSpace(finalBranch) != "" {
			value := strings.TrimSpace(finalBranch)
			branch = &value
		}
		result = append(result, RepositoryEvidence{
			CanonicalIdentity: repo.CanonicalIdentity,
			Branch:            branch, EvidenceRefs: []string{},
		})
	}
	return result
}

func normalizeOptional(value *string, maximum int) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || len([]rune(normalized)) > maximum {
		return nil
	}
	return &normalized
}

func normalizeEvidenceRefs(values []string) ([]string, error) {
	if len(values) > 50 {
		return nil, errors.New("too many evidence references")
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > 500 || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("evidence reference must be a single line up to 500 characters")
		}
		result = append(result, redact.Text(value))
	}
	if result == nil {
		result = []string{}
	}
	return result, nil
}
