/** @vitest-environment jsdom */
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RecoveryHealthStrip } from "./recovery-health-strip";

let result: Record<string, unknown> = {};
vi.mock("@tanstack/react-query", () => ({ useQuery: () => result }));
vi.mock("../../i18n", () => ({
  useT: () => ({
    t: (
      _selector: unknown,
      options?: { action?: string },
    ) => options?.action
      ? `Next: ${options.action}`
      : "Recovery status is temporarily unavailable. The external watchdog continues independently.",
  }),
}));

afterEach(() => {
  cleanup();
  result = {};
});

describe("RecoveryHealthStrip", () => {
  // Healthy is the normal state, and a banner announcing it every day would
  // train the reader to skip the strip on the day it matters.
  it("stays off screen while the data is safe", () => {
    result = {
      data: {
        status: "protected",
        backup_age_hours: 2.8,
        replication: "verified",
        next_action: "Automatic backups and external health checks continue.",
        evidence_current: true,
      },
      isError: false,
    };
    const { container } = render(<RecoveryHealthStrip workspaceId="workspace-1" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("does not turn an unproven backup into protected", () => {
    result = {
      data: {
        status: "recovery_unproven",
        replication: "not_configured",
        next_action: "Run an isolated recovery rehearsal without touching the active pilot.",
        evidence_current: false,
      },
      isError: false,
    };
    render(<RecoveryHealthStrip workspaceId="workspace-1" />);
    expect(screen.getByText("The backup has not been tested")).toBeTruthy();
    // Said in terms of the data at risk, never the mechanism protecting it.
    expect(screen.queryByText(/recovery point|replication|rehearsal/i)).toBeNull();
  });

  // A banner that says "we cannot tell you whether your data is safe" is not
  // information the reader can act on, so the strip stays silent when the
  // status read fails rather than manufacturing an alarm out of its own
  // network error.
  it("says nothing at all when the status cannot be read", () => {
    result = { data: undefined, isError: true };
    const { container } = render(<RecoveryHealthStrip workspaceId="workspace-1" />);
    expect(container).toBeEmptyDOMElement();
  });
});
