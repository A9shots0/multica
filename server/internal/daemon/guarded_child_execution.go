package daemon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
	"github.com/multica-ai/multica/server/pkg/agent"
)

func validateGuardedTaskExposure(task Task, provider string) error {
	if task.GuardedExecution == nil {
		return nil
	}
	if task.Agent == nil {
		return fmt.Errorf("missing guarded agent")
	}
	skillHashes := make([]string, 0, len(task.Agent.Skills))
	for _, skill := range task.Agent.Skills {
		sum := sha256.Sum256([]byte(skill.Content))
		skillHashes = append(skillHashes, hex.EncodeToString(sum[:]))
	}
	return agentqualification.ValidateGuardedClaimExposureV1(task.GuardedExecution, agentqualification.GuardedClaimExposureFactsV1{
		RuntimeProvider: provider, AgentName: task.Agent.Name, Model: task.Agent.Model, ReasoningEffort: task.Agent.ThinkingLevel,
		ServiceTier: task.Agent.ServiceTier, Instructions: task.Agent.Instructions, SkillSHA256: skillHashes,
		CustomEnvCount: len(task.Agent.CustomEnv), CustomArgsCount: len(task.Agent.CustomArgs),
		LazySkills: len(task.Agent.SkillRefs) > 0, DisabledSkills: len(task.Agent.DisabledRuntimeSkills) > 0,
		DirectMCP:     len(bytes.TrimSpace(task.Agent.McpConfig)) > 0,
		RuntimeConfig: len(bytes.TrimSpace(task.Agent.RuntimeConfig)) > 0,
		ConnectedApps: len(task.ConnectedApps) > 0, RemoteMCP: len(task.RemoteMCPConnections) > 0,
		PluginHooks: len(task.PluginHookTools) > 0,
	})
}

// stageGuardedExecutable copies the already-resolved provider executable into
// a fresh task-owned directory while hashing the bytes read from the open
// source handle. The staged file is then hashed independently and is the only
// pathname handed to the backend. Provider updaters can replace the installed
// path after this point without changing the bytes selected for this run.
func stageGuardedExecutable(sourcePath, taskTempDir string) (string, string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("open guarded executable for staging: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("guarded executable source is not a regular file")
	}

	stageDir := filepath.Join(taskTempDir, "guarded-child", "executable")
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create guarded executable stage: %w", err)
	}
	ext := filepath.Ext(sourcePath)
	staged, err := os.CreateTemp(stageDir, "provider-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("create guarded executable stage file: %w", err)
	}
	stagedPath := staged.Name()
	ok := false
	defer func() {
		if !ok {
			_ = staged.Close()
			_ = os.Remove(stagedPath)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(staged, h), source); err != nil {
		return "", "", fmt.Errorf("copy guarded executable into stage: %w", err)
	}
	if err := staged.Sync(); err != nil {
		return "", "", fmt.Errorf("sync guarded executable stage: %w", err)
	}
	if err := staged.Chmod(0o500); err != nil {
		return "", "", fmt.Errorf("make guarded executable stage read-only: %w", err)
	}
	if err := staged.Close(); err != nil {
		return "", "", fmt.Errorf("close guarded executable stage: %w", err)
	}
	sourceDigest := hex.EncodeToString(h.Sum(nil))
	stagedDigest, err := agent.FileSHA256(stagedPath)
	if err != nil {
		return "", "", fmt.Errorf("verify guarded executable stage: %w", err)
	}
	if stagedDigest != sourceDigest {
		return "", "", fmt.Errorf("guarded executable stage digest mismatch")
	}
	ok = true
	return stagedPath, stagedDigest, nil
}

func guardedHostExecutionFacts(provider, executablePath string) (agentqualification.GuardedHostExecutionFactsV1, error) {
	keys, err := agentqualification.GuardedChildEnvironmentKeysV1(provider, runtime.GOOS)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, err
	}
	digest, err := agentqualification.GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, err
	}
	adapter, err := agentqualification.GuardedAdapterV1(provider)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, err
	}
	executableDigest, err := agent.FileSHA256(executablePath)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, fmt.Errorf("hash guarded executable: %w", err)
	}
	return agentqualification.GuardedHostExecutionFactsV1{
		HostOS: runtime.GOOS, Adapter: adapter,
		EnvironmentPolicy:          agentqualification.GuardedEnvironmentPolicyExactV1,
		EnvironmentAllowlistSHA256: digest,
		ToolPolicy:                 agentqualification.GuardedToolPolicyNoneV1,
		HostAttestationCustody:     agentqualification.GuardedHostAttestationParentOnlyV1,
		ExecutableSHA256:           executableDigest,
	}, nil
}

func resolveControllerBoundExecutable(path string) (string, error) {
	if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", fmt.Errorf("controller-bound executable path is not exact and absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect controller-bound executable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("controller-bound executable must be a non-symlink regular file")
	}
	return path, nil
}

func guardedWorkflowBrokerHostExecutionFacts(brokerPath, providerPath string) (agentqualification.GuardedHostExecutionFactsV1, error) {
	keys, err := agentqualification.GuardedWorkflowBrokerEnvironmentKeysV1(runtime.GOOS)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, err
	}
	environmentDigest, err := agentqualification.GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, err
	}
	brokerDigest, err := agent.FileSHA256(brokerPath)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, fmt.Errorf("hash guarded workflow broker: %w", err)
	}
	providerDigest, err := agent.FileSHA256(providerPath)
	if err != nil {
		return agentqualification.GuardedHostExecutionFactsV1{}, fmt.Errorf("hash guarded workflow provider child: %w", err)
	}
	return agentqualification.GuardedHostExecutionFactsV1{
		HostOS: runtime.GOOS, Adapter: agentqualification.GuardedClaudeWorkflowBrokerAdapterV1,
		EnvironmentPolicy:          agentqualification.GuardedEnvironmentPolicyExactV1,
		EnvironmentAllowlistSHA256: environmentDigest, ToolPolicy: agentqualification.GuardedToolPolicyNoneV1,
		HostAttestationCustody: agentqualification.GuardedHostAttestationParentOnlyV1,
		ExecutableSHA256:       brokerDigest, ProviderExecutableSHA256: providerDigest,
	}, nil
}

// buildGuardedChildEnvironment constructs a new environment rather than
// filtering os.Environ. Every writable profile/config/temp path is task-local;
// only SYSTEMROOT is copied on Windows because CreateProcess and native
// runtimes require the OS installation root. No provider credential, MULTICA_*,
// or host-attestation value enters the map.
func buildGuardedChildEnvironment(provider, hostOS, executablePath, taskTempDir, model, effort string) (map[string]string, []string, error) {
	keys, err := agentqualification.GuardedChildEnvironmentKeysV1(provider, hostOS)
	if err != nil {
		return nil, nil, err
	}
	root, err := filepath.Abs(filepath.Join(taskTempDir, "guarded-child"))
	if err != nil {
		return nil, nil, err
	}
	dirs := map[string]string{
		"home": filepath.Join(root, "home"), "tmp": filepath.Join(root, "tmp"),
		"config": filepath.Join(root, "xdg", "config"), "cache": filepath.Join(root, "xdg", "cache"),
		"data": filepath.Join(root, "xdg", "data"), "state": filepath.Join(root, "xdg", "state"),
		"workspace": filepath.Join(root, "workspace"),
		"appdata":   filepath.Join(root, "appdata", "roaming"), "localappdata": filepath.Join(root, "appdata", "local"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, nil, fmt.Errorf("create guarded child profile: %w", err)
		}
	}
	env := map[string]string{
		"HOME": dirs["home"], "USERPROFILE": dirs["home"],
		"PATH": filepath.Dir(executablePath), "PWD": dirs["workspace"],
		"TEMP": dirs["tmp"], "TMP": dirs["tmp"], "TMPDIR": dirs["tmp"],
		"XDG_CONFIG_HOME": dirs["config"], "XDG_CACHE_HOME": dirs["cache"],
		"XDG_DATA_HOME": dirs["data"], "XDG_STATE_HOME": dirs["state"],
	}
	switch strings.ToLower(hostOS) {
	case "windows":
		systemRoot := os.Getenv("SYSTEMROOT")
		if strings.TrimSpace(systemRoot) == "" {
			return nil, nil, fmt.Errorf("guarded Windows child requires SYSTEMROOT")
		}
		env["APPDATA"] = dirs["appdata"]
		env["LOCALAPPDATA"] = dirs["localappdata"]
		env["SYSTEMROOT"] = systemRoot
	}
	switch provider {
	case "claude":
		env["CLAUDE_CONFIG_DIR"] = filepath.Join(dirs["home"], ".claude")
		env["CLAUDE_CODE_TMPDIR"] = dirs["tmp"]
		// Anthropic documents this as the number of failed API-request
		// retries. Zero binds the model-only shadow to one transport attempt.
		env["CLAUDE_CODE_MAX_RETRIES"] = "0"
		if err := os.MkdirAll(env["CLAUDE_CONFIG_DIR"], 0o700); err != nil {
			return nil, nil, fmt.Errorf("create guarded Claude config dir: %w", err)
		}
	case "opencode":
		config, err := agent.GuardedOpenCodeConfigV1(model, effort)
		if err != nil {
			return nil, nil, err
		}
		opencodeDir := filepath.Join(dirs["config"], "opencode")
		if err := os.MkdirAll(opencodeDir, 0o700); err != nil {
			return nil, nil, fmt.Errorf("create guarded OpenCode config dir: %w", err)
		}
		env["OPENCODE_CONFIG_CONTENT"] = config
		env["OPENCODE_CONFIG_DIR"] = opencodeDir
		env["OPENCODE_DISABLE_GLOBAL_CONFIG"] = "true"
		env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
		env["OPENCODE_DISABLE_PRUNE"] = "true"
	}
	actualKeys := make([]string, 0, len(env))
	for key := range env {
		actualKeys = append(actualKeys, key)
	}
	sort.Strings(actualKeys)
	if strings.Join(actualKeys, "\x00") != strings.Join(keys, "\x00") {
		return nil, nil, fmt.Errorf("guarded child environment construction drifted from allowlist")
	}
	return env, keys, nil
}

// buildBoundedWorkflowBrokerEnvironment extends the exact task-local provider
// environment only for an explicitly hash-bound parent broker. The broker is
// the trusted process launched by the daemon; it keeps this task-scoped
// capability parent-only and launches its provider child with a fresh,
// separately constructed tool-free environment. No daemon PAT, custom agent
// environment, repository context, or provider credential is added here.
func buildBoundedWorkflowBrokerEnvironment(
	provider, hostOS, executablePath, providerChildPath, taskTempDir, model, effort string,
	task Task, authority *agentqualification.GuardedWorkflowBrokerV1, serverURL string,
) (map[string]string, []string, error) {
	if provider != "claude" || authority == nil {
		return nil, nil, fmt.Errorf("bounded workflow broker requires Claude Code")
	}
	env, keys, err := buildGuardedChildEnvironment(
		provider, hostOS, executablePath, taskTempDir, model, effort,
	)
	if err != nil {
		return nil, nil, err
	}
	token, err := taskScopedAuthToken(task)
	if err != nil {
		return nil, nil, err
	}
	brokerValues := map[string]string{
		"MULTICA_TOKEN":                token,
		"MULTICA_SERVER_URL":           serverURL,
		"MULTICA_WORKSPACE_ID":         task.WorkspaceID,
		"MULTICA_AGENT_NAME":           authority.AgentName,
		"MULTICA_AGENT_ID":             task.AgentID,
		"MULTICA_TASK_ID":              task.ID,
		"MULTICA_ISSUE_ID":             task.IssueID,
		"MULTICA_WORKFLOW_TASK_KIND":   authority.TaskKind,
		"ITEM14_PROVIDER_CHILD_PATH":   providerChildPath,
		"ITEM14_PROVIDER_CHILD_SHA256": authority.ProviderExecutableSHA256,
		"ITEM14_PROVIDER_PROFILE_KIND": authority.ProviderProfileKind,
		"ITEM14_PROVIDER_PROFILE_PATH": authority.ProviderProfilePath,
	}
	for key, value := range brokerValues {
		if strings.TrimSpace(value) == "" && key != "ITEM14_PROVIDER_PROFILE_PATH" {
			return nil, nil, fmt.Errorf("bounded workflow broker requires %s", key)
		}
		env[key] = value
		keys = append(keys, key)
	}
	keys, err = agentqualification.GuardedWorkflowBrokerEnvironmentKeysV1(hostOS)
	if err != nil {
		return nil, nil, err
	}
	actualKeys := make([]string, 0, len(env))
	for key := range env {
		actualKeys = append(actualKeys, key)
	}
	sort.Strings(actualKeys)
	if strings.Join(actualKeys, "\x00") != strings.Join(keys, "\x00") {
		return nil, nil, fmt.Errorf("bounded workflow broker environment construction drifted from allowlist")
	}
	return env, keys, nil
}

type guardedCredentialLease struct {
	destinationPath string
	cleaned         bool
}

// acquireGuardedCredentialCapsule performs the one authorized source open and
// opaque byte copy into the already-created task-local Claude profile. It does
// not decode, hash, compare, print, or retain credential contents.
func acquireGuardedCredentialCapsule(
	capsule agentqualification.GuardedCredentialCapsuleV1,
	claudeConfigDir string,
	openSource func(string) (io.ReadCloser, error),
) (*guardedCredentialLease, error) {
	if openSource == nil {
		return nil, fmt.Errorf("guarded credential source opener is unavailable")
	}
	if strings.TrimSpace(claudeConfigDir) == "" {
		return nil, fmt.Errorf("guarded Claude config directory is unavailable")
	}
	if capsule.Version != agentqualification.GuardedCredentialCapsuleVersionV1 ||
		capsule.SourceKind != agentqualification.GuardedCredentialSourceClaudeCodeSubscriptionV1 ||
		capsule.Purpose != agentqualification.GuardedCredentialPurposeClaudeCodeSubscriptionInferenceOnceV1 ||
		capsule.CopyPolicy != agentqualification.GuardedCredentialCopyPolicyReadOnlyByteCopyOnceV1 ||
		capsule.CleanupPolicy != agentqualification.GuardedCredentialCleanupPolicyImmediateVerifiedAbsenceV1 {
		return nil, fmt.Errorf("guarded credential capsule policy mismatch")
	}
	source, err := openSource(capsule.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("open guarded credential source: %w", err)
	}
	defer source.Close()

	destinationPath := filepath.Join(claudeConfigDir, ".credentials.json")
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create guarded credential capsule: %w", err)
	}
	lease := &guardedCredentialLease{destinationPath: destinationPath}
	ok := false
	defer func() {
		if !ok {
			_ = destination.Close()
			_ = os.Remove(destinationPath)
		}
	}()
	if _, err := io.Copy(destination, source); err != nil {
		return nil, fmt.Errorf("copy guarded credential capsule: %w", err)
	}
	if err := destination.Sync(); err != nil {
		return nil, fmt.Errorf("sync guarded credential capsule: %w", err)
	}
	if err := destination.Close(); err != nil {
		return nil, fmt.Errorf("close guarded credential capsule: %w", err)
	}
	ok = true
	return lease, nil
}

func (l *guardedCredentialLease) cleanupAndVerify() error {
	if l == nil || l.cleaned {
		return nil
	}
	if err := os.Remove(l.destinationPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete guarded credential capsule: %w", err)
	}
	if _, err := os.Lstat(l.destinationPath); !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("guarded credential capsule remains after deletion")
		}
		return fmt.Errorf("verify guarded credential capsule absence: %w", err)
	}
	l.cleaned = true
	return nil
}
