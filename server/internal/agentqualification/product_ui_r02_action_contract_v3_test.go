package agentqualification

import (
	"strings"
	"testing"
)

func TestProductUIR02ActionContractV3RunsCompleteFakeWorkflow(t *testing.T) {
	if _, err := CurrentProductUIR02ActionContractV3(); err != nil {
		t.Fatal(err)
	}
	actions := productUIR02ValidActionsV3()
	if err := ValidateProductUIR02ActionPlanV3(actions); err != nil {
		t.Fatal(err)
	}
	runFamily := "item14-product-ui-r02-v3-fake"
	workspace, err := PrepareRealisticCaseWorkspaceForRun(t.TempDir(), "R02", runFamily)
	if err != nil {
		t.Fatal(err)
	}
	browser := &scriptedBrowserCollector{proofs: []RealisticBrowserProof{
		{Viewport: "narrow", CaseID: "R02", RunFamilyID: runFamily, WorkspaceTreeSHA256: strings.Repeat("a", 64), RequestSHA256: strings.Repeat("b", 64), ScreenshotArtifactPath: "narrow.png", ScreenshotSHA256: strings.Repeat("c", 64), DOMSnapshotArtifactPath: "narrow.json", DOMSnapshotSHA256: strings.Repeat("d", 64), RenderedTotal: 47, RenderedCardCount: 4},
		{Viewport: "medium", CaseID: "R02", RunFamilyID: runFamily, WorkspaceTreeSHA256: strings.Repeat("a", 64), RequestSHA256: strings.Repeat("b", 64), ScreenshotArtifactPath: "medium.png", ScreenshotSHA256: strings.Repeat("e", 64), DOMSnapshotArtifactPath: "medium.json", DOMSnapshotSHA256: strings.Repeat("f", 64), RenderedTotal: 47, RenderedCardCount: 4},
		{Viewport: "wide", CaseID: "R02", RunFamilyID: runFamily, WorkspaceTreeSHA256: strings.Repeat("a", 64), RequestSHA256: strings.Repeat("b", 64), ScreenshotArtifactPath: "wide.png", ScreenshotSHA256: strings.Repeat("1", 64), DOMSnapshotArtifactPath: "wide.json", DOMSnapshotSHA256: strings.Repeat("2", 64), RenderedTotal: 47, RenderedCardCount: 4},
	}}
	state, err := ExecutePrimarySubscriptionTrialActionsV1(workspace, runFamily, actions, PrimarySubscriptionTrialExecutionStateV1{}, browser)
	if err != nil || state.Controller.TerminalState != "completed" || len(state.Controller.BrowserProofs) != 3 {
		t.Fatalf("R02 v3 fake execution failed: state=%+v err=%v", state, err)
	}
	grade, err := GradePrimarySubscriptionTrialWorkspaceV1(workspace, "product-ui-r02-v3-fake", state.Controller)
	if err != nil || grade.Verdict != "PASS" {
		t.Fatalf("R02 v3 hidden verifier failed: grade=%+v err=%v", grade, err)
	}
}

func TestProductUIR02ActionContractV3RejectsObservedFailurePatterns(t *testing.T) {
	mutations := map[string]func([]RealisticCandidateAction) []RealisticCandidateAction{
		"pre-repair check": func(value []RealisticCandidateAction) []RealisticCandidateAction {
			return append([]RealisticCandidateAction{{Kind: "run_check", Check: "node tests/report.test.mjs"}}, value...)
		},
		"missing css": func(value []RealisticCandidateAction) []RealisticCandidateAction {
			return append(value[:1], value[2:]...)
		},
		"placeholder": func(value []RealisticCandidateAction) []RealisticCandidateAction {
			value[0].Contents = "TODO: controller will replace token"
			return value
		},
		"evidence edit": func(value []RealisticCandidateAction) []RealisticCandidateAction {
			value[1].Path = "EVIDENCE.json"
			return value
		},
		"fixed desktop css": func(value []RealisticCandidateAction) []RealisticCandidateAction {
			value[1].Contents = "main { width: 980px } #cards { display:grid;grid-template-columns:repeat(4,1fr) }"
			return value
		},
		"missing browser": func(value []RealisticCandidateAction) []RealisticCandidateAction {
			return append(value[:4], value[5:]...)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			value := append([]RealisticCandidateAction(nil), productUIR02ValidActionsV3()...)
			if err := ValidateProductUIR02ActionPlanV3(mutate(value)); err == nil {
				t.Fatal("observed R02 failure pattern passed")
			}
		})
	}
}

func productUIR02ValidActionsV3() []RealisticCandidateAction {
	return []RealisticCandidateAction{
		{Kind: "edit", Path: "report/report.js", Contents: "export function canonicalTotal(data) {\n  return data.published_posts.reduce((sum, post) => sum + post.views, 0);\n}\n\nexport function renderReport(data) {\n  document.querySelector(\"#total\").textContent = canonicalTotal(data);\n  document.querySelector(\"#cards\").innerHTML = data.published_posts\n    .map((post) => `<article class=\"card\"><h2>${post.id}</h2><p>${post.views} views</p></article>`)\n    .join(\"\");\n}\n\nif (typeof document !== \"undefined\") {\n  fetch(\"./data.json\").then((response) => response.json()).then(renderReport);\n}\n"},
		{Kind: "edit", Path: "report/styles.css", Contents: "body { background: #111318; color: #f5f7fb; font-family: system-ui, sans-serif; margin: 0; }\nmain { margin: 40px auto; width: min(980px, calc(100% - 32px)); }\n#cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(220px, 100%), 1fr)); gap: 20px; }\n.card { background: #20242c; border: 1px solid #343a46; border-radius: 14px; padding: 20px; min-width: 0; }\n"},
		{Kind: "run_check", Check: "node tests/report.test.mjs"},
		{Kind: "run_check", Check: "local-static-preview report"},
		{Kind: "browser"},
		{Kind: "finish", Summary: "truthful four-card report verified locally at all three required viewports"},
	}
}
