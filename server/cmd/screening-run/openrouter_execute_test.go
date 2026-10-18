package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestCeilingUSDMicros(t *testing.T) {
	for input, want := range map[string]int64{
		"0": 0, "0.0000001": 1, "1.25": 1_250_000, "102.0717236": 102_071_724,
	} {
		got, err := ceilingUSDMicros(input)
		if err != nil || got != want {
			t.Fatalf("ceilingUSDMicros(%q)=%d,%v want %d", input, got, err, want)
		}
	}
	if _, err := ceilingUSDMicros("-1"); err == nil {
		t.Fatal("negative spend accepted")
	}
}

func TestSessionIDFromOpenCodeJSONL(t *testing.T) {
	raw := []byte("{\"sessionID\":\"ses_one\"}\n{\"sessionID\":\"ses_one\"}\n")
	if got, err := sessionIDFromOpenCodeJSONL(raw); err != nil || got != "ses_one" {
		t.Fatalf("unexpected session: %q %v", got, err)
	}
	if _, err := sessionIDFromOpenCodeJSONL([]byte("{\"sessionID\":\"one\"}\n{\"sessionID\":\"two\"}\n")); err == nil {
		t.Fatal("multiple sessions accepted")
	}
}

func TestV6RunDeliversExactPromptOnlyThroughStdinAndPinsOutputCeiling(t *testing.T) {
	batch, err := agentqualification.ThirteenthOpenRouterScreeningBatchPlan()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := agentqualification.ScreeningAssignments(batch)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := agentqualification.BuildOpenCodePlanForTransport(batch.CandidateTransport, assignments[0].Configuration, batch.Cases[0])
	if err != nil {
		t.Fatal(err)
	}
	runtimeState, err := newOpenCodeCommandRuntime(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnvironment(runtimeState.Env, "OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=8000") {
		t.Fatalf("output cap missing from runtime: %v", runtimeState.Env)
	}
	command, err := runtimeState.buildRunCommand(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := io.ReadAll(command.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if string(stdin) != plan.Prompt || sha256String(string(stdin)) != plan.PromptSHA256 {
		t.Fatal("stdin prompt drifted")
	}
	for _, argument := range command.Args {
		if strings.Contains(argument, "<qualification-policy>") || argument == plan.Prompt {
			t.Fatal("multiline prompt leaked back into Windows command arguments")
		}
	}
}

func containsEnvironment(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
