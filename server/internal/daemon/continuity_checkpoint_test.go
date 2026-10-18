package daemon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func testContinuityPlan() *ContinuityCheckpointPlan {
	return &ContinuityCheckpointPlan{
		SchemaVersion: 1, RootTaskID: "11111111-1111-1111-1111-111111111111",
		PackageStep: 1, MaxPackages: 3, MaxCheckpointBytes: 2048,
	}
}

func validContinuityOutput() string {
	return "Package one is safely parked.\n" + continuityCheckpointOpenTag +
		`{"schema_version":1,"package_step":1,"summary":"Implemented the bounded slice.","completed":["Added the parser"],"next_action":"Run the integration path.","verification":["Unit tests pass"],"open_risks":[]}` +
		continuityCheckpointCloseTag
}

func TestParseContinuityCheckpointStripsMachineReceipt(t *testing.T) {
	receipt, visible, err := parseContinuityCheckpoint(validContinuityOutput(), testContinuityPlan())
	if err != nil {
		t.Fatalf("parse checkpoint: %v", err)
	}
	if visible != "Package one is safely parked." {
		t.Fatalf("visible output = %q", visible)
	}
	if receipt.PackageStep != 1 || receipt.NextAction != "Run the integration path." {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestParseContinuityCheckpointFailsClosed(t *testing.T) {
	tests := map[string]string{
		"multiple":   validContinuityOutput() + validContinuityOutput(),
		"unknown":    strings.Replace(validContinuityOutput(), `"summary":`, `"unexpected":true,"summary":`, 1),
		"wrong step": strings.Replace(validContinuityOutput(), `"package_step":1`, `"package_step":2`, 1),
		"trailing":   strings.Replace(validContinuityOutput(), continuityCheckpointCloseTag, ` {}`+continuityCheckpointCloseTag, 1),
	}
	for name, output := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseContinuityCheckpoint(output, testContinuityPlan()); err == nil {
				t.Fatal("expected strict parser rejection")
			}
		})
	}
}

func TestParseContinuityCheckpointAllowsNormalFinalCompletion(t *testing.T) {
	const output = "The accepted goal is complete."
	receipt, visible, err := parseContinuityCheckpoint(output, testContinuityPlan())
	if err != nil || receipt != nil || visible != output {
		t.Fatalf("normal completion changed: receipt=%+v visible=%q err=%v", receipt, visible, err)
	}
}

func TestMeasuredContinuityCheckpointRequiresReceipt(t *testing.T) {
	if _, _, err := parseTaskContinuityCheckpoint("ordinary completion", testContinuityPlan(), true); err == nil {
		t.Fatal("measured checkpoint accepted output without a receipt")
	}
	receipt, _, err := parseTaskContinuityCheckpoint(validContinuityOutput(), testContinuityPlan(), true)
	if err != nil || receipt == nil {
		t.Fatalf("valid measured checkpoint rejected: receipt=%+v err=%v", receipt, err)
	}
}

func TestMeasuredContinuityCheckpointRejectsOversizeReceipt(t *testing.T) {
	output := continuityCheckpointOpenTag +
		`{"schema_version":1,"package_step":1,"summary":"` + strings.Repeat("x", 2050) +
		`","completed":["durable work"],"next_action":"continue","verification":["checked"],"open_risks":[]}` +
		continuityCheckpointCloseTag
	if _, _, err := parseTaskContinuityCheckpoint(output, testContinuityPlan(), true); err == nil {
		t.Fatal("oversized measured checkpoint receipt was accepted")
	}
}

func TestContinuityCheckpointPlanBounds(t *testing.T) {
	for name, mutate := range map[string]func(*ContinuityCheckpointPlan){
		"bad root":        func(p *ContinuityCheckpointPlan) { p.RootTaskID = "not-a-uuid" },
		"too many":        func(p *ContinuityCheckpointPlan) { p.MaxPackages = 4 },
		"terminal step":   func(p *ContinuityCheckpointPlan) { p.PackageStep = p.MaxPackages },
		"oversized bound": func(p *ContinuityCheckpointPlan) { p.MaxCheckpointBytes = 2049 },
	} {
		t.Run(name, func(t *testing.T) {
			plan := testContinuityPlan()
			mutate(plan)
			if err := validateContinuityCheckpointPlan(plan); err == nil {
				t.Fatal("expected invalid plan")
			}
		})
	}
}

func TestBuildPromptAppendsCheckpointAfterResume(t *testing.T) {
	task := Task{
		ID: "22222222-2222-2222-2222-222222222222", Attempt: 2,
		WorkspaceID:              "33333333-3333-3333-3333-333333333333",
		IssueID:                  "44444444-4444-4444-4444-444444444444",
		ContinuityCheckpointPlan: testContinuityPlan(),
	}
	prompt := BuildPrompt(task, "codex")
	checkpointAt := strings.Index(prompt, "## Bounded continuity checkpoint")
	if checkpointAt < 0 || !strings.Contains(prompt[checkpointAt:], continuityCheckpointOpenTag) {
		t.Fatalf("prompt missing bounded checkpoint instructions:\n%s", prompt)
	}

	withoutPlan := task
	withoutPlan.ContinuityCheckpointPlan = nil
	if strings.Contains(BuildPrompt(withoutPlan, "codex"), "Bounded continuity checkpoint") {
		t.Fatal("ordinary prompt changed without a checkpoint plan")
	}
}

func TestBuildMeasuredContinuityCheckpointPromptIsNarrowAndBound(t *testing.T) {
	prompt, err := buildMeasuredContinuityCheckpointPrompt(testContinuityPlan())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Stop starting new implementation work", "exactly one `<continuity-checkpoint>`", `"package_step":1`, "fresh worker"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("measured checkpoint prompt missing %q: %s", want, prompt)
		}
	}
	if strings.Contains(prompt, "Package one is safely parked") {
		t.Fatal("measured checkpoint prompt replayed task output")
	}
	bad := testContinuityPlan()
	bad.PackageStep = bad.MaxPackages
	if _, err := buildMeasuredContinuityCheckpointPrompt(bad); err == nil {
		t.Fatal("invalid authenticated plan was accepted")
	}
}

func TestCompleteTaskWithCheckpointUsesTypedField(t *testing.T) {
	var body map[string]json.RawMessage
	client := NewClient("http://daemon.test")
	client.client.Transport = checkpointRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})

	receipt, _, err := parseContinuityCheckpoint(validContinuityOutput(), testContinuityPlan())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CompleteTaskWithCheckpoint(context.Background(), "task-1", "Package one is safely parked.", "", "", "", false, "", "", receipt); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, ok := body["continuity_checkpoint"]; !ok {
		t.Fatalf("typed checkpoint missing from body: %v", body)
	}
	if strings.Contains(string(body["output"]), "continuity-checkpoint") {
		t.Fatalf("machine tag leaked into output: %s", body["output"])
	}
}

type checkpointRoundTripFunc func(*http.Request) (*http.Response, error)

func (f checkpointRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
