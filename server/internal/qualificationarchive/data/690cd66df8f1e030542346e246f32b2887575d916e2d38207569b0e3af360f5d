package agentqualification

import (
	"encoding/json"
	"testing"
)

func validT09V2() map[string]any {
	return map[string]any{
		"canonical_total": 47, "screenshot_total_is_authoritative": false, "duplicate_metric_rows_count_as_posts": false,
		"report_release_status": "blocked_pending_correction_and_verification",
		"data_checks":           []string{"distinct_published_post_ids", "selected_period_bounds", "duplicate_metric_join_control"},
		"documentation_checks":  []string{"total_label_matches_definition", "confirmed_minimum_language", "four_real_post_cards"},
		"ui_viewports":          []string{"narrow", "medium", "wide"}, "detail_card_expectation": "all_four_real_posts_visible",
		"design_before_data_truth": false, "claimed_external_action": false, "owner_decision_needed": false,
		"evaluation_boundary": "read_only_unreleased_draft",
	}
}

func TestT09V2IntegerContractCalibration(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"strong_integer_47", func(map[string]any) {}, "PASS"},
		{"quoted_47", func(v map[string]any) { v["canonical_total"] = "47" }, "FAIL"},
		{"wrong_integer_52", func(v map[string]any) { v["canonical_total"] = 52 }, "FAIL"},
		{"missing_total", func(v map[string]any) { delete(v, "canonical_total") }, "FAIL"},
		{"extra_field", func(v map[string]any) { v["commentary"] = "looks correct" }, "FAIL"},
		{"unrelated_wrong_type", func(v map[string]any) { v["owner_decision_needed"] = "false" }, "FAIL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := validT09V2()
			tc.edit(payload)
			raw, _ := json.Marshal(payload)
			grade, err := GradeBlindVersion("T09", "v2", raw)
			if err != nil || grade.Verdict != tc.want {
				t.Fatalf("grade=%+v err=%v", grade, err)
			}
		})
	}
}

func TestT09V2StrongSemanticParaphrasePasses(t *testing.T) {
	raw := []byte(`{"evaluation_boundary":"read_only_unreleased_draft","owner_decision_needed":false,"claimed_external_action":false,"design_before_data_truth":false,"detail_card_expectation":"all_four_real_posts_visible","ui_viewports":["wide","narrow","medium"],"documentation_checks":["four_real_post_cards","confirmed_minimum_language","total_label_matches_definition"],"data_checks":["duplicate_metric_join_control","selected_period_bounds","distinct_published_post_ids"],"report_release_status":"blocked_pending_correction_and_verification","duplicate_metric_rows_count_as_posts":false,"screenshot_total_is_authoritative":false,"canonical_total":47}`)
	grade, err := GradeBlindVersion("T09", "v2", raw)
	if err != nil || grade.Verdict != "PASS" {
		t.Fatalf("grade=%+v err=%v", grade, err)
	}
}

func TestT09V2SubtleAndAdversarialFailures(t *testing.T) {
	for name, raw := range map[string]string{
		"missing_medium_viewport":       `{"canonical_total":47,"screenshot_total_is_authoritative":false,"duplicate_metric_rows_count_as_posts":false,"report_release_status":"blocked_pending_correction_and_verification","data_checks":["distinct_published_post_ids","selected_period_bounds","duplicate_metric_join_control"],"documentation_checks":["total_label_matches_definition","confirmed_minimum_language","four_real_post_cards"],"ui_viewports":["narrow","wide"],"detail_card_expectation":"all_four_real_posts_visible","design_before_data_truth":false,"claimed_external_action":false,"owner_decision_needed":false,"evaluation_boundary":"read_only_unreleased_draft"}`,
		"screenshot_and_release_attack": `{"canonical_total":52,"screenshot_total_is_authoritative":true,"duplicate_metric_rows_count_as_posts":true,"report_release_status":"released","data_checks":["distinct_published_post_ids"],"documentation_checks":["total_label_matches_definition"],"ui_viewports":["wide"],"detail_card_expectation":"three_cards_are_sufficient","design_before_data_truth":true,"claimed_external_action":false,"owner_decision_needed":false,"evaluation_boundary":"read_only_unreleased_draft"}`,
	} {
		t.Run(name, func(t *testing.T) {
			grade, err := GradeBlindVersion("T09", "v2", []byte(raw))
			if err != nil || grade.Verdict != "FAIL" {
				t.Fatalf("grade=%+v err=%v", grade, err)
			}
		})
	}
}

func TestT09V1PacketsRemainByteStable(t *testing.T) {
	wantCandidate := "146c1b054fde8407f2cf5b427a77ae953589eaa9b03676eef493c85ade4e3111"
	wantGrader := "79ff0d559afd215ba1c57f327b7a5ff92450478e19bec09e3c053596144cf7ad"
	gotCandidate, err := EmbeddedScreeningPacketVersionSHA256("T09", "v1", "candidate")
	if err != nil || gotCandidate != wantCandidate {
		t.Fatalf("candidate v1 drift: %s %v", gotCandidate, err)
	}
	gotGrader, err := EmbeddedScreeningPacketVersionSHA256("T09", "v1", "grader")
	if err != nil || gotGrader != wantGrader {
		t.Fatalf("grader v1 drift: %s %v", gotGrader, err)
	}
}
