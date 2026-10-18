package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

const guardedBoundaryFakeMarker = ".guarded-boundary-fake"

type guardedBoundaryObservation struct {
	Environment []string `json:"environment"`
	Args        []string `json:"args"`
	Cwd         string   `json:"cwd"`
}

// runGuardedBoundaryFakeChild is reached from TestMain when an adapter
// re-executes the test binary with its real CLI argv and exact child
// environment. The marker lives under the allowlisted task-local state root,
// so the test does not punch a helper-only environment variable through the
// boundary it is trying to verify.
func runGuardedBoundaryFakeChild() bool {
	stateRoot := os.Getenv("XDG_STATE_HOME")
	if stateRoot == "" {
		return false
	}
	rawMode, err := os.ReadFile(filepath.Join(stateRoot, guardedBoundaryFakeMarker))
	if err != nil {
		return false
	}
	mode := strings.TrimSpace(string(rawMode))
	cwd, _ := os.Getwd()
	observation, _ := json.Marshal(guardedBoundaryObservation{
		Environment: os.Environ(), Args: os.Args[1:], Cwd: cwd,
	})
	if err := os.WriteFile(filepath.Join(stateRoot, "observation.json"), observation, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true
	}

	if strings.HasPrefix(mode, "claude_") {
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	} else {
		_, _ = io.ReadAll(os.Stdin)
	}
	switch mode {
	case "claude_success":
		fmt.Fprintln(os.Stdout, `{"type":"result","subtype":"success","is_error":false,"session_id":"guarded","result":"ok"}`)
	case "opencode_success":
		fmt.Fprintln(os.Stdout, `{"type":"step_start","sessionID":"guarded","part":{}}`)
		fmt.Fprintln(os.Stdout, `{"type":"text","sessionID":"guarded","part":{"text":"ok"}}`)
		fmt.Fprintln(os.Stdout, `{"type":"step_finish","sessionID":"guarded","part":{"reason":"stop","tokens":{"input":1,"output":1}}}`)
	case "claude_tool":
		fmt.Fprintln(os.Stdout, `{"type":"assistant","message":{"role":"assistant","model":"bound-model","content":[{"type":"tool_use","id":"call-1","name":"Bash","input":{"command":"forbidden"}}]}}`)
		time.Sleep(2 * time.Second)
		_ = os.WriteFile(filepath.Join(stateRoot, "second-action"), []byte("ran"), 0o600)
	case "claude_control":
		fmt.Fprintln(os.Stdout, `{"type":"control_request","request_id":"control-1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"forbidden"}}}`)
		time.Sleep(2 * time.Second)
		_ = os.WriteFile(filepath.Join(stateRoot, "second-action"), []byte("ran"), 0o600)
	case "opencode_tool":
		fmt.Fprintln(os.Stdout, `{"type":"step_start","sessionID":"guarded","part":{}}`)
		fmt.Fprintln(os.Stdout, `{"type":"tool_use","sessionID":"guarded","part":{"tool":"bash","callID":"call-1","state":{"status":"completed"}}}`)
		time.Sleep(2 * time.Second)
		_ = os.WriteFile(filepath.Join(stateRoot, "second-action"), []byte("ran"), 0o600)
	}
	return true
}

type guardedAdapterFixture struct {
	backend Backend
	opts    ExecOptions
	env     map[string]string
	keys    []string
	state   string
}

func newGuardedAdapterFixture(t *testing.T, provider, mode string) guardedAdapterFixture {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dirs := map[string]string{
		"home": filepath.Join(root, "home"), "tmp": filepath.Join(root, "tmp"),
		"config": filepath.Join(root, "config"), "cache": filepath.Join(root, "cache"),
		"data": filepath.Join(root, "data"), "state": filepath.Join(root, "state"),
		"cwd": filepath.Join(root, "workspace"), "appdata": filepath.Join(root, "appdata"),
		"localappdata": filepath.Join(root, "localappdata"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	env := map[string]string{
		"HOME": dirs["home"], "USERPROFILE": dirs["home"], "PATH": filepath.Dir(self),
		"PWD": dirs["cwd"], "TEMP": dirs["tmp"], "TMP": dirs["tmp"], "TMPDIR": dirs["tmp"],
		"XDG_CONFIG_HOME": dirs["config"], "XDG_CACHE_HOME": dirs["cache"],
		"XDG_DATA_HOME": dirs["data"], "XDG_STATE_HOME": dirs["state"],
	}
	if runtime.GOOS == "windows" {
		env["APPDATA"] = dirs["appdata"]
		env["LOCALAPPDATA"] = dirs["localappdata"]
		env["SYSTEMROOT"] = os.Getenv("SYSTEMROOT")
	}
	if provider == "claude" {
		env["CLAUDE_CONFIG_DIR"] = filepath.Join(dirs["home"], ".claude")
		env["CLAUDE_CODE_TMPDIR"] = dirs["tmp"]
		env["CLAUDE_CODE_MAX_RETRIES"] = "0"
		if err := os.MkdirAll(env["CLAUDE_CONFIG_DIR"], 0o700); err != nil {
			t.Fatal(err)
		}
	} else {
		config, err := GuardedOpenCodeConfigV1("bound-model", "bound-effort")
		if err != nil {
			t.Fatal(err)
		}
		env["OPENCODE_CONFIG_CONTENT"] = config
		env["OPENCODE_CONFIG_DIR"] = filepath.Join(dirs["config"], "opencode")
		env["OPENCODE_DISABLE_GLOBAL_CONFIG"] = "true"
		env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
		env["OPENCODE_DISABLE_PRUNE"] = "true"
		if err := os.MkdirAll(env["OPENCODE_CONFIG_DIR"], 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dirs["state"], guardedBoundaryFakeMarker), []byte(mode), 0o600); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	backend, err := New(provider, Config{
		ExecutablePath: self, Env: env, Logger: slog.Default(),
		EnvironmentPolicy: GuardedEnvironmentPolicyExactV1,
		AllowedEnvKeys:    keys, RequiredToolPolicy: GuardedToolPolicyNoneV1,
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := ExecOptions{
		Cwd: dirs["cwd"], Model: "bound-model", ThinkingLevel: "bound-effort",
		MaxTurns: 1, ToolPolicy: GuardedToolPolicyNoneV1, Timeout: 10 * time.Second,
	}
	if provider == "claude" {
		opts.McpConfig = json.RawMessage(`{"mcpServers":{}}`)
	}
	return guardedAdapterFixture{backend: backend, opts: opts, env: env, keys: keys, state: dirs["state"]}
}

func executeGuardedFixture(t *testing.T, fixture guardedAdapterFixture) Result {
	t.Helper()
	session, err := fixture.backend.Execute(t.Context(), "guarded prompt", fixture.opts)
	if err != nil {
		t.Fatalf("start guarded adapter: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	select {
	case result := <-session.Result:
		return result
	case <-time.After(12 * time.Second):
		t.Fatal("timed out waiting for guarded fake child")
		return Result{}
	}
}

func TestGuardedAdaptersLaunchWithExactEnvironment(t *testing.T) {
	t.Setenv("MULTICA_HOST_ATTESTATION", "parent-only-secret")
	t.Setenv("GUARDED_PARENT_SECRET", "must-not-leak")
	for _, provider := range []string{"claude", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			fixture := newGuardedAdapterFixture(t, provider, provider+"_success")
			result := executeGuardedFixture(t, fixture)
			if result.Status != "completed" || result.Output != "ok" {
				t.Fatalf("guarded adapter result = %+v", result)
			}
			raw, err := os.ReadFile(filepath.Join(fixture.state, "observation.json"))
			if err != nil {
				t.Fatal(err)
			}
			var observation guardedBoundaryObservation
			if err := json.Unmarshal(raw, &observation); err != nil {
				t.Fatal(err)
			}
			actual := make(map[string]string, len(observation.Environment))
			for _, pair := range observation.Environment {
				key, value, ok := strings.Cut(pair, "=")
				if ok {
					actual[key] = value
				}
			}
			if len(actual) != len(fixture.env) {
				t.Fatalf("child environment key count = %d, want %d: %v", len(actual), len(fixture.env), observation.Environment)
			}
			for key, want := range fixture.env {
				if got, ok := actual[key]; !ok || got != want {
					t.Errorf("child environment %s = %q, want %q", key, got, want)
				}
			}
			if _, ok := actual["MULTICA_HOST_ATTESTATION"]; ok {
				t.Fatal("host attestation leaked into guarded child")
			}
			if _, ok := actual["GUARDED_PARENT_SECRET"]; ok {
				t.Fatal("parent secret leaked into guarded child")
			}
			if observation.Cwd != fixture.opts.Cwd {
				t.Errorf("child cwd = %q, want %q", observation.Cwd, fixture.opts.Cwd)
			}
			joinedArgs := strings.Join(observation.Args, " ")
			if provider == "claude" && (!strings.Contains(joinedArgs, "--restricted") || !strings.Contains(joinedArgs, "--tools ")) {
				t.Errorf("Claude guarded argv = %q", joinedArgs)
			}
			if provider == "opencode" && (!strings.HasPrefix(joinedArgs, "--pure run") || !strings.Contains(joinedArgs, "--agent guarded")) {
				t.Errorf("OpenCode guarded argv = %q", joinedArgs)
			}
		})
	}
}

func TestGuardedAdaptersKillChildOnFirstToolEvent(t *testing.T) {
	for _, provider := range []string{"claude", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			fixture := newGuardedAdapterFixture(t, provider, provider+"_tool")
			result := executeGuardedFixture(t, fixture)
			if result.Status != "failed" || !strings.Contains(result.Error, "tool-free") {
				t.Fatalf("tool violation result = %+v", result)
			}
			time.Sleep(2200 * time.Millisecond)
			if _, err := os.Stat(filepath.Join(fixture.state, "second-action")); !os.IsNotExist(err) {
				t.Fatalf("guarded child reached a second action after tool event: %v", err)
			}
		})
	}
}

func TestGuardedClaudeControlRequestFailsClosed(t *testing.T) {
	fixture := newGuardedAdapterFixture(t, "claude", "claude_control")
	result := executeGuardedFixture(t, fixture)
	if result.Status != "failed" || !strings.Contains(result.Error, "control request") {
		t.Fatalf("guarded control request result = %+v", result)
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(fixture.state, "second-action")); !os.IsNotExist(err) {
		t.Fatalf("guarded child reached a second action after control request: %v", err)
	}
}

func TestGuardedExactChildEnvironmentRejectsKeyDrift(t *testing.T) {
	for name, cfg := range map[string]Config{
		"extra":   {EnvironmentPolicy: GuardedEnvironmentPolicyExactV1, AllowedEnvKeys: []string{"HOME"}, Env: map[string]string{"HOME": "x", "TOKEN": "secret"}},
		"missing": {EnvironmentPolicy: GuardedEnvironmentPolicyExactV1, AllowedEnvKeys: []string{"HOME", "PATH"}, Env: map[string]string{"HOME": "x"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := cfg.childEnvironment(); err == nil {
				t.Fatal("environment drift accepted")
			}
		})
	}
}

func TestGuardedClaudeEnvironmentRequiresZeroAPIRetries(t *testing.T) {
	fixture := newGuardedAdapterFixture(t, "claude", "claude_success")
	delete(fixture.env, "CLAUDE_CODE_MAX_RETRIES")
	if _, err := fixture.backend.Execute(t.Context(), "guarded prompt", fixture.opts); err == nil {
		t.Fatal("guarded Claude execution accepted missing retry control")
	}
	fixture = newGuardedAdapterFixture(t, "claude", "claude_success")
	fixture.env["CLAUDE_CODE_MAX_RETRIES"] = "1"
	if _, err := fixture.backend.Execute(t.Context(), "guarded prompt", fixture.opts); err == nil {
		t.Fatal("guarded Claude execution accepted a nonzero retry count")
	}
}

func TestGuardedToolPolicyCannotBeRequestedOrRequiredOneSided(t *testing.T) {
	base := ExecOptions{Model: "model", ThinkingLevel: "effort", MaxTurns: 1}
	if err := validateGuardedToolFreeOptions(Config{}, func() ExecOptions { v := base; v.ToolPolicy = GuardedToolPolicyNoneV1; return v }()); err == nil {
		t.Fatal("tool-free options without a bound backend policy were accepted")
	}
	if err := validateGuardedToolFreeOptions(Config{RequiredToolPolicy: GuardedToolPolicyNoneV1}, base); err == nil {
		t.Fatal("bound backend policy without tool-free options was accepted")
	}
}

func TestGuardedProviderArgsAreToolFreeAndImmutable(t *testing.T) {
	opts := ExecOptions{Cwd: "work", Model: "model", ThinkingLevel: "effort", MaxTurns: 1, ToolPolicy: GuardedToolPolicyNoneV1}
	claude := buildClaudeArgs(opts, slog.Default())
	claudeJoined := strings.Join(claude, " ")
	for _, required := range []string{"--restricted", "--tools", "--strict-mcp-config", "--no-session-persistence", "--permission-prompts none", "--max-turns 1"} {
		if !strings.Contains(claudeJoined, required) {
			t.Fatalf("Claude tool-free flag %q missing: %v", required, claude)
		}
	}
	if strings.Contains(claudeJoined, "bypassPermissions") || strings.Contains(claudeJoined, "--resume") {
		t.Fatalf("Claude guarded args retained ambient authority: %v", claude)
	}
	opencode := buildOpenCodeArgs(opts, slog.Default())
	opencodeJoined := strings.Join(opencode, " ")
	if !strings.HasPrefix(opencodeJoined, "--pure run --format json") || !strings.Contains(opencodeJoined, "--agent guarded") || strings.Contains(opencodeJoined, "dangerously-skip-permissions") {
		t.Fatalf("OpenCode guarded args are not tool-free: %v", opencode)
	}
}

func TestGuardedOpenCodeToolEventFailsClosed(t *testing.T) {
	b := &opencodeBackend{cfg: Config{Logger: slog.Default(), RequiredToolPolicy: GuardedToolPolicyNoneV1}}
	stream := strings.Join([]string{
		`{"type":"step_start","part":{}}`,
		`{"type":"tool_use","part":{"tool":"bash","callID":"call-1","state":{"status":"completed"}}}`,
		`{"type":"step_finish","part":{"reason":"stop","tokens":{"input":1,"output":1}}}`,
	}, "\n")
	result := b.processEvents(strings.NewReader(stream), make(chan Message, 8))
	if result.status != "failed" || !strings.Contains(result.errMsg, "tool-free") {
		t.Fatalf("tool event result = %+v", result)
	}
}

func TestGuardedOpenCodeConfigPinsAllMutableSurfacesOff(t *testing.T) {
	raw, err := GuardedOpenCodeConfigV1("model", "effort")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`"permission":"deny"`, `"tools":{"*":false}`, `"plugin":[]`, `"instructions":[]`, `"mcp":{}`, `"default_agent":"guarded"`} {
		if !strings.Contains(raw, required) {
			t.Fatalf("guarded OpenCode config missing %s: %s", required, raw)
		}
	}
}

func TestResolveClaudeNativeFromShim(t *testing.T) {
	prefix := t.TempDir()
	shim := filepath.Join(prefix, "claude.cmd")
	native := filepath.Join(prefix, "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
	if err := os.MkdirAll(filepath.Dir(native), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(native, []byte("fake native"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveClaudeNativeFromShim(shim, os.Stat); got != native {
		t.Fatalf("native Claude path = %q, want %q", got, native)
	}
	if got := resolveClaudeNativeFromShim(filepath.Join(prefix, "claude.exe"), os.Stat); got != "" {
		t.Fatalf("native executable was treated as a shim: %q", got)
	}
}
