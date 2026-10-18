package designgate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

const (
	testRepo     = "a9shots0/smcrm"
	testRevision = "1111111111111111111111111111111111111111"
	testTaskRev  = "2222222222222222222222222222222222222222"
	testHash     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestBindingFromContextBindsExactApprovedPack(t *testing.T) {
	packet, wantSHA := testContextPacket(t, false)

	binding, err := BindingFromContext(packet, []string{"https://github.com/A9shots0/SMCrm.git"})
	if err != nil {
		t.Fatalf("BindingFromContext() error = %v", err)
	}
	if binding.ProductID != "crm" || binding.RepositoryIdentity != testRepo || binding.PackSHA256 != wantSHA {
		t.Fatalf("unexpected binding: %#v", binding)
	}
	if binding.SourceRevision != testRevision || binding.ApprovalState != "approved" {
		t.Fatalf("binding did not preserve approved revision: %#v", binding)
	}
}

func TestCanonicalJSONMatchesIntelligenceAcrossHTMLPunctuation(t *testing.T) {
	raw := json.RawMessage(`{"name":"Product & UI","rule":"a < b > c","schema_version":1}`)
	canonical, err := canonicalJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	if got, want := hex.EncodeToString(digest[:]), "80deeac8c4593f8d4a3f11b5c958ed5f379cab4a0c7d42eea7009e0980f08ac1"; got != want {
		t.Fatalf("canonical cross-service hash = %s, want %s (canonical JSON %s)", got, want, canonical)
	}
}

func TestBindingFromContextFailsClosed(t *testing.T) {
	packet, _ := testContextPacket(t, false)

	t.Run("missing repository", func(t *testing.T) {
		if _, err := BindingFromContext(packet, []string{"a9shots0/other"}); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("expected missing-pack error, got %v", err)
		}
	})

	t.Run("hash drift", func(t *testing.T) {
		packet, _ := testContextPacket(t, true)
		if _, err := BindingFromContext(packet, []string{testRepo}); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
			t.Fatalf("expected hash-mismatch error, got %v", err)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		var decoded map[string]any
		if err := json.Unmarshal(packet, &decoded); err != nil {
			t.Fatal(err)
		}
		packs := decoded["design_packs"].([]any)
		decoded["design_packs"] = append(packs, packs[0])
		ambiguous, _ := json.Marshal(decoded)
		if _, err := BindingFromContext(ambiguous, []string{testRepo}); err == nil || !strings.Contains(err.Error(), "multiple") {
			t.Fatalf("expected ambiguous-pack error, got %v", err)
		}
	})
}

func TestBindingFromTaskContextRoundTrips(t *testing.T) {
	packet, _ := testContextPacket(t, false)
	binding, err := BindingFromContext(packet, []string{testRepo})
	if err != nil {
		t.Fatal(err)
	}
	context, _ := json.Marshal(map[string]any{"design_delivery_v1": binding, "preserved": true})

	got, err := BindingFromTaskContext(context)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PackSHA256 != binding.PackSHA256 || got.ProductID != "crm" {
		t.Fatalf("unexpected round-trip binding: %#v", got)
	}
}

// This optional acceptance test binds the real product pack without copying it
// into Multica. Release verification supplies the path owned by the product
// repository; ordinary package tests remain hermetic.
func TestExternalApprovedProductPackBindsWhenProvided(t *testing.T) {
	path := strings.TrimSpace(os.Getenv("MULTICA_DESIGN_PACK_ACCEPTANCE_PATH"))
	if path == "" {
		t.Skip("MULTICA_DESIGN_PACK_ACCEPTANCE_PATH is not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var pack productPack
	if err := json.Unmarshal(raw, &pack); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	packSHA := hex.EncodeToString(digest[:])
	if expected := strings.TrimSpace(os.Getenv("MULTICA_DESIGN_PACK_ACCEPTANCE_SHA256")); expected != "" && packSHA != expected {
		t.Fatalf("cross-service pack hash = %s, Intelligence reported %s", packSHA, expected)
	}
	packet, _ := json.Marshal(map[string]any{"design_packs": []any{map[string]any{
		"repository_identity": pack.Product.RepositoryIdentity,
		"product_id":          pack.Product.ID,
		"schema_version":      pack.SchemaVersion,
		"source_revision":     pack.Product.SourceRevision,
		"sha256":              packSHA,
		"approval_state":      pack.OwnerReview.State,
		"pack":                json.RawMessage(raw),
	}}})

	binding, err := BindingFromContext(packet, []string{pack.Product.RepositoryIdentity})
	if err != nil {
		t.Fatalf("real product pack did not bind: %v", err)
	}
	if binding.ProductID != pack.Product.ID || binding.PackSHA256 != packSHA || binding.SourceRevision != pack.Product.SourceRevision {
		t.Fatalf("real pack binding lost identity: %#v", binding)
	}
}

func TestValidateCompletionAcceptsContractBoundToPack(t *testing.T) {
	binding := testBinding(t)
	delivery := validDelivery(t, binding)

	if err := ValidateCompletion("Owner summary\n"+OpenTag+string(delivery)+CloseTag, *binding); err != nil {
		t.Fatalf("ValidateCompletion() error = %v", err)
	}
}

func TestValidateCompletionRejectsUnreviewableResults(t *testing.T) {
	binding := testBinding(t)

	tests := []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{name: "missing receipt", want: "must include"},
		{name: "wrong pack hash", edit: func(v map[string]any) { nested(v, "design_pack")["sha256"] = testHash }, want: "exact approved"},
		{name: "shell redrawn", edit: func(v map[string]any) { nested(v, "reuse")["shell_reused"] = false }, want: "real shell"},
		{name: "unknown component", edit: func(v map[string]any) { nested(v, "reuse")["component_refs"] = []any{"invented.card"} }, want: "unknown component"},
		{name: "missing desktop proof", edit: func(v map[string]any) {
			evidence := nested(v, "evidence")
			captures := evidence["captures"].([]any)
			evidence["captures"] = captures[:3]
		}, want: "at least four"},
		{name: "reused screenshot evidence", edit: func(v map[string]any) {
			captures := nested(v, "evidence")["captures"].([]any)
			captures[1].(map[string]any)["sha256"] = captures[0].(map[string]any)["sha256"]
		}, want: "distinct screenshots"},
		{name: "failed visual check", edit: func(v map[string]any) { nested(nested(v, "evidence"), "quality_checks")["overflow"] = false }, want: "failed quality"},
		{name: "missing independent review", edit: func(v map[string]any) {
			v["reviews"] = v["reviews"].([]any)[:2]
		}, want: "exactly three"},
		{name: "review provenance omitted", edit: func(v map[string]any) {
			delete(v["reviews"].([]any)[0].(map[string]any), "review_task_id")
		}, want: "incomplete"},
		{name: "one reviewer impersonates three roles", edit: func(v map[string]any) {
			reviews := v["reviews"].([]any)
			reviews[1].(map[string]any)["review_task_id"] = reviews[0].(map[string]any)["review_task_id"]
		}, want: "distinct reviewer"},
		{name: "not owner ready", edit: func(v map[string]any) { nested(v, "owner_delivery")["state"] = "blocked" }, want: "not owner-ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.edit == nil {
				if err := ValidateCompletion("ordinary prose", *binding); err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected %q error, got %v", tt.want, err)
				}
				return
			}
			var value map[string]any
			if err := json.Unmarshal(validDelivery(t, binding), &value); err != nil {
				t.Fatal(err)
			}
			tt.edit(value)
			raw, _ := json.Marshal(value)
			err := ValidateCompletion(OpenTag+string(raw)+CloseTag, *binding)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestValidatedReviewVerdictRequiresProfessionalEvidence(t *testing.T) {
	hashes := []string{testHash, strings.Repeat("b", 64)}
	output := validReviewVerdict(t, "product_designer", hashes)
	if _, err := ValidatedReviewVerdict(output, "product_designer", hashes); err != nil {
		t.Fatalf("valid professional review rejected: %v", err)
	}

	var verdict map[string]any
	raw, err := extractSingleTaggedBlock(output, ReviewOpenTag, ReviewCloseTag)
	if err != nil || json.Unmarshal(raw, &verdict) != nil {
		t.Fatal(err)
	}
	verdict["criteria"] = verdict["criteria"].([]any)[:2]
	broken, _ := json.Marshal(verdict)
	if _, err := ValidatedReviewVerdict(ReviewOpenTag+string(broken)+ReviewCloseTag, "product_designer", hashes); err == nil || !strings.Contains(err.Error(), "complete") {
		t.Fatalf("incomplete rubric was accepted: %v", err)
	}

	if err := json.Unmarshal(raw, &verdict); err != nil {
		t.Fatal(err)
	}
	bar := verdict["absolute_bar"].(map[string]any)
	bar["not_only_better_than_current"] = false
	broken, _ = json.Marshal(verdict)
	if _, err := ValidatedReviewVerdict(ReviewOpenTag+string(broken)+ReviewCloseTag, "product_designer", hashes); err == nil || !strings.Contains(err.Error(), "absolute professional") {
		t.Fatalf("relative-only quality claim was accepted: %v", err)
	}

	if err := json.Unmarshal(raw, &verdict); err != nil {
		t.Fatal(err)
	}
	verdict["criteria"].([]any)[0].(map[string]any)["evidence_refs"] = []any{"generic-review-notes.md"}
	broken, _ = json.Marshal(verdict)
	if _, err := ValidatedReviewVerdict(ReviewOpenTag+string(broken)+ReviewCloseTag, "product_designer", hashes); err == nil || !strings.Contains(err.Error(), "delivered screenshot") {
		t.Fatalf("unanchored professional claim was accepted: %v", err)
	}
}

func testContextPacket(t *testing.T, corruptHash bool) (json.RawMessage, string) {
	t.Helper()
	pack := map[string]any{
		"schema_version": 1,
		"product": map[string]any{
			"id": "crm", "repository_identity": testRepo, "source_revision": testRevision,
		},
		"sources":           []any{map[string]any{"id": "authenticated.shell"}},
		"component_library": []any{map[string]any{"id": "data.table"}},
		"page_patterns":     []any{map[string]any{"id": "operational.index"}},
		"owner_review":      map[string]any{"state": "approved"},
	}
	packRaw, _ := json.Marshal(pack)
	digest := sha256.Sum256(packRaw)
	packSHA := hex.EncodeToString(digest[:])
	declaredSHA := packSHA
	if corruptHash {
		declaredSHA = testHash
	}
	packet, _ := json.Marshal(map[string]any{"design_packs": []any{map[string]any{
		"repository_identity": testRepo,
		"product_id":          "crm",
		"schema_version":      1,
		"source_revision":     testRevision,
		"sha256":              declaredSHA,
		"approval_state":      "approved",
		"pack":                pack,
	}}})
	return packet, packSHA
}

func testBinding(t *testing.T) *Binding {
	t.Helper()
	packet, _ := testContextPacket(t, false)
	binding, err := BindingFromContext(packet, []string{testRepo})
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func validDelivery(t *testing.T, binding *Binding) []byte {
	t.Helper()
	delivery := map[string]any{
		"schema_version": 2,
		"created_at":     "2026-10-03T14:00:00Z",
		"task": map[string]any{
			"id": "crm.clients.redesign", "product_id": "crm", "request_summary": "Improve the clients page",
			"target_surface": "Clients", "represented_role": "operator", "repository_identity": testRepo,
			"repository_revision": testTaskRev,
		},
		"design_pack": map[string]any{
			"product_id": "crm", "schema_version": 1, "source_revision": testRevision,
			"sha256": binding.PackSHA256, "approval_state": "approved", "validated_without_drift": true,
		},
		"source_review": map[string]any{
			"files_inspected": []any{"src/pages/Clients.tsx"}, "workflow_summary": "Operators compare client health.",
			"decisions_supported": []any{"Which client needs attention?"}, "available_data": []any{"client status"},
			"unavailable_data": []any{}, "permissions_checked": true,
		},
		"reuse": map[string]any{
			"shell_reused": true, "shell_source_ids": []any{"authenticated.shell"},
			"page_pattern_ref": "operational.index", "component_refs": []any{"data.table"}, "new_components": []any{},
		},
		"evidence": map[string]any{
			"preview_url": "http://127.0.0.1:8765/clients",
			"captures": []any{
				capture("phone", "minimum"), capture("medium", "typical"),
				capture("desktop_1440", "maximum"), capture("desktop_1920", "typical"),
			},
			"quality_checks": map[string]any{
				"alignment": true, "overflow": true, "focus_and_keyboard": true,
				"interaction_states": true, "copy": true, "data_truth": true,
			},
		},
		"reviews": []any{
			review("product_designer", 1), review("data_visualization_designer", 2), review("creative_director", 3),
		},
		"owner_delivery": map[string]any{"state": "ready", "summary": "Ready for owner review.", "open_decisions": []any{}},
	}
	raw, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func capture(viewport, state string) map[string]any {
	hashes := map[string]string{
		"phone": strings.Repeat("a", 64), "medium": strings.Repeat("b", 64),
		"desktop_1440": strings.Repeat("c", 64), "desktop_1920": strings.Repeat("d", 64),
	}
	return map[string]any{"viewport": viewport, "data_state": state, "location": "/tmp/" + viewport + ".png", "sha256": hashes[viewport]}
}

func review(role string, identity int) map[string]any {
	return map[string]any{
		"role": role, "verdict": "ship", "summary": "Independent review passed.", "evidence_ref": role + ".md",
		"review_task_id":    fmt.Sprintf("00000000-0000-7000-8000-%012d", identity),
		"reviewer_agent_id": fmt.Sprintf("00000000-0000-7000-9000-%012d", identity),
		"verdict_sha256":    testHash,
	}
}

func validReviewVerdict(t *testing.T, role string, hashes []string) string {
	t.Helper()
	criteria := make([]any, 0, len(requiredReviewCriteria[role]))
	for _, id := range requiredReviewCriteria[role] {
		criteria = append(criteria, map[string]any{
			"id": id, "status": "pass", "severity": "none",
			"rationale":     "The inspected screenshots and source provide task-specific evidence.",
			"evidence_refs": []any{hashes[0]},
		})
	}
	verdict := map[string]any{
		"schema_version": 1, "role": role, "verdict": "ship", "summary": "Meets the professional release bar.",
		"standards_applied":       []any{"WCAG 2.2", "Nielsen usability heuristics"},
		"reviewed_capture_sha256": hashes, "criteria": criteria,
		"absolute_bar": map[string]any{
			"professional_standard_met": true, "not_only_better_than_current": true,
			"task_specific_rationale_present": true, "no_unresolved_high_severity_findings": true,
			"summary": "The design stands on its own against product-specific professional criteria.",
		},
		"blocking_findings": []any{}, "required_changes": []any{},
	}
	raw, err := json.Marshal(verdict)
	if err != nil {
		t.Fatal(err)
	}
	return ReviewOpenTag + string(raw) + ReviewCloseTag
}

func nested(value map[string]any, key string) map[string]any {
	return value[key].(map[string]any)
}
