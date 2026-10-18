package designgate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	OpenTag        = "<design-task-delivery>"
	CloseTag       = "</design-task-delivery>"
	ReviewOpenTag  = "<design-review-verdict>"
	ReviewCloseTag = "</design-review-verdict>"
)

var (
	sha256Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	revisionPattern   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
)

type Binding struct {
	SchemaVersion      int             `json:"schema_version"`
	ProductID          string          `json:"product_id"`
	RepositoryIdentity string          `json:"repository_identity"`
	PackSchemaVersion  int             `json:"pack_schema_version"`
	SourceRevision     string          `json:"source_revision"`
	PackSHA256         string          `json:"pack_sha256"`
	ApprovalState      string          `json:"approval_state"`
	Pack               json.RawMessage `json:"pack"`
}

type contextPacket struct {
	DesignPacks []contextPack `json:"design_packs"`
}

type contextPack struct {
	RepositoryIdentity string          `json:"repository_identity"`
	ProductID          string          `json:"product_id"`
	SchemaVersion      int             `json:"schema_version"`
	SourceRevision     string          `json:"source_revision"`
	SHA256             string          `json:"sha256"`
	ApprovalState      string          `json:"approval_state"`
	Pack               json.RawMessage `json:"pack"`
}

type productPack struct {
	SchemaVersion int `json:"schema_version"`
	Product       struct {
		ID                 string `json:"id"`
		RepositoryIdentity string `json:"repository_identity"`
		SourceRevision     string `json:"source_revision"`
	} `json:"product"`
	Sources []struct {
		ID string `json:"id"`
	} `json:"sources"`
	ComponentLibrary []struct {
		ID string `json:"id"`
	} `json:"component_library"`
	PagePatterns []struct {
		ID string `json:"id"`
	} `json:"page_patterns"`
	OwnerReview struct {
		State string `json:"state"`
	} `json:"owner_review"`
}

func BindingFromContext(packet json.RawMessage, repositoryIdentities []string) (*Binding, error) {
	if len(bytes.TrimSpace(packet)) == 0 {
		return nil, errors.New("approved product design pack is missing from development context")
	}
	var context contextPacket
	if err := json.Unmarshal(packet, &context); err != nil {
		return nil, errors.New("development context is not valid JSON")
	}
	wanted := map[string]struct{}{}
	for _, identity := range repositoryIdentities {
		if normalized := normalizeRepositoryIdentity(identity); normalized != "" {
			wanted[normalized] = struct{}{}
		}
	}
	matches := make([]contextPack, 0, len(context.DesignPacks))
	for _, candidate := range context.DesignPacks {
		if _, ok := wanted[normalizeRepositoryIdentity(candidate.RepositoryIdentity)]; ok {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, errors.New("approved product design pack is missing for the task repository")
	}
	if len(matches) > 1 {
		identities := make([]string, 0, len(matches))
		for _, match := range matches {
			identities = append(identities, match.RepositoryIdentity+":"+match.ProductID)
		}
		sort.Strings(identities)
		return nil, fmt.Errorf("design task matches multiple product packs: %s", strings.Join(identities, ", "))
	}
	match := matches[0]
	var pack productPack
	if err := json.Unmarshal(match.Pack, &pack); err != nil {
		return nil, errors.New("approved product design pack payload is invalid")
	}
	canonical, err := canonicalJSON(match.Pack)
	if err != nil {
		return nil, errors.New("approved product design pack payload is invalid")
	}
	digest := sha256.Sum256(canonical)
	actualSHA := hex.EncodeToString(digest[:])
	checks := []struct{ name, actual, expected string }{
		{"product id", match.ProductID, pack.Product.ID},
		{"repository identity", normalizeRepositoryIdentity(match.RepositoryIdentity), normalizeRepositoryIdentity(pack.Product.RepositoryIdentity)},
		{"source revision", match.SourceRevision, pack.Product.SourceRevision},
		{"approval state", match.ApprovalState, pack.OwnerReview.State},
		{"pack hash", match.SHA256, actualSHA},
	}
	for _, check := range checks {
		if check.actual == "" || check.actual != check.expected {
			return nil, fmt.Errorf("design pack %s mismatch", check.name)
		}
	}
	if match.SchemaVersion < 1 || match.SchemaVersion != pack.SchemaVersion {
		return nil, errors.New("design pack schema version mismatch")
	}
	if match.ApprovalState != "approved" {
		return nil, errors.New("design pack is not owner approved")
	}
	if !revisionPattern.MatchString(match.SourceRevision) || !sha256Pattern.MatchString(match.SHA256) {
		return nil, errors.New("design pack revision or hash is malformed")
	}
	return &Binding{
		SchemaVersion: 1, ProductID: match.ProductID,
		RepositoryIdentity: normalizeRepositoryIdentity(match.RepositoryIdentity),
		PackSchemaVersion:  match.SchemaVersion, SourceRevision: match.SourceRevision,
		PackSHA256: match.SHA256, ApprovalState: match.ApprovalState,
		Pack: append(json.RawMessage(nil), match.Pack...),
	}, nil
}

func BindingFromTaskContext(context json.RawMessage) (*Binding, error) {
	context = bytes.TrimSpace(context)
	if len(context) == 0 || bytes.Equal(context, []byte("null")) {
		return nil, nil
	}
	var wrapper struct {
		Binding json.RawMessage `json:"design_delivery_v1"`
	}
	if err := json.Unmarshal(context, &wrapper); err != nil {
		return nil, errors.New("task context is invalid JSON")
	}
	if len(bytes.TrimSpace(wrapper.Binding)) == 0 || bytes.Equal(bytes.TrimSpace(wrapper.Binding), []byte("null")) {
		return nil, nil
	}
	var binding Binding
	if err := json.Unmarshal(wrapper.Binding, &binding); err != nil {
		return nil, errors.New("design delivery task binding is invalid")
	}
	if binding.SchemaVersion != 1 || binding.ProductID == "" || binding.RepositoryIdentity == "" ||
		binding.PackSchemaVersion < 1 || !revisionPattern.MatchString(binding.SourceRevision) ||
		!sha256Pattern.MatchString(binding.PackSHA256) || binding.ApprovalState != "approved" ||
		len(bytes.TrimSpace(binding.Pack)) == 0 {
		return nil, errors.New("design delivery task binding is incomplete")
	}
	return &binding, nil
}

type Delivery struct {
	SchemaVersion int    `json:"schema_version"`
	CreatedAt     string `json:"created_at"`
	Task          struct {
		ID                 string `json:"id"`
		ProductID          string `json:"product_id"`
		RequestSummary     string `json:"request_summary"`
		TargetSurface      string `json:"target_surface"`
		RepresentedRole    string `json:"represented_role"`
		RepositoryIdentity string `json:"repository_identity"`
		RepositoryRevision string `json:"repository_revision"`
	} `json:"task"`
	DesignPack struct {
		ProductID             string `json:"product_id"`
		SchemaVersion         int    `json:"schema_version"`
		SourceRevision        string `json:"source_revision"`
		SHA256                string `json:"sha256"`
		ApprovalState         string `json:"approval_state"`
		ValidatedWithoutDrift bool   `json:"validated_without_drift"`
	} `json:"design_pack"`
	SourceReview struct {
		FilesInspected     []string `json:"files_inspected"`
		WorkflowSummary    string   `json:"workflow_summary"`
		DecisionsSupported []string `json:"decisions_supported"`
		AvailableData      []string `json:"available_data"`
		UnavailableData    []string `json:"unavailable_data"`
		PermissionsChecked bool     `json:"permissions_checked"`
	} `json:"source_review"`
	Reuse struct {
		ShellReused    bool     `json:"shell_reused"`
		ShellSourceIDs []string `json:"shell_source_ids"`
		PagePatternRef string   `json:"page_pattern_ref"`
		ComponentRefs  []string `json:"component_refs"`
		NewComponents  []struct {
			Name             string `json:"name"`
			Reason           string `json:"reason"`
			ProposedContract string `json:"proposed_contract"`
		} `json:"new_components"`
	} `json:"reuse"`
	Evidence struct {
		PreviewURL string `json:"preview_url"`
		Captures   []struct {
			Viewport  string `json:"viewport"`
			DataState string `json:"data_state"`
			Location  string `json:"location"`
			SHA256    string `json:"sha256"`
		} `json:"captures"`
		QualityChecks struct {
			Alignment         bool `json:"alignment"`
			Overflow          bool `json:"overflow"`
			FocusAndKeyboard  bool `json:"focus_and_keyboard"`
			InteractionStates bool `json:"interaction_states"`
			Copy              bool `json:"copy"`
			DataTruth         bool `json:"data_truth"`
		} `json:"quality_checks"`
	} `json:"evidence"`
	Reviews []struct {
		Role            string `json:"role"`
		Verdict         string `json:"verdict"`
		Summary         string `json:"summary"`
		EvidenceRef     string `json:"evidence_ref"`
		ReviewTaskID    string `json:"review_task_id"`
		ReviewerAgentID string `json:"reviewer_agent_id"`
		VerdictSHA256   string `json:"verdict_sha256"`
	} `json:"reviews"`
	OwnerDelivery struct {
		State         string `json:"state"`
		Summary       string `json:"summary"`
		OpenDecisions []struct {
			Question     string `json:"question"`
			WhyItMatters string `json:"why_it_matters"`
		} `json:"open_decisions"`
	} `json:"owner_delivery"`
}

type ReviewVerdict struct {
	SchemaVersion int      `json:"schema_version"`
	Role          string   `json:"role"`
	Verdict       string   `json:"verdict"`
	Summary       string   `json:"summary"`
	Standards     []string `json:"standards_applied"`
	CaptureHashes []string `json:"reviewed_capture_sha256"`
	Criteria      []struct {
		ID           string   `json:"id"`
		Status       string   `json:"status"`
		Severity     string   `json:"severity"`
		Rationale    string   `json:"rationale"`
		EvidenceRefs []string `json:"evidence_refs"`
	} `json:"criteria"`
	AbsoluteBar struct {
		ProfessionalStandardMet  bool   `json:"professional_standard_met"`
		NotOnlyBetterThanCurrent bool   `json:"not_only_better_than_current"`
		TaskSpecificRationale    bool   `json:"task_specific_rationale_present"`
		NoUnresolvedHighSeverity bool   `json:"no_unresolved_high_severity_findings"`
		Summary                  string `json:"summary"`
	} `json:"absolute_bar"`
	BlockingFindings []string `json:"blocking_findings"`
	RequiredChanges  []string `json:"required_changes"`
}

var requiredReviewCriteria = map[string][]string{
	"product_designer": {
		"workflow_fit", "information_architecture", "interaction_design", "responsive_composition",
		"accessibility_usability", "content_clarity", "cognitive_load_progressive_disclosure", "edge_state_coherence",
	},
	"data_visualization_designer": {
		"decision_relevance", "data_truth_provenance", "metric_semantics", "comparison_integrity",
		"visual_encoding", "density_scanability", "missing_zero_unknown", "state_coverage",
	},
	"creative_director": {
		"visual_hierarchy", "brand_product_specificity", "shell_component_fidelity", "composition_balance",
		"typography_spacing", "color_emphasis_restraint", "polish_consistency", "owner_readiness",
	},
}

func ValidateCompletion(output string, binding Binding) error {
	_, err := ValidatedCompletion(output, binding)
	return err
}

func ValidatedCompletion(output string, binding Binding) (*Delivery, error) {
	raw, err := extractSingleBlock(output)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var delivery Delivery
	if err := decoder.Decode(&delivery); err != nil {
		return nil, fmt.Errorf("design delivery JSON does not match the required contract: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, err
	}
	if delivery.SchemaVersion != 2 || binding.SchemaVersion != 1 {
		return nil, errors.New("design delivery schema version must be 2")
	}
	if _, parseErr := time.Parse(time.RFC3339, delivery.CreatedAt); parseErr != nil {
		return nil, errors.New("design delivery created_at must be an RFC3339 timestamp")
	}
	if !identifierPattern.MatchString(delivery.Task.ID) ||
		strings.TrimSpace(delivery.Task.RequestSummary) == "" ||
		strings.TrimSpace(delivery.Task.TargetSurface) == "" ||
		strings.TrimSpace(delivery.Task.RepresentedRole) == "" {
		return nil, errors.New("design delivery task identity and scope are incomplete")
	}
	if delivery.Task.ProductID != binding.ProductID || delivery.DesignPack.ProductID != binding.ProductID {
		return nil, errors.New("design delivery product does not match the bound pack")
	}
	if normalizeRepositoryIdentity(delivery.Task.RepositoryIdentity) != binding.RepositoryIdentity {
		return nil, errors.New("design delivery repository does not match the bound pack")
	}
	if !revisionPattern.MatchString(delivery.Task.RepositoryRevision) {
		return nil, errors.New("design delivery repository revision must be a full Git SHA")
	}
	if delivery.DesignPack.SchemaVersion != binding.PackSchemaVersion ||
		delivery.DesignPack.SourceRevision != binding.SourceRevision ||
		delivery.DesignPack.SHA256 != binding.PackSHA256 ||
		delivery.DesignPack.ApprovalState != "approved" || !delivery.DesignPack.ValidatedWithoutDrift {
		return nil, errors.New("design delivery did not prove the exact approved drift-free pack")
	}
	if !delivery.SourceReview.PermissionsChecked || !nonEmptyUnique(delivery.SourceReview.FilesInspected) ||
		strings.TrimSpace(delivery.SourceReview.WorkflowSummary) == "" ||
		len(delivery.SourceReview.DecisionsSupported) == 0 || len(delivery.SourceReview.DecisionsSupported) > 3 ||
		!allNonEmpty(delivery.SourceReview.DecisionsSupported) || !allNonEmpty(delivery.SourceReview.AvailableData) ||
		len(delivery.SourceReview.AvailableData) == 0 || !allNonEmpty(delivery.SourceReview.UnavailableData) {
		return nil, errors.New("design delivery source and permission review is incomplete")
	}
	if !delivery.Reuse.ShellReused || !identifierList(delivery.Reuse.ShellSourceIDs) ||
		!identifierList(delivery.Reuse.ComponentRefs) || !identifierPattern.MatchString(delivery.Reuse.PagePatternRef) {
		return nil, errors.New("design delivery did not prove reuse of the real shell and pack patterns")
	}
	for _, component := range delivery.Reuse.NewComponents {
		if strings.TrimSpace(component.Name) == "" || strings.TrimSpace(component.Reason) == "" || strings.TrimSpace(component.ProposedContract) == "" {
			return nil, errors.New("design delivery contains an incomplete new component proposal")
		}
	}
	parsedURL, urlErr := url.Parse(delivery.Evidence.PreviewURL)
	if urlErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("design delivery preview_url must be browser-openable over HTTP or HTTPS")
	}
	if len(delivery.Evidence.Captures) < 4 {
		return nil, errors.New("design delivery requires at least four viewport captures")
	}
	allowedViewports := map[string]bool{"phone": true, "medium": true, "desktop_1440": true, "desktop_1920": true}
	allowedStates := map[string]bool{"minimum": true, "typical": true, "maximum": true}
	viewports, states, captureHashes := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, capture := range delivery.Evidence.Captures {
		if strings.TrimSpace(capture.Location) == "" || !sha256Pattern.MatchString(capture.SHA256) {
			return nil, errors.New("design delivery capture evidence is incomplete")
		}
		if !allowedViewports[capture.Viewport] || !allowedStates[capture.DataState] {
			return nil, errors.New("design delivery capture uses an unsupported viewport or data state")
		}
		if captureHashes[capture.SHA256] {
			return nil, errors.New("design delivery capture evidence must use distinct screenshots")
		}
		captureHashes[capture.SHA256] = true
		viewports[capture.Viewport] = true
		states[capture.DataState] = true
	}
	for _, required := range []string{"phone", "medium", "desktop_1440", "desktop_1920"} {
		if !viewports[required] {
			return nil, fmt.Errorf("design delivery is missing the %s viewport", required)
		}
	}
	for _, required := range []string{"minimum", "typical", "maximum"} {
		if !states[required] {
			return nil, fmt.Errorf("design delivery is missing the %s data state", required)
		}
	}
	checks := delivery.Evidence.QualityChecks
	if !checks.Alignment || !checks.Overflow || !checks.FocusAndKeyboard || !checks.InteractionStates || !checks.Copy || !checks.DataTruth {
		return nil, errors.New("design delivery has one or more failed quality checks")
	}
	if len(delivery.Reviews) != 3 {
		return nil, errors.New("design delivery requires exactly three independent ship reviews")
	}
	allowedReviews := map[string]bool{"product_designer": true, "data_visualization_designer": true, "creative_director": true}
	reviewed, reviewTasks, reviewerAgents := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, review := range delivery.Reviews {
		if !allowedReviews[review.Role] {
			return nil, fmt.Errorf("design delivery review role %s is unsupported", review.Role)
		}
		if review.Verdict != "ship" || strings.TrimSpace(review.Summary) == "" || strings.TrimSpace(review.EvidenceRef) == "" ||
			strings.TrimSpace(review.ReviewTaskID) == "" || strings.TrimSpace(review.ReviewerAgentID) == "" ||
			!sha256Pattern.MatchString(review.VerdictSHA256) {
			return nil, fmt.Errorf("design delivery review %s is incomplete or non-ship", review.Role)
		}
		if reviewed[review.Role] {
			return nil, fmt.Errorf("design delivery review role %s is duplicated", review.Role)
		}
		if reviewTasks[review.ReviewTaskID] || reviewerAgents[review.ReviewerAgentID] {
			return nil, errors.New("design delivery requires three distinct reviewer tasks and reviewer agents")
		}
		reviewed[review.Role] = true
		reviewTasks[review.ReviewTaskID] = true
		reviewerAgents[review.ReviewerAgentID] = true
	}
	for _, required := range []string{"product_designer", "data_visualization_designer", "creative_director"} {
		if !reviewed[required] {
			return nil, fmt.Errorf("design delivery is missing the %s review", required)
		}
	}
	if delivery.OwnerDelivery.State != "ready" || strings.TrimSpace(delivery.OwnerDelivery.Summary) == "" {
		return nil, errors.New("design delivery is not owner-ready")
	}
	for _, decision := range delivery.OwnerDelivery.OpenDecisions {
		if strings.TrimSpace(decision.Question) == "" || strings.TrimSpace(decision.WhyItMatters) == "" {
			return nil, errors.New("design delivery contains an incomplete owner decision")
		}
	}
	if err := validatePackReferences(delivery, binding); err != nil {
		return nil, err
	}
	return &delivery, nil
}

// ValidatedReviewVerdict proves that one independent reviewer applied the
// complete role-specific rubric to the same screenshots delivered to the
// owner. It intentionally rejects a generic "looks good" result, a numeric
// score without reasons, and "better than current" as the only quality bar.
func ValidatedReviewVerdict(output, expectedRole string, expectedCaptureHashes []string) (*ReviewVerdict, error) {
	raw, err := extractSingleTaggedBlock(output, ReviewOpenTag, ReviewCloseTag)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var verdict ReviewVerdict
	if err := decoder.Decode(&verdict); err != nil {
		return nil, fmt.Errorf("design review JSON does not match the required contract: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, err
	}
	required, ok := requiredReviewCriteria[expectedRole]
	if !ok || verdict.SchemaVersion != 1 || verdict.Role != expectedRole {
		return nil, errors.New("design review role or schema does not match the assigned lens")
	}
	if verdict.Verdict != "ship" || strings.TrimSpace(verdict.Summary) == "" || len(verdict.BlockingFindings) != 0 || len(verdict.RequiredChanges) != 0 {
		return nil, errors.New("design review is not an unblocked ship verdict")
	}
	if len(verdict.Standards) < 2 || !nonEmptyUnique(verdict.Standards) {
		return nil, errors.New("design review must name at least two applied professional standards")
	}
	bar := verdict.AbsoluteBar
	if !bar.ProfessionalStandardMet || !bar.NotOnlyBetterThanCurrent || !bar.TaskSpecificRationale || !bar.NoUnresolvedHighSeverity || strings.TrimSpace(bar.Summary) == "" {
		return nil, errors.New("design review did not prove the absolute professional quality bar")
	}

	expectedCaptures := make(map[string]bool, len(expectedCaptureHashes))
	for _, hash := range expectedCaptureHashes {
		if !sha256Pattern.MatchString(hash) {
			return nil, errors.New("design delivery contains an invalid capture hash")
		}
		expectedCaptures[hash] = true
	}
	if len(verdict.CaptureHashes) != len(expectedCaptures) || !nonEmptyUnique(verdict.CaptureHashes) {
		return nil, errors.New("design review did not inspect every delivered capture exactly once")
	}
	for _, hash := range verdict.CaptureHashes {
		if !expectedCaptures[hash] {
			return nil, errors.New("design review references a screenshot outside the delivered evidence")
		}
	}

	criteria := make(map[string]bool, len(verdict.Criteria))
	for _, item := range verdict.Criteria {
		if criteria[item.ID] {
			return nil, fmt.Errorf("design review criterion %s is duplicated", item.ID)
		}
		criteria[item.ID] = true
		if item.Status != "pass" || item.Severity != "none" || len(strings.TrimSpace(item.Rationale)) < 24 || !nonEmptyUnique(item.EvidenceRefs) {
			return nil, fmt.Errorf("design review criterion %s lacks a passing evidence-backed rationale", item.ID)
		}
		hasDeliveredCapture := false
		for _, ref := range item.EvidenceRefs {
			if expectedCaptures[ref] {
				hasDeliveredCapture = true
				break
			}
		}
		if !hasDeliveredCapture {
			return nil, fmt.Errorf("design review criterion %s is not anchored to a delivered screenshot", item.ID)
		}
	}
	if len(criteria) != len(required) {
		return nil, errors.New("design review does not contain the complete role-specific rubric")
	}
	for _, criterion := range required {
		if !criteria[criterion] {
			return nil, fmt.Errorf("design review is missing criterion %s", criterion)
		}
	}
	return &verdict, nil
}

func allNonEmpty(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func nonEmptyUnique(values []string) bool {
	if len(values) == 0 || !allNonEmpty(values) {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func identifierList(values []string) bool {
	if !nonEmptyUnique(values) {
		return false
	}
	for _, value := range values {
		if !identifierPattern.MatchString(value) {
			return false
		}
	}
	return true
}

func validatePackReferences(delivery Delivery, binding Binding) error {
	var pack productPack
	if json.Unmarshal(binding.Pack, &pack) != nil {
		return errors.New("bound design pack payload is invalid")
	}
	sources, components, patterns := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, item := range pack.Sources {
		sources[item.ID] = true
	}
	for _, item := range pack.ComponentLibrary {
		components[item.ID] = true
	}
	for _, item := range pack.PagePatterns {
		patterns[item.ID] = true
	}
	for _, id := range delivery.Reuse.ShellSourceIDs {
		if !sources[id] {
			return fmt.Errorf("design delivery references unknown shell source %s", id)
		}
	}
	for _, id := range delivery.Reuse.ComponentRefs {
		if !components[id] {
			return fmt.Errorf("design delivery references unknown component %s", id)
		}
	}
	if !patterns[delivery.Reuse.PagePatternRef] {
		return fmt.Errorf("design delivery references unknown page pattern %s", delivery.Reuse.PagePatternRef)
	}
	return nil
}

func extractSingleBlock(output string) ([]byte, error) {
	return extractSingleTaggedBlock(output, OpenTag, CloseTag)
}

func extractSingleTaggedBlock(output, openTag, closeTag string) ([]byte, error) {
	start := strings.Index(output, openTag)
	if start < 0 {
		return nil, fmt.Errorf("completed design work must include one %s JSON block", openTag)
	}
	if strings.Index(output[start+len(openTag):], openTag) >= 0 {
		return nil, errors.New("design delivery contains more than one opening block")
	}
	contentStart := start + len(openTag)
	endOffset := strings.Index(output[contentStart:], closeTag)
	if endOffset < 0 {
		return nil, errors.New("design delivery closing tag is missing")
	}
	end := contentStart + endOffset
	if strings.Contains(output[end+len(closeTag):], closeTag) {
		return nil, errors.New("design delivery contains more than one closing block")
	}
	return []byte(strings.TrimSpace(output[contentStart:end])), nil
}

func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	// Intelligence canonical JSON preserves literal HTML punctuation. Go's
	// default encoder escapes &, < and >, which would give the same approved
	// pack a different cross-service identity.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buffer.Bytes()), nil
}

func requireEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("design delivery JSON contains multiple values")
		}
		return fmt.Errorf("design delivery JSON has trailing data: %w", err)
	}
	return nil
}

func normalizeRepositoryIdentity(value string) string {
	identity := strings.ToLower(strings.TrimSpace(value))
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "ssh://git@github.com/", "github.com/", "git@github.com:"} {
		identity = strings.TrimPrefix(identity, prefix)
	}
	return strings.Trim(strings.TrimSuffix(identity, ".git"), "/")
}
