import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type {
  Issue,
  AgentTask,
  IssueCurrentPictureCompact,
  OwnerIssueStage,
  OwnerMeaningfulChange,
} from "@multica/core/types";
import enCommon from "../../locales/en/common.json";
import enIssues from "../../locales/en/issues.json";
import {
  IssueNow,
  OperationalBoardCard,
  newestMeaningfulChange,
  operationalLaneForIssue,
} from "./operational-workflow";
import { OperationalBoardView } from "./board-view";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...await importOriginal<typeof import("@tanstack/react-query")>(),
  useQuery: () => ({ data: undefined }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({ issueDetail: (id: string) => `/pilot/issues/${id}` }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, newTabTitle: _newTabTitle, ...props }: React.ComponentProps<"a"> & { newTabTitle?: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const resources = { en: { common: enCommon, issues: enIssues } };

function summaryPicture(
  stage: OwnerIssueStage,
  options: { signals?: string[]; automatic?: boolean; revision?: number; contractVersion?: 2 | 3 } = {},
): IssueCurrentPictureCompact {
  const revision = options.revision ?? 8;
  const signals = options.signals ?? (stage === "technical_stop"
    ? ["technical_stop", "current_incident", "owner_attention_required"]
    : []);
  const base = {
    contract_version: options.contractVersion ?? 2,
    revision,
    scope_revision: 1,
    authority_revision: 1,
    outcome_state: "open",
    outcome_open: stage !== "complete",
    execution_state: "active",
    release_state: "",
    health_state: "healthy",
    attention_state: stage === "needs_owner" ? "unread" : "none",
    current_milestone: stage,
    latest_meaningful_change: {},
    next_automatic_action: {},
    active_actor: {},
    wait_reason: {},
    candidate: {},
    target: {},
    evidence_summary: {},
    unresolved_decisions: [],
    unresolved_incidents: [],
    unresolved_dependencies: [],
    terminal_reason: "",
    next_actor: {},
    generated_at: "2026-09-10T12:00:00Z",
    owner_summary: {
      schema_version: 1 as const,
      current_picture_revision: revision,
      stage,
      signals: signals.map((code) => ({ code, references: [] })),
      next_action: options.automatic
        ? { kind: "continue_workflow" as const, automatic: true }
        : stage === "technical_stop"
          ? { kind: "inspect_incident" as const, automatic: false, actor_type: "owner" }
          : stage === "needs_owner"
            ? { kind: "answer_decision" as const, automatic: false }
            : null,
      latest_meaningful_change: {
        code: "execution_changed" as const,
        result_code: "started" as const,
        occurred_at: "2026-09-10T11:59:00Z",
        reference: { type: "execution", id: `ref-${stage}`, revision: 1 },
      },
    },
  };
  if (options.contractVersion !== 3) return base as IssueCurrentPictureCompact;
  const passedGate = {
    state: "passed" as const,
    refs: [],
    observed_at: "2026-09-10T12:00:00Z",
    limitations: [],
  };
  return {
    ...base,
    contract_version: 3,
    release_status: {
      schema_version: 1,
      phase: stage === "complete" ? "verified_live" : "deployed_checking",
      binding: {
        release_id: "release-1", release_revision: 1, candidate_id: "candidate-1",
        candidate_revision: 1, source_revision: "abc", artifact_digest: "artifact",
        build_manifest_digest: "manifest", scope_revision: 1, acceptance_version: 1,
        authority_revision: 1, target_key: "local", target_generation: 1,
        deployed_revision: "abc",
      },
      gates: {
        review: passedGate,
        build: passedGate,
        deployment: passedGate,
        exposure: passedGate,
        runtime: passedGate,
        business: passedGate,
        regression: passedGate,
        rollback: passedGate,
        restoration: passedGate,
      },
      historical: { was_verified_live: stage === "complete", last_verified_live_at: "2026-09-10T12:00:00Z" },
      next_action: null,
      stop: null,
    },
  } as IssueCurrentPictureCompact;
}

function issue(id: string, stage?: OwnerIssueStage, picture?: IssueCurrentPictureCompact): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: Number(id.replace(/\D/g, "")) || 1,
    identifier: `SILV-${id.replace(/\D/g, "") || "1"}`,
    title: `Outcome ${id}`,
    description: null,
    status: "in_progress",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "owner-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-09-10T10:00:00Z",
    updated_at: "2026-09-10T12:00:00Z",
    current_picture: picture ?? (stage ? summaryPicture(stage) : undefined),
  };
}

function renderWithI18n(node: React.ReactNode) {
  return render(<I18nProvider resources={resources} locale="en">{node}</I18nProvider>);
}

describe("operational workflow presentation", () => {
  it("keeps a completed ordinary worker's question actionable without declaring Done", () => {
    const target = { ...issue("ordinary"), owner_question: { id: "question-1", task_id: "worker", question: "May I use the approved contract?", consequence: "Your answer determines the next action." } };
    const task = { id: "worker", issue_id: target.id, status: "completed", created_at: "2026-09-20T21:00:00Z" } as AgentTask;
    expect(operationalLaneForIssue(target, [task])).toBe("needs_owner");
    renderWithI18n(<><OperationalBoardCard issue={target} tasks={[task]} /><IssueNow issue={target} tasks={[task]} /></>);
    expect(screen.getAllByText(target.owner_question.question)).toHaveLength(2);
    expect(screen.getAllByText("Answer the question in Inbox.")).toHaveLength(2);
    expect(screen.getAllByText("Worker finished; the requested outcome is still open.")).toHaveLength(2);
    expect(screen.queryByText("Status unavailable")).not.toBeInTheDocument();
    expect(screen.queryByText("Done")).not.toBeInTheDocument();
    expect(operationalLaneForIssue({ ...target, owner_question: undefined }, [task])).toBe("needs_next_step");
    expect(operationalLaneForIssue({ ...target, current_picture: summaryPicture("in_review") }, [task])).toBe("in_review");
  });

  it("uses real task execution for no-picture intake without inventing completion or overriding a picture", () => {
    const target = { ...issue("intake"), status: "todo" as const };
    const task = { id: "worker", issue_id: target.id, status: "queued" } as AgentTask;
    expect(operationalLaneForIssue(target, [task])).toBe("waiting_automatic");
    expect(operationalLaneForIssue(target, [{ ...task, status: "running" }])).toBe("active");
    const stopped = { ...target, status: "in_progress" as const };
    expect(operationalLaneForIssue(stopped, [{ ...task, status: "completed" }])).toBe("needs_next_step");
    expect(operationalLaneForIssue(stopped, [])).toBe("needs_next_step");
    const managed = { ...stopped, current_picture: summaryPicture("needs_owner") };
    expect(operationalLaneForIssue(managed, [{ ...task, status: "running" }])).toBe("needs_owner");
    renderWithI18n(<><OperationalBoardCard issue={target} tasks={[task]} /><IssueNow issue={stopped} tasks={[]} /></>);
    expect(screen.getByText("Work is queued; no worker is running yet.")).toBeVisible();
    expect(screen.getByText("Inspect the latest evidence and choose the next step.")).toBeVisible();
  });
  it.each(["todo", "backlog"] as const)("keeps observed terminal evidence on a %s issue instead of claiming nothing started", (status) => {
    const target = { ...issue("intake-terminal"), status };
    const task = { id: "worker", issue_id: target.id, status: "completed" } as AgentTask;
    for (const terminal of ["completed", "failed", "cancelled"] as const) {
      expect(operationalLaneForIssue(target, [{ ...task, status: terminal }])).toBe("needs_next_step");
    }
    renderWithI18n(<><OperationalBoardCard issue={target} tasks={[task]} /><IssueNow issue={target} tasks={[task]} /></>);
    expect(screen.getAllByText("Inspect the latest evidence and choose the next step.")).toHaveLength(2);
    expect(screen.queryByText("Not started yet")).not.toBeInTheDocument();
    expect(screen.queryByText("Nothing has happened yet")).not.toBeInTheDocument();
  });
  it("shows the recorded dependency and next step on both parked issue surfaces", () => {
    const parked = issue("2");
    parked.status = "blocked";
    parked.metadata = {
      "recovery.wait_kind": "external_dependency",
      "recovery.wait_reason": "The required external service is unavailable.",
      "recovery.next_action": "Resume the same issue when the external service recovers.",
    };
    renderWithI18n(<><OperationalBoardCard issue={parked} /><IssueNow issue={parked} /></>);
    expect(screen.getAllByText("The required external service is unavailable.")).toHaveLength(2);
    expect(screen.getAllByText("Resume the same issue when the external service recovers.")).toHaveLength(2);
    expect(screen.queryByText("No verified progress yet")).not.toBeInTheDocument();
    expect(screen.queryByText("No next step is recorded")).not.toBeInTheDocument();
    expect(screen.getByText("Nothing needed from you")).toBeVisible();
  });

  it("shows an outside agent report on the board and in issue history without declaring the task done", () => {
    const target = issue("external", "status_unavailable", summaryPicture("status_unavailable", { signals: ["projection_unavailable"] }));
    target.current_picture!.latest_meaningful_change = {
      external_work: {
        provider: "codex",
        reported_state: "complete",
        change_since_last_receipt: "The importer is merged and all protected checks passed.",
        next_action: "Verify it in the next Multica release.",
      },
    };
    const update = {
      id: "receipt-1",
      provider: "codex" as const,
      reported_state: "complete" as const,
      observed_at: "2026-10-14T21:00:00Z",
      recorded_at: "2026-10-14T21:00:01Z",
      receipt_revision: 1,
      current_milestone: "Importer merged",
      summary: "The importer is merged and all protected checks passed.",
      evidence: [{ type: "pr" as const, reference: "https://github.com/example/repo/pull/36" }],
      evidence_total: 1,
      blockers: ["Production has not been deployed."],
      blocker_total: 1,
      next_action: "Verify it in the next Multica release.",
      acceptance_stage: "merged" as const,
      acceptance_status: "accepted" as const,
      resource_usage: {
        source: "measured" as const,
        attempts: 2,
        wall_clock_seconds: 180,
        active_compute_seconds: 120,
        input_tokens: 1000,
        output_tokens: 200,
        cost_usd: 0.42,
        capacity_window_ref: null,
      },
      provider_report_only: true as const,
    };
    renderWithI18n(<>
      <OperationalBoardCard issue={target} />
      <IssueNow issue={target} externalUpdates={[update]} />
    </>);
    expect(screen.getAllByText(/Codex reported complete: The importer is merged/)).toHaveLength(2);
    expect(screen.getAllByText("Verify it in the next Multica release.")).toHaveLength(3);
    expect(screen.getByLabelText("Outside work history")).toBeVisible();
    fireEvent.click(screen.getByLabelText("Outside work history"));
    expect(screen.getByText("Reported by Codex")).toBeVisible();
    expect(screen.getByText("Blocked by: Production has not been deployed.")).toBeVisible();
    expect(screen.getAllByText("Next step needed").length).toBeGreaterThan(0);
    expect(screen.queryByText("Done")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Evidence and details"));
    expect(screen.getByText("https://github.com/example/repo/pull/36")).toBeInTheDocument();
    expect(screen.getByText("Measured usage: 2 attempts · 2m active · 1,200 tokens · $0.42")).toBeInTheDocument();
  });

  it("reports unavailable outside history without hiding the rest of the issue", () => {
    const target = issue("external-error", "active");
    renderWithI18n(<IssueNow issue={target} externalUpdatesFailed />);
    expect(screen.getByText("Outside work history is temporarily unavailable.")).toBeVisible();
    expect(screen.getByRole("region", { name: "Now" })).toBeVisible();
  });

  it("shows the exact Current Picture owner question without using conflicting metadata", () => {
    const waiting = issue("1", "needs_owner");
    waiting.status = "blocked";
    waiting.current_picture!.wait_reason = {
      question: "Approve this exact candidate?",
      consequence: "Resume this issue after approval.",
    };
    waiting.metadata = {
      "recovery.wait_kind": "owner",
      "recovery.wait_reason": "Stale owner question",
      "recovery.next_action": "Stale next action",
    };
    renderWithI18n(<><OperationalBoardCard issue={waiting} /><IssueNow issue={waiting} /></>);
    const now = screen.getByRole("region", { name: "Now" });
    expect(within(now).getByText("Approve this exact candidate?")).toBeVisible();
    expect(within(now).getByText("Execution changed — started")).toBeVisible();
    expect(screen.getAllByText("Resume this issue after approval.")).toHaveLength(2);
    expect(screen.queryByText("Stale owner question")).not.toBeInTheDocument();
    expect(screen.queryByText("Stale next action")).not.toBeInTheDocument();
  });

  it.each(["resolved", "authoritative", "misaligned"])("does not project recovery metadata over %s state", (state) => {
    const managed = issue("3", state === "resolved" ? undefined : "active");
    managed.status = state === "resolved" ? "done" : "blocked";
    managed.metadata = {
      "recovery.wait_kind": "external_dependency",
      "recovery.wait_reason": "Old dependency reason",
      "recovery.next_action": "Old next step",
    };
    if (state === "misaligned") managed.current_picture!.owner_summary.current_picture_revision = 1;
    renderWithI18n(<><OperationalBoardCard issue={managed} /><IssueNow issue={managed} /></>);
    expect(screen.queryByText("Old dependency reason")).not.toBeInTheDocument();
    expect(screen.queryByText("Old next step")).not.toBeInTheDocument();
  });

  it.each([
    ["needs_owner", [], false, "needs_owner"],
    ["active", [], false, "active"],
    ["active", ["waiting_automatic"], false, "waiting_automatic"],
    ["active", [], true, "waiting_automatic"],
    ["in_review", [], false, "in_review"],
    ["releasing", [], false, "releasing"],
    ["verifying", [], false, "deployed_checking"],
    ["recovering", [], false, "recovering"],
    ["blocked", [], false, "technical_stop"],
    ["technical_stop", [], false, "technical_stop"],
    ["complete", ["verified_live"], false, "verified_live"],
    ["complete", ["outcome_done"], false, "done"],
    ["status_unavailable", [], false, "needs_next_step"],
  ] as const)("maps %s truth to %s", (stage, signals, automatic, expected) => {
    expect(operationalLaneForIssue(issue("1", undefined, summaryPicture(stage, { signals: [...signals], automatic })))).toBe(expected);
  });

  it("shows a failed workflow as a technical incident with an explicit owner next step", () => {
    const stopped = issue("14", "technical_stop");
    renderWithI18n(<>
      <OperationalBoardCard issue={stopped} />
      <IssueNow issue={stopped} />
    </>);
    expect(screen.getAllByText("Stopped by a technical problem")).toHaveLength(2);
    expect(screen.getAllByText("Inspect the technical incident")).toHaveLength(2);
    expect(screen.getByText("Your response is required")).toBeVisible();
  });

  it("fails revision-misaligned rows closed, and exposes routed rows with no picture", () => {
    // Raw in-progress status and stale summaries cannot establish active work.
    expect(operationalLaneForIssue(issue("never-run"))).toBe("needs_next_step");
    const mismatched = summaryPicture("active", { revision: 8 });
    mismatched.owner_summary.stage = "complete";
    mismatched.owner_summary.signals = [{ code: "verified_live", references: [] }];
    mismatched.owner_summary.current_picture_revision = 7;
    const staleIssue = issue("2", undefined, mismatched);
    expect(operationalLaneForIssue(staleIssue)).toBe("needs_next_step");
    renderWithI18n(<>
      <OperationalBoardCard issue={staleIssue} />
      <IssueNow issue={staleIssue} activityRevision={7} />
    </>);
    expect(screen.getAllByText("Next step needed")).toHaveLength(2);
    expect(screen.getAllByText("No verified progress yet")).toHaveLength(2);
    expect(screen.queryByText("Verified Live")).not.toBeInTheDocument();
    expect(screen.queryByText("Done")).not.toBeInTheDocument();
    expect(screen.queryByText("Work is active")).not.toBeInTheDocument();
    expect(screen.queryByText("No value")).not.toBeInTheDocument();
  });

  it.each([
    ["current_regression", "Regression detected"],
    ["rollback_in_progress", "Rollback in progress"],
    ["service_restored_outcome_open", "Service restored; requested outcome remains open"],
  ] as const)("shows finite recovery copy for %s", (signal, copy) => {
    const picture = summaryPicture("recovering", { signals: [signal] });
    renderWithI18n(<OperationalBoardCard issue={issue(signal, undefined, picture)} />);
    expect(screen.getByText(copy)).toBeVisible();
  });

  it("renders minimum, typical, and maximum cards without avatars and keeps links keyboard reachable", () => {
    const typical = issue("22", "active");
    const max = issue("999", "active");
    max.title = "A very long business outcome ".repeat(20);
    renderWithI18n(<>
      <OperationalBoardCard issue={issue("1")} />
      <OperationalBoardCard issue={typical} />
      <OperationalBoardCard issue={max} />
    </>);
    const links = screen.getAllByRole("link");
    expect(links).toHaveLength(3);
    expect(links[0]).toHaveAttribute("href", "/pilot/issues/1");
    expect(document.querySelector("img")).toBeNull();
    expect(document.querySelector("[data-avatar]")).toBeNull();
  });

  it("groups every owner state into the six owner columns with no drag authority", () => {
    const fixtures = [
      issue("1", "active"),
      issue("2", undefined, summaryPicture("active", { automatic: true })),
      issue("3", "needs_owner"),
      issue("4", "in_review"),
      issue("5", "releasing"),
      issue("6", "verifying"),
      issue("7", undefined, summaryPicture("complete", { signals: ["verified_live"] })),
      issue("8", "recovering"),
      issue("9", "technical_stop"),
      issue("10", "complete"),
      issue("11"),
    ];
    renderWithI18n(<OperationalBoardView issues={fixtures} />);
    const board = screen.getByRole("region", { name: "Operational workflow board" });
    expect(within(board).getAllByRole("heading", { level: 2 })).toHaveLength(6);
    for (const name of ["Needs You", "Not started", "Active work", "In review", "Done", "Blocked"]) {
      expect(within(board).getByRole("heading", { name })).toBeVisible();
    }
    expect(within(board).queryByRole("heading", { name: "Status unavailable" })).not.toBeInTheDocument();
    // Releasing, Deployed-checking, Verified Live and Regression/rollback are
    // all "an agent has it" to the owner, and work with no nameable next step
    // is stuck rather than quietly progressing. None of them is its own column.
    for (const folded of ["Releasing", "Deployed — checking", "Verified Live", "Regression / rollback", "Next step needed", "Waiting automatically"]) {
      expect(within(board).queryByRole("heading", { name: folded })).not.toBeInTheDocument();
    }
    expect(board.querySelector("[data-board-card]")).toBeNull();
    expect(board.querySelector("[aria-roledescription=sortable]")).toBeNull();
    expect(within(board).queryByText("Ready for review")).not.toBeInTheDocument();
    expect(within(board).queryByText("No value")).not.toBeInTheDocument();
  });

  it("tells the owner a freshly filed issue has not started instead of claiming no status", () => {
    const filed = issue("41");
    filed.status = "backlog";
    filed.status_category = "backlog";
    renderWithI18n(<OperationalBoardCard issue={filed} />);
    expect(screen.getByText("Not started yet")).toBeVisible();
    expect(screen.getByText("Nothing has happened yet")).toBeVisible();
    expect(screen.getByText("Assign it, or ask Mika to route it")).toBeVisible();
    expect(screen.queryByText("Status unavailable")).not.toBeInTheDocument();
    expect(screen.queryByText("No verified progress yet")).not.toBeInTheDocument();
    expect(screen.queryByText("No next step is recorded")).not.toBeInTheDocument();
  });

  it("does not tell the owner to assign an issue that already has an owner", () => {
    const routed = issue("41");
    routed.status = "backlog";
    routed.status_category = "backlog";
    routed.assignee_type = "agent";
    routed.assignee_id = "agent-phone-farm";
    renderWithI18n(<OperationalBoardCard issue={routed} />);
    expect(screen.getByText("Not started yet")).toBeVisible();
    expect(screen.getByText("Start it when you are ready")).toBeVisible();
    expect(screen.queryByText("Assign it, or ask Mika to route it")).not.toBeInTheDocument();
  });

  it("uses only same-revision activity and shows release and owner truth in Now", () => {
    const managed = issue("12", undefined, summaryPicture("needs_owner", { contractVersion: 3 }));
    const newest: OwnerMeaningfulChange = {
      code: "release_changed",
      result_code: "succeeded",
      occurred_at: "2026-09-10T12:01:00Z",
      reference: { type: "release", id: "release-1", revision: 2 },
    };
    renderWithI18n(<IssueNow issue={managed} activityRevision={8} newestActivity={newest} />);
    const now = screen.getByRole("region", { name: "Now" });
    expect(within(now).getByText("Release changed — succeeded")).toBeVisible();
    expect(within(now).getByText("Your response is required")).toBeVisible();
    expect(within(now).getByText("Deployed, verification pending")).toBeVisible();
    expect(now.querySelector("dl")?.className).toContain("sm:grid-cols-2");
    expect(now.querySelector("dl")?.className).toContain("xl:grid-cols-3");
  });

  it("ignores stale realtime activity and retains the summary's meaningful update", () => {
    const summary = summaryPicture("active").owner_summary;
    const stale: OwnerMeaningfulChange = {
      ...summary.latest_meaningful_change!,
      code: "release_changed",
    };
    expect(newestMeaningfulChange(summary, 7, stale)).toEqual(summary.latest_meaningful_change);
  });

  it("renders Board and Issue Now from the same aligned summary revision", () => {
    const shared = issue("14", "in_review");
    renderWithI18n(<>
      <OperationalBoardCard issue={shared} />
      <IssueNow issue={shared} activityRevision={8} />
    </>);
    expect(screen.getAllByText("Execution changed — started")).toHaveLength(2);
    expect(screen.getAllByText("In review")).toHaveLength(2);
  });
});
