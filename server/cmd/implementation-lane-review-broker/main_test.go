package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/laneevidence"
)

const (
	testIssue    = "11111111-1111-4111-8111-111111111111"
	testLeader   = "22222222-2222-4222-8222-222222222222"
	testWorker   = "33333333-3333-4333-8333-333333333333"
	testReviewer = "44444444-4444-4444-8444-444444444444"
	testTask     = "55555555-5555-4555-8555-555555555555"
)

func bind(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	state, runs := filepath.Join(root, "bridge"), filepath.Join(root, "runs")
	for _, dir := range []string{state, runs} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	laneIssueID, laneRoleIDs, reviewerAgentName = testIssue, testLeader+","+testWorker+","+testReviewer, "Lane Evidence Reviewer"
	stateDirectory, evidenceRoot, serverPort = state, runs, "18190"
	return state, runs
}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func reviewerEnv() map[string]string {
	return map[string]string{"MULTICA_ISSUE_ID": testIssue, "MULTICA_TASK_ID": testTask, "MULTICA_AGENT_NAME": "Lane Evidence Reviewer",
		"MULTICA_AGENT_ID": testReviewer, "MULTICA_WORKFLOW_TASK_KIND": "review", "MULTICA_TOKEN": "mat_test"}
}

func writeRecord(t *testing.T, state, runs string, outcome map[string]any) laneevidence.Record {
	t.Helper()
	dir := filepath.Join(runs, "issue-proof-run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	patch := []byte("diff --git a/packages/core/api/client.ts b/packages/core/api/client.ts\n")
	outcomeRaw, _ := json.Marshal(outcome)
	_ = os.WriteFile(filepath.Join(dir, "final.patch"), patch, 0o600)
	_ = os.WriteFile(filepath.Join(dir, "outcome.json"), outcomeRaw, 0o600)
	record := laneevidence.Record{IssueID: testIssue, TaskID: "issue-proof-list-agents", RunID: "run-1", EvidenceDir: dir,
		Repository: "C:/Dev/multica", PatchSHA256: laneevidence.Digest(patch), OutcomeSHA256: laneevidence.Digest(outcomeRaw), RecordedAt: time.Now().UTC()}
	raw, _ := json.Marshal(record)
	if err := os.WriteFile(laneevidence.RecordPath(state, testIssue), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return record
}

func passingOutcome() map[string]any {
	return map[string]any{"succeeded": true, "verification_passed": true, "reviewer_passed": true, "test_fails_before_change": true,
		"authority_revoked": true, "worktree_removed": true, "regression_passed": true, "lane_freeze": map[string]any{"intact": true}}
}

func TestOnlyTheBoundReviewersReviewTurnIsAdmitted(t *testing.T) {
	bind(t)
	roles, err := boundRoles()
	if err != nil {
		t.Fatalf("bound roles: %v", err)
	}
	if err := checkGrantEnvironment(env(reviewerEnv()), roles); err != nil {
		t.Fatalf("the bound reviewer was refused: %v", err)
	}
	for name, change := range map[string][2]string{
		"other issue":     {"MULTICA_ISSUE_ID", "66666666-6666-4666-8666-666666666666"},
		"implementer":     {"MULTICA_AGENT_ID", testWorker},
		"correction kind": {"MULTICA_WORKFLOW_TASK_KIND", "correction"},
		"leader kind":     {"MULTICA_WORKFLOW_TASK_KIND", "leader"},
		"other name":      {"MULTICA_AGENT_NAME", "Lane Implementer"},
		"member token":    {"MULTICA_TOKEN", "mul_owner"},
		"missing task":    {"MULTICA_TASK_ID", ""},
	} {
		values := reviewerEnv()
		values[change[0]] = change[1]
		if err := checkGrantEnvironment(env(values), roles); err == nil {
			t.Fatalf("%s was admitted", name)
		}
	}
}

func TestTheBrokerRefusesToRunWhenNotBoundToOneIssue(t *testing.T) {
	bind(t)
	laneRoleIDs = testLeader + "," + testReviewer + "," + testReviewer
	if _, err := boundRoles(); err == nil {
		t.Fatal("duplicate role ids were accepted")
	}
	bind(t)
	laneIssueID = ""
	if _, err := boundRoles(); err == nil {
		t.Fatal("an unbound issue was accepted")
	}
}

func TestLaneEvidenceIsReverifiedFromDisk(t *testing.T) {
	state, runs := bind(t)
	record := writeRecord(t, state, runs, passingOutcome())
	got, problems, patch := verifyLaneEvidence(state, testIssue, runs)
	if len(problems) != 0 || got.RunID != record.RunID || !strings.Contains(patch, "client.ts") {
		t.Fatalf("problems=%v run=%q patch=%q", problems, got.RunID, patch)
	}
	if err := os.WriteFile(filepath.Join(record.EvidenceDir, "final.patch"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, problems, patch = verifyLaneEvidence(state, testIssue, runs)
	if len(problems) == 0 || patch != "" {
		t.Fatalf("a tampered patch passed: problems=%v patch=%q", problems, patch)
	}
	_, problems, _ = verifyLaneEvidence(state, "77777777-7777-4777-8777-777777777777", runs)
	if len(problems) == 0 {
		t.Fatal("a missing lane record passed")
	}
}

func TestADeterministicFailureAlwaysFailsTheReview(t *testing.T) {
	record := laneevidence.Record{RunID: "run-1", PatchSHA256: strings.Repeat("a", 64)}
	pass := modelResult{Summary: "The change validates the list with the agent schema.", Verdict: "pass", Findings: []string{}}
	event, comment := decide(record, nil, pass)
	if event["event"] != "review_passed" || !strings.Contains(comment, "passed") {
		t.Fatalf("clean evidence and a pass did not pass: %v", event)
	}
	event, _ = decide(record, []string{"gate not passed: reviewer_passed"}, pass)
	if event["event"] != "review_failed" {
		t.Fatalf("a deterministic failure passed: %v", event)
	}
	correction, _ := event["correction"].(map[string]any)
	for _, key := range []string{"failed_criteria", "evidence", "required_correction"} {
		if correction[key] == nil {
			t.Fatalf("review_failed is missing %s", key)
		}
	}
	fail := modelResult{Summary: "The test never calls the client method.", Verdict: "fail", Findings: []string{"test bypasses ApiClient.listAgents"}}
	event, comment = decide(record, nil, fail)
	if event["event"] != "review_failed" || !strings.Contains(comment, "ApiClient.listAgents") {
		t.Fatalf("a model failure did not fail with its finding: %v %q", event, comment)
	}
}

func TestOnlyTheStrictReviewResultIsAccepted(t *testing.T) {
	if _, err := parseResult("```json\n{\"summary\":\"The change is correct and tested.\",\"verdict\":\"pass\",\"findings\":[]}\n```"); err != nil {
		t.Fatalf("a fenced strict result was refused: %v", err)
	}
	for name, text := range map[string]string{
		"extra field":         `{"summary":"The change is correct and tested.","verdict":"pass","findings":[],"tools":["bash"]}`,
		"pass with findings":  `{"summary":"The change is correct and tested.","verdict":"pass","findings":["x"]}`,
		"fail without reason": `{"summary":"The change is not correct at all.","verdict":"fail","findings":[]}`,
		"routing":             `{"summary":"Ask mention://agent/1 to redo this.","verdict":"fail","findings":["x"]}`,
		"two results":         `{"summary":"The change is correct and tested.","verdict":"pass","findings":[]}{}`,
		"oversized":           `{"summary":"` + strings.Repeat("a", 700) + `","verdict":"pass","findings":[]}`,
		"not json":            "Looks good to me.",
	} {
		if _, err := parseResult(text); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestTheBrokerAPISurfaceIsThreePaths(t *testing.T) {
	bind(t)
	for _, path := range []string{"/api/issues/" + testIssue + "/task-runs?active=true", "/api/issues/" + testIssue + "/workflow-events", "/api/issues/" + testIssue + "/comments"} {
		if !allowedBrokerPath(path) {
			t.Fatalf("%s was refused", path)
		}
	}
	for _, path := range []string{"/api/issues/" + testIssue, "/api/issues/66666666-6666-4666-8666-666666666666/workflow-events", "/api/agents", "/api/issues/" + testIssue + "/workflow-plan"} {
		if allowedBrokerPath(path) {
			t.Fatalf("%s was allowed", path)
		}
	}
}

func TestThePromptCarriesVerificationAndBoundsThePatch(t *testing.T) {
	record := laneevidence.Record{RunID: "run-1", PatchSHA256: strings.Repeat("b", 64)}
	spec := laneTaskSpec{TaskID: "issue-proof-list-agents", AllowedPaths: []string{"packages/core/api/"}, Objective: "Validate listAgents."}
	prompt := reviewPrompt(record, []string{"final.patch is missing or changed since publication"}, strings.Repeat("x", maxPatchPromptBytes+500), spec)
	if !strings.Contains(prompt, "deterministic_verification\":\"fail: final.patch") || !strings.Contains(prompt, "truncated") {
		t.Fatal("the prompt does not carry the failed verification and truncation notice")
	}
	if strings.Count(prompt, "x") > maxPatchPromptBytes+50 {
		t.Fatal("the patch was not bounded")
	}
	if !strings.Contains(prompt, "no tools") {
		t.Fatal("the prompt does not state the tool-free boundary")
	}
}

func TestTheProviderSlotCanBeReservedOnlyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), testIssue+".review.consumed.json")
	if err := writeNew(path, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := writeNew(path, []byte("{}")); err == nil {
		t.Fatal("a consumed review slot was reserved again")
	}
}

func TestTheLoopbackServerIsBoundToItsPort(t *testing.T) {
	bind(t)
	t.Setenv("MULTICA_SERVER_URL", "http://127.0.0.1:18190")
	if _, err := requireLoopbackServer(); err != nil {
		t.Fatalf("the bound loopback server was refused: %v", err)
	}
	for _, value := range []string{"http://127.0.0.1:8080", "https://127.0.0.1:18190", "http://example.com:18190", "http://127.0.0.1:18190/api"} {
		t.Setenv("MULTICA_SERVER_URL", value)
		if _, err := requireLoopbackServer(); err == nil {
			t.Fatalf("%s was accepted", value)
		}
	}
}
