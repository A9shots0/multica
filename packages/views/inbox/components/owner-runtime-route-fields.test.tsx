/** @vitest-environment jsdom */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { OwnerRuntimeRouteFields } from "./owner-runtime-route-fields";

vi.mock("@multica/core/workspace/queries", () => ({ agentListOptions: () => ({ kind: "agents" }), squadListOptions: () => ({ kind: "squads" }) }));
vi.mock("@multica/core/runtimes/queries", () => ({ runtimeListOptions: () => ({ kind: "runtimes" }) }));
vi.mock("@tanstack/react-query", () => ({ useQuery: ({ kind }: { kind: string }) => ({ data: kind === "agents" ? [
  { id: "impl", name: "Specialist", runtime_id: "runtime", archived_at: null },
  { id: "qc", name: "QC", runtime_id: "runtime", archived_at: null },
  { id: "fleet", name: "Fleet", runtime_id: "runtime", archived_at: null },
] : kind === "squads" ? [{ id: "squad", name: "Existing squad", archived_at: null }] : [{ id: "runtime", name: "Configured Mac runtime", daemon_id: "host", status: "online" }] }) }));
vi.mock("../../i18n", async () => {
  const resource = (await import("../../locales/en/inbox.json")).default;
  return { useT: () => ({ t: (selector: (value: typeof resource) => string) => selector(resource) }) };
});

describe("explicit owner runtime route", () => {
  it("never admits incomplete selection as an ordinary continuation", async () => {
    const changed = vi.fn();
    render(<OwnerRuntimeRouteFields workspaceId="workspace" onChange={changed} />);
    expect(changed).toHaveBeenLastCalledWith(undefined);
    fireEvent.click(screen.getByRole("checkbox", { name: "Declare review and release roles for this issue" }));
    await waitFor(() => expect(changed).toHaveBeenLastCalledWith(null));
    expect(screen.getByText(/Choose all roles on the same host/)).toBeTruthy();
    expect(screen.getByRole("checkbox", { name: /Allow a brief deployment-only/ }).getAttribute("aria-checked")).toBe("false");
    fireEvent.click(screen.getByRole("checkbox", { name: "Declare review and release roles for this issue" }));
    await waitFor(() => expect(changed).toHaveBeenLastCalledWith(undefined));
  });
});
