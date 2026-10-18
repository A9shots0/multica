package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/multica-ai/multica/server/internal/contextmanifest"
	"github.com/multica-ai/multica/server/pkg/agent"
)

const shadowContextManifestLimitBytes = 1 << 20

func buildTaskContextManifest(task Task, prompt string, opts agent.ExecOptions, runtimeBrief string, providerAttempt int) (contextmanifest.Manifest, error) {
	blocks := make([]contextmanifest.Block, 0, 8)
	addExact := func(kind, identity, reason string, content []byte, cacheable bool) {
		if len(content) == 0 {
			return
		}
		blocks = append(blocks, contextmanifest.Block{
			Kind: kind, SourceIdentity: identity, SourceRevision: contentRevision(content),
			ContentBytes: int64(len(content)), Reason: reason, Cacheable: cacheable,
			MeasurementState: contextmanifest.MeasurementExact,
		})
	}
	addUnknown := func(kind, identity, revision, reason string) {
		blocks = append(blocks, contextmanifest.Block{
			Kind: kind, SourceIdentity: identity, SourceRevision: revision,
			Reason: reason, MeasurementState: contextmanifest.MeasurementUnknown,
		})
	}

	addExact("user_prompt", "provider_input:user_prompt", "Final user-channel prompt rendered by Multica", []byte(prompt), false)
	if opts.SystemPrompt != "" {
		addExact("system_prompt", "provider_input:system_prompt", "Inline Multica runtime brief", []byte(opts.SystemPrompt), true)
	} else {
		addExact("runtime_brief", "provider_input:runtime_brief", "Multica runtime brief written into provider-native context", []byte(runtimeBrief), true)
	}
	addExact("authority_prompt", "provider_input:authority_prompt", "Server-authenticated owner authority", []byte(opts.AuthorityPrompt), false)
	addExact("mcp_config", "provider_input:mcp_config", "Task-scoped MCP tool configuration", opts.McpConfig, true)
	if opts.ResumeSessionID != "" {
		addUnknown("provider_history", "provider_input:resumed_history", contentRevision([]byte(opts.ResumeSessionID)), "Provider-owned resumed history is not observable to Multica")
	}
	addUnknown("provider_tools", "provider_input:tool_catalog", "provider:runtime", "Provider-owned tool schemas may add context outside the rendered prompt")
	addUnknown("repository_instructions", "provider_input:repository_instructions", "workspace:runtime", "Provider-native repository instruction discovery is not yet measured")
	addUnknown("provider_skills", "provider_input:provider_skills", "workspace:runtime", "Provider-native skill loading is not yet measured")

	return contextmanifest.New(
		fmt.Sprintf("%s:%d", task.ID, task.Attempt),
		strconv.Itoa(providerAttempt),
		shadowContextManifestLimitBytes,
		blocks,
	)
}

func (d *Daemon) reportContextManifestShadow(ctx context.Context, task Task, prompt string, opts agent.ExecOptions, runtimeBrief string, providerAttempt int, taskLog *slog.Logger) {
	manifest, err := buildTaskContextManifest(task, prompt, opts, runtimeBrief, providerAttempt)
	if err != nil {
		taskLog.Warn("context manifest shadow build failed; task continues", "provider_attempt", providerAttempt, "error", err)
		return
	}
	reportCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.client.ReportTaskContextManifest(reportCtx, task.ID, manifest); err != nil {
		taskLog.Warn("context manifest shadow report failed; task continues", "provider_attempt", providerAttempt, "error", err)
	}
}

func contentRevision(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}
