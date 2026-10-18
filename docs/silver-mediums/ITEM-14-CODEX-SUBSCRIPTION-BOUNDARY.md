# Item 14 Codex subscription model-only boundary

Status: provider-free fail-closed compatibility result; Codex CLI subscription
lane held for model-only qualification

Snapshot: 2026-09-14 14:17 America/New_York

## Question

Can the installed Codex CLI use the existing Codex subscription while
preventively exposing no tools, reading no repository, and sending one exact
model request suitable for Item 14 configuration qualification?

## Provider-free capture

The installed runtime is `codex-cli 0.153.2`. A fresh external work directory
and fresh task-local profile were used with a loopback fake Responses provider,
a dummy environment credential, and zero request and stream retries. No real
credential or provider was accessed.

The invocation ignored user config and project rules, disabled project-document
loading, emptied project-root markers, disabled web search, and disabled every
available tool-bearing feature relevant to the boundary, including shell,
unified exec, code mode and its host, request input/permissions, image,
Browser, Computer Use, apps, plugins, skills, multi-agent, and workspace
dependencies. Desktop tool-pipe and session identity variables were also
removed from the child environment.

The single captured `POST /v1/responses` request correctly pinned
`gpt-5.6-luna`, streaming, no storage, and zero transport retries. It did not
contain a top-level `tools` field and did not contain a repository document.
It nevertheless contained one `additional_tools` developer item exposing:

- `functions.exec`;
- `functions.wait`;
- `functions.request_user_input`.

The request also retained `tool_choice: "auto"` and ambient runtime
instructions. Disabling code-mode host made the tool fail closed at execution
time, but did not remove its callable definition from the model request. That
does not meet a preventive model-only boundary.

OpenAI's current CLI reference confirms the isolation flags used here, and the
configuration reference documents custom Responses providers plus explicit
zero request/stream retry settings. The Responses API itself supports
`tool_choice: "none"`, but the Codex CLI configuration surface does not expose
an equivalent model-only request control for this run:

- <https://learn.chatgpt.com/docs/developer-commands?surface=cli>
- <https://learn.chatgpt.com/docs/config-file/config-reference>
- <https://developers.openai.com/api/reference/cli/resources/responses/methods/create>

## Decision

Codex CLI 0.153.2 is unsuitable for this exact model-only Item 14 boundary.
The subscription lane remains held. No real Codex call is justified, because
the provider-free precursor already disproved the required preventive control.

This does not judge Codex model competence, reject Codex as the likely primary
development lane, or select another provider. It means qualification requiring
zero exposed tools must wait for a separately versioned client or supported
mode whose outbound request proves no tool definitions or callable tool
namespace. Normal tool-enabled development remains a different future
configuration class and needs its own exact capability contract.

Machine evidence is frozen in
`server/internal/agentqualification/codex_subscription_boundary_v1.json`,
SHA-256
`de00340714977c7060b8ae46a6ca779b16ba5cb25aa547c2a42f4313997915c3`.
The current transport catalog is v2, SHA-256
`8a98dcbfdfaa5fe16ec814473a656a46067740d82940270e0eaea1ab8cbca348`;
the historical v1 catalog remains byte-preserved for frozen antecedents.

Focused mutation tests passed 20 repetitions. They reject a false tool-free
claim, hidden tool, retry, duplicate request, model drift, credential access,
provider access, or promotion. The three disposable exploratory roots were
deleted after their exact paths were verified, and all three are absent.
