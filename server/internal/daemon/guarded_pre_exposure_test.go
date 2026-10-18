package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestLoadConfigReadsDistinctGuardedHostAttestation(t *testing.T) {
	// Configuration parsing must not discover or probe an installed provider.
	stubAgentProbe(t, map[string]AgentEntry{})
	t.Setenv("MULTICA_HOST_ATTESTATION", "  alienware_windows  ")
	t.Setenv("MULTICA_SERVER_URL", "http://localhost:8080")
	t.Setenv("MULTICA_DAEMON_ID", "11111111-1111-1111-1111-111111111111")
	cfg, err := LoadConfig(Overrides{AllowNoAgents: true, WorkspacesRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HostAttestation != "alienware_windows" {
		t.Fatalf("host attestation = %q", cfg.HostAttestation)
	}
}

func TestBuildGuardedChildEnvironmentIsExactAndCredentialFree(t *testing.T) {
	t.Setenv("MULTICA_HOST_ATTESTATION", "parent-only")
	t.Setenv("ANTHROPIC_API_KEY", "parent-provider-secret")
	if runtime.GOOS == "windows" && os.Getenv("SYSTEMROOT") == "" {
		t.Setenv("SYSTEMROOT", `C:\Windows`)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"claude", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			root := t.TempDir()
			env, keys, err := buildGuardedChildEnvironment(provider, runtime.GOOS, executable, root, "bound-model", "bound-effort")
			if err != nil {
				t.Fatal(err)
			}
			if len(env) != len(keys) {
				t.Fatalf("env keys = %d, allowlist = %d", len(env), len(keys))
			}
			for key, value := range env {
				if strings.HasPrefix(strings.ToUpper(key), "MULTICA_") || strings.Contains(key, "API_KEY") || strings.Contains(value, "parent-only") || strings.Contains(value, "parent-provider-secret") {
					t.Fatalf("credential or attestation escaped into %s", key)
				}
				if key == "HOME" || key == "USERPROFILE" || key == "PWD" || key == "TEMP" || key == "TMP" || key == "TMPDIR" || strings.HasPrefix(key, "XDG_") || key == "APPDATA" || key == "LOCALAPPDATA" || key == "CLAUDE_CONFIG_DIR" || key == "CLAUDE_CODE_TMPDIR" || key == "OPENCODE_CONFIG_DIR" {
					rel, relErr := filepath.Rel(root, value)
					if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
						t.Fatalf("%s escaped guarded temp root: %q", key, value)
					}
				}
			}
			if provider == "opencode" {
				expected, _ := agent.GuardedOpenCodeConfigV1("bound-model", "bound-effort")
				if env["OPENCODE_CONFIG_CONTENT"] != expected {
					t.Fatal("OpenCode inline tool-free config drifted")
				}
			}
		})
	}
}

func TestGuardedHostFactsMatchCanonicalAllowlistAndExecutable(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	facts, err := guardedHostExecutionFacts("claude", executable)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := agentqualification.GuardedChildEnvironmentKeysV1("claude", runtime.GOOS)
	digest, _ := agentqualification.GuardedEnvironmentAllowlistSHA256V1(keys)
	executableDigest, _ := agent.FileSHA256(executable)
	if facts.EnvironmentAllowlistSHA256 != digest || facts.ExecutableSHA256 != executableDigest || facts.HostAttestationCustody != agentqualification.GuardedHostAttestationParentOnlyV1 || facts.ToolPolicy != agentqualification.GuardedToolPolicyNoneV1 {
		t.Fatalf("guarded host facts = %+v", facts)
	}
}

func TestStageGuardedExecutablePinsOpenedBytesAwayFromInstalledPath(t *testing.T) {
	source := filepath.Join(t.TempDir(), "provider.bin")
	if err := os.WriteFile(source, []byte("qualified-bytes"), 0o700); err != nil {
		t.Fatal(err)
	}
	staged, digest, err := stageGuardedExecutable(source, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("updater-replacement"), 0o700); err != nil {
		t.Fatal(err)
	}
	stagedRaw, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(stagedRaw) != "qualified-bytes" {
		t.Fatalf("staged executable changed with installed path: %q", stagedRaw)
	}
	stagedDigest, err := agent.FileSHA256(staged)
	if err != nil {
		t.Fatal(err)
	}
	if stagedDigest != digest {
		t.Fatalf("staged digest = %s, want %s", stagedDigest, digest)
	}
}
